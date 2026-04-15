package com.example.myapplication

import android.app.Application
import android.content.ComponentName
import android.content.Context
import android.content.Intent
import android.content.ServiceConnection
import android.os.Build
import android.os.IBinder
import androidx.lifecycle.AndroidViewModel
import androidx.lifecycle.viewModelScope
import kotlinx.coroutines.launch
import kotlinx.coroutines.flow.MutableStateFlow
import kotlinx.coroutines.flow.StateFlow
import kotlinx.coroutines.withTimeoutOrNull
import org.json.JSONObject

enum class Screen { LOGIN, CHAT, DM, ROOM }

data class DMConversation(
    val partner: String,
    val messages: List<ChatMessage> = emptyList()
)

data class RoomState(
    val id: Long,
    val name: String,
    val isPublic: Boolean = false,
    val owner: String = "",
    val members: List<String> = emptyList(),
    val messages: List<ChatMessage> = emptyList()
)

data class UiState(
    val screen: Screen = Screen.LOGIN,
    val status: String = "",
    val isError: Boolean = false,
    val isLoading: Boolean = false,
    val messages: List<ChatMessage> = emptyList(),
    val onlineUsers: List<String> = emptyList(),
    val myUsername: String = "",
    val config: AppConfig = AppConfig(),
    val knownServers: List<String> = emptyList(),
    val dmConversations: Map<String, List<ChatMessage>> = emptyMap(),
    val rooms: Map<Long, RoomState> = emptyMap(),
    val currentDMPartner: String = "",
    val currentRoomId: Long = 0L,
    val unreadDMs: Map<String, Int> = emptyMap(),
    val unreadRooms: Map<Long, Int> = emptyMap(),
    val pendingRoomJoin: Long = 0L  // Room ID we're trying to join
)

class MessengerViewModel(app: Application) : AndroidViewModel(app) {

    private val prefs = app.getSharedPreferences("messenger", Context.MODE_PRIVATE)
    private val _state = MutableStateFlow(UiState(config = loadConfig()))
    val state: StateFlow<UiState> = _state

    private var service: MessengerService? = null
    private var bound = false

    private val connection = object : ServiceConnection {
        override fun onServiceConnected(name: ComponentName, binder: IBinder) {
            service = (binder as MessengerService.LocalBinder).getService()
            bound = true
            viewModelScope.launch {
                service!!.events.collect { event ->
                    handleEvent(event)
                }
            }
        }
        override fun onServiceDisconnected(name: ComponentName) {
            bound = false
            service = null
        }
    }

    init {
        startAndBindService()
    }

    private fun startAndBindService() {
        val ctx = getApplication<Application>()
        val intent = Intent(ctx, MessengerService::class.java)
        if (Build.VERSION.SDK_INT >= Build.VERSION_CODES.O) {
            ctx.startForegroundService(intent)
        } else {
            ctx.startService(intent)
        }
        ctx.bindService(intent, connection, 0)
    }

    // ---- Config ----
    fun loadConfig(): AppConfig {
        val json = prefs.getString("config", null) ?: return AppConfig()
        return try {
            val j = JSONObject(json)
            AppConfig(
                serverAddr = j.optString("server_addr", "94.103.169.82:9999"),
                proxyAddr  = j.optString("proxy_addr",  "127.0.0.1:18000"),
                directMode = j.optBoolean("direct_mode", false)
            )
        } catch (_: Exception) { AppConfig() }
    }

    fun saveConfig(cfg: AppConfig) {
        val j = JSONObject()
        j.put("server_addr", cfg.serverAddr)
        j.put("proxy_addr",  cfg.proxyAddr)
        j.put("direct_mode", cfg.directMode)
        prefs.edit().putString("config", j.toString()).apply()
        _state.value = _state.value.copy(config = cfg)
    }

    // ---- Register ----
    fun register(login: String, pass: String) {
        val ctx = getApplication<Application>()
        if (login.isBlank() || pass.isBlank()) {
            setStatus(ctx.getString(R.string.error_fill_all_fields), error = true); return
        }
        viewModelScope.launch {
            setStatus(ctx.getString(R.string.status_connecting), loading = true)
            val c = MessengerClient()
            val connResult = withTimeoutOrNull(12_000) { c.connect(_state.value.config) }
            if (connResult == null || connResult.isFailure) {
                c.destroy()
                setStatus(ctx.getString(R.string.error_connection_failed, connResult?.exceptionOrNull()?.message ?: ctx.getString(R.string.error_connection_timeout)), error = true)
                return@launch
            }
            val regResult = withTimeoutOrNull(6_000) { c.register(login, pass) }
            c.destroy()
            when {
                regResult == null -> setStatus(ctx.getString(R.string.error_registration_timeout), error = true)
                regResult.getOrNull() == true -> setStatus(ctx.getString(R.string.success_account_created))
                else -> setStatus(ctx.getString(R.string.error_username_taken), error = true)
            }
        }
    }

