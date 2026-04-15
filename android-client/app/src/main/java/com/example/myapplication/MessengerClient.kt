package com.example.myapplication

import android.content.SharedPreferences
import android.util.Base64
import kotlinx.coroutines.*
import kotlinx.coroutines.flow.MutableSharedFlow
import kotlinx.coroutines.flow.SharedFlow
import kotlinx.coroutines.flow.onSubscription
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
import javax.crypto.Mac
import javax.crypto.spec.SecretKeySpec

// ---- Protocol commands ----
object Cmd {
    const val REGISTER          = 0x01.toByte()
    const val LOGIN             = 0x02.toByte()
    // 0x03-0x05 retired (plaintext msg/incoming/history removed)
    const val ACK               = 0x06.toByte()
    const val HISTORY_END       = 0x07.toByte()
    const val LOGIN_OK          = 0x08.toByte()
    const val LOGIN_FAIL        = 0x09.toByte()
    const val ONLINE_LIST       = 0x0A.toByte()
    const val ONLINE_ADD        = 0x0B.toByte()
    const val ONLINE_REMOVE     = 0x0C.toByte()
    const val FRAGMENT          = 0x0D.toByte()
    const val SERVER_LIST       = 0x0E.toByte()
    // E2E
    const val SET_PUBLIC_KEY     = 0x0F.toByte()
    const val PUBLIC_KEY         = 0x10.toByte()
    const val PUBLIC_KEY_REQUEST = 0x11.toByte()
    const val E2E_MSG            = 0x12.toByte()
    const val E2E_INCOMING       = 0x13.toByte()
    const val E2E_HISTORY        = 0x14.toByte()
    // Direct messages
    const val DM          = 0x15.toByte()
    const val DM_INCOMING = 0x16.toByte()
    const val DM_HISTORY  = 0x17.toByte()
    // Rooms
    const val CREATE_ROOM        = 0x18.toByte()
    const val ROOM_CREATED       = 0x19.toByte()
    const val ROOM_LIST          = 0x1A.toByte()
    const val JOIN_ROOM          = 0x1B.toByte()
    const val LEAVE_ROOM         = 0x1C.toByte()
    const val ROOM_MSG           = 0x1D.toByte()
    const val ROOM_MSG_INCOMING  = 0x1E.toByte()
    const val ROOM_HISTORY       = 0x1F.toByte()
    const val ROOM_INVITE        = 0x20.toByte()
    const val ROOM_MEMBERS       = 0x21.toByte()
    const val ROOM_MEMBER_ADD    = 0x22.toByte()
    const val ROOM_MEMBER_REM    = 0x23.toByte()
}

private const val MAX_FRAME_SIZE = 180

data class AppConfig(
    val serverAddr: String  = "94.103.169.82:9999",
    val proxyAddr:  String  = "127.0.0.1:18000",
    val directMode: Boolean = false,
    val language:   String  = "en"
)

data class ChatMessage(
    val sender: String,
    val text:   String,
    val time:   String,
    val own:    Boolean = false
)

sealed class ServerEvent {
    data class Message(val msg: ChatMessage)  : ServerEvent()
    data class History(val msg: ChatMessage)  : ServerEvent()
    object HistoryEnd                         : ServerEvent()
    data class OnlineList(val users: List<String>) : ServerEvent()
    data class OnlineAdd(val sid: Int, val name: String) : ServerEvent()
    data class OnlineRemove(val sid: Int, val name: String) : ServerEvent()
    object LoginOk                            : ServerEvent()
    object LoginFail                          : ServerEvent()
    object Disconnected                       : ServerEvent()
    data class ServerList(val addrs: List<String>) : ServerEvent()
    // DM events
    data class DMMessage(val sender: String, val text: String, val time: String) : ServerEvent()
    data class DMHistory(val sender: String, val recipient: String, val text: String, val time: String) : ServerEvent()
    // Room events
    data class RoomInfo(val id: Long, val name: String, val isPublic: Boolean, val owner: String) : ServerEvent()
    data class RoomList(val rooms: List<RoomInfo>) : ServerEvent()
    data class RoomMembers(val roomId: Long, val members: List<String>) : ServerEvent()
    data class RoomMemberAdd(val roomId: Long, val login: String) : ServerEvent()
    data class RoomMemberRem(val roomId: Long, val login: String) : ServerEvent()
    data class RoomMessage(val roomId: Long, val sender: String, val text: String, val time: String) : ServerEvent()
    data class RoomHistoryMsg(val roomId: Long, val sender: String, val text: String, val time: String) : ServerEvent()
}

class MessengerClient {
    private var socket: Socket? = null
    private var din:    DataInputStream? = null
    private var output: OutputStream?    = null
    private var sharedKey: ByteArray?    = null
    var sessionID: Int = 0
        private set

    private val _events = MutableSharedFlow<ServerEvent>(extraBufferCapacity = 64)
    val events: SharedFlow<ServerEvent> = _events

    private var readerJob: Job? = null
    private val scope = CoroutineScope(Dispatchers.IO + SupervisorJob())

    // Per-session counters
    private val fragCounter = AtomicInteger(0)

    // SID → username (kept in sync with Online events)
    val sidNames = ConcurrentHashMap<Int, String>()

    // E2E state
    private var e2ePrivKey: X25519PrivateKeyParameters? = null
    private var e2ePubKey:  X25519PublicKeyParameters?  = null
    @Volatile private var myLogin = ""

    // username → 32-byte pubkey
    private val knownPubkeys = ConcurrentHashMap<String, ByteArray>()

    // Messages waiting for pubkeys
    private val pendingMessages = mutableListOf<String>()
    private val pendingLock     = Any()

    // Room state: id → name, id → member logins
    val rooms = ConcurrentHashMap<Long, String>()
    val roomMembers = ConcurrentHashMap<Long, MutableSet<String>>()

    // ---- Fragment reassembly ----
    private data class FragKey(val msgId: Byte, val count: Byte)
    private val fragMap = ConcurrentHashMap<FragKey, Array<ByteArray?>>()

