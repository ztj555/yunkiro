package com.yunkiro.dialer.relay

import android.util.Log
import kotlinx.coroutines.*
import okhttp3.*
import java.util.concurrent.TimeUnit
import java.util.concurrent.atomic.AtomicBoolean

/**
 * Manages WebSocket connection to the relay server.
 * Handles authentication, reconnection with exponential backoff, and heartbeat.
 *
 * Sends application-level {"type":"ping"} messages every 30s to keep the relay's
 * inactivity reaper from closing idle connections. OkHttp-level pings are not
 * surfaced to gorilla/websocket's ReadMessage on the server side, so we rely
 * on app-level pings instead.
 */
class RelayConnection(
    private val messageHandler: MessageHandler,
    private val onConnectionStateChanged: (ConnectionState) -> Unit
) {
    companion object {
        private const val TAG = "RelayConnection"
        private const val APP_PING_INTERVAL_MS = 30_000L
        private const val INITIAL_BACKOFF_MS = 1_000L
        private const val MAX_BACKOFF_MS = 30_000L
        private const val CONNECT_TIMEOUT_S = 10L
        private const val READ_TIMEOUT_S = 45L

        // If no inbound data (pong or any message) arrives within this window,
        // the socket is considered a half-open "zombie" (common after Doze) and
        // is rebuilt. Relay replies a pong to every 30s ping, so 75s = ~2.5
        // missed cycles is a safe threshold.
        private const val STALE_TIMEOUT_MS = 75_000L

        // Screen-on health check: how long to wait for a ping response before
        // forcing a reconnect, so the phone is usable within ~3s of waking.
        private const val HEALTH_CHECK_TIMEOUT_MS = 3_000L
    }

    enum class ConnectionState {
        DISCONNECTED,
        CONNECTING,
        CONNECTED,
        RECONNECTING
    }

    private val client = OkHttpClient.Builder()
        .connectTimeout(CONNECT_TIMEOUT_S, TimeUnit.SECONDS)
        .readTimeout(READ_TIMEOUT_S, TimeUnit.SECONDS)
        .build()

    private var webSocket: WebSocket? = null
    private var relayUrl: String = ""
    private var pin: String = ""
    private var deviceId: String = ""
    private var deviceName: String = ""

    private val scope = CoroutineScope(Dispatchers.IO + SupervisorJob())
    private val isRunning = AtomicBoolean(false)
    private var reconnectJob: Job? = null
    private var pingJob: Job? = null
    private var currentBackoff = INITIAL_BACKOFF_MS

    // Timestamp of the last inbound message (any type). Used to detect
    // half-open zombie connections that Doze can leave behind.
    @Volatile
    private var lastInboundAt = System.currentTimeMillis()

    @Volatile
    var state: ConnectionState = ConnectionState.DISCONNECTED
        private set(value) {
            field = value
            onConnectionStateChanged(value)
        }

    /**
     * Connect to the relay server with the given parameters.
     */
    fun connect(url: String, pin: String, deviceId: String, deviceName: String) {
        this.relayUrl = url
        this.pin = pin
        this.deviceId = deviceId
        this.deviceName = deviceName
        isRunning.set(true)
        doConnect()
    }

    /**
     * Disconnect and stop reconnection attempts.
     */
    fun disconnect() {
        isRunning.set(false)
        pingJob?.cancel()
        pingJob = null
        reconnectJob?.cancel()
        reconnectJob = null
        webSocket?.close(1000, "User disconnect")
        webSocket = null
        state = ConnectionState.DISCONNECTED
    }

    /**
     * Send a raw JSON string through the WebSocket.
     */
    fun send(message: String): Boolean {
        return webSocket?.send(message) ?: false
    }

    /**
     * Force a reconnection attempt. Resets the backoff so recovery is fast
     * (used by network-available callbacks and the screen-on health check).
     */
    fun reconnect() {
        if (!isRunning.get()) return
        currentBackoff = INITIAL_BACKOFF_MS
        webSocket?.close(1000, "Reconnecting")
        webSocket = null
        scheduleReconnect()
    }

    /**
     * Proactively verify the connection is alive — called when the screen turns
     * on after the phone may have been in Doze. Sends a ping and forces a
     * reconnect if no response arrives within HEALTH_CHECK_TIMEOUT_MS, so the
     * user gets a usable connection within ~3 seconds of waking the phone.
     */
    fun checkHealthAndRecover() {
        if (!isRunning.get()) return
        val ws = webSocket
        if (ws == null) {
            scheduleReconnect()
            return
        }
        val before = lastInboundAt
        val sent = ws.send(Protocol.Ping.toJson())
        if (!sent) {
            Log.w(TAG, "Health check: ping send failed -> reconnect")
            reconnect()
            return
        }
        scope.launch {
            delay(HEALTH_CHECK_TIMEOUT_MS)
            if (isRunning.get() && lastInboundAt == before) {
                Log.w(TAG, "Health check: no response in ${HEALTH_CHECK_TIMEOUT_MS}ms (zombie) -> reconnect")
                reconnect()
            }
        }
    }

    private fun doConnect() {
        if (!isRunning.get()) return
        state = if (state == ConnectionState.DISCONNECTED) {
            ConnectionState.CONNECTING
        } else {
            ConnectionState.RECONNECTING
        }

        val request = Request.Builder()
            .url(relayUrl)
            .build()

        webSocket = client.newWebSocket(request, object : WebSocketListener() {
            override fun onOpen(webSocket: WebSocket, response: Response) {
                Log.i(TAG, "WebSocket connected to $relayUrl")
                currentBackoff = INITIAL_BACKOFF_MS
                lastInboundAt = System.currentTimeMillis()

                // Send auth message
                val authMsg = Protocol.AuthRequest(
                    pin = pin,
                    deviceId = deviceId,
                    deviceName = deviceName
                ).toJson()
                webSocket.send(authMsg)

                // Start application-level ping loop to keep the relay's
                // inactivity reaper from closing this connection.
                startPingLoop()
            }

            override fun onMessage(webSocket: WebSocket, text: String) {
                Log.d(TAG, "Received: $text")
                lastInboundAt = System.currentTimeMillis()
                messageHandler.handleMessage(text)
            }

            override fun onClosing(webSocket: WebSocket, code: Int, reason: String) {
                Log.i(TAG, "WebSocket closing: $code $reason")
                webSocket.close(code, reason)
            }

            override fun onClosed(webSocket: WebSocket, code: Int, reason: String) {
                Log.i(TAG, "WebSocket closed: $code $reason")
                stopPingLoop()
                if (isRunning.get()) {
                    scheduleReconnect()
                } else {
                    state = ConnectionState.DISCONNECTED
                }
            }

            override fun onFailure(webSocket: WebSocket, t: Throwable, response: Response?) {
                Log.e(TAG, "WebSocket failure: ${t.message}")
                stopPingLoop()
                if (isRunning.get()) {
                    scheduleReconnect()
                } else {
                    state = ConnectionState.DISCONNECTED
                }
            }
        })
    }

    private fun startPingLoop() {
        pingJob?.cancel()
        pingJob = scope.launch {
            while (isActive) {
                delay(APP_PING_INTERVAL_MS)
                val ws = webSocket ?: break
                // Zombie detection: the relay replies a pong to every ping, so
                // we should have received *something* within the stale window.
                // If not, the socket is half-open (typical after Doze) and we
                // rebuild it instead of silently staying "connected".
                if (System.currentTimeMillis() - lastInboundAt > STALE_TIMEOUT_MS) {
                    Log.w(TAG, "No inbound data for >${STALE_TIMEOUT_MS}ms (zombie) -> reconnect")
                    reconnect()
                    break
                }
                ws.send(Protocol.Ping.toJson())
            }
        }
    }

    private fun stopPingLoop() {
        pingJob?.cancel()
        pingJob = null
    }

    private fun scheduleReconnect() {
        if (!isRunning.get()) return
        state = ConnectionState.RECONNECTING

        reconnectJob?.cancel()
        reconnectJob = scope.launch {
            delay(currentBackoff)
            currentBackoff = (currentBackoff * 2).coerceAtMost(MAX_BACKOFF_MS)
            doConnect()
        }
    }

    /**
     * Clean up resources.
     */
    fun destroy() {
        disconnect()
        scope.cancel()
    }
}
