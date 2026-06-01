package hub

import (
	"sync"

	"github.com/ztj555/yunkiro/relay/internal/logger"
	"github.com/ztj555/yunkiro/relay/internal/protocol"
	"github.com/ztj555/yunkiro/relay/internal/ws"
)

// Group holds connections for a single PIN group.
type Group struct {
	mu     sync.RWMutex
	pin    string
	pcs    map[string]*ws.Conn // keyed by DeviceID
	phones map[string]*ws.Conn // keyed by DeviceID
	log    *logger.Logger
}

// NewGroup creates a new Group for the given PIN.
func NewGroup(pin string, log *logger.Logger) *Group {
	return &Group{
		pin:    pin,
		pcs:    make(map[string]*ws.Conn),
		phones: make(map[string]*ws.Conn),
		log:    log,
	}
}

// AddPC adds a PC connection to the group.
func (g *Group) AddPC(conn *ws.Conn) {
	g.mu.Lock()
	if old, ok := g.pcs[conn.DeviceID]; ok && old != conn {
		go old.Close()
	}
	g.pcs[conn.DeviceID] = conn
	g.mu.Unlock()
}

// AddPhone adds a phone connection to the group.
func (g *Group) AddPhone(conn *ws.Conn) {
	g.mu.Lock()
	if old, ok := g.phones[conn.DeviceID]; ok && old != conn {
		go old.Close()
	}
	g.phones[conn.DeviceID] = conn
	g.mu.Unlock()
}

// RemoveClient removes a connection from the group, but ONLY if it is still the
// currently-registered connection for its DeviceID. This prevents a stale/old
// connection (that was already replaced by a reconnect with the same DeviceID)
// from accidentally evicting the new live connection from the routing map.
//
// Returns:
//   - removed: true if this connection was the current one and was removed.
//   - empty:   true if the group has no connections left.
func (g *Group) RemoveClient(conn *ws.Conn) (removed bool, empty bool) {
	g.mu.Lock()
	defer g.mu.Unlock()
	switch conn.Role {
	case protocol.RolePC:
		if g.pcs[conn.DeviceID] == conn {
			delete(g.pcs, conn.DeviceID)
			removed = true
		}
	case protocol.RolePhone:
		if g.phones[conn.DeviceID] == conn {
			delete(g.phones, conn.DeviceID)
			removed = true
		}
	}
	empty = len(g.pcs) == 0 && len(g.phones) == 0
	return removed, empty
}

// BroadcastToPCs sends a message to all PC connections in the group.
func (g *Group) BroadcastToPCs(data []byte) {
	g.mu.RLock()
	defer g.mu.RUnlock()
	for _, conn := range g.pcs {
		conn.Send(data)
	}
}

// RouteToPhone sends a message to a specific phone by DeviceID.
// Returns false if the phone is not found.
func (g *Group) RouteToPhone(deviceID string, data []byte) bool {
	g.mu.RLock()
	defer g.mu.RUnlock()
	phone, ok := g.phones[deviceID]
	if !ok {
		return false
	}
	return phone.Send(data)
}

// OnlinePhones returns a list of currently online phones.
func (g *Group) OnlinePhones() []protocol.PhoneInfo {
	g.mu.RLock()
	defer g.mu.RUnlock()
	phones := make([]protocol.PhoneInfo, 0, len(g.phones))
	for _, conn := range g.phones {
		phones = append(phones, protocol.PhoneInfo{
			DeviceID:   conn.DeviceID,
			DeviceName: conn.DeviceName,
		})
	}
	return phones
}

// IsEmpty returns true if the group has no connections.
func (g *Group) IsEmpty() bool {
	g.mu.RLock()
	defer g.mu.RUnlock()
	return len(g.pcs) == 0 && len(g.phones) == 0
}

// AllConnections returns all connections in the group (for cleanup).
func (g *Group) AllConnections() []*ws.Conn {
	g.mu.RLock()
	defer g.mu.RUnlock()
	conns := make([]*ws.Conn, 0, len(g.pcs)+len(g.phones))
	for _, c := range g.pcs {
		conns = append(conns, c)
	}
	for _, c := range g.phones {
		conns = append(conns, c)
	}
	return conns
}