    // ---- HKDF (RFC 5869, HMAC-SHA256) ----
    private fun hkdf(ikm: ByteArray, salt: ByteArray, info: ByteArray, length: Int): ByteArray {
        val mac = Mac.getInstance("HmacSHA256")
        // Extract
        mac.init(SecretKeySpec(salt, "HmacSHA256"))
        val prk = mac.doFinal(ikm)
        // Expand
        var output = ByteArray(0)
        var T      = ByteArray(0)
        var counter = 1
        while (output.size < length) {
            mac.init(SecretKeySpec(prk, "HmacSHA256"))
            mac.update(T); mac.update(info); mac.update(counter.toByte())
            T = mac.doFinal()
            output += T
            counter++
        }
        return output.copyOf(length)
    }

    // ---- NaCl-style envelope: ephPub(32) + ChaCha20Poly1305(msgKey)(48) ----
    private fun sealEnvelope(recipientPubBytes: ByteArray, msgKey: ByteArray): ByteArray {
        val gen = X25519KeyPairGenerator()
        gen.init(X25519KeyGenerationParameters(SecureRandom()))
        val kp      = gen.generateKeyPair()
        val ephPriv = kp.private  as X25519PrivateKeyParameters
        val ephPub  = kp.public   as X25519PublicKeyParameters
        val ephPubBytes = ephPub.encoded

        val agreement = X25519Agreement()
        agreement.init(ephPriv)
        val shared = ByteArray(agreement.agreementSize)
        agreement.calculateAgreement(X25519PublicKeyParameters(recipientPubBytes), shared, 0)

        val km = hkdf(shared, ephPubBytes, "dnstt-e2e-v1".toByteArray(), 44)
        val ct = chachaEncrypt(km.copyOf(32), km.copyOfRange(32, 44), msgKey)
        return ephPubBytes + ct  // 32 + 48 = 80 bytes
    }

    private fun openEnvelope(envelope: ByteArray): ByteArray {
        require(envelope.size == 80) { "Invalid envelope size: ${envelope.size}" }
        val ephPubBytes = envelope.copyOf(32)
        val ct          = envelope.copyOfRange(32, 80)

        val agreement = X25519Agreement()
        agreement.init(e2ePrivKey!!)
        val shared = ByteArray(agreement.agreementSize)
        agreement.calculateAgreement(X25519PublicKeyParameters(ephPubBytes), shared, 0)

        val km = hkdf(shared, ephPubBytes, "dnstt-e2e-v1".toByteArray(), 44)
        return chachaDecrypt(km.copyOf(32), km.copyOfRange(32, 44), ct)
    }

    // ---- E2E key management ----
    fun loadOrGenerateE2EKey(prefs: SharedPreferences) {
        val privB64 = prefs.getString("e2e_privkey", null)
        val pubB64  = prefs.getString("e2e_pubkey",  null)
        if (privB64 != null && pubB64 != null) {
            try {
                val privBytes = Base64.decode(privB64, Base64.DEFAULT)
                e2ePrivKey = X25519PrivateKeyParameters(privBytes)
                e2ePubKey  = e2ePrivKey!!.generatePublicKey()
                return
            } catch (_: Exception) {}
        }
        // Generate
        val gen = X25519KeyPairGenerator()
        gen.init(X25519KeyGenerationParameters(SecureRandom()))
        val kp = gen.generateKeyPair()
        e2ePrivKey = kp.private  as X25519PrivateKeyParameters
        e2ePubKey  = kp.public   as X25519PublicKeyParameters
        prefs.edit()
            .putString("e2e_privkey", Base64.encodeToString(e2ePrivKey!!.encoded, Base64.DEFAULT))
            .putString("e2e_pubkey",  Base64.encodeToString(e2ePubKey!!.encoded,  Base64.DEFAULT))
            .apply()
    }

    // ---- Connect ----
    suspend fun connect(cfg: AppConfig, prefs: SharedPreferences? = null): Result<Unit> =
        withContext(Dispatchers.IO) {
            try {
                try { socket?.close() } catch (_: Exception) {}
                socket = null; din = null; output = null

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
                socket    = sock
                val rawIn = BufferedInputStream(sock.getInputStream())
                din       = DataInputStream(rawIn)
                output    = sock.getOutputStream()
                sharedKey = ecdhHandshake()
                sock.soTimeout = 0

                // Load or generate E2E keypair
                if (prefs != null) loadOrGenerateE2EKey(prefs)

                Result.success(Unit)
            } catch (e: Exception) {
                Result.failure(e)
            }
        }

    // ---- Start background reader ----
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
        myLogin = login
        val lBytes  = login.toByteArray()
        val pBytes  = pass.toByteArray()
        val payload = byteArrayOf(lBytes.size.toByte()) + lBytes + pBytes
        return withContext(Dispatchers.IO) {
            try {
                val deferred   = CompletableDeferred<Result<Int>>()
                val subscribed = CompletableDeferred<Unit>()
                val job = scope.launch {
                    events.onSubscription { subscribed.complete(Unit) }.collect { event ->
                        when (event) {
                            is ServerEvent.LoginOk   -> { deferred.complete(Result.success(sessionID)); cancel() }
                            is ServerEvent.LoginFail -> { deferred.complete(Result.failure(Exception("Invalid credentials"))); cancel() }
                            is ServerEvent.Disconnected -> { deferred.complete(Result.failure(Exception("Disconnected"))); cancel() }
                            else -> {}
                        }
                    }
                }
                subscribed.await()
                writeFrame(Cmd.LOGIN, payload)
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
            val lBytes  = login.toByteArray()
            val pBytes  = pass.toByteArray()
            val payload = byteArrayOf(lBytes.size.toByte()) + lBytes + pBytes
            writeFrame(Cmd.REGISTER, payload)
            socket!!.soTimeout = 5000
            val frame = readFrame() ?: return@withContext Result.failure(Exception("No response"))
            socket!!.soTimeout = 0
            Result.success(frame.isNotEmpty() && frame[0] == 0x01.toByte())
        } catch (e: Exception) {
            Result.failure(e)
        }
    }

