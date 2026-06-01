package hub

import (
	"encoding/json"
	"sync"
	"time"

	"github.com/ztj555/yunkiro/relay/internal/logger"
	"github.com/ztj555/yunkiro/relay/internal/protocol"
	"github.com/ztj555/yunkiro/relay/internal/ws"
)

const (
	// HeartbeatCheckInterval is how often to check for stale connections.
	HeartbeatCheckInterval = 15 * time.Second

	// HeartbeatTimeout is the maximum allowed inactivity time.
	HeartbeatTimeout = 45 * time.Second
)

// Hub manages all PIN groups and connection routing.
type Hub struct {
	mu     sync.RWMutex
	groups map[string]*Group
	log    *logger.Logger
	done   chan struct{}
}

// New creates a new Hub.
func New(log *logger.Logger) *Hub {
	return &Hub{
		groups: make(map[string]*Group),
		log:    log,
		done:   make(chan struct{}),
	}
}

// Start begins the heartbeat monitor.
func (h *Hub) Start() {
	go h.heartbeatLoop()
}

// Stop signals the heartbeat loop to stop.
func (h *Hub) Stop() {
	close(h.done)
}

// Register adds a connection to the appropriate PIN group.
// It returns the list of online phones (for PC auth result).
func (h *Hub) Register(conn *ws.Conn) []protocol.PhoneInfo {
	h.mu.Lock()
	group, ok := h.groups[conn.PIN]
	if !ok {
		group = NewGroup(conn.PIN, h.log)
		h.groups[conn.PIN] = group
	}
	h.mu.Unlock()

	switch conn.Role {
	case protocol.RolePC:
		group.AddPC(conn)
		h.log.Info("PC registered: device=%s pin=%s", conn.DeviceID, conn.PIN)
	case protocol.RolePhone:
		group.AddPhone(conn)
		h.log.Info("Phone registered: device=%s name=%s pin=%s", conn.DeviceID, conn.DeviceName, conn.PIN)
		// Broadcast phone_online to all PCs in group
		msg := protocol.PhoneOnline{
			Type:       protocol.TypePhoneOnline,
			DeviceID:   conn.DeviceID,
			DeviceName: conn.DeviceName,
		}
		data, _ := json.Marshal(msg)
		group.BroadcastToPCs(data)
	}

	return group.OnlinePhones()
}

// Unregister removes a connection from its group.
func (h *Hub) Unregister(conn *ws.Conn) {
	h.mu.RLock()
	group, ok := h.groups[conn.PIN]
	h.mu.RUnlock()
	if !ok {
		return
	}

	// Remove only if this is still the current connection for its DeviceID.
	// A stale connection (already replaced by a reconnect with the same
	// DeviceID) must NOT trigger phone_offline or evict the new connection
	// from the routing map. This is what caused "phone reopened -> PC shows
	// offline / device not found" after a quick reconnect.
	removed, empty := group.RemoveClient(conn)
	if !removed {
		h.log.Debug("Stale connection closed (already replaced): device=%s pin=%s", conn.DeviceID, conn.PIN)
		return
	}

	if conn.Role == protocol.RolePhone {
		// Broadcast phone_offline to PCs now that the phone is really gone.
		msg := protocol.PhoneOffline{
			Type:       protocol.TypePhoneOffline,
			DeviceID:   conn.DeviceID,
			DeviceName: conn.DeviceName,
		}
		data, _ := json.Marshal(msg)
		group.BroadcastToPCs(data)
		h.log.Info("Phone disconnected: device=%s pin=%s", conn.DeviceID, conn.PIN)
	} else {
		h.log.Info("PC disconnected: device=%s pin=%s", conn.DeviceID, conn.PIN)
	}

	if empty {
		h.mu.Lock()
		// Double-check it's still empty (another goroutine might have added)
		if group.IsEmpty() {
			delete(h.groups, conn.PIN)
			h.log.Debug("Removed empty group: pin=%s", conn.PIN)
		}
		h.mu.Unlock()
	}
}

// Route handles an incoming message from a connection.
func (h *Hub) Route(conn *ws.Conn, data []byte) {
	msgType, err := protocol.ParseEnvelope(data)
	if err != nil {
		h.log.Warn("Failed to parse message from device=%s: %v", conn.DeviceID, err)
		return
	}

	switch msgType {
	case protocol.TypePing:
		// Respond with pong
		pong := protocol.PongMessage{Type: protocol.TypePong}
		resp, _ := json.Marshal(pong)
		conn.Send(resp)

	case protocol.TypePong:
		// Just activity touch, already done in readPump

	case protocol.TypeAck:
		// ACK messages are consumed by the relay (no forwarding needed)
		h.log.Debug("ACK received from device=%s", conn.DeviceID)

	case protocol.TypeDial, protocol.TypeHangup, protocol.TypeSMS:
		// PC -> Phone: route to target phone
		h.routePCToPhone(conn, msgType, data)

	case protocol.TypeDialResult, protocol.TypeSMSResult, protocol.TypeDeviceStatus:
		// Phone -> All PCs in group
		h.routePhoneToPCs(conn, data)

	default:
		h.log.Warn("Unknown message type=%s from device=%s", msgType, conn.DeviceID)
	}
}

