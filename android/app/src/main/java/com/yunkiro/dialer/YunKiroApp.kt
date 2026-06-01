package com.yunkiro.dialer

import android.app.Application
import android.app.NotificationChannel
import android.app.NotificationManager
import android.os.Build

class YunKiroApp : Application() {

    companion object {
        const val CHANNEL_ID = "yunkiro_keep_alive"
        const val CHANNEL_NAME = "Connection Service"
    }

    override fun onCreate() {
        super.onCreate()
        createNotificationChannel()
    }

    private fun createNotificationChannel() {
        if (Build.VERSION.SDK_INT >= Build.VERSION_CODES.O) {
            val channel = NotificationChannel(
                CHANNEL_ID,
                CHANNEL_NAME,
                NotificationManager.IMPORTANCE_LOW
            ).apply {
                description = "Keeps the relay connection alive"
                setShowBadge(false)
            }
            val manager = getSystemService(NotificationManager::class.java)
            manager.createNotificationChannel(channel)
        }
    }
}
