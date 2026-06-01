package main

import (
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"testing"
	"time"

	"github.com/gorilla/websocket"
	"github.com/ztj555/yunkiro/relay/internal/handler"
	"github.com/ztj555/yunkiro/relay/internal/hub"
	"github.com/ztj555/yunkiro/relay/internal/logger"
	"github.com/ztj555/yunkiro/relay/internal/protocol"
)

func startTestServer(t *testing.T) (string, func()) {
	t.Helper()
	log := logger.New(logger.LevelDebug)
	h := hub.New(log)
	h.Start()

	wsHandler := handler.New(h, log)
	mux := http.NewServeMux()
	mux.Handle("/ws", wsHandler)

	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("Failed to listen: %v", err)
	}

	server := &http.Server{Handler: mux}
	go server.Serve(listener)

	addr := fmt.Sprintf("ws://%s/ws", listener.Addr().String())
	cleanup := func() {
		h.Stop()
		h.CloseAll()
		server.Close()
	}

	return addr, cleanup
}

func connectAndAuth(t *testing.T, url string, pin, role, deviceID, deviceName string) *websocket.Conn {
	t.Helper()
	dialer := websocket.Dialer{
		HandshakeTimeout: 5 * time.Second,
	}
	conn, _, err := dialer.Dial(url, nil)
	if err != nil {
		t.Fatalf("Failed to connect: %v", err)
	}

	authMsg := protocol.AuthRequest{
		Type:       protocol.TypeAuth,
		PIN:        pin,
		Role:       role,
		DeviceID:   deviceID,
		DeviceName: deviceName,
	}
	data, _ := json.Marshal(authMsg)
	if err := conn.WriteMessage(websocket.TextMessage, data); err != nil {
		t.Fatalf("Failed to write auth: %v", err)
	}

	// Read auth result
	_, msg, err := conn.ReadMessage()
	if err != nil {
		t.Fatalf("Failed to read auth result: %v", err)
	}

	var result protocol.AuthResult
	if err := json.Unmarshal(msg, &result); err != nil {
		t.Fatalf("Failed to parse auth result: %v", err)
	}

	if !result.Success {
		t.Fatalf("Auth failed: %s", result.Message)
	}

	return conn
}

func readMessage(t *testing.T, conn *websocket.Conn, timeout time.Duration) []byte {
	t.Helper()
	conn.SetReadDeadline(time.Now().Add(timeout))
	_, msg, err := conn.ReadMessage()
	if err != nil {
		t.Fatalf("Failed to read message: %v", err)
	}
	conn.SetReadDeadline(time.Time{})
	return msg
}

func TestIntegrationAuthSuccess(t *testing.T) {
	addr, cleanup := startTestServer(t)
	defer cleanup()

	// Connect a PC
	pcConn := connectAndAuth(t, addr, "1234", "pc", "pc-001", "My PC")
	defer pcConn.Close()
}

func TestIntegrationAuthInvalidPIN(t *testing.T) {
	addr, cleanup := startTestServer(t)
	defer cleanup()

	dialer := websocket.Dialer{HandshakeTimeout: 5 * time.Second}
	conn, _, err := dialer.Dial(addr, nil)
	if err != nil {
		t.Fatalf("Failed to connect: %v", err)
	}
	defer conn.Close()

	// Send auth with invalid PIN
	authMsg := protocol.AuthRequest{
		Type:       protocol.TypeAuth,
		PIN:        "abc",
		Role:       "pc",
		DeviceID:   "pc-001",
		DeviceName: "My PC",
	}
	data, _ := json.Marshal(authMsg)
	conn.WriteMessage(websocket.TextMessage, data)

	// Read auth result - the server writes the result then closes
	conn.SetReadDeadline(time.Now().Add(2 * time.Second))
	_, msg, err := conn.ReadMessage()
	if err != nil {
		// Connection might be closed before we can read, which is acceptable
		// The important thing is the server didn't crash
		t.Logf("Connection closed (expected): %v", err)
		return
	}

	var result protocol.AuthResult
	json.Unmarshal(msg, &result)
	if result.Success {
		t.Error("Auth should have failed with invalid PIN")
	}
}

