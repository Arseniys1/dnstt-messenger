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
import java.util.concurrent.ConcurrentHashMap
import java.util.concurrent.atomic.AtomicInteger
import java.util.concurrent.atomic.AtomicLong

// ---- Protocol commands ----
object Cmd {
    const val REGISTER      = 0x01.toByte()
    const val LOGIN         = 0x02.toByte()
    const val MSG           = 0x03.toByte()
    const val INCOMING      = 0x04.toByte()
    const val HISTORY       = 0x05.toByte()
    const val ACK           = 0x06.toByte()
    const val HISTORY_END   = 0x07.toByte()
    const val LOGIN_OK      = 0x08.toByte()
    const val LOGIN_FAIL    = 0x09.toByte()
    const val ONLINE_LIST   = 0x0A.toByte()
    const val ONLINE_ADD    = 0x0B.toByte()
    const val ONLINE_REMOVE = 0x0C.toByte()
    const val FRAGMENT      = 0x0D.toByte()
}

private const val MAX_FRAME_SIZE = 180 // conservative limit for DNS tunnel MTU

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
    data class OnlineAdd(val sid: Int, val name: String) : ServerEvent()
    data class OnlineRemove(val sid: Int, val name: String) : ServerEvent()
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

    private val _events = MutableSharedFlow<ServerEvent>(extraBufferCapacity = 64)
    val events: SharedFlow<ServerEvent> = _events

    private var readerJob: Job? = null
    private val scope = CoroutineScope(Dispatchers.IO + SupervisorJob())

    // Per-connection nonce counter (client→server direction)
    private val sendCounter = AtomicLong(0)
    // Fragment message ID counter
    private val fragCounter = AtomicInteger(0)
    // SID → username map, kept in sync with CmdOnlineList/Add/Remove
    val sidNames = ConcurrentHashMap<Int, String>()

    // ---- Connect ----
    suspend fun connect(cfg: AppConfig): Result<Unit> = withContext(Dispatchers.IO) {
        try {
            val (host, port) = parseAddr(cfg.serverAddr)
            val sock = Socket()
            sock.soTimeout = 15_000
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
            sock.soTimeout = 0
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

    // ---- Login ----
    suspend fun login(login: String, pass: String): Result<Int> {
        val lBytes = login.toByteArray()
        val pBytes = pass.toByteArray()
        val payload = byteArrayOf(lBytes.size.toByte()) + lBytes + pBytes
        return withContext(Dispatchers.IO) {
            try {
                writeFrame(Cmd.LOGIN, payload)
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
            val payload = byteArrayOf(lBytes.size.toByte()) + lBytes + pBytes
            writeFrame(Cmd.REGISTER, payload)
            // Read the framed register response directly (startReader not called yet)
            socket!!.soTimeout = 5000
            val frame = readFrame() ?: return@withContext Result.failure(Exception("No response"))
            socket!!.soTimeout = 0
            Result.success(frame.isNotEmpty() && frame[0] == 0x01.toByte())
        } catch (e: Exception) {
            Result.failure(e)
        }
    }

    // ---- Send message (with counter nonce and optional fragmentation) ----
    suspend fun sendMessage(text: String): Result<Unit> = withContext(Dispatchers.IO) {
        try {
            val key = sharedKey ?: return@withContext Result.failure(Exception("No key"))
            val cnt = sendCounter.incrementAndGet()
            // Nonce: 8-byte LE counter + 4 zero bytes = 12 bytes
            val nonce = ByteArray(12)
            for (i in 0..7) nonce[i] = ((cnt ushr (i * 8)) and 0xFF).toByte()

            val ciphertext = chachaEncrypt(key, nonce, text.toByteArray(Charsets.UTF_8))

            // msgPayload = [Counter(8 LE)][Ciphertext(N+16)]
            val msgPayload = nonce.sliceArray(0..7) + ciphertext

            // Frame = 2 (len prefix) + 1 (cmd) + msgPayload.size
            if (3 + msgPayload.size <= MAX_FRAME_SIZE) {
                writeFrame(Cmd.MSG, msgPayload)
            } else {
                // Fragment the message
                val msgId    = (fragCounter.incrementAndGet() and 0xFF).toByte()
                val maxChunk = MAX_FRAME_SIZE - 6 // 2 (len) + 1 (cmd) + 3 (msgID, fragIdx, fragCount)
                val chunks   = msgPayload.toList().chunked(maxChunk.coerceAtLeast(1))
                val fragCount = chunks.size.toByte()
                chunks.forEachIndexed { idx, chunk ->
                    val fp = byteArrayOf(msgId, idx.toByte(), fragCount) + chunk.toByteArray()
                    writeFrame(Cmd.FRAGMENT, fp)
                }
            }
            Result.success(Unit)
        } catch (e: Exception) {
            Result.failure(e)
        }
    }

    // ---- Read exactly one framed event (blocking) ----
    private fun readOneEvent(): ServerEvent? {
        val frame = readFrame() ?: return ServerEvent.Disconnected
        if (frame.isEmpty()) return null
        val cmd     = frame[0]
        val payload = if (frame.size > 1) frame.sliceArray(1 until frame.size) else ByteArray(0)
        return when (cmd) {
            Cmd.LOGIN_OK -> {
                if (payload.size < 2) return null
                sessionID = ((payload[0].toInt() and 0xFF) shl 8) or (payload[1].toInt() and 0xFF)
                ServerEvent.LoginOk
            }
            Cmd.LOGIN_FAIL  -> ServerEvent.LoginFail
            Cmd.ACK         -> null
            Cmd.HISTORY_END -> ServerEvent.HistoryEnd
            Cmd.HISTORY     -> parseHistory(payload)
            Cmd.INCOMING    -> parseIncoming(payload)
            Cmd.ONLINE_LIST -> parseOnlineList(payload)
            Cmd.ONLINE_ADD  -> parseOnlineAdd(payload)
            Cmd.ONLINE_REMOVE -> parseOnlineRemove(payload)
            else -> null
        }
    }

    // Read one complete frame: reads 2-byte LE length then that many bytes.
    // Returns null on EOF/error.
    private fun readFrame(): ByteArray? {
        val inp = din ?: return null
        return try {
            val lenBytes = ByteArray(2)
            inp.readFully(lenBytes)
            val frameLen = (lenBytes[0].toInt() and 0xFF) or ((lenBytes[1].toInt() and 0xFF) shl 8)
            if (frameLen < 1) return ByteArray(0)
            val frame = ByteArray(frameLen)
            inp.readFully(frame)
            frame
        } catch (_: Exception) {
            null
        }
    }

    // ---- Parsers ----

    private fun parseHistory(payload: ByteArray): ServerEvent? {
        // Payload: [SenderLen(1)][Sender(N)][Timestamp(4 BE)][Content...]
        if (payload.size < 6) return null
        val senderLen = payload[0].toInt() and 0xFF
        if (payload.size < 1 + senderLen + 4) return null
        val sender  = String(payload, 1, senderLen)
        val tsOff   = 1 + senderLen
        val ts      = ((payload[tsOff].toInt() and 0xFF) shl 24) or
                      ((payload[tsOff + 1].toInt() and 0xFF) shl 16) or
                      ((payload[tsOff + 2].toInt() and 0xFF) shl 8) or
                      (payload[tsOff + 3].toInt() and 0xFF)
        val content = String(payload, tsOff + 4, payload.size - (tsOff + 4))
        val timeStr = java.text.SimpleDateFormat("yyyy-MM-dd HH:mm", java.util.Locale.getDefault())
            .format(java.util.Date(ts.toLong() * 1000L))
        return ServerEvent.History(ChatMessage(sender = sender, text = content, time = timeStr))
    }

    private fun parseIncoming(payload: ByteArray): ServerEvent? {
        // Payload: [SenderSID(2 BE)][Counter(8 LE)][Ciphertext(N+16)]
        if (payload.size < 2 + 8 + 16) return null
        val senderSID = ((payload[0].toInt() and 0xFF) shl 8) or (payload[1].toInt() and 0xFF)
        // Nonce: bytes 2-9 = counter (8 bytes LE), bytes 10-11 = zero (already zero in ByteArray)
        val nonce = ByteArray(12)
        payload.copyInto(nonce, destinationOffset = 0, startIndex = 2, endIndex = 10)
        val ciphertext = payload.sliceArray(10 until payload.size)
        val key = sharedKey ?: return null
        return try {
            val plain      = chachaDecrypt(key, nonce, ciphertext)
            val senderName = sidNames[senderSID] ?: "SID:$senderSID"
            val now        = java.text.SimpleDateFormat("HH:mm", java.util.Locale.getDefault())
                .format(java.util.Date())
            ServerEvent.Message(ChatMessage(sender = senderName, text = String(plain), time = now))
        } catch (_: Exception) { null }
    }

    private fun parseOnlineList(payload: ByteArray): ServerEvent? {
        // Payload: [Count(1)][SID(2 BE)][NameLen(1)][Name]...
        if (payload.isEmpty()) return null
        val count  = payload[0].toInt() and 0xFF
        var off    = 1
        val newMap = HashMap<Int, String>()
        val names  = mutableListOf<String>()
        repeat(count) {
            if (off + 3 > payload.size) return null
            val sid  = ((payload[off].toInt() and 0xFF) shl 8) or (payload[off + 1].toInt() and 0xFF)
            off += 2
            val nLen = payload[off].toInt() and 0xFF; off++
            if (off + nLen > payload.size) return null
            val name = String(payload, off, nLen); off += nLen
            newMap[sid] = name
            names += name
        }
        sidNames.clear()
        sidNames.putAll(newMap)
        return ServerEvent.OnlineList(names)
    }

    private fun parseOnlineAdd(payload: ByteArray): ServerEvent? {
        // Payload: [SID(2 BE)][NameLen(1)][Name]
        if (payload.size < 4) return null
        val sid  = ((payload[0].toInt() and 0xFF) shl 8) or (payload[1].toInt() and 0xFF)
        val nLen = payload[2].toInt() and 0xFF
        if (3 + nLen > payload.size) return null
        val name = String(payload, 3, nLen)
        sidNames[sid] = name
        return ServerEvent.OnlineAdd(sid, name)
    }

    private fun parseOnlineRemove(payload: ByteArray): ServerEvent? {
        // Payload: [SID(2 BE)]
        if (payload.size < 2) return null
        val sid  = ((payload[0].toInt() and 0xFF) shl 8) or (payload[1].toInt() and 0xFF)
        val name = sidNames.remove(sid) ?: ""
        return ServerEvent.OnlineRemove(sid, name)
    }

    // ---- Write helpers ----

    // writeFrame sends [TotalLen(2 LE)][cmd(1)][payload...] and flushes.
    private fun writeFrame(cmd: Byte, payload: ByteArray) {
        val out = output ?: return
        val total = 1 + payload.size
        val frame = ByteArray(2 + total)
        frame[0] = (total and 0xFF).toByte()
        frame[1] = ((total ushr 8) and 0xFF).toByte()
        frame[2] = cmd
        payload.copyInto(frame, 3)
        out.write(frame)
        out.flush()
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
        val kp      = gen.generateKeyPair()
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