    // ---- Login ----
    fun login(login: String, pass: String) {
        val ctx = getApplication<Application>()
        if (login.isBlank() || pass.isBlank()) {
            setStatus(ctx.getString(R.string.error_fill_all_fields), error = true); return
        }
        viewModelScope.launch {
            // Wait up to 3s for service to bind
            val svc = waitForService() ?: run {
                setStatus(ctx.getString(R.string.error_service_not_ready), error = true); return@launch
            }

            // Clear previous session state before connecting — avoids the race where
            // OnlineList/History events arrive and are processed by handleEvent BEFORE
            // the login-success block runs, causing them to be wiped by emptyList().
            _state.value = _state.value.copy(
                messages = emptyList(),
                onlineUsers = emptyList(),
                status = ctx.getString(R.string.status_connecting),
                isLoading = true,
                isError = false
            )
            val connResult = withTimeoutOrNull(12_000) { svc.connect(_state.value.config) }
            if (connResult == null || connResult.isFailure) {
                setStatus(ctx.getString(R.string.error_connection_failed, connResult?.exceptionOrNull()?.message ?: ctx.getString(R.string.error_connection_timeout)), error = true)
                return@launch
            }

            setStatus(ctx.getString(R.string.status_authorizing), loading = true)
            // login() has its own 8s timeout internally
            val loginResult = svc.login(login, pass)
            if (loginResult.isFailure) {
                val msg = loginResult.exceptionOrNull()?.message ?: ""
                val text = if (msg == "Invalid credentials") ctx.getString(R.string.error_invalid_credentials)
                           else ctx.getString(R.string.error_login_failed, msg)
                setStatus(text, error = true)
                return@launch
            }

            svc.setUsername(login)
            _state.value = _state.value.copy(
                screen = Screen.CHAT,
                myUsername = login,
                status = "",
                isLoading = false,
                isError = false
            )
        }
    }

    // Wait up to 3 seconds for service to bind
    private suspend fun waitForService(): MessengerService? {
        if (service != null) return service
        repeat(30) {
            kotlinx.coroutines.delay(100)
            if (service != null) return service
        }
        return null
    }

    // ---- Send message ----
    fun sendMessage(text: String) {
        val trimmed = text.trim()
        if (trimmed.isEmpty()) return
        viewModelScope.launch {
            service?.sendMessage(trimmed)
            val now = java.text.SimpleDateFormat("HH:mm", java.util.Locale.getDefault())
                .format(java.util.Date())
            _state.value = _state.value.copy(
                messages = _state.value.messages + ChatMessage(
                    sender = _state.value.myUsername, text = trimmed, time = now, own = true
                )
            )
        }
    }

    // ---- Send DM ----
    fun sendDM(recipientLogin: String, text: String) {
        val trimmed = text.trim()
        if (trimmed.isEmpty()) return
        viewModelScope.launch {
            service?.sendDM(recipientLogin, trimmed)
            val now = java.text.SimpleDateFormat("HH:mm", java.util.Locale.getDefault())
                .format(java.util.Date())
            val msg = ChatMessage(sender = _state.value.myUsername, text = trimmed, time = now, own = true)
            val updated = _state.value.dmConversations.toMutableMap()
            updated[recipientLogin] = (updated[recipientLogin] ?: emptyList()) + msg
            _state.value = _state.value.copy(dmConversations = updated)
        }
    }

    // ---- Room actions ----
    fun createRoom(name: String, isPublic: Boolean, description: String = "") {
        viewModelScope.launch {
            service?.createRoom(name, isPublic, description)
        }
    }

    fun joinRoom(roomId: Long) {
        viewModelScope.launch {
            service?.joinRoom(roomId)
        }
    }
    
    fun leaveRoom(roomId: Long) {
        viewModelScope.launch {
            service?.leaveRoom(roomId)
        }
    }
    
    fun inviteToRoom(roomId: Long, username: String) {
        viewModelScope.launch {
            service?.inviteToRoom(roomId, username)
        }
    }

    fun sendRoomMessage(roomId: Long, text: String) {
        val trimmed = text.trim()
        if (trimmed.isEmpty()) return
        viewModelScope.launch {
            service?.sendRoomMessage(roomId, trimmed)
            val now = java.text.SimpleDateFormat("HH:mm", java.util.Locale.getDefault())
                .format(java.util.Date())
            val msg = ChatMessage(sender = _state.value.myUsername, text = trimmed, time = now, own = true)
            val updatedRooms = _state.value.rooms.toMutableMap()
            val room = updatedRooms[roomId]
            if (room != null) updatedRooms[roomId] = room.copy(messages = room.messages + msg)
            _state.value = _state.value.copy(rooms = updatedRooms)
        }
    }

