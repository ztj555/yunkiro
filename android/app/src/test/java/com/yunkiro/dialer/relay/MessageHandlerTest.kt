package com.yunkiro.dialer.relay

import org.json.JSONObject
import org.junit.Assert.*
import org.junit.Before
import org.junit.Test

/**
 * Unit tests for MessageHandler: deduplication, dispatch, and edge cases.
 */
class MessageHandlerTest {

    private val dialCommands = mutableListOf<Protocol.DialCommand>()
    private val hangupCommands = mutableListOf<Protocol.HangupCommand>()
    private val smsCommands = mutableListOf<Protocol.SmsCommand>()
    private val authResults = mutableListOf<Protocol.AuthResult>()
    private var pingCount = 0
    private val ackedIds = mutableListOf<String>()

    private lateinit var handler: MessageHandler

    @Before
    fun setup() {
        dialCommands.clear()
        hangupCommands.clear()
        smsCommands.clear()
        authResults.clear()
        pingCount = 0
        ackedIds.clear()

        handler = MessageHandler(
            onDial = { dialCommands.add(it) },
            onHangup = { hangupCommands.add(it) },
            onSms = { smsCommands.add(it) },
            onAuthResult = { authResults.add(it) },
            onPing = { pingCount++ },
            sendAck = { ackedIds.add(it) }
        )
    }

    @Test
    fun testDialDispatch() {
        val msg = JSONObject().apply {
            put("type", "dial")
            put("message_id", "msg-001")
            put("phone_number", "13800138000")
            put("device_id", "dev-1")
            put("sim_slot", 0)
        }.toString()

        val result = handler.handleMessage(msg)

        assertTrue(result)
        assertEquals(1, dialCommands.size)
        assertEquals("msg-001", dialCommands[0].messageId)
        assertEquals("13800138000", dialCommands[0].phoneNumber)
        assertEquals(1, ackedIds.size)
        assertEquals("msg-001", ackedIds[0])
    }

    @Test
    fun testHangupDispatch() {
        val msg = JSONObject().apply {
            put("type", "hangup")
            put("message_id", "msg-002")
            put("device_id", "dev-1")
        }.toString()

        val result = handler.handleMessage(msg)

        assertTrue(result)
        assertEquals(1, hangupCommands.size)
        assertEquals("msg-002", hangupCommands[0].messageId)
    }

    @Test
    fun testSmsDispatch() {
        val msg = JSONObject().apply {
            put("type", "sms")
            put("message_id", "msg-003")
            put("phone_number", "13800138000")
            put("content", "Test SMS")
            put("device_id", "dev-1")
        }.toString()

        val result = handler.handleMessage(msg)

        assertTrue(result)
        assertEquals(1, smsCommands.size)
        assertEquals("Test SMS", smsCommands[0].content)
    }

    @Test
    fun testAuthResultDispatch() {
        val msg = JSONObject().apply {
            put("type", "auth_result")
            put("success", true)
            put("message", "ok")
        }.toString()

        val result = handler.handleMessage(msg)

        assertTrue(result)
        assertEquals(1, authResults.size)
        assertTrue(authResults[0].success)
    }

    @Test
    fun testPingDispatch() {
        val msg = """{"type":"ping"}"""

        val result = handler.handleMessage(msg)

        assertTrue(result)
        assertEquals(1, pingCount)
    }

    @Test
    fun testDeduplicationSameMessageId() {
        val msg = JSONObject().apply {
            put("type", "dial")
            put("message_id", "msg-dup")
            put("phone_number", "13800138000")
            put("device_id", "dev-1")
            put("sim_slot", 0)
        }.toString()

        // First time should succeed
        assertTrue(handler.handleMessage(msg))
        assertEquals(1, dialCommands.size)

        // Second time with same message_id should be ignored
        assertFalse(handler.handleMessage(msg))
        assertEquals(1, dialCommands.size) // Still 1, not 2

        // ACK should only be sent once
        assertEquals(1, ackedIds.size)
    }

    @Test
    fun testDeduplicationDifferentMessageIds() {
        for (i in 1..5) {
            val msg = JSONObject().apply {
                put("type", "dial")
                put("message_id", "msg-$i")
                put("phone_number", "1380013800$i")
                put("device_id", "dev-1")
                put("sim_slot", 0)
            }.toString()

            assertTrue(handler.handleMessage(msg))
        }

        assertEquals(5, dialCommands.size)
        assertEquals(5, ackedIds.size)
    }

    @Test
    fun testDeduplicationLRUEviction() {
        // Fill up to MAX_DEDUP_SIZE (100)
        for (i in 1..100) {
            val msg = JSONObject().apply {
                put("type", "dial")
                put("message_id", "msg-$i")
                put("phone_number", "13800138000")
                put("device_id", "dev-1")
                put("sim_slot", 0)
            }.toString()
            handler.handleMessage(msg)
        }

        assertEquals(100, handler.getProcessedCount())

        // Add one more, which should evict the oldest (msg-1)
        val newMsg = JSONObject().apply {
            put("type", "dial")
            put("message_id", "msg-101")
            put("phone_number", "13800138000")
            put("device_id", "dev-1")
            put("sim_slot", 0)
        }.toString()
        handler.handleMessage(newMsg)

        // msg-1 should no longer be in the dedup set
        assertFalse(handler.isDuplicate("msg-1"))
        // msg-2 should still be there
        assertTrue(handler.isDuplicate("msg-2"))
        // msg-101 should be there
        assertTrue(handler.isDuplicate("msg-101"))
    }

    @Test
    fun testInvalidJson() {
        assertFalse(handler.handleMessage("not json"))
        assertEquals(0, dialCommands.size)
    }

    @Test
    fun testMissingTypeField() {
        val msg = """{"message_id":"msg-1"}"""
        assertFalse(handler.handleMessage(msg))
    }

    @Test
    fun testUnknownType() {
        val msg = """{"type":"unknown_type","message_id":"msg-1"}"""
        assertFalse(handler.handleMessage(msg))
    }

    @Test
    fun testMessageWithoutMessageId() {
        val msg = JSONObject().apply {
            put("type", "dial")
            put("phone_number", "13800138000")
            put("device_id", "dev-1")
        }.toString()

        assertFalse(handler.handleMessage(msg))
        assertEquals(0, dialCommands.size)
    }

    @Test
    fun testPongReceived() {
        val msg = """{"type":"pong"}"""
        assertTrue(handler.handleMessage(msg))
        // Pong is a no-op, just verify it is handled
    }

    @Test
    fun testAckReceived() {
        val msg = """{"type":"ack","message_id":"msg-1"}"""
        assertTrue(handler.handleMessage(msg))
    }
}
