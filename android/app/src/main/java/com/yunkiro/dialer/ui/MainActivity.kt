package com.yunkiro.dialer.ui

import android.Manifest
import android.content.SharedPreferences
import android.content.pm.PackageManager
import android.os.Build
import android.os.Bundle
import android.widget.Toast
import androidx.appcompat.app.AppCompatActivity
import androidx.core.app.ActivityCompat
import androidx.core.content.ContextCompat
import com.yunkiro.dialer.databinding.ActivityMainBinding
import com.yunkiro.dialer.service.KeepAliveService
import com.yunkiro.dialer.util.DeviceInfo

/**
 * Main activity providing PIN input, connection control, and status display.
 */
class MainActivity : AppCompatActivity() {

    companion object {
        private const val PREFS_NAME = "yunkiro_prefs"
        private const val KEY_RELAY_URL = "relay_url"
        private const val KEY_PIN = "pin"
        private const val KEY_AUTOSTART = "autostart"
        private const val PERMISSION_REQUEST_CODE = 100
    }

    private lateinit var binding: ActivityMainBinding
    private lateinit var prefs: SharedPreferences
    private var isConnected = false

    override fun onCreate(savedInstanceState: Bundle?) {
        super.onCreate(savedInstanceState)
        binding = ActivityMainBinding.inflate(layoutInflater)
        setContentView(binding.root)

        prefs = getSharedPreferences(PREFS_NAME, MODE_PRIVATE)

        setupUI()
        requestPermissions()
    }

    private fun setupUI() {
        // Load saved values
        binding.editRelayUrl.setText(prefs.getString(KEY_RELAY_URL, "ws://192.168.1.100:8080/ws"))
        binding.editPin.setText(prefs.getString(KEY_PIN, ""))

        // Display device info
        val deviceId = DeviceInfo.getDeviceId(this)
        val deviceName = DeviceInfo.getDeviceName()
        binding.textDeviceInfo.text = "Device: $deviceName\nID: ${deviceId.take(8)}..."

        // Connect/Disconnect button
        binding.buttonConnect.setOnClickListener {
            if (isConnected) {
                disconnect()
            } else {
                connect()
            }
        }

        updateStatus("Disconnected")
    }

    private fun connect() {
        val url = binding.editRelayUrl.text.toString().trim()
        val pin = binding.editPin.text.toString().trim()

        if (url.isBlank()) {
            Toast.makeText(this, "Please enter relay URL", Toast.LENGTH_SHORT).show()
            return
        }

        if (pin.length != 4 || !pin.all { it.isDigit() }) {
            Toast.makeText(this, "PIN must be 4 digits", Toast.LENGTH_SHORT).show()
            return
        }

        // Save settings (enable autostart so boot + sticky-restart reconnect)
        prefs.edit()
            .putString(KEY_RELAY_URL, url)
            .putString(KEY_PIN, pin)
            .putBoolean(KEY_AUTOSTART, true)
            .apply()

        // Start foreground service
        KeepAliveService.startService(this, url, pin)

        isConnected = true
        binding.buttonConnect.text = "Disconnect"
        updateStatus("Connecting...")
    }

    private fun disconnect() {
        // Disable autostart so a manual disconnect stays disconnected across
        // reboots / system restarts (v6 scenario 2 expectation).
        prefs.edit().putBoolean(KEY_AUTOSTART, false).apply()

        KeepAliveService.stopService(this)

        isConnected = false
        binding.buttonConnect.text = "Connect"
        updateStatus("Disconnected")
    }

    private fun updateStatus(status: String) {
        binding.textStatus.text = status
    }

    private fun requestPermissions() {
        val permissions = mutableListOf(
            Manifest.permission.CALL_PHONE,
            Manifest.permission.READ_PHONE_STATE,
            Manifest.permission.SEND_SMS
        )

        if (Build.VERSION.SDK_INT >= Build.VERSION_CODES.TIRAMISU) {
            permissions.add(Manifest.permission.POST_NOTIFICATIONS)
        }

        val needed = permissions.filter {
            ContextCompat.checkSelfPermission(this, it) != PackageManager.PERMISSION_GRANTED
        }

        if (needed.isNotEmpty()) {
            ActivityCompat.requestPermissions(this, needed.toTypedArray(), PERMISSION_REQUEST_CODE)
        }
    }
}