    // ---- Logout ----
    fun logout() {
        service?.stopConnection()
        _state.value = _state.value.copy(
            screen = Screen.LOGIN,
            messages = emptyList(),
            onlineUsers = emptyList(),
            myUsername = "",
            status = "",
            isLoading = false,
            dmConversations = emptyMap(),
            rooms = emptyMap(),
            currentDMPartner = "",
            currentRoomId = 0L,
            unreadDMs = emptyMap(),
            unreadRooms = emptyMap(),
            pendingRoomJoin = 0L
        )
        startAndBindService()
    }

    // ---- Navigation ----
    fun switchToGlobalChat() {
        _state.value = _state.value.copy(screen = Screen.CHAT, currentDMPartner = "", currentRoomId = 0L)
    }

    fun switchToDM(partner: String) {
        val updated = _state.value.unreadDMs.toMutableMap()
        updated.remove(partner)
        _state.value = _state.value.copy(
            screen = Screen.DM,
            currentDMPartner = partner,
            currentRoomId = 0L,
            unreadDMs = updated
        )
    }

    fun switchToRoom(roomId: Long) {
        val updated = _state.value.unreadRooms.toMutableMap()
        updated.remove(roomId)
        
        // Check if room exists in state
        val room = _state.value.rooms[roomId]
        if (room == null) {
            // Room not in state yet, mark as pending and join
            _state.value = _state.value.copy(
                pendingRoomJoin = roomId,
                unreadRooms = updated
            )
            joinRoom(roomId)
            return
        }
        
        _state.value = _state.value.copy(
            screen = Screen.ROOM,
            currentDMPartner = "",
            currentRoomId = roomId,
            unreadRooms = updated,
            pendingRoomJoin = 0L
        )
        
        // Auto-join public rooms if not a member
        if (room.isPublic && !room.members.contains(_state.value.myUsername)) {
            joinRoom(roomId)
        }
    }