func TestIntegrationPhoneOnlineOffline(t *testing.T) {
	addr, cleanup := startTestServer(t)
	defer cleanup()

	// Connect PC first
	pcConn := connectAndAuth(t, addr, "1234", "pc", "pc-001", "My PC")
	defer pcConn.Close()

	// Connect phone - PC should receive phone_online
	phoneConn := connectAndAuth(t, addr, "1234", "phone", "phone-001", "Test Phone")

	// PC should receive phone_online notification
	msg := readMessage(t, pcConn, 2*time.Second)
	var online protocol.PhoneOnline
	if err := json.Unmarshal(msg, &online); err != nil {
		t.Fatalf("Failed to parse phone_online: %v", err)
	}
	if online.Type != protocol.TypePhoneOnline {
		t.Errorf("Type = %q, want %q", online.Type, protocol.TypePhoneOnline)
	}
	if online.DeviceID != "phone-001" {
		t.Errorf("DeviceID = %q, want %q", online.DeviceID, "phone-001")
	}
	if online.DeviceName != "Test Phone" {
		t.Errorf("DeviceName = %q, want %q", online.DeviceName, "Test Phone")
	}

	// Disconnect phone - PC should receive phone_offline
	phoneConn.Close()

	msg = readMessage(t, pcConn, 2*time.Second)
	var offline protocol.PhoneOffline
	if err := json.Unmarshal(msg, &offline); err != nil {
		t.Fatalf("Failed to parse phone_offline: %v", err)
	}
	if offline.Type != protocol.TypePhoneOffline {
		t.Errorf("Type = %q, want %q", offline.Type, protocol.TypePhoneOffline)
	}
	if offline.DeviceID != "phone-001" {
		t.Errorf("DeviceID = %q, want %q", offline.DeviceID, "phone-001")
	}
}

func TestIntegrationDialFlow(t *testing.T) {
	addr, cleanup := startTestServer(t)
	defer cleanup()

	// Connect PC and phone with same PIN
	pcConn := connectAndAuth(t, addr, "5678", "pc", "pc-001", "My PC")
	defer pcConn.Close()

	phoneConn := connectAndAuth(t, addr, "5678", "phone", "phone-001", "Test Phone")
	defer phoneConn.Close()

	// PC should receive phone_online
	readMessage(t, pcConn, 2*time.Second)

	// PC sends dial command to phone
	dialMsg := protocol.DialMessage{
		Type:        protocol.TypeDial,
		MessageID:   "msg-001",
		PhoneNumber: "13800138000",
		DeviceID:    "phone-001",
		SIMSlot:     0,
	}
	data, _ := json.Marshal(dialMsg)
	if err := pcConn.WriteMessage(websocket.TextMessage, data); err != nil {
		t.Fatalf("Failed to send dial: %v", err)
	}

	// Phone should receive the dial command
	msg := readMessage(t, phoneConn, 2*time.Second)
	var received protocol.DialMessage
	if err := json.Unmarshal(msg, &received); err != nil {
		t.Fatalf("Failed to parse dial on phone: %v", err)
	}
	if received.Type != protocol.TypeDial {
		t.Errorf("Phone received type = %q, want %q", received.Type, protocol.TypeDial)
	}
	if received.PhoneNumber != "13800138000" {
		t.Errorf("PhoneNumber = %q, want %q", received.PhoneNumber, "13800138000")
	}
	if received.MessageID != "msg-001" {
		t.Errorf("MessageID = %q, want %q", received.MessageID, "msg-001")
	}

	// Phone sends dial_result back
	resultMsg := protocol.DialResult{
		Type:      protocol.TypeDialResult,
		MessageID: "msg-001",
		Success:   true,
		Error:     "",
	}
	data, _ = json.Marshal(resultMsg)
	if err := phoneConn.WriteMessage(websocket.TextMessage, data); err != nil {
		t.Fatalf("Failed to send dial_result: %v", err)
	}

	// PC should receive the dial_result
	msg = readMessage(t, pcConn, 2*time.Second)
	var dialResult protocol.DialResult
	if err := json.Unmarshal(msg, &dialResult); err != nil {
		t.Fatalf("Failed to parse dial_result on PC: %v", err)
	}
	if dialResult.Type != protocol.TypeDialResult {
		t.Errorf("PC received type = %q, want %q", dialResult.Type, protocol.TypeDialResult)
	}
	if !dialResult.Success {
		t.Error("dial_result.Success should be true")
	}
}

