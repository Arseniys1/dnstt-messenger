package com.example.myapplication

import kotlinx.coroutines.*
import kotlinx.coroutines.flow.MutableSharedFlow
import kotlinx.coroutines.flow.SharedFlow
import org.bouncycastle.crypto.agreement.X25519Agreement
import org.bouncycastle.crypto.generators.X25519KeyPairGenerator
import org.bouncycastle.crypto.modes.ChaCha20Poly1305
import org.bouncycastle.crypto.params.AEADParameters
import org.bouncycastle.crypto.params.KeyParameter
import org.bouncycastle.crypto.params.X25519KeyGenerationParameters
import org.bouncycastle.crypto.params.X25519PrivateKeyParameters
import org.bouncycastle.crypto.params.X25519PublicKeyParameters
import java.io.BufferedInputStream
import java.io.DataInputStream
import java.io.InputStream
import java.io.OutputStream
import java.net.InetSocketAddress
import java.net.Socket
import java.security.SecureRandom

// ---- Protocol commands ----
object Cmd {
    const val REGISTER    = 0x01.toByte()
    const val LOGIN       = 0x02.toByte()
    const val MSG         = 0x03.toByte()
    const val INCOMING    = 0x04.toByte()
    const val HISTORY     = 0x05.toByte()
    const val ACK         = 0x06.toByte()
    const val HISTORY_END = 0x07.toByte()
    const val LOGIN_OK    = 0x08.toByte()
    const val LOGIN_FAIL  = 0x09.toByte()
    const val ONLINE_LIST = 0x0A.toByte()
}

data class AppConfig(
    val serverAddr: String = "94.103.169.82:9999",
    val proxyAddr: String  = "127.0.0.1:18000",
    val directMode: Boolean = false
)

data class ChatMessage(
    val sender: String,
    val text: String,
    val time: String,
    val own: Boolean = false
)

sealed class ServerEvent {
    data class Message(val msg: ChatMessage) : ServerEvent()
    data class History(val msg: ChatMessage) : ServerEvent()
    object HistoryEnd : ServerEvent()
    data class OnlineList(val users: List<String>) : ServerEvent()
    object LoginOk : ServerEvent()
    object LoginFail : ServerEvent()
    object Disconnected : ServerEvent()
}

class MessengerClient {
    private var socket: Socket? = null
    private var din: DataInputStream? = null
    private var output: OutputStream? = null
    private var sharedKey: ByteArray? = null
    var sessionID: Int = 0
        private set

    // Single event stream — both login result and chat events come through here
    private val _events = MutableSharedFlow<ServerEvent>(extraBufferCapacity = 64)
    val events: SharedFlow<ServerEvent> = _events

    private var readerJob: Job? = null
    private val scope = CoroutineScope(Dispatchers.IO + SupervisorJob())

    // ---- Connect ----
    suspend fun connect(cfg: AppConfig): Result<Unit> = withContext(Dispatchers.IO) {
        try {
            val (host, port) = parseAddr(cfg.serverAddr)
            val sock = Socket()
            sock.soTimeout = 15_000 // read timeout during handshake
            if (cfg.directMode) {
                sock.connect(InetSocketAddress(host, port), 10_000)
            } else {
                val (proxyHost, proxyPort) = parseAddr(cfg.proxyAddr)
                sock.connect(InetSocketAddress(proxyHost, proxyPort), 10_000)
                socks5Handshake(sock, host, port)
            }
            socket = sock
            val rawIn = BufferedInputStream(sock.getInputStream())
            din    = DataInputStream(rawIn)
            output = sock.getOutputStream()
            sharedKey = ecdhHandshake()
            sock.soTimeout = 0 // remove timeout after handshake — reads are blocking in reader loop
            Result.success(Unit)
        } catch (e: Exception) {
            Result.failure(e)
        }
    }

    // ---- Start background reader loop ----
    fun startReader() {
        readerJob?.cancel()
        readerJob = scope.launch {
            try {
                while (isActive) {
                    val event = readOneEvent() ?: continue
                    _events.emit(event)
                    if (event is ServerEvent.Disconnected) break
                }
            } catch (_: Exception) {
                _events.emit(ServerEvent.Disconnected)
            }
        }
    }

