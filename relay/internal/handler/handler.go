package handler

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/gorilla/websocket"
	"github.com/ztj555/yunkiro/relay/internal/hub"
	"github.com/ztj555/yunkiro/relay/internal/logger"
	"github.com/ztj555/yunkiro/relay/internal/protocol"
	"github.com/ztj555/yunkiro/relay/internal/ws"
)

const (
	// AuthTimeout is the time allowed for the client to send an auth message.
	AuthTimeout = 5 * time.Second
)

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	CheckOrigin: func(r *http.Request) bool {
		return true // Allow all origins for relay
	},
}

// Handler handles WebSocket connections.
type Handler struct {
	hub *hub.Hub
	log *logger.Logger
}

// New creates a new Handler.
func New(h *hub.Hub, log *logger.Logger) *Handler {
	return &Handler{
		hub: h,
		log: log,
	}
}

// ServeHTTP handles the WebSocket upgrade and connection lifecycle.
func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	wsConn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		h.log.Error("WebSocket upgrade failed: %v", err)
		return
	}

	conn := ws.NewConn(wsConn, h.log)

	// Wait for auth message
	data, err := conn.ReadRaw(AuthTimeout)
	if err != nil {
		h.log.Warn("Auth read timeout or error: %v", err)
		h.sendAuthResult(conn, false, "auth timeout", nil)
		wsConn.Close()
		return
	}

	// Parse auth message
	authReq, err := protocol.ParseAuth(data)
	if err != nil || authReq.Type != protocol.TypeAuth {
		h.log.Warn("Invalid auth message: %v", err)
		h.sendAuthResult(conn, false, "invalid auth message", nil)
		wsConn.Close()
		return
	}

	// Validate PIN
	if !protocol.ValidatePIN(authReq.PIN) {
		h.log.Warn("Invalid PIN format: %s", authReq.PIN)
		h.sendAuthResult(conn, false, "invalid PIN format (must be 4 digits)", nil)
		wsConn.Close()
		return
	}

	// Validate role
	if authReq.Role != protocol.RolePC && authReq.Role != protocol.RolePhone {
		h.log.Warn("Invalid role: %s", authReq.Role)
		h.sendAuthResult(conn, false, "invalid role (must be pc or phone)", nil)
		wsConn.Close()
		return
	}

	// Set connection identity
	conn.PIN = authReq.PIN
	conn.Role = authReq.Role
	conn.DeviceID = authReq.DeviceID
	conn.DeviceName = authReq.DeviceName

	// Set up callbacks
	conn.OnMessage = func(c *ws.Conn, msg []byte) {
		h.hub.Route(c, msg)
	}
	conn.OnClose = func(c *ws.Conn) {
		h.hub.Unregister(c)
	}

	// Register in hub (returns online phones for PC auth result)
	onlinePhones := h.hub.Register(conn)

	// Send auth success
	var phones []protocol.PhoneInfo
	if conn.Role == protocol.RolePC {
		phones = onlinePhones
	}
	h.sendAuthResult(conn, true, "ok", phones)

	// Start read/write goroutines
	conn.Start()
}

func (h *Handler) sendAuthResult(conn *ws.Conn, success bool, message string, phones []protocol.PhoneInfo) {
	result := protocol.AuthResult{
		Type:         protocol.TypeAuthResult,
		Success:      success,
		Message:      message,
		OnlinePhones: phones,
	}
	data, err := json.Marshal(result)
	if err != nil {
		h.log.Error("Failed to marshal auth result: %v", err)
		return
	}
	// Write directly (write pump not started yet during auth phase)
	conn.WriteRaw(data)
}