func TestIntegrationSMSFlow(t *testing.T) {
	addr, cleanup := startTestServer(t)
	defer cleanup()

	pcConn := connectAndAuth(t, addr, "9999", "pc", "pc-001", "My PC")
	defer pcConn.Close()

	phoneConn := connectAndAuth(t, addr, "9999", "phone", "phone-001", "Test Phone")
	defer phoneConn.Close()

	// PC should receive phone_online
	readMessage(t, pcConn, 2*time.Second)

	// PC sends SMS command
	smsMsg := protocol.SMSMessage{
		Type:        protocol.TypeSMS,
		MessageID:   "msg-002",
		PhoneNumber: "13800138000",
		Content:     "Hello World",
		DeviceID:    "phone-001",
	}
	data, _ := json.Marshal(smsMsg)
	pcConn.WriteMessage(websocket.TextMessage, data)

	// Phone should receive SMS command
	msg := readMessage(t, phoneConn, 2*time.Second)
	var received protocol.SMSMessage
	json.Unmarshal(msg, &received)
	if received.Content != "Hello World" {
		t.Errorf("Content = %q, want %q", received.Content, "Hello World")
	}

	// Phone sends sms_result
	smsResult := protocol.SMSResult{
		Type:      protocol.TypeSMSResult,
		MessageID: "msg-002",
		Success:   true,
		Error:     "",
	}
	data, _ = json.Marshal(smsResult)
	phoneConn.WriteMessage(websocket.TextMessage, data)

	// PC should receive sms_result
	msg = readMessage(t, pcConn, 2*time.Second)
	var result protocol.SMSResult
	json.Unmarshal(msg, &result)
	if !result.Success {
		t.Error("sms_result.Success should be true")
	}
}

func TestIntegrationPingPong(t *testing.T) {
	addr, cleanup := startTestServer(t)
	defer cleanup()

	pcConn := connectAndAuth(t, addr, "1234", "pc", "pc-001", "My PC")
	defer pcConn.Close()

	// Send ping
	ping := protocol.PingMessage{Type: protocol.TypePing}
	data, _ := json.Marshal(ping)
	pcConn.WriteMessage(websocket.TextMessage, data)

	// Should receive pong
	msg := readMessage(t, pcConn, 2*time.Second)
	msgType, _ := protocol.ParseEnvelope(msg)
	if msgType != protocol.TypePong {
		t.Errorf("Expected pong, got type=%q", msgType)
	}
}

func TestIntegrationPINIsolation(t *testing.T) {
	addr, cleanup := startTestServer(t)
	defer cleanup()

	// Two PCs in different groups
	pc1 := connectAndAuth(t, addr, "1111", "pc", "pc-1", "PC 1")
	defer pc1.Close()

	pc2 := connectAndAuth(t, addr, "2222", "pc", "pc-2", "PC 2")
	defer pc2.Close()

	// Phone joins group 1111
	phone1 := connectAndAuth(t, addr, "1111", "phone", "phone-1", "Phone 1")
	defer phone1.Close()

	// PC1 should receive phone_online
	msg := readMessage(t, pc1, 2*time.Second)
	var online protocol.PhoneOnline
	json.Unmarshal(msg, &online)
	if online.DeviceID != "phone-1" {
		t.Errorf("PC1 got wrong phone: %q", online.DeviceID)
	}

	// PC2 should NOT receive anything (different group)
	pc2.SetReadDeadline(time.Now().Add(500 * time.Millisecond))
	_, _, err := pc2.ReadMessage()
	if err == nil {
		t.Error("PC2 should NOT receive phone_online from different group")
	}
}