func (h *Hub) routePCToPhone(conn *ws.Conn, msgType string, data []byte) {
	if conn.Role != protocol.RolePC {
		h.log.Warn("Non-PC tried to send %s: device=%s", msgType, conn.DeviceID)
		return
	}

	// Extract device_id (target phone) and message_id
	var envelope struct {
		DeviceID  string `json:"device_id"`
		MessageID string `json:"message_id"`
	}
	if err := json.Unmarshal(data, &envelope); err != nil {
		h.log.Warn("Failed to parse device_id from %s message", msgType)
		return
	}

	h.mu.RLock()
	group, ok := h.groups[conn.PIN]
	h.mu.RUnlock()
	if !ok {
		h.log.Warn("Group not found for pin=%s", conn.PIN)
		h.sendDeviceNotFoundError(conn, msgType, envelope.MessageID)
		return
	}

	if !group.RouteToPhone(envelope.DeviceID, data) {
		h.log.Warn("Target phone not found: device=%s in pin=%s", envelope.DeviceID, conn.PIN)
		h.sendDeviceNotFoundError(conn, msgType, envelope.MessageID)
	}
}

// sendDeviceNotFoundError sends an error response back to the PC when
// the target phone is not reachable.
func (h *Hub) sendDeviceNotFoundError(conn *ws.Conn, msgType string, messageID string) {
	var resp interface{}
	switch msgType {
	case protocol.TypeDial:
		resp = protocol.DialResult{
			Type:      protocol.TypeDialResult,
			MessageID: messageID,
			Success:   false,
			Error:     "device not found",
		}
	case protocol.TypeHangup:
		resp = protocol.HangupResult{
			Type:      protocol.TypeHangupResult,
			MessageID: messageID,
			Success:   false,
			Error:     "device not found",
		}
	case protocol.TypeSMS:
		resp = protocol.SMSResult{
			Type:      protocol.TypeSMSResult,
			MessageID: messageID,
			Success:   false,
			Error:     "device not found",
		}
	default:
		return
	}
	data, err := json.Marshal(resp)
	if err != nil {
		h.log.Warn("Failed to marshal error response: %v", err)
		return
	}
	conn.Send(data)
}

func (h *Hub) routePhoneToPCs(conn *ws.Conn, data []byte) {
	if conn.Role != protocol.RolePhone {
		h.log.Warn("Non-phone tried to send result: device=%s", conn.DeviceID)
		return
	}

	h.mu.RLock()
	group, ok := h.groups[conn.PIN]
	h.mu.RUnlock()
	if !ok {
		return
	}

	group.BroadcastToPCs(data)
}

func (h *Hub) heartbeatLoop() {
	ticker := time.NewTicker(HeartbeatCheckInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			h.checkHeartbeats()
		case <-h.done:
			return
		}
	}
}

func (h *Hub) checkHeartbeats() {
	// Collect a snapshot of groups under read lock
	h.mu.RLock()
	groupsCopy := make([]*Group, 0, len(h.groups))
	for _, g := range h.groups {
		groupsCopy = append(groupsCopy, g)
	}
	h.mu.RUnlock()

	// Now iterate groups without holding hub lock
	var stale []*ws.Conn
	for _, group := range groupsCopy {
		for _, conn := range group.AllConnections() {
			if time.Since(conn.LastActivity()) > HeartbeatTimeout {
				stale = append(stale, conn)
			}
		}
	}

	for _, conn := range stale {
		h.log.Info("Heartbeat timeout: device=%s pin=%s", conn.DeviceID, conn.PIN)
		conn.Close()
	}
}

// CloseAll closes all connections in all groups.
func (h *Hub) CloseAll() {
	h.mu.RLock()
	var allConns []*ws.Conn
	for _, group := range h.groups {
		allConns = append(allConns, group.AllConnections()...)
	}
	h.mu.RUnlock()

	for _, conn := range allConns {
		conn.Close()
	}
}

// Stats returns the number of groups and total connections.
func (h *Hub) Stats() (groups int, connections int) {
	h.mu.RLock()
	defer h.mu.RUnlock()
	groups = len(h.groups)
	for _, g := range h.groups {
		conns := g.AllConnections()
		connections += len(conns)
	}
	return
}
