package com.yunkiro.dialer.relay

import org.json.JSONObject

/**
 * Protocol data classes matching the relay WebSocket JSON protocol.
 */
object Protocol {

    // --- Outgoing messages (phone -> relay) ---

    data class AuthRequest(
        val pin: String,
        val role: String = "phone",
        val deviceId: String,
        val deviceName: String
    ) {
        fun toJson(): String {
            val obj = JSONObject()
            obj.put("type", "auth")
            obj.put("pin", pin)
            obj.put("role", role)
            obj.put("device_id", deviceId)
            obj.put("device_name", deviceName)
            return obj.toString()
        }
    }

    data class AuthResult(
        val success: Boolean,
        val message: String
    ) {
        companion object {
            fun fromJson(json: JSONObject): AuthResult {
                return AuthResult(
                    success = json.getBoolean("success"),
                    message = json.optString("message", "")
                )
            }
        }
    }

    data class DialCommand(
        val messageId: String,
        val phoneNumber: String,
        val deviceId: String,
        val simSlot: Int
    ) {
        companion object {
            fun fromJson(json: JSONObject): DialCommand {
                return DialCommand(
                    messageId = json.getString("message_id"),
                    phoneNumber = json.getString("phone_number"),
                    deviceId = json.getString("device_id"),
                    simSlot = json.optInt("sim_slot", 0)
                )
            }
        }
    }

    data class HangupCommand(
        val messageId: String,
        val deviceId: String
    ) {
        companion object {
            fun fromJson(json: JSONObject): HangupCommand {
                return HangupCommand(
                    messageId = json.getString("message_id"),
                    deviceId = json.getString("device_id")
                )
            }
        }
    }

    data class SmsCommand(
        val messageId: String,
        val phoneNumber: String,
        val content: String,
        val deviceId: String
    ) {
        companion object {
            fun fromJson(json: JSONObject): SmsCommand {
                return SmsCommand(
                    messageId = json.getString("message_id"),
                    phoneNumber = json.getString("phone_number"),
                    content = json.getString("content"),
                    deviceId = json.getString("device_id")
                )
            }
        }
    }

    data class DialResult(
        val messageId: String,
        val success: Boolean,
        val error: String = ""
    ) {
        fun toJson(): String {
            val obj = JSONObject()
            obj.put("type", "dial_result")
            obj.put("message_id", messageId)
            obj.put("success", success)
            obj.put("error", error)
            return obj.toString()
        }
    }

    data class SmsResult(
        val messageId: String,
        val success: Boolean,
        val error: String = ""
    ) {
        fun toJson(): String {
            val obj = JSONObject()
            obj.put("type", "sms_result")
            obj.put("message_id", messageId)
            obj.put("success", success)
            obj.put("error", error)
            return obj.toString()
        }
    }

    data class DeviceStatus(
        val deviceId: String,
        val status: String
    ) {
        fun toJson(): String {
            val obj = JSONObject()
            obj.put("type", "device_status")
            obj.put("device_id", deviceId)
            obj.put("status", status)
            return obj.toString()
        }
    }

    data class Ack(
        val messageId: String
    ) {
        fun toJson(): String {
            val obj = JSONObject()
            obj.put("type", "ack")
            obj.put("message_id", messageId)
            return obj.toString()
        }
    }

    object Ping {
        fun toJson(): String {
            val obj = JSONObject()
            obj.put("type", "ping")
            return obj.toString()
        }
    }

    object Pong {
        fun toJson(): String {
            val obj = JSONObject()
            obj.put("type", "pong")
            return obj.toString()
        }
    }

    /**
     * Parse a raw JSON string and return the "type" field value.
     */
    fun parseType(raw: String): String? {
        return try {
            JSONObject(raw).optString("type", null)
        } catch (e: Exception) {
            null
        }
    }

    /**
     * Parse raw JSON string into a JSONObject.
     */
    fun parseJson(raw: String): JSONObject? {
        return try {
            JSONObject(raw)
        } catch (e: Exception) {
            null
        }
    }
}