func TestIntegrationOnlinePhonesOnAuth(t *testing.T) {
	addr, cleanup := startTestServer(t)
	defer cleanup()

	// Phone connects first
	phoneConn := connectAndAuth(t, addr, "4321", "phone", "phone-001", "First Phone")
	defer phoneConn.Close()

	// PC connects - should get online_phones list in auth result
	dialer := websocket.Dialer{HandshakeTimeout: 5 * time.Second}
	conn, _, err := dialer.Dial(addr, nil)
	if err != nil {
		t.Fatalf("Failed to connect: %v", err)
	}
	defer conn.Close()

	authMsg := protocol.AuthRequest{
		Type:       protocol.TypeAuth,
		PIN:        "4321",
		Role:       "pc",
		DeviceID:   "pc-001",
		DeviceName: "My PC",
	}
	data, _ := json.Marshal(authMsg)
	conn.WriteMessage(websocket.TextMessage, data)

	_, msg, err := conn.ReadMessage()
	if err != nil {
		t.Fatalf("Failed to read auth result: %v", err)
	}

	var result protocol.AuthResult
	json.Unmarshal(msg, &result)
	if !result.Success {
		t.Fatalf("Auth failed: %s", result.Message)
	}
	if len(result.OnlinePhones) != 1 {
		t.Fatalf("OnlinePhones length = %d, want 1", len(result.OnlinePhones))
	}
	if result.OnlinePhones[0].DeviceID != "phone-001" {
		t.Errorf("OnlinePhones[0].DeviceID = %q, want %q", result.OnlinePhones[0].DeviceID, "phone-001")
	}
}

func TestIntegrationDeviceStatus(t *testing.T) {
	addr, cleanup := startTestServer(t)
	defer cleanup()

	pcConn := connectAndAuth(t, addr, "7777", "pc", "pc-001", "My PC")
	defer pcConn.Close()

	phoneConn := connectAndAuth(t, addr, "7777", "phone", "phone-001", "Test Phone")
	defer phoneConn.Close()

	// PC receives phone_online
	readMessage(t, pcConn, 2*time.Second)

	// Phone sends device_status
	status := protocol.DeviceStatus{
		Type:     protocol.TypeDeviceStatus,
		DeviceID: "phone-001",
		Status:   "in_call",
	}
	data, _ := json.Marshal(status)
	phoneConn.WriteMessage(websocket.TextMessage, data)

	// PC should receive device_status
	msg := readMessage(t, pcConn, 2*time.Second)
	var received protocol.DeviceStatus
	json.Unmarshal(msg, &received)
	if received.Status != "in_call" {
		t.Errorf("Status = %q, want %q", received.Status, "in_call")
	}
}

func TestIntegrationHangup(t *testing.T) {
	addr, cleanup := startTestServer(t)
	defer cleanup()

	pcConn := connectAndAuth(t, addr, "6666", "pc", "pc-001", "My PC")
	defer pcConn.Close()

	phoneConn := connectAndAuth(t, addr, "6666", "phone", "phone-001", "Test Phone")
	defer phoneConn.Close()

	// PC receives phone_online
	readMessage(t, pcConn, 2*time.Second)

	// PC sends hangup
	hangupMsg := protocol.HangupMessage{
		Type:      protocol.TypeHangup,
		MessageID: "msg-003",
		DeviceID:  "phone-001",
	}
	data, _ := json.Marshal(hangupMsg)
	pcConn.WriteMessage(websocket.TextMessage, data)

	// Phone should receive hangup
	msg := readMessage(t, phoneConn, 2*time.Second)
	var received protocol.HangupMessage
	json.Unmarshal(msg, &received)
	if received.Type != protocol.TypeHangup {
		t.Errorf("Type = %q, want %q", received.Type, protocol.TypeHangup)
	}
	if received.MessageID != "msg-003" {
		t.Errorf("MessageID = %q, want %q", received.MessageID, "msg-003")
	}
}
