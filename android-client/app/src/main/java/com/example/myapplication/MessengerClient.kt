package com.example.myapplication

import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.withContext
import org.bouncycastle.crypto.agreement.X25519Agreement
import org.bouncycastle.crypto.generators.X25519KeyPairGenerator
import org.bouncycastle.crypto.params.X25519KeyGenerationParameters
import org.bouncycastle.crypto.params.X25519PrivateKeyParameters
import org.bouncycastle.crypto.params.X25519PublicKeyParameters
import org.bouncycastle.crypto.engines.ChaCha7539Engine
import org.bouncycastle.crypto.modes.ChaCha20Poly1305
import org.bouncycastle.crypto.params.AEADParameters
import org.bouncycastle.crypto.params.KeyParameter
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
    object Disconnected : ServerEvent()
}

class MessengerClient {
    private var socket: Socket? = null
    private var input: InputStream? = null
    private var output: OutputStream? = null
    private var sharedKey: ByteArray? = null
    var sessionID: Int = 0
        private set

    // ---- Connect ----
    suspend fun connect(cfg: AppConfig): Result<Unit> = withContext(Dispatchers.IO) {
        try {
            val (host, port) = parseAddr(cfg.serverAddr)
            val sock = Socket()
            if (cfg.directMode) {
                sock.connect(InetSocketAddress(host, port), 10_000)
            } else {
                val (proxyHost, proxyPort) = parseAddr(cfg.proxyAddr)
                sock.connect(InetSocketAddress(proxyHost, proxyPort), 10_000)
                socks5Handshake(sock, host, port)
            }
            socket = sock
            input  = sock.getInputStream()
            output = sock.getOutputStream()
            sharedKey = ecdhHandshake()
            Result.success(Unit)
        } catch (e: Exception) {
            Result.failure(e)
        }
    }

    // ---- SOCKS5 handshake ----
    private fun socks5Handshake(sock: Socket, dstHost: String, dstPort: Int) {
        val out = sock.getOutputStream()
        val inp = sock.getInputStream()

        // greeting: no auth
        out.write(byteArrayOf(0x05, 0x01, 0x00))
        out.flush()
        val gr = ByteArray(2)
        readFull(inp, gr)
        if (gr[0] != 0x05.toByte() || gr[1] != 0x00.toByte())
            throw Exception("SOCKS5 auth negotiation failed")

        // connect request
        val hostBytes = dstHost.toByteArray(Charsets.US_ASCII)
        val req = mutableListOf<Byte>()
        req += listOf(0x05, 0x01, 0x00, 0x03).map { it.toByte() }
        req += hostBytes.size.toByte()
        req += hostBytes.toList()
        req += ((dstPort shr 8) and 0xFF).toByte()
        req += (dstPort and 0xFF).toByte()
        out.write(req.toByteArray())
        out.flush()

        // response: 4 bytes header + addr + port
        val resp = ByteArray(4)
        readFull(inp, resp)
        if (resp[1] != 0x00.toByte()) throw Exception("SOCKS5 connect failed: ${resp[1]}")
        when (resp[3]) {
            0x01.toByte() -> { val b = ByteArray(4); readFull(inp, b) }
            0x03.toByte() -> { val l = inp.read(); val b = ByteArray(l); readFull(inp, b) }
            0x04.toByte() -> { val b = ByteArray(16); readFull(inp, b) }
        }
        val portBytes = ByteArray(2); readFull(inp, portBytes)
    }

