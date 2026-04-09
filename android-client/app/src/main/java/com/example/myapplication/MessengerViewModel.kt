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

enum class Screen { LOGIN, CHAT }

data class UiState(
    val screen: Screen = Screen.LOGIN,
    val status: String = "",
    val isError: Boolean = false,
    val isLoading: Boolean = false,
    val messages: List<ChatMessage> = emptyList(),
    val onlineUsers: List<String> = emptyList(),
    val myUsername: String = "",
    val config: AppConfig = AppConfig()
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
        if (login.isBlank() || pass.isBlank()) {
            setStatus("Заполните все поля", error = true); return
        }
        viewModelScope.launch {
            setStatus("Подключение...", loading = true)
            val c = MessengerClient()
            val connResult = withTimeoutOrNull(12_000) { c.connect(_state.value.config) }
            if (connResult == null || connResult.isFailure) {
                c.destroy()
                setStatus("Ошибка подключения: ${connResult?.exceptionOrNull()?.message ?: "таймаут"}", error = true)
                return@launch
            }
            val regResult = withTimeoutOrNull(6_000) { c.register(login, pass) }
            c.destroy()
            when {
                regResult == null -> setStatus("Таймаут регистрации", error = true)
                regResult.getOrNull() == true -> setStatus("Аккаунт создан! Теперь войдите.")
                else -> setStatus("Логин уже занят", error = true)
            }
        }
    }

    // ---- Login ----
    fun login(login: String, pass: String) {
        if (login.isBlank() || pass.isBlank()) {
            setStatus("Заполните все поля", error = true); return
        }
        viewModelScope.launch {
            // Wait up to 3s for service to bind
            val svc = waitForService() ?: run {
                setStatus("Сервис не готов, попробуйте снова", error = true); return@launch
            }

            setStatus("Подключение...", loading = true)
            val connResult = withTimeoutOrNull(12_000) { svc.connect(_state.value.config) }
            if (connResult == null || connResult.isFailure) {
                setStatus("Ошибка подключения: ${connResult?.exceptionOrNull()?.message ?: "таймаут"}", error = true)
                return@launch
            }

            setStatus("Авторизация...", loading = true)
            // login() has its own 8s timeout internally
            val loginResult = svc.login(login, pass)
            if (loginResult.isFailure) {
                setStatus("Неверный логин или пароль", error = true)
                return@launch
            }

            svc.setUsername(login)
            _state.value = _state.value.copy(
                screen = Screen.CHAT,
                myUsername = login,
                messages = emptyList(),
                onlineUsers = emptyList(),
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

    // ---- Logout ----
    fun logout() {
        service?.stopConnection()
        _state.value = _state.value.copy(
            screen = Screen.LOGIN,
            messages = emptyList(),
            onlineUsers = emptyList(),
            myUsername = "",
            status = "",
            isLoading = false
        )
        startAndBindService()
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
            is ServerEvent.Disconnected -> {
                if (_state.value.screen == Screen.CHAT) {
                    _state.value = _state.value.copy(
                        screen = Screen.LOGIN,
                        status = "Соединение разорвано",
                        isError = true,
                        isLoading = false,
                        messages = emptyList(),
                        onlineUsers = emptyList()
                    )
                }
                startAndBindService()
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