    // ---- Login (sends packet, waits for LoginOk/LoginFail via events) ----
    suspend fun login(login: String, pass: String): Result<Int> {
        val lBytes = login.toByteArray()
        val pBytes = pass.toByteArray()
        val pkt = byteArrayOf(Cmd.LOGIN, lBytes.size.toByte()) + lBytes + pBytes
        return withContext(Dispatchers.IO) {
            try {
                output!!.write(pkt)
                output!!.flush()
                // Wait for LOGIN_OK or LOGIN_FAIL from the reader loop
                val deferred = CompletableDeferred<Result<Int>>()
                val job = scope.launch {
                    events.collect { event ->
                        when (event) {
                            is ServerEvent.LoginOk -> {
                                deferred.complete(Result.success(sessionID))
                                cancel()
                            }
                            is ServerEvent.LoginFail -> {
                                deferred.complete(Result.failure(Exception("Invalid credentials")))
                                cancel()
                            }
                            is ServerEvent.Disconnected -> {
                                deferred.complete(Result.failure(Exception("Disconnected")))
                                cancel()
                            }
                            else -> {}
                        }
                    }
                }
                // Timeout
                val result = withTimeoutOrNull(8000) { deferred.await() }
                    ?: Result.failure(Exception("Login timeout"))
                job.cancel()
                result
            } catch (e: Exception) {
                Result.failure(e)
            }
        }
    }

    // ---- Register ----
    suspend fun register(login: String, pass: String): Result<Boolean> = withContext(Dispatchers.IO) {
        try {
            val lBytes = login.toByteArray()
            val pBytes = pass.toByteArray()
            val pkt = byteArrayOf(Cmd.REGISTER, lBytes.size.toByte()) + lBytes + pBytes
            output!!.write(pkt)
            output!!.flush()
            socket!!.soTimeout = 5000
            val res = ByteArray(1)
            din!!.readFully(res)
            socket!!.soTimeout = 0
            Result.success(res[0] == 0x01.toByte())
        } catch (e: Exception) {
            Result.failure(e)
        }
    }

    // ---- Send message ----
    suspend fun sendMessage(text: String): Result<Unit> = withContext(Dispatchers.IO) {
        try {
            val key = sharedKey ?: return@withContext Result.failure(Exception("No key"))
            val nonce = ByteArray(12).also { SecureRandom().nextBytes(it) }
            val ciphertext = chachaEncrypt(key, nonce, text.toByteArray(Charsets.UTF_8))
            val sid = sessionID
            val pkt = byteArrayOf(Cmd.MSG, ((sid shr 8) and 0xFF).toByte(), (sid and 0xFF).toByte()) +
                      nonce + ciphertext
            output!!.write(pkt)
            output!!.flush()
            Result.success(Unit)
        } catch (e: Exception) {
            Result.failure(e)
        }
    }

    // ---- Read exactly one event from stream (blocking) ----
    private fun readOneEvent(): ServerEvent? {
        val inp = din ?: return ServerEvent.Disconnected
        val cmdByte = inp.read()
        if (cmdByte == -1) return ServerEvent.Disconnected
        return when (cmdByte.toByte()) {
            Cmd.LOGIN_OK -> {
                val hi = inp.read(); val lo = inp.read()
                if (hi == -1 || lo == -1) return ServerEvent.Disconnected
                sessionID = (hi shl 8) or lo
                ServerEvent.LoginOk
            }
            Cmd.LOGIN_FAIL -> ServerEvent.LoginFail
            Cmd.ACK        -> null  // skip, read next
            Cmd.HISTORY_END -> ServerEvent.HistoryEnd
            Cmd.HISTORY    -> parseHistory(inp)
            Cmd.INCOMING   -> parseIncoming(inp)
            Cmd.ONLINE_LIST -> parseOnlineList(inp)
            else -> null  // unknown byte, skip
        }
    }

    // ---- Parsers ----

    private fun parseHistory(inp: DataInputStream): ServerEvent? {
        val senderLen = inp.read().takeIf { it >= 0 } ?: return ServerEvent.Disconnected
        val senderBytes = ByteArray(senderLen).also { inp.readFully(it) }
        val timeLen = inp.read().takeIf { it >= 0 } ?: return ServerEvent.Disconnected
        val timeBytes = ByteArray(timeLen).also { inp.readFully(it) }
        val msgLen = (inp.read() shl 8) or inp.read()
        val msgBytes = ByteArray(msgLen).also { inp.readFully(it) }
        return ServerEvent.History(
            ChatMessage(
                sender = String(senderBytes),
                text   = String(msgBytes),
                time   = String(timeBytes).take(16)
            )
        )
    }

    private fun parseIncoming(inp: DataInputStream): ServerEvent? {
        val senderLen = inp.read().takeIf { it >= 0 } ?: return ServerEvent.Disconnected
        val senderBytes = ByteArray(senderLen).also { inp.readFully(it) }
        val nonce = ByteArray(12).also { inp.readFully(it) }
        val ctLen = (inp.read() shl 8) or inp.read()
        val ct = ByteArray(ctLen).also { inp.readFully(it) }
        val key = sharedKey ?: return null
        return try {
            val plain = chachaDecrypt(key, nonce, ct)
            val now = java.text.SimpleDateFormat("HH:mm", java.util.Locale.getDefault())
                .format(java.util.Date())
            ServerEvent.Message(ChatMessage(sender = String(senderBytes), text = String(plain), time = now))
        } catch (_: Exception) { null }
    }