    // ---- ECDH X25519 handshake ----
    private fun ecdhHandshake(): ByteArray {
        val inp = input!!
        val out = output!!

        val gen = X25519KeyPairGenerator()
        gen.init(X25519KeyGenerationParameters(SecureRandom()))
        val kp = gen.generateKeyPair()
        val privKey = kp.private as X25519PrivateKeyParameters
        val pubKey  = kp.public  as X25519PublicKeyParameters

        // Read server public key (32 bytes)
        val serverPubBytes = ByteArray(32)
        readFull(inp, serverPubBytes)

        // Send our public key
        out.write(pubKey.encoded)
        out.flush()

        // Compute shared secret
        val agreement = X25519Agreement()
        agreement.init(privKey)
        val serverPub = X25519PublicKeyParameters(serverPubBytes)
        val shared = ByteArray(agreement.agreementSize)
        agreement.calculateAgreement(serverPub, shared, 0)
        return shared
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
            input!!.read(res)
            socket!!.soTimeout = 0
            Result.success(res[0] == 0x01.toByte())
        } catch (e: Exception) {
            Result.failure(e)
        }
    }

    // ---- Login ----
    suspend fun login(login: String, pass: String): Result<Int> = withContext(Dispatchers.IO) {
        try {
            val lBytes = login.toByteArray()
            val pBytes = pass.toByteArray()
            val pkt = byteArrayOf(Cmd.LOGIN, lBytes.size.toByte()) + lBytes + pBytes
            output!!.write(pkt)
            output!!.flush()
            socket!!.soTimeout = 8000
            val buf = ByteArray(3)
            val n = input!!.read(buf)
            socket!!.soTimeout = 0
            if (n >= 1 && buf[0] == Cmd.LOGIN_OK && n >= 3) {
                val sid = ((buf[1].toInt() and 0xFF) shl 8) or (buf[2].toInt() and 0xFF)
                sessionID = sid
                Result.success(sid)
            } else {
                Result.failure(Exception("Invalid credentials"))
            }
        } catch (e: Exception) {
            Result.failure(e)
        }
    }

    // ---- Send message (ChaCha20-Poly1305) ----
    suspend fun sendMessage(text: String): Result<Unit> = withContext(Dispatchers.IO) {
        try {
            val key = sharedKey ?: return@withContext Result.failure(Exception("No key"))
            val nonce = ByteArray(12).also { SecureRandom().nextBytes(it) }
            val plaintext = text.toByteArray(Charsets.UTF_8)
            val ciphertext = chachaEncrypt(key, nonce, plaintext)

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

    // ---- Read one event (blocking, call from IO coroutine) ----
    fun readEvent(): ServerEvent? {
        val inp = input ?: return null
        return try {
            val cmdByte = inp.read()
            if (cmdByte == -1) return ServerEvent.Disconnected
            val cmd = cmdByte.toByte()
            when (cmd) {
                Cmd.LOGIN_OK -> {
                    val b = ByteArray(2)
                    readFull(inp, b)
                    val sid = ((b[0].toInt() and 0xFF) shl 8) or (b[1].toInt() and 0xFF)
                    sessionID = sid
                    null // handled separately
                }
                Cmd.LOGIN_FAIL -> null
                Cmd.ACK -> null
                Cmd.HISTORY_END -> ServerEvent.HistoryEnd
                Cmd.HISTORY -> parseHistory(inp)
                Cmd.INCOMING -> parseIncoming(inp)
                Cmd.ONLINE_LIST -> parseOnlineList(inp)
                else -> null
            }
        } catch (e: Exception) {
            ServerEvent.Disconnected
        }
    }

    private fun parseHistory(inp: InputStream): ServerEvent.History? {
        val senderLen = inp.read()
        if (senderLen < 0) return null
        val senderBytes = ByteArray(senderLen); readFull(inp, senderBytes)
        val timeLen = inp.read()
        val timeBytes = ByteArray(timeLen); readFull(inp, timeBytes)
        val msgLenHi = inp.read(); val msgLenLo = inp.read()
        val msgLen = (msgLenHi shl 8) or msgLenLo
        val msgBytes = ByteArray(msgLen); readFull(inp, msgBytes)
        return ServerEvent.History(
            ChatMessage(
                sender = String(senderBytes),
                text   = String(msgBytes),
                time   = String(timeBytes).take(16)
            )
        )
    }

    private fun parseIncoming(inp: InputStream): ServerEvent.Message? {
        val senderLen = inp.read()
        if (senderLen < 0) return null
        val senderBytes = ByteArray(senderLen); readFull(inp, senderBytes)
        val nonce = ByteArray(12); readFull(inp, nonce)
        // ChaCha20-Poly1305: ciphertext = plaintext + 16-byte tag, min 17 bytes
        // Read in chunks until we have at least 17 bytes
        val ctBuf = mutableListOf<Byte>()
        val tmp = ByteArray(4096)
        // Block until we get at least 17 bytes (tag size)
        while (ctBuf.size < 17) {
            val n = inp.read(tmp)
            if (n == -1) throw Exception("Connection closed")
            for (i in 0 until n) ctBuf.add(tmp[i])
        }
        // Drain any immediately available bytes
        var avail = inp.available()
        while (avail > 0) {
            val n = inp.read(tmp, 0, minOf(avail, tmp.size))
            if (n == -1) break
            for (i in 0 until n) ctBuf.add(tmp[i])
            avail = inp.available()
        }
        val ct = ctBuf.toByteArray()
        val key = sharedKey ?: return null
        return try {
            val plain = chachaDecrypt(key, nonce, ct)
            val now = java.text.SimpleDateFormat("HH:mm", java.util.Locale.getDefault())
                .format(java.util.Date())
            ServerEvent.Message(
                ChatMessage(sender = String(senderBytes), text = String(plain), time = now)
            )
        } catch (e: Exception) { null }
    }

    private fun parseOnlineList(inp: InputStream): ServerEvent.OnlineList? {
        val count = inp.read()
        if (count < 0) return null
        val names = mutableListOf<String>()
        repeat(count) {
            val nLen = inp.read()
            val nb = ByteArray(nLen); readFull(inp, nb)
            names += String(nb)
        }
        return ServerEvent.OnlineList(names)
    }

    // ---- Crypto helpers ----
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

    // ---- Helpers ----
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
        try { socket?.close() } catch (_: Exception) {}
        socket = null; input = null; output = null
    }
}
