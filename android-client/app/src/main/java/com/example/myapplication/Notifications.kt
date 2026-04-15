package com.example.myapplication

import android.app.NotificationChannel
import android.app.NotificationManager
import android.content.Context
import android.os.Build
import androidx.core.app.NotificationCompat
import androidx.core.app.NotificationManagerCompat

object Notifications {
    const val SERVICE_CHANNEL_ID = "service"
    private const val MSG_CHANNEL_ID = "messages"
    private var notifId = 10 // start from 10 to avoid collision with service notif

    fun createChannel(context: Context) {
        if (Build.VERSION.SDK_INT >= Build.VERSION_CODES.O) {
            val nm = context.getSystemService(NotificationManager::class.java)
            nm.createNotificationChannel(
                NotificationChannel(MSG_CHANNEL_ID, "Сообщения", NotificationManager.IMPORTANCE_HIGH)
                    .apply { description = "Входящие сообщения" }
            )
            nm.createNotificationChannel(
                NotificationChannel(SERVICE_CHANNEL_ID, "Фоновое соединение", NotificationManager.IMPORTANCE_LOW)
                    .apply { description = "Поддержание соединения с сервером" }
            )
        }
    }

    fun show(context: Context, sender: String, text: String) {
        val notif = NotificationCompat.Builder(context, MSG_CHANNEL_ID)
            .setSmallIcon(android.R.drawable.ic_dialog_email)
            .setContentTitle(sender)
            .setContentText(text)
            .setPriority(NotificationCompat.PRIORITY_HIGH)
            .setAutoCancel(true)
            .build()
        try {
            NotificationManagerCompat.from(context).notify(notifId++, notif)
        } catch (_: SecurityException) {}
    }
}