    private fun parseOnlineList(inp: DataInputStream): ServerEvent? {
        val count = inp.read().takeIf { it >= 0 } ?: return ServerEvent.Disconnected
        val names = mutableListOf<String>()
        repeat(count) {
            val nLen = inp.read()
            val nb = ByteArray(nLen).also { inp.readFully(it) }
            names += String(nb)
        }
        return ServerEvent.OnlineList(names)
    }

    // ---- SOCKS5 ----
    private fun socks5Handshake(sock: Socket, dstHost: String, dstPort: Int) {
        val out = sock.getOutputStream()
        val inp = sock.getInputStream()
        out.write(byteArrayOf(0x05, 0x01, 0x00)); out.flush()
        val gr = ByteArray(2); readFull(inp, gr)
        if (gr[0] != 0x05.toByte() || gr[1] != 0x00.toByte())
            throw Exception("SOCKS5 auth failed")
        val hostBytes = dstHost.toByteArray(Charsets.US_ASCII)
        val req = mutableListOf<Byte>()
        req += listOf(0x05, 0x01, 0x00, 0x03).map { it.toByte() }
        req += hostBytes.size.toByte()
        req += hostBytes.toList()
        req += ((dstPort shr 8) and 0xFF).toByte()
        req += (dstPort and 0xFF).toByte()
        out.write(req.toByteArray()); out.flush()
        val resp = ByteArray(4); readFull(inp, resp)
        if (resp[1] != 0x00.toByte()) throw Exception("SOCKS5 connect failed: ${resp[1]}")
        when (resp[3]) {
            0x01.toByte() -> readFull(inp, ByteArray(4))
            0x03.toByte() -> readFull(inp, ByteArray(inp.read()))
            0x04.toByte() -> readFull(inp, ByteArray(16))
        }
        readFull(inp, ByteArray(2))
    }

    // ---- ECDH ----
    private fun ecdhHandshake(): ByteArray {
        val inp = din!!; val out = output!!
        val gen = X25519KeyPairGenerator()
        gen.init(X25519KeyGenerationParameters(SecureRandom()))
        val kp = gen.generateKeyPair()
        val privKey = kp.private as X25519PrivateKeyParameters
        val pubKey  = kp.public  as X25519PublicKeyParameters
        val serverPubBytes = ByteArray(32); inp.readFully(serverPubBytes)
        out.write(pubKey.encoded); out.flush()
        val agreement = X25519Agreement()
        agreement.init(privKey)
        val shared = ByteArray(agreement.agreementSize)
        agreement.calculateAgreement(X25519PublicKeyParameters(serverPubBytes), shared, 0)
        return shared
    }

    // ---- Crypto ----
    private fun chachaEncrypt(key: ByteArray, nonce: ByteArray, plaintext: ByteArray): ByteArray {
        val cipher = ChaCha20Poly1305()
        cipher.init(true, AEADParameters(KeyParameter(key), 128, nonce))
        val out = ByteArray(cipher.getOutputSize(plaintext.size))
        val len = cipher.processBytes(plaintext, 0, plaintext.size, out, 0)
        cipher.doFinal(out, len)
        return out
    }

    private fun chachaDecrypt(key: ByteArray, nonce: ByteArray, ciphertext: ByteArray): ByteArray {
        val cipher = ChaCha20Poly1305()
        cipher.init(false, AEADParameters(KeyParameter(key), 128, nonce))
        val out = ByteArray(cipher.getOutputSize(ciphertext.size))
        val len = cipher.processBytes(ciphertext, 0, ciphertext.size, out, 0)
        cipher.doFinal(out, len)
        return out
    }

    private fun readFull(inp: InputStream, buf: ByteArray) {
        var total = 0
        while (total < buf.size) {
            val n = inp.read(buf, total, buf.size - total)
            if (n == -1) throw Exception("Connection closed")
            total += n
        }
    }

    private fun parseAddr(addr: String): Pair<String, Int> {
        val idx = addr.lastIndexOf(':')
        return addr.substring(0, idx) to addr.substring(idx + 1).toInt()
    }

    fun destroy() {
        readerJob?.cancel()
        scope.cancel()
        try { socket?.close() } catch (_: Exception) {}
        socket = null; din = null; output = null
    }
}
