package com.yunkiro.dialer.receiver

import android.content.BroadcastReceiver
import android.content.Context
import android.content.Intent
import android.net.ConnectivityManager
import android.util.Log

/**
 * Monitors network connectivity changes and can trigger relay reconnection.
 * Note: For Android 7+, the preferred approach is using ConnectivityManager.NetworkCallback
 * (done in KeepAliveService). This receiver is a fallback for explicit broadcasts.
 */
class NetworkChangeReceiver : BroadcastReceiver() {

    companion object {
        private const val TAG = "NetworkChangeReceiver"
    }

    var onNetworkAvailable: (() -> Unit)? = null

    override fun onReceive(context: Context, intent: Intent) {
        val connectivityManager =
            context.getSystemService(Context.CONNECTIVITY_SERVICE) as ConnectivityManager
        val activeNetwork = connectivityManager.activeNetworkInfo

        if (activeNetwork != null && activeNetwork.isConnected) {
            Log.i(TAG, "Network connected: ${activeNetwork.typeName}")
            onNetworkAvailable?.invoke()
        } else {
            Log.i(TAG, "Network disconnected")
        }
    }
}
