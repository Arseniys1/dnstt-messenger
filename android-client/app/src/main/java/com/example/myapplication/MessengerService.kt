package com.example.myapplication

import android.app.Service
import android.content.Intent
import android.os.Binder
import android.os.IBinder
import kotlinx.coroutines.*
import kotlinx.coroutines.flow.MutableStateFlow
import kotlinx.coroutines.flow.StateFlow

/**
 * ForegroundService — держит TCP соединение живым когда приложение свёрнуто.
 * UI биндится к сервису и получает события через StateFlow.
 */
class MessengerService : Service() {

    inner class LocalBinder : Binder() {
        fun getService(): MessengerService = this@MessengerService
    }

    private val binder = LocalBinder()
    private val scope = CoroutineScope(Dispatchers.IO + SupervisorJob())

    val client = MessengerClient()

    private val _events = MutableStateFlow<ServerEvent?>(null)
    val events: StateFlow<ServerEvent?> = _events

    private var readJob: Job? = null
    private var isConnected = false

    // ---- Lifecycle ----

    override fun onCreate() {
        super.onCreate()
        Notifications.createChannel(this)
        startForeground(NOTIF_ID_SERVICE, buildServiceNotification())
    }

    override fun onBind(intent: Intent): IBinder = binder

    override fun onDestroy() {
        scope.cancel()
        client.destroy()
        super.onDestroy()
    }

    // ---- API called from ViewModel ----

    suspend fun connect(cfg: AppConfig): Result<Unit> {
        val result = client.connect(cfg)
        if (result.isSuccess) isConnected = true
        return result
    }

    suspend fun register(login: String, pass: String) = client.register(login, pass)

    suspend fun login(login: String, pass: String) = client.login(login, pass)

    suspend fun sendMessage(text: String) = client.sendMessage(text)

    fun startReadLoop(myUsername: String) {
        readJob?.cancel()
        readJob = scope.launch {
            while (isActive) {
                val event = client.readEvent() ?: continue
                _events.value = event
                // Show notification for incoming messages
                if (event is ServerEvent.Message && event.msg.sender != myUsername) {
                    Notifications.show(
                        applicationContext,
                        event.msg.sender,
                        event.msg.text
                    )
                }
                if (event is ServerEvent.Disconnected) {
                    isConnected = false
                    break
                }
            }
        }
    }

    fun stopConnection() {
        readJob?.cancel()
        client.destroy()
        isConnected = false
        stopForeground(STOP_FOREGROUND_REMOVE)
        stopSelf()
    }

    // ---- Foreground notification ----

    private fun buildServiceNotification() =
        androidx.core.app.NotificationCompat.Builder(this, Notifications.SERVICE_CHANNEL_ID)
            .setSmallIcon(android.R.drawable.ic_dialog_email)
            .setContentTitle("DNSTT Messenger")
            .setContentText("Подключено")
            .setPriority(androidx.core.app.NotificationCompat.PRIORITY_LOW)
            .setOngoing(true)
            .build()

    companion object {
        const val NOTIF_ID_SERVICE = 1
    }
}
