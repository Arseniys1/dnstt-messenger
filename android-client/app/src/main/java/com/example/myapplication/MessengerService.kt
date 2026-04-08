package com.example.myapplication

import android.app.Service
import android.content.Intent
import android.os.Binder
import android.os.IBinder
import kotlinx.coroutines.*
import kotlinx.coroutines.flow.MutableSharedFlow
import kotlinx.coroutines.flow.SharedFlow

class MessengerService : Service() {

    inner class LocalBinder : Binder() {
        fun getService(): MessengerService = this@MessengerService
    }

    private val binder = LocalBinder()
    private val scope = CoroutineScope(Dispatchers.IO + SupervisorJob())

    val client = MessengerClient()

    // Relay client events to ViewModel
    private val _events = MutableSharedFlow<ServerEvent>(extraBufferCapacity = 64)
    val events: SharedFlow<ServerEvent> = _events

    private var relayJob: Job? = null
    private var myUsername: String = ""

    override fun onCreate() {
        super.onCreate()
        Notifications.createChannel(this)
        startForeground(NOTIF_ID_SERVICE, buildServiceNotification())
    }

    override fun onStartCommand(intent: Intent?, flags: Int, startId: Int): Int = START_STICKY

    override fun onBind(intent: Intent): IBinder = binder

    override fun onUnbind(intent: Intent?): Boolean = true // allow rebind

    override fun onDestroy() {
        scope.cancel()
        client.destroy()
        super.onDestroy()
    }

    // ---- API ----

    suspend fun connect(cfg: AppConfig): Result<Unit> = client.connect(cfg)

    suspend fun register(login: String, pass: String) = client.register(login, pass)

    suspend fun login(login: String, pass: String): Result<Int> {
        // Start reader before sending login so we don't miss the response
        client.startReader()
        startRelay()
        return client.login(login, pass)
    }

    suspend fun sendMessage(text: String) = client.sendMessage(text)

    fun setUsername(username: String) { myUsername = username }

    fun stopConnection() {
        relayJob?.cancel()
        client.destroy()
        stopForeground(STOP_FOREGROUND_REMOVE)
        stopSelf()
    }

    // ---- Relay events from client to ViewModel, show notifications ----
    private fun startRelay() {
        relayJob?.cancel()
        relayJob = scope.launch {
            client.events.collect { event ->
                _events.emit(event)
                if (event is ServerEvent.Message && event.msg.sender != myUsername) {
                    Notifications.show(applicationContext, event.msg.sender, event.msg.text)
                }
                if (event is ServerEvent.Disconnected) cancel()
            }
        }
    }

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
