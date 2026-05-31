package hub

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/ztj555/yunkiro/relay/internal/logger"
	"github.com/ztj555/yunkiro/relay/internal/protocol"
	"github.com/ztj555/yunkiro/relay/internal/ws"
)

func TestGroupAddRemove(t *testing.T) {
	log := logger.New(logger.LevelDebug)
	group := NewGroup("1234", log)

	// Simulate a PC connection
	pcConn := &ws.Conn{
		PIN:      "1234",
		Role:     protocol.RolePC,
		DeviceID: "pc-001",
	}

	phoneConn := &ws.Conn{
		PIN:        "1234",
		Role:       protocol.RolePhone,
		DeviceID:   "phone-001",
		DeviceName: "Test Phone",
	}

	group.AddPC(pcConn)
	group.AddPhone(phoneConn)

	// Check online phones
	phones := group.OnlinePhones()
	if len(phones) != 1 {
		t.Fatalf("OnlinePhones() = %d, want 1", len(phones))
	}
	if phones[0].DeviceID != "phone-001" {
		t.Errorf("PhoneInfo.DeviceID = %q, want %q", phones[0].DeviceID, "phone-001")
	}

	// Check all connections
	all := group.AllConnections()
	if len(all) != 2 {
		t.Fatalf("AllConnections() = %d, want 2", len(all))
	}

	// Remove phone
	empty := group.RemoveClient(phoneConn)
	if empty {
		t.Error("Group should not be empty after removing phone (PC still there)")
	}

	phones = group.OnlinePhones()
	if len(phones) != 0 {
		t.Errorf("OnlinePhones() = %d, want 0", len(phones))
	}

	// Remove PC
	empty = group.RemoveClient(pcConn)
	if !empty {
		t.Error("Group should be empty after removing both")
	}

	if !group.IsEmpty() {
		t.Error("IsEmpty() should return true")
	}
}

func TestGroupRouteToPhone(t *testing.T) {
	log := logger.New(logger.LevelDebug)
	group := NewGroup("1234", log)

	// RouteToPhone with no phones should return false
	ok := group.RouteToPhone("nonexistent", []byte(`{"type":"dial"}`))
	if ok {
		t.Error("RouteToPhone should return false for nonexistent device")
	}
}

func TestHubRegisterUnregister(t *testing.T) {
	log := logger.New(logger.LevelDebug)
	h := New(log)
	h.Start()
	defer h.Stop()

	// Create mock connections with initialized channels
	pcConn := ws.NewMockConn("1234", protocol.RolePC, "pc-001", "My PC", log)
	phoneConn := ws.NewMockConn("1234", protocol.RolePhone, "phone-001", "Test Phone", log)

	// Register PC first
	phones := h.Register(pcConn)
	if len(phones) != 0 {
		t.Errorf("Expected 0 phones, got %d", len(phones))
	}

	groups, conns := h.Stats()
	if groups != 1 || conns != 1 {
		t.Errorf("Stats = (%d, %d), want (1, 1)", groups, conns)
	}

	// Register phone (will broadcast phone_online to PC)
	h.Register(phoneConn)

	groups, conns = h.Stats()
	if groups != 1 || conns != 2 {
		t.Errorf("Stats = (%d, %d), want (1, 2)", groups, conns)
	}

	// Unregister phone
	h.Unregister(phoneConn)

	groups, conns = h.Stats()
	if groups != 1 || conns != 1 {
		t.Errorf("Stats = (%d, %d), want (1, 1)", groups, conns)
	}

	// Unregister PC (group should be cleaned up)
	h.Unregister(pcConn)

	groups, conns = h.Stats()
	if groups != 0 || conns != 0 {
		t.Errorf("Stats = (%d, %d), want (0, 0)", groups, conns)
	}
}

func TestHubPINIsolation(t *testing.T) {
	log := logger.New(logger.LevelDebug)
	h := New(log)
	h.Start()
	defer h.Stop()

	// Two different PIN groups
	pc1 := ws.NewMockConn("1111", protocol.RolePC, "pc-1", "PC 1", log)
	phone1 := ws.NewMockConn("1111", protocol.RolePhone, "phone-1", "Phone 1", log)
	pc2 := ws.NewMockConn("2222", protocol.RolePC, "pc-2", "PC 2", log)
	phone2 := ws.NewMockConn("2222", protocol.RolePhone, "phone-2", "Phone 2", log)

	h.Register(pc1)
	h.Register(phone1)
	h.Register(pc2)
	h.Register(phone2)

	groups, conns := h.Stats()
	if groups != 2 || conns != 4 {
		t.Errorf("Stats = (%d, %d), want (2, 4)", groups, conns)
	}
}