    // ---- Send E2E message ----
    suspend fun sendMessage(text: String): Result<Unit> = withContext(Dispatchers.IO) {
        try {
            val privKey = e2ePrivKey
                ?: return@withContext Result.failure(Exception("E2E key not initialized"))

            // Collect recipients + detect missing pubkeys.
            // Self is excluded from the missing check — we always have our own key.
            val recipients = mutableMapOf<String, ByteArray>()
            val missing    = mutableListOf<String>()
            for ((_, name) in sidNames) {
                if (name == myLogin) continue // self handled below
                val pk = knownPubkeys[name]
                if (pk != null) recipients[name] = pk else missing += name
            }
            // Add self unconditionally
            if (myLogin.isNotEmpty()) {
                val selfPk = e2ePubKey!!.encoded
                recipients[myLogin] = selfPk
                knownPubkeys[myLogin] = selfPk
            }

            if (missing.isNotEmpty()) {
                synchronized(pendingLock) { pendingMessages += text }
                missing.forEach { sendPublicKeyRequestInternal(it) }
                return@withContext Result.success(Unit)
            }

            doSendE2EMessage(text, recipients)
            Result.success(Unit)
        } catch (e: Exception) {
            Result.failure(e)
        }
    }

    private fun doSendE2EMessage(text: String, recipients: Map<String, ByteArray>) {
        val rng = SecureRandom()

        // 1. Random msgKey + nonce
        val msgKey = ByteArray(32).also { rng.nextBytes(it) }
        val nonce  = ByteArray(12).also { rng.nextBytes(it) }

        // 2. Encrypt content
        val encContent = chachaEncrypt(msgKey, nonce, text.toByteArray(Charsets.UTF_8))

        // 3. Build envelopes
        data class Env(val login: String, val data: ByteArray)
        val envelopes = recipients.mapNotNull { (login, pubkey) ->
            try { Env(login, sealEnvelope(pubkey, msgKey)) } catch (_: Exception) { null }
        }

        // 4. Assemble payload
        // [Nonce(12)][EncContentLen(2 LE)][EncContent(N)][EnvelopeCount(1)]
        // per envelope: [LoginLen(1)][Login(N)][Envelope(80)]
        var assembled = nonce
        val ecLenBuf = ByteArray(2)
        ecLenBuf[0] = (encContent.size and 0xFF).toByte()
        ecLenBuf[1] = ((encContent.size shr 8) and 0xFF).toByte()
        assembled += ecLenBuf + encContent
        assembled += byteArrayOf(envelopes.size.toByte())
        for (env in envelopes) {
            val lb = env.login.toByteArray()
            assembled += byteArrayOf(lb.size.toByte()) + lb + env.data
        }

        // 5. Fragment with CmdE2EMsg prefix
        sendFragmented(Cmd.E2E_MSG, assembled)
    }

    // ---- Fragment and send with cmd prefix ----
    private fun sendFragmented(cmd: Byte, payload: ByteArray) {
        val full      = byteArrayOf(cmd) + payload
        val msgId     = (fragCounter.incrementAndGet() and 0xFF).toByte()
        val maxChunk  = MAX_FRAME_SIZE - 6
        val chunks    = full.toList().chunked(maxChunk.coerceAtLeast(1))
        if (chunks.size > 255) return
        val fragCount = chunks.size.toByte()
        chunks.forEachIndexed { idx, chunk ->
            val fp = byteArrayOf(msgId, idx.toByte(), fragCount) + chunk.toByteArray()
            writeFrame(Cmd.FRAGMENT, fp)
        }
    }

    // ---- Upload our E2E public key ----
    private fun sendSetPublicKey() {
        val pub = e2ePubKey ?: return
        writeFrame(Cmd.SET_PUBLIC_KEY, pub.encoded)
    }

    // ---- Request a user's public key ----
    private fun sendPublicKeyRequestInternal(username: String) {
        val ub = username.toByteArray()
        writeFrame(Cmd.PUBLIC_KEY_REQUEST, byteArrayOf(ub.size.toByte()) + ub)
    }

    // ---- Flush pending messages once all pubkeys are available ----
    private fun flushPendingMessages() {
        val msgs = synchronized(pendingLock) {
            val copy = pendingMessages.toList()
            pendingMessages.clear()
            copy
        }
        for (text in msgs) {
            val recipients = mutableMapOf<String, ByteArray>()
            val missing    = mutableListOf<String>()
            for ((_, name) in sidNames) {
                if (name == myLogin) continue // self handled below
                val pk = knownPubkeys[name]
                if (pk != null) recipients[name] = pk else missing += name
            }
            if (myLogin.isNotEmpty()) recipients[myLogin] = e2ePubKey!!.encoded
            if (missing.isNotEmpty()) {
                synchronized(pendingLock) { pendingMessages += text }
            } else {
                doSendE2EMessage(text, recipients)
            }
        }
    }

