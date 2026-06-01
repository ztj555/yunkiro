package com.yunkiro.dialer.relay

import android.util.Log
import org.json.JSONObject
import java.util.LinkedHashSet

/**
 * Parses incoming WebSocket messages, deduplicates by message_id,
 * and dispatches to appropriate handlers.
 */
class MessageHandler(
    private val onDial: (Protocol.DialCommand) -> Unit,
    private val onHangup: (Protocol.HangupCommand) -> Unit,
    private val onSms: (Protocol.SmsCommand) -> Unit,
    private val onAuthResult: (Protocol.AuthResult) -> Unit,
    private val onPing: () -> Unit,
    private val sendAck: (String) -> Unit
) {
    companion object {
        private const val TAG = "MessageHandler"
        private const val MAX_DEDUP_SIZE = 100
    }

    // LRU set for deduplication - maintains insertion order, oldest entries removed first
    private val processedIds = LinkedHashSet<String>()
    private val dedupLock = Any()

    private fun addProcessedId(id: String) {
        synchronized(dedupLock) {
            if (processedIds.size >= MAX_DEDUP_SIZE) {
                val first = processedIds.iterator().next()
                processedIds.remove(first)
            }
            processedIds.add(id)
        }
    }

    /**
     * Handle an incoming raw JSON message string.
     * Returns true if the message was processed, false if it was a duplicate or invalid.
     */
    fun handleMessage(raw: String): Boolean {
        val json = Protocol.parseJson(raw) ?: run {
            Log.w(TAG, "Invalid JSON received: $raw")
            return false
        }

        val type = json.optString("type", "") 
        if (type.isEmpty()) {
            Log.w(TAG, "Message missing type field")
            return false
        }

        return when (type) {
            "auth_result" -> {
                val result = Protocol.AuthResult.fromJson(json)
                onAuthResult(result)
                true
            }
            "dial" -> handleWithDedup(json) { obj ->
                val cmd = Protocol.DialCommand.fromJson(obj)
                onDial(cmd)
            }
            "hangup" -> handleWithDedup(json) { obj ->
                val cmd = Protocol.HangupCommand.fromJson(obj)
                onHangup(cmd)
            }
            "sms" -> handleWithDedup(json) { obj ->
                val cmd = Protocol.SmsCommand.fromJson(obj)
                onSms(cmd)
            }
            "ping" -> {
                onPing()
                true
            }
            "pong" -> {
                // Heartbeat response, no action needed
                true
            }
            "ack" -> {
                // Acknowledgment received, can be used for delivery confirmation
                true
            }
            else -> {
                Log.w(TAG, "Unknown message type: $type")
                false
            }
        }
    }

    private fun handleWithDedup(json: JSONObject, handler: (JSONObject) -> Unit): Boolean {
        val messageId = json.optString("message_id", "")
        if (messageId.isEmpty()) {
            Log.w(TAG, "Message missing message_id")
            return false
        }

        if (isDuplicate(messageId)) {
            Log.d(TAG, "Duplicate message ignored: $messageId")
            return false
        }

        markProcessed(messageId)
        sendAck(messageId)
        handler(json)
        return true
    }

    /**
     * Check if a message ID has already been processed.
     */
    fun isDuplicate(messageId: String): Boolean {
        synchronized(dedupLock) {
            return processedIds.contains(messageId)
        }
    }

    /**
     * Mark a message ID as processed.
     */
    fun markProcessed(messageId: String) {
        addProcessedId(messageId)
    }

    /**
     * Get the current number of tracked message IDs (for testing).
     */
    fun getProcessedCount(): Int {
        synchronized(dedupLock) {
            return processedIds.size
        }
    }
}
