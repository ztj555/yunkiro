package com.yunkiro.dialer.relay

import org.json.JSONObject
import org.junit.Assert.*
import org.junit.Test

/**
 * Unit tests for Protocol data classes JSON serialization/deserialization.
 */
class ProtocolTest {

    @Test
    fun testAuthRequestSerialization() {
        val auth = Protocol.AuthRequest(
            pin = "1234",
            deviceId = "test-uuid-123",
            deviceName = "Pixel 6"
        )
        val json = JSONObject(auth.toJson())

        assertEquals("auth", json.getString("type"))
        assertEquals("1234", json.getString("pin"))
        assertEquals("phone", json.getString("role"))
        assertEquals("test-uuid-123", json.getString("device_id"))
        assertEquals("Pixel 6", json.getString("device_name"))
    }

    @Test
    fun testAuthResultDeserialization() {
        val json = JSONObject().apply {
            put("type", "auth_result")
            put("success", true)
            put("message", "ok")
        }
        val result = Protocol.AuthResult.fromJson(json)

        assertTrue(result.success)
        assertEquals("ok", result.message)
    }

    @Test
    fun testAuthResultFailure() {
        val json = JSONObject().apply {
            put("type", "auth_result")
            put("success", false)
            put("message", "invalid pin")
        }
        val result = Protocol.AuthResult.fromJson(json)

        assertFalse(result.success)
        assertEquals("invalid pin", result.message)
    }

    @Test
    fun testDialCommandDeserialization() {
        val json = JSONObject().apply {
            put("type", "dial")
            put("message_id", "msg-001")
            put("phone_number", "13800138000")
            put("device_id", "device-abc")
            put("sim_slot", 1)
        }
        val cmd = Protocol.DialCommand.fromJson(json)

        assertEquals("msg-001", cmd.messageId)
        assertEquals("13800138000", cmd.phoneNumber)
        assertEquals("device-abc", cmd.deviceId)
        assertEquals(1, cmd.simSlot)
    }

    @Test
    fun testDialCommandDefaultSimSlot() {
        val json = JSONObject().apply {
            put("type", "dial")
            put("message_id", "msg-002")
            put("phone_number", "13900139000")
            put("device_id", "device-xyz")
        }
        val cmd = Protocol.DialCommand.fromJson(json)

        assertEquals(0, cmd.simSlot)
    }

    @Test
    fun testHangupCommandDeserialization() {
        val json = JSONObject().apply {
            put("type", "hangup")
            put("message_id", "msg-003")
            put("device_id", "device-abc")
        }
        val cmd = Protocol.HangupCommand.fromJson(json)

        assertEquals("msg-003", cmd.messageId)
        assertEquals("device-abc", cmd.deviceId)
    }

    @Test
    fun testSmsCommandDeserialization() {
        val json = JSONObject().apply {
            put("type", "sms")
            put("message_id", "msg-004")
            put("phone_number", "13800138000")
            put("content", "Hello World")
            put("device_id", "device-abc")
        }
        val cmd = Protocol.SmsCommand.fromJson(json)

        assertEquals("msg-004", cmd.messageId)
        assertEquals("13800138000", cmd.phoneNumber)
        assertEquals("Hello World", cmd.content)
        assertEquals("device-abc", cmd.deviceId)
    }

    @Test
    fun testDialResultSerialization() {
        val result = Protocol.DialResult(
            messageId = "msg-001",
            success = true,
            error = ""
        )
        val json = JSONObject(result.toJson())

        assertEquals("dial_result", json.getString("type"))
        assertEquals("msg-001", json.getString("message_id"))
        assertTrue(json.getBoolean("success"))
        assertEquals("", json.getString("error"))
    }

    @Test
    fun testDialResultWithError() {
        val result = Protocol.DialResult(
            messageId = "msg-002",
            success = false,
            error = "Permission denied"
        )
        val json = JSONObject(result.toJson())

        assertEquals("dial_result", json.getString("type"))
        assertEquals("msg-002", json.getString("message_id"))
        assertFalse(json.getBoolean("success"))
        assertEquals("Permission denied", json.getString("error"))
    }

    @Test
    fun testSmsResultSerialization() {
        val result = Protocol.SmsResult(
            messageId = "msg-005",
            success = true,
            error = ""
        )
        val json = JSONObject(result.toJson())

        assertEquals("sms_result", json.getString("type"))
        assertEquals("msg-005", json.getString("message_id"))
        assertTrue(json.getBoolean("success"))
        assertEquals("", json.getString("error"))
    }

    @Test
    fun testDeviceStatusSerialization() {
        val status = Protocol.DeviceStatus(
            deviceId = "device-abc",
            status = "idle"
        )
        val json = JSONObject(status.toJson())

        assertEquals("device_status", json.getString("type"))
        assertEquals("device-abc", json.getString("device_id"))
        assertEquals("idle", json.getString("status"))
    }

    @Test
    fun testAckSerialization() {
        val ack = Protocol.Ack(messageId = "msg-001")
        val json = JSONObject(ack.toJson())

        assertEquals("ack", json.getString("type"))
        assertEquals("msg-001", json.getString("message_id"))
    }

    @Test
    fun testPingSerialization() {
        val json = JSONObject(Protocol.Ping.toJson())
        assertEquals("ping", json.getString("type"))
    }

    @Test
    fun testPongSerialization() {
        val json = JSONObject(Protocol.Pong.toJson())
        assertEquals("pong", json.getString("type"))
    }

    @Test
    fun testParseTypeValid() {
        val raw = """{"type":"dial","message_id":"123"}"""
        assertEquals("dial", Protocol.parseType(raw))
    }

    @Test
    fun testParseTypeInvalid() {
        assertNull(Protocol.parseType("not json"))
    }

    @Test
    fun testParseTypeEmpty() {
        val raw = """{"foo":"bar"}"""
        // optString returns "" when key is missing if default is null
        val type = Protocol.parseType(raw)
        assertTrue(type == null || type.isEmpty())
    }

    @Test
    fun testParseJsonValid() {
        val raw = """{"type":"ping"}"""
        val json = Protocol.parseJson(raw)
        assertNotNull(json)
        assertEquals("ping", json!!.getString("type"))
    }

    @Test
    fun testParseJsonInvalid() {
        assertNull(Protocol.parseJson("invalid"))
    }
}
