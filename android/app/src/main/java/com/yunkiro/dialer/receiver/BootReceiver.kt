package com.yunkiro.dialer.receiver

import android.content.BroadcastReceiver
import android.content.Context
import android.content.Intent
import android.content.SharedPreferences
import android.util.Log
import com.yunkiro.dialer.service.KeepAliveService

/**
 * Starts the KeepAliveService when the device boots,
 * if autostart is enabled and connection settings are saved.
 */
class BootReceiver : BroadcastReceiver() {

    companion object {
        private const val TAG = "BootReceiver"
        private const val PREFS_NAME = "yunkiro_prefs"
        private const val KEY_RELAY_URL = "relay_url"
        private const val KEY_PIN = "pin"
        private const val KEY_AUTOSTART = "autostart"
    }

    override fun onReceive(context: Context, intent: Intent) {
        if (intent.action != Intent.ACTION_BOOT_COMPLETED) return

        val prefs = context.getSharedPreferences(PREFS_NAME, Context.MODE_PRIVATE)
        val autostart = prefs.getBoolean(KEY_AUTOSTART, false)

        if (!autostart) {
            Log.d(TAG, "Autostart disabled, skipping")
            return
        }

        val relayUrl = prefs.getString(KEY_RELAY_URL, null)
        val pin = prefs.getString(KEY_PIN, null)

        if (relayUrl.isNullOrBlank() || pin.isNullOrBlank()) {
            Log.w(TAG, "Missing relay URL or PIN, cannot autostart")
            return
        }

        Log.i(TAG, "Boot completed, starting KeepAliveService")
        KeepAliveService.startService(context, relayUrl, pin)
    }
}
