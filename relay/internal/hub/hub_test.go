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
	_, empty := group.RemoveClient(phoneConn)
	if empty {
		t.Error("Group should not be empty after removing phone (PC still there)")
	}

	phones = group.OnlinePhones()
	if len(phones) != 0 {
		t.Errorf("OnlinePhones() = %d, want 0", len(phones))
	}

	// Remove PC
	_, empty = group.RemoveClient(pcConn)
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

func TestHubRouteDialToDisconnectedPhone(t *testing.T) {
	log := logger.New(logger.LevelDebug)
	h := New(log)
	h.Start()
	defer h.Stop()

	pcConn := ws.NewMockConn("1234", protocol.RolePC, "pc-001", "My PC", log)
	h.Register(pcConn)

	// PC sends dial to a phone that doesn't exist
	dial := protocol.DialMessage{
		Type:        protocol.TypeDial,
		MessageID:   "msg-99",
		PhoneNumber: "13800138000",
		DeviceID:    "phone-nonexist",
		SIMSlot:     0,
	}
	data, _ := json.Marshal(dial)
	h.Route(pcConn, data)

	// PC should receive an error dial_result
	msg := pcConn.ReadSent()
	if msg == nil {
		t.Fatal("PC did not receive error response for unreachable phone")
	}
	var result protocol.DialResult
	if err := json.Unmarshal(msg, &result); err != nil {
		t.Fatalf("Failed to unmarshal error response: %v", err)
	}
	if result.Type != protocol.TypeDialResult {
		t.Errorf("Error response type=%q, want %q", result.Type, protocol.TypeDialResult)
	}
	if result.Success {
		t.Error("Error response should have success=false")
	}
	if result.MessageID != "msg-99" {
		t.Errorf("Error response message_id=%q, want %q", result.MessageID, "msg-99")
	}
	if result.Error != "device not found" {
		t.Errorf("Error response error=%q, want %q", result.Error, "device not found")
	}
}

func TestHubRouteSmsToDisconnectedPhone(t *testing.T) {
	log := logger.New(logger.LevelDebug)
	h := New(log)
	h.Start()
	defer h.Stop()

	pcConn := ws.NewMockConn("1234", protocol.RolePC, "pc-001", "My PC", log)
	h.Register(pcConn)

	// PC sends sms to a phone that doesn't exist
	sms := protocol.SMSMessage{
		Type:        protocol.TypeSMS,
		MessageID:   "msg-sms-1",
		PhoneNumber: "13800138000",
		Content:     "Hello",
		DeviceID:    "phone-nonexist",
	}
	data, _ := json.Marshal(sms)
	h.Route(pcConn, data)

	// PC should receive an error sms_result
	msg := pcConn.ReadSent()
	if msg == nil {
		t.Fatal("PC did not receive error response for unreachable phone (sms)")
	}
	var result protocol.SMSResult
	if err := json.Unmarshal(msg, &result); err != nil {
		t.Fatalf("Failed to unmarshal error response: %v", err)
	}
	if result.Type != protocol.TypeSMSResult {
		t.Errorf("Error response type=%q, want %q", result.Type, protocol.TypeSMSResult)
	}
	if result.Success {
		t.Error("Error response should have success=false")
	}
	if result.MessageID != "msg-sms-1" {
		t.Errorf("Error response message_id=%q, want %q", result.MessageID, "msg-sms-1")
	}
}