    // ---- Read exactly one framed event (blocking) ----
    private fun readOneEvent(): ServerEvent? {
        val frame   = readFrame() ?: return ServerEvent.Disconnected
        if (frame.isEmpty()) return null
        val cmd     = frame[0]
        val payload = if (frame.size > 1) frame.sliceArray(1 until frame.size) else ByteArray(0)
        return when (cmd) {
            Cmd.LOGIN_OK -> {
                if (payload.size < 2) return null
                sessionID = ((payload[0].toInt() and 0xFF) shl 8) or (payload[1].toInt() and 0xFF)
                // Upload E2E pubkey immediately after login
                sendSetPublicKey()
                ServerEvent.LoginOk
            }
            Cmd.LOGIN_FAIL    -> ServerEvent.LoginFail
            Cmd.ACK           -> null
            Cmd.HISTORY_END   -> ServerEvent.HistoryEnd
            Cmd.E2E_HISTORY   -> parseE2EHistory(payload)
            Cmd.E2E_INCOMING  -> parseE2EIncoming(payload)
            Cmd.PUBLIC_KEY    -> {
                parsePublicKeyResponse(payload)
                null  // handled internally; no UI event
            }
            Cmd.FRAGMENT -> {
                handleFragment(payload)
                null
            }
            Cmd.ONLINE_LIST   -> parseOnlineList(payload)
            Cmd.ONLINE_ADD    -> parseOnlineAdd(payload)
            Cmd.ONLINE_REMOVE -> parseOnlineRemove(payload)
            Cmd.SERVER_LIST   -> parseServerList(payload)
            // DM
            Cmd.DM_INCOMING   -> parseDMIncoming(payload)
            Cmd.DM_HISTORY    -> parseDMHistory(payload)
            // Rooms
            Cmd.ROOM_LIST         -> parseRoomList(payload)
            Cmd.ROOM_CREATED      -> parseRoomCreated(payload)
            Cmd.ROOM_MEMBERS      -> parseRoomMembers(payload)
            Cmd.ROOM_MEMBER_ADD   -> parseRoomMemberAdd(payload)
            Cmd.ROOM_MEMBER_REM   -> parseRoomMemberRem(payload)
            Cmd.ROOM_MSG_INCOMING -> parseRoomMsgIncoming(payload)
            Cmd.ROOM_HISTORY      -> parseRoomHistory(payload)
            else -> null
        }
    }

    // ---- Fragment reassembly ----
    private fun handleFragment(data: ByteArray): ServerEvent? {
        if (data.size < 4) return null
        val msgId     = data[0]
        val fragIdx   = data[1].toInt() and 0xFF
        val fragCount = data[2]
        val chunk     = data.sliceArray(3 until data.size)
        if (fragCount == 0.toByte() || fragIdx >= (fragCount.toInt() and 0xFF)) return null

        val key = FragKey(msgId, fragCount)
        val buf = fragMap.getOrPut(key) { arrayOfNulls((fragCount.toInt() and 0xFF)) }
        if (buf[fragIdx] == null) buf[fragIdx] = chunk

        if (buf.any { it == null }) return null

        // All received
        fragMap.remove(key)
        val assembled = buf.fold(ByteArray(0)) { acc, b -> acc + b!! }
        if (assembled.isEmpty()) return null

        // assembled[0] = cmd tag, assembled[1:] = payload
        val innerCmd = assembled[0]
        val innerPayload = assembled.sliceArray(1 until assembled.size)
        return when (innerCmd) {
            Cmd.E2E_MSG -> {
                // Route to E2E message handler (emitted via scope)
                scope.launch { handleE2EAssembled(innerPayload) }
                null
            }
            else -> null
        }
    }

    private suspend fun handleE2EAssembled(data: ByteArray) {
        // This path is for client receiving its own assembled E2E msg back (shouldn't happen normally)
        // but we include it for completeness. In practice clients don't receive CmdE2EMsg back.
    }

    // ---- E2E parsers ----

    // CmdE2EIncoming: [SenderLen(1)][Sender(N)][MsgID(4 LE)][storedBlob(12+N)][Envelope(80)]
    private fun parseE2EIncoming(payload: ByteArray): ServerEvent? {
        if (payload.size < 1 + 1 + 4 + 14 + 80) return null
        val sLen = payload[0].toInt() and 0xFF
        if (payload.size < 1 + sLen + 4 + 14 + 80) return null
        val senderName = String(payload, 1, sLen)
        // MsgID at offset 1+sLen (4 bytes LE) — unused on client side
        val storedBlob = payload.sliceArray(1 + sLen + 4 until payload.size - 80)
        val envelope   = payload.sliceArray(payload.size - 80 until payload.size)
        if (storedBlob.size < 14) return null

        val nonce         = storedBlob.copyOf(12)
        val encContentLen = ((storedBlob[12].toInt() and 0xFF)) or ((storedBlob[13].toInt() and 0xFF) shl 8)
        if (storedBlob.size < 14 + encContentLen) return null
        val ciphertext = storedBlob.sliceArray(14 until 14 + encContentLen)

        return try {
            val msgKey = openEnvelope(envelope)
            val plain  = chachaDecrypt(msgKey, nonce, ciphertext)
            val now = java.text.SimpleDateFormat("HH:mm", java.util.Locale.getDefault())
                .format(java.util.Date())
            ServerEvent.Message(ChatMessage(sender = senderName, text = String(plain), time = now))
        } catch (_: Exception) { null }
    }

    // CmdE2EHistory: [SenderLen(1)][Sender(N)][Timestamp(4 BE)][MsgID(4 LE)][storedBlob(12+N)][Envelope(80)]
    private fun parseE2EHistory(payload: ByteArray): ServerEvent? {
        if (payload.size < 103) return null
        val senderLen = payload[0].toInt() and 0xFF
        if (1 + senderLen + 4 + 4 + 12 + 2 + 80 > payload.size) return null
        val sender  = String(payload, 1, senderLen)
        val tsOff   = 1 + senderLen
        val ts      = ((payload[tsOff].toInt() and 0xFF) shl 24) or
                      ((payload[tsOff + 1].toInt() and 0xFF) shl 16) or
                      ((payload[tsOff + 2].toInt() and 0xFF) shl 8) or
                      (payload[tsOff + 3].toInt() and 0xFF)
        // MsgID at tsOff+4 (4 bytes, LE) — skip
        val blobOff    = tsOff + 4 + 4
        val storedBlob = payload.sliceArray(blobOff until payload.size - 80)
        val envelope   = payload.sliceArray(payload.size - 80 until payload.size)
        if (storedBlob.size < 14) return null

        val nonce         = storedBlob.copyOf(12)
        val encContentLen = ((storedBlob[12].toInt() and 0xFF)) or ((storedBlob[13].toInt() and 0xFF) shl 8)
        if (storedBlob.size < 14 + encContentLen) return null
        val ciphertext = storedBlob.sliceArray(14 until 14 + encContentLen)

        return try {
            val msgKey  = openEnvelope(envelope)
            val plain   = chachaDecrypt(msgKey, nonce, ciphertext)
            val timeStr = java.text.SimpleDateFormat("yyyy-MM-dd HH:mm", java.util.Locale.getDefault())
                .format(java.util.Date(ts.toLong() * 1000L))
            ServerEvent.History(ChatMessage(sender = sender, text = String(plain), time = timeStr))
        } catch (_: Exception) { null }
    }

