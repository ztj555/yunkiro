package com.yunkiro.dialer.service

import android.app.Notification
import android.app.PendingIntent
import android.app.Service
import android.content.Context
import android.content.Intent
import android.net.ConnectivityManager
import android.net.Network
import android.net.NetworkCapabilities
import android.net.NetworkRequest
import android.os.IBinder
import android.os.PowerManager
import android.util.Log
import androidx.core.app.NotificationCompat
import com.yunkiro.dialer.R
import com.yunkiro.dialer.YunKiroApp
import com.yunkiro.dialer.relay.MessageHandler
import com.yunkiro.dialer.relay.Protocol
import com.yunkiro.dialer.relay.RelayConnection
import com.yunkiro.dialer.ui.MainActivity
import com.yunkiro.dialer.util.DeviceInfo

/**
 * Foreground service that maintains the relay connection in the background.
 * Shows a persistent notification with connection status.
 */
class KeepAliveService : Service() {

    companion object {
        private const val TAG = "KeepAliveService"
        private const val NOTIFICATION_ID = 1
        private const val WAKELOCK_TAG = "YunKiro:KeepAlive"

        const val ACTION_CONNECT = "com.yunkiro.dialer.action.CONNECT"
        const val ACTION_DISCONNECT = "com.yunkiro.dialer.action.DISCONNECT"
        const val EXTRA_RELAY_URL = "relay_url"
        const val EXTRA_PIN = "pin"

        fun startService(context: Context, relayUrl: String, pin: String) {
            val intent = Intent(context, KeepAliveService::class.java).apply {
                action = ACTION_CONNECT
                putExtra(EXTRA_RELAY_URL, relayUrl)
                putExtra(EXTRA_PIN, pin)
            }
            context.startForegroundService(intent)
        }

        fun stopService(context: Context) {
            val intent = Intent(context, KeepAliveService::class.java).apply {
                action = ACTION_DISCONNECT
            }
            context.startService(intent)
        }
    }

    private var relayConnection: RelayConnection? = null
    private var dialService: DialService? = null
    private var wakeLock: PowerManager.WakeLock? = null
    private var networkCallback: ConnectivityManager.NetworkCallback? = null

    override fun onBind(intent: Intent?): IBinder? = null

    override fun onCreate() {
        super.onCreate()
        dialService = DialService(this)
    }

    override fun onStartCommand(intent: Intent?, flags: Int, startId: Int): Int {
        when (intent?.action) {
            ACTION_CONNECT -> {
                val url = intent.getStringExtra(EXTRA_RELAY_URL) ?: return START_NOT_STICKY
                val pin = intent.getStringExtra(EXTRA_PIN) ?: return START_NOT_STICKY

                startForeground(NOTIFICATION_ID, createNotification("Connecting..."))
                connect(url, pin)
            }
            ACTION_DISCONNECT -> {
                disconnect()
                stopForeground(STOP_FOREGROUND_REMOVE)
                stopSelf()
            }
            else -> {
                startForeground(NOTIFICATION_ID, createNotification("Idle"))
            }
        }
        return START_STICKY
    }

    private fun connect(url: String, pin: String) {
        val deviceId = DeviceInfo.getDeviceId(this)
        val deviceName = DeviceInfo.getDeviceName()

        val messageHandler = MessageHandler(
            onDial = { cmd -> handleDial(cmd) },
            onHangup = { cmd -> handleHangup(cmd) },
            onSms = { cmd -> handleSms(cmd) },
            onAuthResult = { result -> handleAuthResult(result) },
            onPing = { relayConnection?.send(Protocol.Pong.toJson()) },
            sendAck = { messageId ->
                relayConnection?.send(Protocol.Ack(messageId).toJson())
            }
        )

        relayConnection = RelayConnection(messageHandler) { state ->
            Log.i(TAG, "Connection state: $state")
            updateNotification(state.name)
        }

        relayConnection?.connect(url, pin, deviceId, deviceName)
        registerNetworkCallback()
        acquireWakeLock()
    }

    private fun disconnect() {
        releaseWakeLock()
        unregisterNetworkCallback()
        relayConnection?.destroy()
        relayConnection = null
    }

    private fun handleDial(cmd: Protocol.DialCommand) {
        val result = dialService?.executeDial(cmd) ?: Protocol.DialResult(
            messageId = cmd.messageId,
            success = false,
            error = "Service not available"
        )
        relayConnection?.send(result.toJson())
    }

    private fun handleHangup(cmd: Protocol.HangupCommand) {
        dialService?.executeHangup(cmd)
    }

    private fun handleSms(cmd: Protocol.SmsCommand) {
        val result = dialService?.executeSms(cmd) ?: Protocol.SmsResult(
            messageId = cmd.messageId,
            success = false,
            error = "Service not available"
        )
        relayConnection?.send(result.toJson())
    }

    private fun handleAuthResult(result: Protocol.AuthResult) {
        if (result.success) {
            updateNotification("Connected")
        } else {
            updateNotification("Auth failed: ${result.message}")
        }
    }

    private fun createNotification(status: String): Notification {
        val pendingIntent = PendingIntent.getActivity(
            this,
            0,
            Intent(this, MainActivity::class.java),
            PendingIntent.FLAG_UPDATE_CURRENT or PendingIntent.FLAG_IMMUTABLE
        )

        return NotificationCompat.Builder(this, YunKiroApp.CHANNEL_ID)
            .setContentTitle("YunKiro Dialer")
            .setContentText(status)
            .setSmallIcon(R.drawable.ic_notification)
            .setContentIntent(pendingIntent)
            .setOngoing(true)
            .build()
    }

    private fun updateNotification(status: String) {
        val notification = createNotification(status)
        val manager = getSystemService(NOTIFICATION_SERVICE) as android.app.NotificationManager
        manager.notify(NOTIFICATION_ID, notification)
    }

    private fun acquireWakeLock() {
        val powerManager = getSystemService(POWER_SERVICE) as PowerManager
        wakeLock = powerManager.newWakeLock(
            PowerManager.PARTIAL_WAKE_LOCK,
            WAKELOCK_TAG
        ).apply {
            acquire() // No timeout - held for the lifetime of the foreground service
        }
    }

    private fun releaseWakeLock() {
        wakeLock?.let {
            if (it.isHeld) it.release()
        }
        wakeLock = null
    }

    private fun registerNetworkCallback() {
        val connectivityManager = getSystemService(CONNECTIVITY_SERVICE) as ConnectivityManager
        val request = NetworkRequest.Builder()
            .addCapability(NetworkCapabilities.NET_CAPABILITY_INTERNET)
            .build()

        networkCallback = object : ConnectivityManager.NetworkCallback() {
            override fun onAvailable(network: Network) {
                Log.i(TAG, "Network available, triggering reconnect")
                relayConnection?.reconnect()
            }

            override fun onLost(network: Network) {
                Log.i(TAG, "Network lost")
            }
        }

        connectivityManager.registerNetworkCallback(request, networkCallback!!)
    }

    private fun unregisterNetworkCallback() {
        networkCallback?.let {
            val connectivityManager = getSystemService(CONNECTIVITY_SERVICE) as ConnectivityManager
            connectivityManager.unregisterNetworkCallback(it)
        }
        networkCallback = null
    }

    override fun onDestroy() {
        disconnect()
        super.onDestroy()
    }
}