func TestHubRouteHangupToDisconnectedPhone(t *testing.T) {
	log := logger.New(logger.LevelDebug)
	h := New(log)
	h.Start()
	defer h.Stop()

	pcConn := ws.NewMockConn("1234", protocol.RolePC, "pc-001", "My PC", log)
	h.Register(pcConn)

	// PC sends hangup to a phone that doesn't exist
	hangup := protocol.HangupMessage{
		Type:      protocol.TypeHangup,
		MessageID: "msg-hangup-1",
		DeviceID:  "phone-nonexist",
	}
	data, _ := json.Marshal(hangup)
	h.Route(pcConn, data)

	// PC should receive an error hangup_result
	msg := pcConn.ReadSent()
	if msg == nil {
		t.Fatal("PC did not receive error response for unreachable phone (hangup)")
	}
	var result protocol.HangupResult
	if err := json.Unmarshal(msg, &result); err != nil {
		t.Fatalf("Failed to unmarshal error response: %v", err)
	}
	if result.Type != protocol.TypeHangupResult {
		t.Errorf("Error response type=%q, want %q", result.Type, protocol.TypeHangupResult)
	}
	if result.Success {
		t.Error("Error response should have success=false")
	}
	if result.MessageID != "msg-hangup-1" {
		t.Errorf("Error response message_id=%q, want %q", result.MessageID, "msg-hangup-1")
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


// TestGroupReconnectSameDeviceID is a regression test for the v6 scenario 1/7/8
// bug: when a phone reconnects with the SAME device_id, closing the OLD
// connection must NOT evict the NEW connection from the routing map.
func TestGroupReconnectSameDeviceID(t *testing.T) {
	log := logger.New(logger.LevelDebug)
	group := NewGroup("1234", log)

	conn1 := ws.NewMockConn("1234", protocol.RolePhone, "phone-001", "Phone", log)
	group.AddPhone(conn1)

	// Phone reconnects with the same DeviceID -> a new connection replaces it.
	conn2 := ws.NewMockConn("1234", protocol.RolePhone, "phone-001", "Phone", log)
	group.AddPhone(conn2)

	// The NEW connection must be the one in the routing map.
	if ok := group.RouteToPhone("phone-001", []byte("hello")); !ok {
		t.Fatal("phone-001 should be routable after reconnect")
	}
	if conn2.ReadSent() == nil {
		t.Error("message should have been routed to the NEW connection (conn2)")
	}

	// Now the stale OLD connection closes late. It must report removed=false
	// and must NOT remove conn2 from the map.
	removed, _ := group.RemoveClient(conn1)
	if removed {
		t.Error("removing the stale old connection should report removed=false")
	}

	// phone-001 must still be routable (conn2 still registered).
	if ok := group.RouteToPhone("phone-001", []byte("again")); !ok {
		t.Fatal("phone-001 must remain routable after the stale old connection closed")
	}
}

// TestHubReconnectNoSpuriousOffline verifies that closing a stale (replaced)
// phone connection does not broadcast a spurious phone_offline to PCs.
func TestHubReconnectNoSpuriousOffline(t *testing.T) {
	log := logger.New(logger.LevelDebug)
	h := New(log)
	h.Start()
	defer h.Stop()

	pc := ws.NewMockConn("1234", protocol.RolePC, "pc-001", "PC", log)
	h.Register(pc)
	drain(pc)

	phone1 := ws.NewMockConn("1234", protocol.RolePhone, "phone-001", "Phone", log)
	h.Register(phone1) // broadcasts phone_online
	drain(pc)

	// Phone reconnects with the same device_id (e.g. after a network blip).
	phone2 := ws.NewMockConn("1234", protocol.RolePhone, "phone-001", "Phone", log)
	h.Register(phone2) // replaces phone1, broadcasts phone_online again
	drain(pc)

	// The old connection closes late -> Unregister(phone1).
	h.Unregister(phone1)

	// PC must NOT receive a phone_offline for phone-001.
	for {
		msg := pc.ReadSent()
		if msg == nil {
			break
		}
		msgType, _ := protocol.ParseEnvelope(msg)
		if msgType == protocol.TypePhoneOffline {
			t.Error("must NOT broadcast phone_offline for a stale replaced connection")
		}
	}

	// phone-001 must still be reachable via the live connection (phone2).
	dial := protocol.DialMessage{
		Type:      protocol.TypeDial,
		MessageID: "m1",
		DeviceID:  "phone-001",
	}
	data, _ := json.Marshal(dial)
	h.Route(pc, data)
	if phone2.ReadSent() == nil {
		t.Error("dial should be routed to the live reconnected phone (phone2)")
	}
}

// TestHubRealOfflineStillBroadcasts ensures that a genuine disconnect (no
// replacement) still broadcasts phone_offline.
func TestHubRealOfflineStillBroadcasts(t *testing.T) {
	log := logger.New(logger.LevelDebug)
	h := New(log)
	h.Start()
	defer h.Stop()

	pc := ws.NewMockConn("1234", protocol.RolePC, "pc-001", "PC", log)
	h.Register(pc)
	drain(pc)

	phone := ws.NewMockConn("1234", protocol.RolePhone, "phone-001", "Phone", log)
	h.Register(phone)
	drain(pc)

	// Genuine disconnect.
	h.Unregister(phone)

	gotOffline := false
	for {
		msg := pc.ReadSent()
		if msg == nil {
			break
		}
		msgType, _ := protocol.ParseEnvelope(msg)
		if msgType == protocol.TypePhoneOffline {
			gotOffline = true
		}
	}
	if !gotOffline {
		t.Error("a genuine phone disconnect must broadcast phone_offline")
	}
}

func drain(c *ws.Conn) {
	for c.ReadSent() != nil {
	}
}