    // CmdPublicKey: [UsernameLen(1)][Username(N)][pubkey(32)]
    private fun parsePublicKeyResponse(payload: ByteArray) {
        if (payload.size < 1 + 32) return
        val lLen = payload[0].toInt() and 0xFF
        if (payload.size < 1 + lLen + 32) return
        val username = String(payload, 1, lLen)
        val pubkey   = payload.sliceArray(1 + lLen until 1 + lLen + 32)
        knownPubkeys[username] = pubkey
        flushPendingMessages()
    }

    // ---- Online parsers ----
    private fun parseOnlineList(payload: ByteArray): ServerEvent? {
        if (payload.isEmpty()) return null
        val count  = payload[0].toInt() and 0xFF
        var off    = 1
        val newMap = HashMap<Int, String>()
        val names  = mutableListOf<String>()
        repeat(count) {
            if (off + 3 > payload.size) return null
            val sid  = ((payload[off].toInt() and 0xFF) shl 8) or (payload[off + 1].toInt() and 0xFF); off += 2
            val nLen = payload[off].toInt() and 0xFF; off++
            if (off + nLen > payload.size) return null
            val name = String(payload, off, nLen); off += nLen
            newMap[sid] = name; names += name
        }
        sidNames.clear(); sidNames.putAll(newMap)
        // Request pubkeys for all online users
        names.forEach { n -> if (!knownPubkeys.containsKey(n)) sendPublicKeyRequestInternal(n) }
        return ServerEvent.OnlineList(names)
    }

    private fun parseOnlineAdd(payload: ByteArray): ServerEvent? {
        if (payload.size < 4) return null
        val sid  = ((payload[0].toInt() and 0xFF) shl 8) or (payload[1].toInt() and 0xFF)
        val nLen = payload[2].toInt() and 0xFF
        if (3 + nLen > payload.size) return null
        val name = String(payload, 3, nLen)
        sidNames[sid] = name
        if (!knownPubkeys.containsKey(name)) sendPublicKeyRequestInternal(name)
        return ServerEvent.OnlineAdd(sid, name)
    }

    private fun parseServerList(payload: ByteArray): ServerEvent? {
        if (payload.isEmpty()) return null
        val count = payload[0].toInt() and 0xFF
        var off = 1
        val servers = mutableListOf<String>()
        repeat(count) {
            if (off >= payload.size) return@repeat
            val aLen = payload[off++].toInt() and 0xFF
            if (off + aLen > payload.size) return@repeat
            servers += String(payload, off, aLen); off += aLen
        }
        return ServerEvent.ServerList(servers)
    }

    private fun parseOnlineRemove(payload: ByteArray): ServerEvent? {
        if (payload.size < 2) return null
        val sid  = ((payload[0].toInt() and 0xFF) shl 8) or (payload[1].toInt() and 0xFF)
        val name = sidNames.remove(sid) ?: ""
        return ServerEvent.OnlineRemove(sid, name)
    }

