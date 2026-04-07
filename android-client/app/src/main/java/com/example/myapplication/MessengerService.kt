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
 *
 * Запускается через startForegroundService() и живёт независимо от UI.
 * UI биндится к сервису и получает события через StateFlow.
 * START_STICKY гарантирует перезапуск системой если процесс убит.
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

    // ---- Lifecycle ----

    override fun onCreate() {
        super.onCreate()
        Notifications.createChannel(this)
        // Must call startForeground immediately in onCreate to avoid ANR on Android 8+
        startForeground(NOTIF_ID_SERVICE, buildServiceNotification())
    }

    override fun onStartCommand(intent: Intent?, flags: Int, startId: Int): Int {
        // START_STICKY: system restarts service after kill, intent will be null on restart
        return START_STICKY
    }

    override fun onBind(intent: Intent): IBinder = binder

    // Called when last client unbinds — do NOT stop, keep running
    override fun onUnbind(intent: Intent?): Boolean {
        return true // allow rebind
    }

    override fun onDestroy() {
        scope.cancel()
        client.destroy()
        super.onDestroy()
    }

    // ---- API called from ViewModel ----

    suspend fun connect(cfg: AppConfig): Result<Unit> = client.connect(cfg)

    suspend fun register(login: String, pass: String) = client.register(login, pass)

    suspend fun login(login: String, pass: String) = client.login(login, pass)

    suspend fun sendMessage(text: String) = client.sendMessage(text)

    fun startReadLoop(myUsername: String) {
        readJob?.cancel()
        readJob = scope.launch {
            while (isActive) {
                val event = client.readEvent() ?: continue
                _events.value = event
                if (event is ServerEvent.Message && event.msg.sender != myUsername) {
                    Notifications.show(applicationContext, event.msg.sender, event.msg.text)
                }
                if (event is ServerEvent.Disconnected) break
            }
        }
    }

    fun stopConnection() {
        readJob?.cancel()
        client.destroy()
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
