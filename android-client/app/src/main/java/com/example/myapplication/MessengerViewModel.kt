package com.example.myapplication

import android.app.Application
import android.content.Context
import androidx.lifecycle.AndroidViewModel
import androidx.lifecycle.viewModelScope
import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.Job
import kotlinx.coroutines.flow.MutableStateFlow
import kotlinx.coroutines.flow.StateFlow
import kotlinx.coroutines.launch
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

    private var client: MessengerClient? = null
    private var readJob: Job? = null

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
            val connResult = c.connect(_state.value.config)
            if (connResult.isFailure) {
                c.destroy()
                setStatus("Ошибка подключения: ${connResult.exceptionOrNull()?.message}", error = true)
                return@launch
            }
            val regResult = c.register(login, pass)
            c.destroy()
            if (regResult.isSuccess && regResult.getOrNull() == true) {
                setStatus("Аккаунт создан! Теперь войдите.")
            } else {
                setStatus("Логин уже занят", error = true)
            }
        }
    }

    // ---- Login ----
    fun login(login: String, pass: String) {
        if (login.isBlank() || pass.isBlank()) {
            setStatus("Заполните все поля", error = true); return
        }
        viewModelScope.launch {
            setStatus("Подключение...", loading = true)
            val c = MessengerClient()
            val connResult = c.connect(_state.value.config)
            if (connResult.isFailure) {
                c.destroy()
                setStatus("Ошибка подключения: ${connResult.exceptionOrNull()?.message}", error = true)
                return@launch
            }
            setStatus("Авторизация...", loading = true)
            val loginResult = c.login(login, pass)
            if (loginResult.isFailure) {
                c.destroy()
                setStatus("Неверный логин или пароль", error = true)
                return@launch
            }
            client = c
            _state.value = _state.value.copy(
                screen = Screen.CHAT,
                myUsername = login,
                messages = emptyList(),
                onlineUsers = emptyList(),
                status = "",
                isLoading = false,
                isError = false
            )
            startReadLoop()
        }
    }

    // ---- Send message ----
    fun sendMessage(text: String) {
        val trimmed = text.trim()
        if (trimmed.isEmpty()) return
        viewModelScope.launch {
            client?.sendMessage(trimmed)
            val now = java.text.SimpleDateFormat("HH:mm", java.util.Locale.getDefault())
                .format(java.util.Date())
            val msg = ChatMessage(
                sender = _state.value.myUsername,
                text   = trimmed,
                time   = now,
                own    = true
            )
            _state.value = _state.value.copy(messages = _state.value.messages + msg)
        }
    }

    // ---- Logout ----
    fun logout() {
        readJob?.cancel()
        client?.destroy()
        client = null
        _state.value = _state.value.copy(
            screen = Screen.LOGIN,
            messages = emptyList(),
            onlineUsers = emptyList(),
            myUsername = "",
            status = ""
        )
    }

    // ---- Read loop ----
    private fun startReadLoop() {
        readJob?.cancel()
        readJob = viewModelScope.launch(Dispatchers.IO) {
            val c = client ?: return@launch
            while (true) {
                val event = c.readEvent() ?: continue
                when (event) {
                    is ServerEvent.History -> {
                        val msg = event.msg.copy(own = event.msg.sender == _state.value.myUsername)
                        _state.value = _state.value.copy(messages = _state.value.messages + msg)
                    }
                    is ServerEvent.HistoryEnd -> {
                        // history loaded — no special action needed
                    }
                    is ServerEvent.Message -> {
                        val msg = event.msg.copy(own = event.msg.sender == _state.value.myUsername)
                        _state.value = _state.value.copy(messages = _state.value.messages + msg)
                    }
                    is ServerEvent.OnlineList -> {
                        _state.value = _state.value.copy(onlineUsers = event.users)
                    }
                    is ServerEvent.Disconnected -> {
                        _state.value = _state.value.copy(
                            screen = Screen.LOGIN,
                            status = "Соединение разорвано",
                            isError = true,
                            messages = emptyList(),
                            onlineUsers = emptyList()
                        )
                        client = null
                        break
                    }
                }
            }
        }
    }

    private fun setStatus(msg: String, error: Boolean = false, loading: Boolean = false) {
        _state.value = _state.value.copy(status = msg, isError = error, isLoading = loading)
    }

    override fun onCleared() {
        super.onCleared()
        client?.destroy()
    }
}
