package com.yunkiro.dialer.service

import android.Manifest
import android.content.Context
import android.content.Intent
import android.content.pm.PackageManager
import android.net.Uri
import android.os.Build
import android.telecom.PhoneAccountHandle
import android.telecom.TelecomManager
import android.telephony.SubscriptionManager
import android.util.Log
import androidx.core.content.ContextCompat
import com.yunkiro.dialer.relay.Protocol

/**
 * Handles dial commands by initiating phone calls via Intent.ACTION_CALL.
 * Supports dual-SIM selection via PhoneAccountHandle.
 */
class DialService(private val context: Context) {

    companion object {
        private const val TAG = "DialService"
    }

    /**
     * Execute a dial command. Returns a DialResult indicating success or failure.
     */
    fun executeDial(command: Protocol.DialCommand): Protocol.DialResult {
        // Check permission
        if (ContextCompat.checkSelfPermission(context, Manifest.permission.CALL_PHONE)
            != PackageManager.PERMISSION_GRANTED
        ) {
            return Protocol.DialResult(
                messageId = command.messageId,
                success = false,
                error = "CALL_PHONE permission not granted"
            )
        }

        return try {
            val intent = Intent(Intent.ACTION_CALL).apply {
                data = Uri.parse("tel:${command.phoneNumber}")
                flags = Intent.FLAG_ACTIVITY_NEW_TASK

                // Handle dual-SIM selection
                val phoneAccountHandle = getPhoneAccountHandle(command.simSlot)
                if (phoneAccountHandle != null) {
                    putExtra("android.telecom.extra.PHONE_ACCOUNT_HANDLE", phoneAccountHandle)
                }
            }

            context.startActivity(intent)
            Log.i(TAG, "Dial initiated: ${command.phoneNumber} on SIM slot ${command.simSlot}")

            Protocol.DialResult(
                messageId = command.messageId,
                success = true,
                error = ""
            )
        } catch (e: Exception) {
            Log.e(TAG, "Dial failed: ${e.message}")
            Protocol.DialResult(
                messageId = command.messageId,
                success = false,
                error = e.message ?: "Unknown error"
            )
        }
    }

    /**
     * Get the PhoneAccountHandle for the specified SIM slot.
     * Returns null if the slot is invalid or unavailable.
     */
    private fun getPhoneAccountHandle(simSlot: Int): PhoneAccountHandle? {
        if (ContextCompat.checkSelfPermission(context, Manifest.permission.READ_PHONE_STATE)
            != PackageManager.PERMISSION_GRANTED
        ) {
            return null
        }

        return try {
            val telecomManager = context.getSystemService(Context.TELECOM_SERVICE) as TelecomManager
            val accounts = telecomManager.callCapablePhoneAccounts

            if (simSlot in accounts.indices) {
                accounts[simSlot]
            } else {
                null
            }
        } catch (e: Exception) {
            Log.w(TAG, "Failed to get phone account for slot $simSlot: ${e.message}")
            null
        }
    }

    /**
     * Execute a hangup command by ending the current call.
     */
    fun executeHangup(command: Protocol.HangupCommand): Boolean {
        return try {
            if (Build.VERSION.SDK_INT >= Build.VERSION_CODES.P) {
                val telecomManager = context.getSystemService(Context.TELECOM_SERVICE) as TelecomManager
                telecomManager.endCall()
            } else {
                false
            }
        } catch (e: Exception) {
            Log.e(TAG, "Hangup failed: ${e.message}")
            false
        }
    }

    /**
     * Send an SMS message.
     */
    fun executeSms(command: Protocol.SmsCommand): Protocol.SmsResult {
        if (ContextCompat.checkSelfPermission(context, Manifest.permission.SEND_SMS)
            != PackageManager.PERMISSION_GRANTED
        ) {
            return Protocol.SmsResult(
                messageId = command.messageId,
                success = false,
                error = "SEND_SMS permission not granted"
            )
        }

        return try {
            val smsManager = if (Build.VERSION.SDK_INT >= Build.VERSION_CODES.S) {
                context.getSystemService(android.telephony.SmsManager::class.java)
            } else {
                @Suppress("DEPRECATION")
                android.telephony.SmsManager.getDefault()
            }

            smsManager.sendTextMessage(
                command.phoneNumber,
                null,
                command.content,
                null,
                null
            )

            Log.i(TAG, "SMS sent to: ${command.phoneNumber}")
            Protocol.SmsResult(
                messageId = command.messageId,
                success = true,
                error = ""
            )
        } catch (e: Exception) {
            Log.e(TAG, "SMS failed: ${e.message}")
            Protocol.SmsResult(
                messageId = command.messageId,
                success = false,
                error = e.message ?: "Unknown error"
            )
        }
    }
}