func TestHubRoutePingPong(t *testing.T) {
	log := logger.New(logger.LevelDebug)
	h := New(log)
	h.Start()
	defer h.Stop()

	pcConn := ws.NewMockConn("1234", protocol.RolePC, "pc-001", "My PC", log)
	h.Register(pcConn)

	// Send ping - should respond with pong
	ping, _ := json.Marshal(protocol.PingMessage{Type: protocol.TypePing})
	h.Route(pcConn, ping)

	// Check that pong was sent
	msg := pcConn.ReadSent()
	if msg != nil {
		msgType, _ := protocol.ParseEnvelope(msg)
		if msgType != protocol.TypePong {
			t.Errorf("Expected pong, got type=%q", msgType)
		}
	}

	// ACK should not panic
	ack, _ := json.Marshal(protocol.AckMessage{Type: protocol.TypeAck, MessageID: "msg-1"})
	h.Route(pcConn, ack)

	// Invalid JSON should be handled gracefully
	h.Route(pcConn, []byte("invalid json"))
}

func TestHubRouteDialToPhone(t *testing.T) {
	log := logger.New(logger.LevelDebug)
	h := New(log)
	h.Start()
	defer h.Stop()

	pcConn := ws.NewMockConn("1234", protocol.RolePC, "pc-001", "My PC", log)
	phoneConn := ws.NewMockConn("1234", protocol.RolePhone, "phone-001", "Test Phone", log)

	h.Register(pcConn)
	h.Register(phoneConn)

	// Drain the phone_online message from PC's buffer
	pcConn.ReadSent()

	// PC sends dial to phone
	dial := protocol.DialMessage{
		Type:        protocol.TypeDial,
		MessageID:   "msg-1",
		PhoneNumber: "13800138000",
		DeviceID:    "phone-001",
		SIMSlot:     0,
	}
	data, _ := json.Marshal(dial)
	h.Route(pcConn, data)

	// Phone should have received it
	msg := phoneConn.ReadSent()
	if msg == nil {
		t.Fatal("Phone did not receive dial message")
	}
	msgType, _ := protocol.ParseEnvelope(msg)
	if msgType != protocol.TypeDial {
		t.Errorf("Phone received type=%q, want %q", msgType, protocol.TypeDial)
	}
}

func TestHubRouteResultToPCs(t *testing.T) {
	log := logger.New(logger.LevelDebug)
	h := New(log)
	h.Start()
	defer h.Stop()

	pcConn := ws.NewMockConn("1234", protocol.RolePC, "pc-001", "My PC", log)
	phoneConn := ws.NewMockConn("1234", protocol.RolePhone, "phone-001", "Test Phone", log)

	h.Register(pcConn)
	h.Register(phoneConn)

	// Drain the phone_online message
	pcConn.ReadSent()

	// Phone sends dial_result
	result := protocol.DialResult{
		Type:      protocol.TypeDialResult,
		MessageID: "msg-1",
		Success:   true,
		Error:     "",
	}
	data, _ := json.Marshal(result)
	h.Route(phoneConn, data)

	// PC should have received it
	msg := pcConn.ReadSent()
	if msg == nil {
		t.Fatal("PC did not receive dial_result")
	}
	msgType, _ := protocol.ParseEnvelope(msg)
	if msgType != protocol.TypeDialResult {
		t.Errorf("PC received type=%q, want %q", msgType, protocol.TypeDialResult)
	}
}

func TestHeartbeatTimeout(t *testing.T) {
	log := logger.New(logger.LevelDebug)
	conn := ws.NewMockConn("1234", protocol.RolePhone, "phone-001", "Test Phone", log)

	conn.TouchActivity()
	time.Sleep(10 * time.Millisecond)

	if time.Since(conn.LastActivity()) > time.Second {
		t.Error("LastActivity should be recent")
	}
}
