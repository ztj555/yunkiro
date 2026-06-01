package com.yunkiro.dialer.util

import android.content.Context
import android.os.Build
import java.util.UUID

/**
 * Utility for generating and persisting device identification.
 */
object DeviceInfo {

    private const val PREFS_NAME = "yunkiro_device"
    private const val KEY_DEVICE_ID = "device_id"

    /**
     * Get a stable device ID, generating and persisting one if needed.
     */
    fun getDeviceId(context: Context): String {
        val prefs = context.getSharedPreferences(PREFS_NAME, Context.MODE_PRIVATE)
        var deviceId = prefs.getString(KEY_DEVICE_ID, null)

        if (deviceId.isNullOrBlank()) {
            deviceId = UUID.randomUUID().toString()
            prefs.edit().putString(KEY_DEVICE_ID, deviceId).apply()
        }

        return deviceId
    }

    /**
     * Get a human-readable device name.
     */
    fun getDeviceName(): String {
        val manufacturer = Build.MANUFACTURER.replaceFirstChar { it.uppercase() }
        val model = Build.MODEL
        return if (model.startsWith(manufacturer, ignoreCase = true)) {
            model
        } else {
            "$manufacturer $model"
        }
    }
}