    // ---- Handle events ----
    private fun handleEvent(event: ServerEvent) {
        when (event) {
            is ServerEvent.History -> {
                val msg = event.msg.copy(own = event.msg.sender == _state.value.myUsername)
                _state.value = _state.value.copy(messages = _state.value.messages + msg)
            }
            is ServerEvent.HistoryEnd -> {}
            is ServerEvent.LoginOk -> {}
            is ServerEvent.LoginFail -> {}
            is ServerEvent.Message -> {
                val msg = event.msg.copy(own = event.msg.sender == _state.value.myUsername)
                _state.value = _state.value.copy(messages = _state.value.messages + msg)
            }
            is ServerEvent.OnlineList -> {
                _state.value = _state.value.copy(onlineUsers = event.users)
            }
            is ServerEvent.OnlineAdd -> {
                val updated = (_state.value.onlineUsers + event.name).distinct()
                _state.value = _state.value.copy(onlineUsers = updated)
            }
            is ServerEvent.OnlineRemove -> {
                if (event.name.isNotEmpty()) {
                    _state.value = _state.value.copy(
                        onlineUsers = _state.value.onlineUsers.filter { it != event.name }
                    )
                }
            }
            is ServerEvent.ServerList -> {
                _state.value = _state.value.copy(knownServers = event.addrs)
            }
            is ServerEvent.Disconnected -> {
                if (_state.value.screen == Screen.CHAT) {
                    val ctx = getApplication<Application>()
                    _state.value = _state.value.copy(
                        screen = Screen.LOGIN,
                        status = ctx.getString(R.string.status_disconnected),
                        isError = true,
                        isLoading = false,
                        messages = emptyList(),
                        onlineUsers = emptyList()
                    )
                }
                startAndBindService()
            }
            // DM events
            is ServerEvent.DMMessage -> {
                val updated = _state.value.dmConversations.toMutableMap()
                val now = java.text.SimpleDateFormat("HH:mm", java.util.Locale.getDefault()).format(java.util.Date())
                val msg = ChatMessage(sender = event.sender, text = event.text, time = event.time, own = false)
                updated[event.sender] = (updated[event.sender] ?: emptyList()) + msg
                
                // Increment unread if not viewing this DM
                val unread = if (_state.value.screen != Screen.DM || _state.value.currentDMPartner != event.sender) {
                    val counts = _state.value.unreadDMs.toMutableMap()
                    counts[event.sender] = (counts[event.sender] ?: 0) + 1
                    counts
                } else _state.value.unreadDMs
                
                _state.value = _state.value.copy(dmConversations = updated, unreadDMs = unread)
            }
            is ServerEvent.DMHistory -> {
                val partner = if (event.sender == _state.value.myUsername) event.recipient else event.sender
                val updated = _state.value.dmConversations.toMutableMap()
                val own = event.sender == _state.value.myUsername
                val msg = ChatMessage(sender = event.sender, text = event.text, time = event.time, own = own)
                updated[partner] = (updated[partner] ?: emptyList()) + msg
                _state.value = _state.value.copy(dmConversations = updated)
            }
            // Room events
            is ServerEvent.RoomList -> {
                val updatedRooms = _state.value.rooms.toMutableMap()
                for (info in event.rooms) {
                    if (!updatedRooms.containsKey(info.id)) {
                        updatedRooms[info.id] = RoomState(info.id, info.name, info.isPublic, info.owner)
                    }
                }
                _state.value = _state.value.copy(rooms = updatedRooms)
            }
            is ServerEvent.RoomInfo -> {
                val updatedRooms = _state.value.rooms.toMutableMap()
                val existing = updatedRooms[event.id]
                updatedRooms[event.id] = existing?.copy(name = event.name, isPublic = event.isPublic, owner = event.owner)
                    ?: RoomState(event.id, event.name, event.isPublic, event.owner)
                
                // Check if this is the room we're waiting to join
                val shouldSwitch = _state.value.pendingRoomJoin == event.id || 
                                  (event.owner == _state.value.myUsername && existing == null)
                
                _state.value = _state.value.copy(
                    rooms = updatedRooms,
                    screen = if (shouldSwitch) Screen.ROOM else _state.value.screen,
                    currentRoomId = if (shouldSwitch) event.id else _state.value.currentRoomId,
                    pendingRoomJoin = if (shouldSwitch) 0L else _state.value.pendingRoomJoin
                )
            }
            is ServerEvent.RoomMembers -> {
                val updatedRooms = _state.value.rooms.toMutableMap()
                val room = updatedRooms[event.roomId]
                if (room != null) updatedRooms[event.roomId] = room.copy(members = event.members)
                _state.value = _state.value.copy(rooms = updatedRooms)
            }
            is ServerEvent.RoomMemberAdd -> {
                val updatedRooms = _state.value.rooms.toMutableMap()
                val room = updatedRooms[event.roomId]
                if (room != null && !room.members.contains(event.login)) {
                    updatedRooms[event.roomId] = room.copy(members = room.members + event.login)
                }
                _state.value = _state.value.copy(rooms = updatedRooms)
            }
            is ServerEvent.RoomMemberRem -> {
                val updatedRooms = _state.value.rooms.toMutableMap()
                val room = updatedRooms[event.roomId]
                if (room != null) {
                    if (event.login == _state.value.myUsername) {
                        // We were removed from the room
                        updatedRooms.remove(event.roomId)
                        val newState = _state.value.copy(rooms = updatedRooms)
                        _state.value = if (_state.value.screen == Screen.ROOM && _state.value.currentRoomId == event.roomId) {
                            newState.copy(screen = Screen.CHAT, currentRoomId = 0L)
                        } else {
                            newState
                        }
                    } else {
                        updatedRooms[event.roomId] = room.copy(members = room.members.filter { it != event.login })
                        _state.value = _state.value.copy(rooms = updatedRooms)
                    }
                }
            }
            is ServerEvent.RoomMessage -> {
                val updatedRooms = _state.value.rooms.toMutableMap()
                val room = updatedRooms[event.roomId]
                if (room != null) {
                    val own = event.sender == _state.value.myUsername
                    val msg = ChatMessage(sender = event.sender, text = event.text, time = event.time, own = own)
                    updatedRooms[event.roomId] = room.copy(messages = room.messages + msg)
                    
                    // Increment unread if not viewing this room
                    val unread = if (_state.value.screen != Screen.ROOM || _state.value.currentRoomId != event.roomId) {
                        val counts = _state.value.unreadRooms.toMutableMap()
                        counts[event.roomId] = (counts[event.roomId] ?: 0) + 1
                        counts
                    } else _state.value.unreadRooms
                    
                    _state.value = _state.value.copy(rooms = updatedRooms, unreadRooms = unread)
                }
            }
            is ServerEvent.RoomHistoryMsg -> {
                val updatedRooms = _state.value.rooms.toMutableMap()
                val room = updatedRooms[event.roomId]
                if (room != null) {
                    val own = event.sender == _state.value.myUsername
                    val msg = ChatMessage(sender = event.sender, text = event.text, time = event.time, own = own)
                    updatedRooms[event.roomId] = room.copy(messages = room.messages + msg)
                }
                _state.value = _state.value.copy(rooms = updatedRooms)
            }
        }
    }

    private fun setStatus(msg: String, error: Boolean = false, loading: Boolean = false) {
        _state.value = _state.value.copy(status = msg, isError = error, isLoading = loading)
    }

    override fun onCleared() {
        super.onCleared()
        if (bound) {
            getApplication<Application>().unbindService(connection)
            bound = false
        }
    }
}