    // ---- Read one complete frame ----
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
        } catch (_: Exception) { null }
    }

    // ---- Write helpers ----
    private fun writeFrame(cmd: Byte, payload: ByteArray) {
        val out   = output ?: return
        val total = 1 + payload.size
        val frame = ByteArray(2 + total)
        frame[0] = (total and 0xFF).toByte()
        frame[1] = ((total ushr 8) and 0xFF).toByte()
        frame[2] = cmd
        payload.copyInto(frame, 3)
        out.write(frame); out.flush()
    }

    // ---- SOCKS5 ----
    private fun socks5Handshake(sock: Socket, dstHost: String, dstPort: Int) {
        val out = sock.getOutputStream(); val inp = sock.getInputStream()
        out.write(byteArrayOf(0x05, 0x01, 0x00)); out.flush()
        val gr = ByteArray(2); readFull(inp, gr)
        if (gr[0] != 0x05.toByte() || gr[1] != 0x00.toByte()) throw Exception("SOCKS5 auth failed")
        val hostBytes = dstHost.toByteArray(Charsets.US_ASCII)
        val req = mutableListOf<Byte>()
        req += listOf(0x05, 0x01, 0x00, 0x03).map { it.toByte() }
        req += hostBytes.size.toByte()
        req += hostBytes.toList()
        req += ((dstPort shr 8) and 0xFF).toByte(); req += (dstPort and 0xFF).toByte()
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

    // ---- ECDH session handshake ----
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

    // ---- ChaCha20-Poly1305 ----
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

    // ─── Direct Messages ──────────────────────────────────────────────────────

    suspend fun sendDM(recipientLogin: String, text: String) = withContext(Dispatchers.IO) {
        val privKey = e2ePrivKey ?: return@withContext
        val recipPub = knownPubkeys[recipientLogin] ?: run {
            sendPublicKeyRequestInternal(recipientLogin)
            return@withContext
        }
        val rng = SecureRandom()
        val msgKey = ByteArray(32).also { rng.nextBytes(it) }
        val nonce  = ByteArray(12).also { rng.nextBytes(it) }
        val encContent = chachaEncrypt(msgKey, nonce, text.toByteArray(Charsets.UTF_8))

        data class Env(val login: String, val data: ByteArray)
        val envelopes = mutableListOf<Env>()
        try { envelopes += Env(recipientLogin, sealEnvelope(recipPub, msgKey)) } catch (_: Exception) { return@withContext }
        val selfPub = knownPubkeys[myLogin] ?: e2ePubKey?.encoded
        if (selfPub != null && myLogin.isNotEmpty()) {
            try { envelopes += Env(myLogin, sealEnvelope(selfPub, msgKey)) } catch (_: Exception) {}
        }

        val rl = recipientLogin.toByteArray()
        val ecLenBuf = ByteArray(2).also {
            it[0] = (encContent.size and 0xFF).toByte()
            it[1] = ((encContent.size shr 8) and 0xFF).toByte()
        }
        var assembled = byteArrayOf(rl.size.toByte()) + rl + nonce + ecLenBuf + encContent
        assembled += byteArrayOf(envelopes.size.toByte())
        for (env in envelopes) {
            val lb = env.login.toByteArray()
            assembled += byteArrayOf(lb.size.toByte()) + lb + env.data
        }
        sendFragmented(Cmd.DM, assembled)
    }

    // CmdDMIncoming: [SenderLen(1)][Sender(N)][MsgID(4LE)][storedBlob][Envelope(80)]
    private fun parseDMIncoming(payload: ByteArray): ServerEvent? {
        if (payload.size < 1 + 1 + 4 + 14 + 80) return null
        val sLen = payload[0].toInt() and 0xFF
        if (payload.size < 1 + sLen + 4 + 14 + 80) return null
        val sender     = String(payload, 1, sLen)
        val storedBlob = payload.sliceArray(1 + sLen + 4 until payload.size - 80)
        val envelope   = payload.sliceArray(payload.size - 80 until payload.size)
        if (storedBlob.size < 14) return null
        return try {
            val msgKey = openEnvelope(envelope)
            val nonce  = storedBlob.copyOf(12)
            val ecLen  = ((storedBlob[12].toInt() and 0xFF)) or ((storedBlob[13].toInt() and 0xFF) shl 8)
            val plain  = chachaDecrypt(msgKey, nonce, storedBlob.sliceArray(14 until 14 + ecLen))
            val now = java.text.SimpleDateFormat("HH:mm", java.util.Locale.getDefault()).format(java.util.Date())
            ServerEvent.DMMessage(sender = sender, text = String(plain), time = now)
        } catch (_: Exception) { null }
    }

    // CmdDMHistory: [SenderLen(1)][Sender(N)][RecipLen(1)][Recip(N)][Timestamp(4BE)][MsgID(4LE)][storedBlob][Envelope(80)]
    private fun parseDMHistory(payload: ByteArray): ServerEvent? {
        if (payload.size < 1 + 1 + 1 + 4 + 4 + 14 + 80) return null
        val sLen = payload[0].toInt() and 0xFF
        if (payload.size < 1 + sLen + 1) return null
        val sender = String(payload, 1, sLen)
        val rLen   = payload[1 + sLen].toInt() and 0xFF
        if (payload.size < 1 + sLen + 1 + rLen + 4 + 4 + 14 + 80) return null
        val recip  = String(payload, 1 + sLen + 1, rLen)
        val tsOff  = 1 + sLen + 1 + rLen
        val ts     = ((payload[tsOff].toInt() and 0xFF) shl 24) or
                     ((payload[tsOff + 1].toInt() and 0xFF) shl 16) or
                     ((payload[tsOff + 2].toInt() and 0xFF) shl 8) or
                     (payload[tsOff + 3].toInt() and 0xFF)
        val blobOff    = tsOff + 4 + 4
        val storedBlob = payload.sliceArray(blobOff until payload.size - 80)
        val envelope   = payload.sliceArray(payload.size - 80 until payload.size)
        if (storedBlob.size < 14) return null
        return try {
            val msgKey = openEnvelope(envelope)
            val nonce  = storedBlob.copyOf(12)
            val ecLen  = ((storedBlob[12].toInt() and 0xFF)) or ((storedBlob[13].toInt() and 0xFF) shl 8)
            val plain  = chachaDecrypt(msgKey, nonce, storedBlob.sliceArray(14 until 14 + ecLen))
            val timeStr = java.text.SimpleDateFormat("yyyy-MM-dd HH:mm", java.util.Locale.getDefault())
                .format(java.util.Date(ts.toLong() * 1000L))
            ServerEvent.DMHistory(sender = sender, recipient = recip, text = String(plain), time = timeStr)
        } catch (_: Exception) { null }
    }

    // ─── Rooms ────────────────────────────────────────────────────────────────

    suspend fun createRoom(name: String, isPublic: Boolean, description: String = "") = withContext(Dispatchers.IO) {
        val nb = name.toByteArray()
        val db = description.toByteArray()
        val descLenBuf = ByteArray(2).also {
            it[0] = (db.size and 0xFF).toByte()
            it[1] = ((db.size shr 8) and 0xFF).toByte()
        }
        val payload = byteArrayOf(nb.size.toByte()) + nb +
                byteArrayOf(if (isPublic) 1 else 0) + descLenBuf + db
        writeFrame(Cmd.CREATE_ROOM, payload)
    }

    suspend fun joinRoom(roomId: Long) = withContext(Dispatchers.IO) {
        val buf = ByteArray(4)
        buf[0] = (roomId and 0xFF).toByte()
        buf[1] = ((roomId shr 8) and 0xFF).toByte()
        buf[2] = ((roomId shr 16) and 0xFF).toByte()
        buf[3] = ((roomId shr 24) and 0xFF).toByte()
        writeFrame(Cmd.JOIN_ROOM, buf)
    }

    suspend fun leaveRoom(roomId: Long) = withContext(Dispatchers.IO) {
        val buf = ByteArray(4)
        buf[0] = (roomId and 0xFF).toByte()
        buf[1] = ((roomId shr 8) and 0xFF).toByte()
        buf[2] = ((roomId shr 16) and 0xFF).toByte()
        buf[3] = ((roomId shr 24) and 0xFF).toByte()
        writeFrame(Cmd.LEAVE_ROOM, buf)
    }

    suspend fun inviteToRoom(roomId: Long, username: String) = withContext(Dispatchers.IO) {
        val ub = username.toByteArray()
        val idBuf = ByteArray(4)
        idBuf[0] = (roomId and 0xFF).toByte()
        idBuf[1] = ((roomId shr 8) and 0xFF).toByte()
        idBuf[2] = ((roomId shr 16) and 0xFF).toByte()
        idBuf[3] = ((roomId shr 24) and 0xFF).toByte()
        writeFrame(Cmd.ROOM_INVITE, idBuf + byteArrayOf(ub.size.toByte()) + ub)
    }

    suspend fun sendRoomMessage(roomId: Long, text: String) = withContext(Dispatchers.IO) {
        val rng = SecureRandom()
        val msgKey = ByteArray(32).also { rng.nextBytes(it) }
        val nonce  = ByteArray(12).also { rng.nextBytes(it) }
        val encContent = chachaEncrypt(msgKey, nonce, text.toByteArray(Charsets.UTF_8))

        data class Env(val login: String, val data: ByteArray)
        val envelopes = mutableListOf<Env>()
        // Use room members (not all online users) so offline members get their envelope
        val members = roomMembers[roomId] ?: emptySet<String>()
        for (name in members) {
            if (name == myLogin) continue // self handled below
            val pk = knownPubkeys[name] ?: continue
            try { envelopes += Env(name, sealEnvelope(pk, msgKey)) } catch (_: Exception) {}
        }
        if (myLogin.isNotEmpty()) {
            val selfPk = knownPubkeys[myLogin] ?: e2ePubKey?.encoded
            if (selfPk != null) {
                try { envelopes += Env(myLogin, sealEnvelope(selfPk, msgKey)) } catch (_: Exception) {}
            }
        }

        val idBuf = ByteArray(4)
        idBuf[0] = (roomId and 0xFF).toByte()
        idBuf[1] = ((roomId shr 8) and 0xFF).toByte()
        idBuf[2] = ((roomId shr 16) and 0xFF).toByte()
        idBuf[3] = ((roomId shr 24) and 0xFF).toByte()
        val ecLenBuf = ByteArray(2).also {
            it[0] = (encContent.size and 0xFF).toByte()
            it[1] = ((encContent.size shr 8) and 0xFF).toByte()
        }
        var assembled = idBuf + nonce + ecLenBuf + encContent + byteArrayOf(envelopes.size.toByte())
        for (env in envelopes) {
            val lb = env.login.toByteArray()
            assembled += byteArrayOf(lb.size.toByte()) + lb + env.data
        }
        sendFragmented(Cmd.ROOM_MSG, assembled)
    }

    // CmdRoomList: [RoomCount(2LE)] per: [RoomID(4LE)][NameLen(1)][Name(N)][IsPublic(1)][OwnerLen(1)][Owner(N)][MemberCount(2LE)]
    private fun parseRoomList(payload: ByteArray): ServerEvent? {
        if (payload.size < 2) return null
        val count = ((payload[0].toInt() and 0xFF)) or ((payload[1].toInt() and 0xFF) shl 8)
        var off = 2
        val list = mutableListOf<ServerEvent.RoomInfo>()
        repeat(count) {
            if (off + 4 + 1 > payload.size) return@repeat
            val id = ((payload[off].toInt() and 0xFF)) or
                     ((payload[off+1].toInt() and 0xFF) shl 8) or
                     ((payload[off+2].toInt() and 0xFF) shl 16) or
                     ((payload[off+3].toInt() and 0xFF) shl 24)
            off += 4
            val nLen = payload[off++].toInt() and 0xFF
            if (off + nLen + 1 + 1 + 2 > payload.size) return@repeat
            val name = String(payload, off, nLen); off += nLen
            val isPublic = payload[off++] != 0.toByte()
            val oLen = payload[off++].toInt() and 0xFF
            if (off + oLen + 2 > payload.size) return@repeat
            val owner = String(payload, off, oLen); off += oLen
            off += 2 // memberCount
            rooms[id.toLong()] = name
            list += ServerEvent.RoomInfo(id.toLong(), name, isPublic, owner)
        }
        return ServerEvent.RoomList(list)
    }

    // CmdRoomCreated: [RoomID(4LE)][NameLen(1)][Name(N)][IsPublic(1)][OwnerLen(1)][Owner(N)] [opt: InviterLen(1)][Inviter(N)]
    private fun parseRoomCreated(payload: ByteArray): ServerEvent? {
        if (payload.size < 4 + 1 + 1 + 1) return null
        val id = ((payload[0].toInt() and 0xFF)) or
                 ((payload[1].toInt() and 0xFF) shl 8) or
                 ((payload[2].toInt() and 0xFF) shl 16) or
                 ((payload[3].toInt() and 0xFF) shl 24)
        val nLen = payload[4].toInt() and 0xFF
        if (payload.size < 5 + nLen + 1 + 1) return null
        val name = String(payload, 5, nLen)
        val isPublic = payload[5 + nLen] != 0.toByte()
        val oLen = payload[6 + nLen].toInt() and 0xFF
        if (payload.size < 7 + nLen + oLen) return null
        val owner = String(payload, 7 + nLen, oLen)
        rooms[id.toLong()] = name
        return ServerEvent.RoomInfo(id.toLong(), name, isPublic, owner)
    }

    // CmdRoomMembers: [RoomID(4LE)][MemberCount(2LE)] per: [LoginLen(1)][Login(N)][IsAdmin(1)]
    private fun parseRoomMembers(payload: ByteArray): ServerEvent? {
        if (payload.size < 6) return null
        val id = ((payload[0].toInt() and 0xFF)) or
                 ((payload[1].toInt() and 0xFF) shl 8) or
                 ((payload[2].toInt() and 0xFF) shl 16) or
                 ((payload[3].toInt() and 0xFF) shl 24)
        val count = ((payload[4].toInt() and 0xFF)) or ((payload[5].toInt() and 0xFF) shl 8)
        var off = 6
        val members = mutableListOf<String>()
        repeat(count) {
            if (off >= payload.size) return@repeat
            val lLen = payload[off++].toInt() and 0xFF
            if (off + lLen + 1 > payload.size) return@repeat
            members += String(payload, off, lLen); off += lLen
            off++ // isAdmin
            if (!knownPubkeys.containsKey(members.last())) sendPublicKeyRequestInternal(members.last())
        }
        // Update local room members cache
        roomMembers[id.toLong()] = members.toMutableSet()
        return ServerEvent.RoomMembers(id.toLong(), members)
    }

    // CmdRoomMemberAdd: [RoomID(4LE)][LoginLen(1)][Login(N)]
    private fun parseRoomMemberAdd(payload: ByteArray): ServerEvent? {
        if (payload.size < 6) return null
        val id = ((payload[0].toInt() and 0xFF)) or
                 ((payload[1].toInt() and 0xFF) shl 8) or
                 ((payload[2].toInt() and 0xFF) shl 16) or
                 ((payload[3].toInt() and 0xFF) shl 24)
        val lLen = payload[4].toInt() and 0xFF
        if (payload.size < 5 + lLen) return null
        val login = String(payload, 5, lLen)
        roomMembers.getOrPut(id.toLong()) { mutableSetOf() }.add(login)
        if (!knownPubkeys.containsKey(login)) sendPublicKeyRequestInternal(login)
        return ServerEvent.RoomMemberAdd(id.toLong(), login)
    }

    // CmdRoomMemberRem: [RoomID(4LE)][LoginLen(1)][Login(N)]
    private fun parseRoomMemberRem(payload: ByteArray): ServerEvent? {
        if (payload.size < 6) return null
        val id = ((payload[0].toInt() and 0xFF)) or
                 ((payload[1].toInt() and 0xFF) shl 8) or
                 ((payload[2].toInt() and 0xFF) shl 16) or
                 ((payload[3].toInt() and 0xFF) shl 24)
        val lLen = payload[4].toInt() and 0xFF
        if (payload.size < 5 + lLen) return null
        val login = String(payload, 5, lLen)
        roomMembers[id.toLong()]?.remove(login)
        if (login == myLogin) {
            roomMembers.remove(id.toLong())
            rooms.remove(id.toLong())
        }
        return ServerEvent.RoomMemberRem(id.toLong(), login)
    }

    // CmdRoomMsgIncoming: [RoomID(4LE)][SenderLen(1)][Sender(N)][MsgID(4LE)][storedBlob][Envelope(80)]
    private fun parseRoomMsgIncoming(payload: ByteArray): ServerEvent? {
        if (payload.size < 4 + 1 + 1 + 4 + 14 + 80) return null
        val id = ((payload[0].toInt() and 0xFF)) or
                 ((payload[1].toInt() and 0xFF) shl 8) or
                 ((payload[2].toInt() and 0xFF) shl 16) or
                 ((payload[3].toInt() and 0xFF) shl 24)
        val sLen = payload[4].toInt() and 0xFF
        if (payload.size < 5 + sLen + 4 + 14 + 80) return null
        val sender     = String(payload, 5, sLen)
        val storedBlob = payload.sliceArray(5 + sLen + 4 until payload.size - 80)
        val envelope   = payload.sliceArray(payload.size - 80 until payload.size)
        if (storedBlob.size < 14) return null
        return try {
            val msgKey = openEnvelope(envelope)
            val nonce  = storedBlob.copyOf(12)
            val ecLen  = ((storedBlob[12].toInt() and 0xFF)) or ((storedBlob[13].toInt() and 0xFF) shl 8)
            val plain  = chachaDecrypt(msgKey, nonce, storedBlob.sliceArray(14 until 14 + ecLen))
            val now = java.text.SimpleDateFormat("HH:mm", java.util.Locale.getDefault()).format(java.util.Date())
            ServerEvent.RoomMessage(id.toLong(), sender, String(plain), now)
        } catch (_: Exception) { null }
    }

    // CmdRoomHistory: [RoomID(4LE)][SenderLen(1)][Sender(N)][Timestamp(4BE)][MsgID(4LE)][storedBlob][Envelope(80)]
    private fun parseRoomHistory(payload: ByteArray): ServerEvent? {
        if (payload.size < 4 + 1 + 1 + 4 + 4 + 14 + 80) return null
        val id = ((payload[0].toInt() and 0xFF)) or
                 ((payload[1].toInt() and 0xFF) shl 8) or
                 ((payload[2].toInt() and 0xFF) shl 16) or
                 ((payload[3].toInt() and 0xFF) shl 24)
        val sLen = payload[4].toInt() and 0xFF
        if (payload.size < 5 + sLen + 4 + 4 + 14 + 80) return null
        val sender = String(payload, 5, sLen)
        val tsOff  = 5 + sLen
        val ts     = ((payload[tsOff].toInt() and 0xFF) shl 24) or
                     ((payload[tsOff + 1].toInt() and 0xFF) shl 16) or
                     ((payload[tsOff + 2].toInt() and 0xFF) shl 8) or
                     (payload[tsOff + 3].toInt() and 0xFF)
        val blobOff    = tsOff + 4 + 4
        val storedBlob = payload.sliceArray(blobOff until payload.size - 80)
        val envelope   = payload.sliceArray(payload.size - 80 until payload.size)
        if (storedBlob.size < 14) return null
        return try {
            val msgKey = openEnvelope(envelope)
            val nonce  = storedBlob.copyOf(12)
            val ecLen  = ((storedBlob[12].toInt() and 0xFF)) or ((storedBlob[13].toInt() and 0xFF) shl 8)
            val plain  = chachaDecrypt(msgKey, nonce, storedBlob.sliceArray(14 until 14 + ecLen))
            val timeStr = java.text.SimpleDateFormat("yyyy-MM-dd HH:mm", java.util.Locale.getDefault())
                .format(java.util.Date(ts.toLong() * 1000L))
            ServerEvent.RoomHistoryMsg(id.toLong(), sender, String(plain), timeStr)
        } catch (_: Exception) { null }
    }

    fun destroy() {
        readerJob?.cancel(); scope.cancel()
        try { socket?.close() } catch (_: Exception) {}
        socket = null; din = null; output = null
    }
}
