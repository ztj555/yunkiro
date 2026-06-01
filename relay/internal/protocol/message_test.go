package protocol

import (
	"encoding/json"
	"testing"
)

func TestParseEnvelope(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		want    string
		wantErr bool
	}{
		{"auth", `{"type":"auth","pin":"1234"}`, TypeAuth, false},
		{"dial", `{"type":"dial","message_id":"123"}`, TypeDial, false},
		{"ping", `{"type":"ping"}`, TypePing, false},
		{"empty type", `{"type":""}`, "", false},
		{"invalid json", `not json`, "", true},
		{"no type field", `{"foo":"bar"}`, "", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ParseEnvelope([]byte(tt.input))
			if (err != nil) != tt.wantErr {
				t.Fatalf("ParseEnvelope() error = %v, wantErr %v", err, tt.wantErr)
			}
			if got != tt.want {
				t.Errorf("ParseEnvelope() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestValidatePIN(t *testing.T) {
	tests := []struct {
		pin  string
		want bool
	}{
		{"1234", true},
		{"0000", true},
		{"9999", true},
		{"123", false},
		{"12345", false},
		{"abcd", false},
		{"12a4", false},
		{"", false},
		{"123 ", false},
	}

	for _, tt := range tests {
		t.Run(tt.pin, func(t *testing.T) {
			if got := ValidatePIN(tt.pin); got != tt.want {
				t.Errorf("ValidatePIN(%q) = %v, want %v", tt.pin, got, tt.want)
			}
		})
	}
}

func TestAuthRequestSerialization(t *testing.T) {
	msg := AuthRequest{
		Type:       TypeAuth,
		PIN:        "1234",
		Role:       RolePhone,
		DeviceID:   "device-001",
		DeviceName: "My Phone",
	}

	data, err := json.Marshal(msg)
	if err != nil {
		t.Fatalf("Marshal failed: %v", err)
	}

	var parsed AuthRequest
	if err := json.Unmarshal(data, &parsed); err != nil {
		t.Fatalf("Unmarshal failed: %v", err)
	}

	if parsed.Type != TypeAuth {
		t.Errorf("Type = %q, want %q", parsed.Type, TypeAuth)
	}
	if parsed.PIN != "1234" {
		t.Errorf("PIN = %q, want %q", parsed.PIN, "1234")
	}
	if parsed.Role != RolePhone {
		t.Errorf("Role = %q, want %q", parsed.Role, RolePhone)
	}
	if parsed.DeviceID != "device-001" {
		t.Errorf("DeviceID = %q, want %q", parsed.DeviceID, "device-001")
	}
	if parsed.DeviceName != "My Phone" {
		t.Errorf("DeviceName = %q, want %q", parsed.DeviceName, "My Phone")
	}
}

func TestAuthResultSerialization(t *testing.T) {
	msg := AuthResult{
		Type:    TypeAuthResult,
		Success: true,
		Message: "ok",
		OnlinePhones: []PhoneInfo{
			{DeviceID: "phone-1", DeviceName: "Phone 1"},
		},
	}

	data, err := json.Marshal(msg)
	if err != nil {
		t.Fatalf("Marshal failed: %v", err)
	}

	var parsed AuthResult
	if err := json.Unmarshal(data, &parsed); err != nil {
		t.Fatalf("Unmarshal failed: %v", err)
	}

	if !parsed.Success {
		t.Error("Success should be true")
	}
	if len(parsed.OnlinePhones) != 1 {
		t.Fatalf("OnlinePhones length = %d, want 1", len(parsed.OnlinePhones))
	}
	if parsed.OnlinePhones[0].DeviceID != "phone-1" {
		t.Errorf("PhoneInfo.DeviceID = %q, want %q", parsed.OnlinePhones[0].DeviceID, "phone-1")
	}
}

func TestDialMessageSerialization(t *testing.T) {
	msg := DialMessage{
		Type:        TypeDial,
		MessageID:   "msg-001",
		PhoneNumber: "13800138000",
		DeviceID:    "phone-1",
		SIMSlot:     0,
	}

	data, err := json.Marshal(msg)
	if err != nil {
		t.Fatalf("Marshal failed: %v", err)
	}

	var parsed DialMessage
	if err := json.Unmarshal(data, &parsed); err != nil {
		t.Fatalf("Unmarshal failed: %v", err)
	}

	if parsed.PhoneNumber != "13800138000" {
		t.Errorf("PhoneNumber = %q, want %q", parsed.PhoneNumber, "13800138000")
	}
	if parsed.SIMSlot != 0 {
		t.Errorf("SIMSlot = %d, want 0", parsed.SIMSlot)
	}
}

func TestSMSMessageSerialization(t *testing.T) {
	msg := SMSMessage{
		Type:        TypeSMS,
		MessageID:   "msg-002",
		PhoneNumber: "13800138000",
		Content:     "Hello World",
		DeviceID:    "phone-1",
	}

	data, err := json.Marshal(msg)
	if err != nil {
		t.Fatalf("Marshal failed: %v", err)
	}

	var parsed SMSMessage
	if err := json.Unmarshal(data, &parsed); err != nil {
		t.Fatalf("Unmarshal failed: %v", err)
	}

	if parsed.Content != "Hello World" {
		t.Errorf("Content = %q, want %q", parsed.Content, "Hello World")
	}
}

func TestDialResultSerialization(t *testing.T) {
	msg := DialResult{
		Type:      TypeDialResult,
		MessageID: "msg-001",
		Success:   true,
		Error:     "",
	}

	data, err := json.Marshal(msg)
	if err != nil {
		t.Fatalf("Marshal failed: %v", err)
	}

	var parsed DialResult
	if err := json.Unmarshal(data, &parsed); err != nil {
		t.Fatalf("Unmarshal failed: %v", err)
	}

	if !parsed.Success {
		t.Error("Success should be true")
	}
}

func TestPhoneOnlineOfflineSerialization(t *testing.T) {
	online := PhoneOnline{
		Type:       TypePhoneOnline,
		DeviceID:   "phone-1",
		DeviceName: "My Phone",
	}

	data, err := json.Marshal(online)
	if err != nil {
		t.Fatalf("Marshal PhoneOnline failed: %v", err)
	}

	var parsedOnline PhoneOnline
	if err := json.Unmarshal(data, &parsedOnline); err != nil {
		t.Fatalf("Unmarshal PhoneOnline failed: %v", err)
	}
	if parsedOnline.DeviceName != "My Phone" {
		t.Errorf("DeviceName = %q, want %q", parsedOnline.DeviceName, "My Phone")
	}

	offline := PhoneOffline{
		Type:       TypePhoneOffline,
		DeviceID:   "phone-1",
		DeviceName: "My Phone",
	}

	data, err = json.Marshal(offline)
	if err != nil {
		t.Fatalf("Marshal PhoneOffline failed: %v", err)
	}

	var parsedOffline PhoneOffline
	if err := json.Unmarshal(data, &parsedOffline); err != nil {
		t.Fatalf("Unmarshal PhoneOffline failed: %v", err)
	}
	if parsedOffline.Type != TypePhoneOffline {
		t.Errorf("Type = %q, want %q", parsedOffline.Type, TypePhoneOffline)
	}
}

func TestPingPongSerialization(t *testing.T) {
	ping := PingMessage{Type: TypePing}
	data, err := json.Marshal(ping)
	if err != nil {
		t.Fatalf("Marshal Ping failed: %v", err)
	}

	msgType, err := ParseEnvelope(data)
	if err != nil {
		t.Fatalf("ParseEnvelope failed: %v", err)
	}
	if msgType != TypePing {
		t.Errorf("Type = %q, want %q", msgType, TypePing)
	}

	pong := PongMessage{Type: TypePong}
	data, err = json.Marshal(pong)
	if err != nil {
		t.Fatalf("Marshal Pong failed: %v", err)
	}

	msgType, err = ParseEnvelope(data)
	if err != nil {
		t.Fatalf("ParseEnvelope failed: %v", err)
	}
	if msgType != TypePong {
		t.Errorf("Type = %q, want %q", msgType, TypePong)
	}
}

func TestParseAuth(t *testing.T) {
	input := `{"type":"auth","pin":"5678","role":"pc","device_id":"pc-001","device_name":"My PC"}`
	msg, err := ParseAuth([]byte(input))
	if err != nil {
		t.Fatalf("ParseAuth failed: %v", err)
	}
	if msg.PIN != "5678" {
		t.Errorf("PIN = %q, want %q", msg.PIN, "5678")
	}
	if msg.Role != RolePC {
		t.Errorf("Role = %q, want %q", msg.Role, RolePC)
	}
}
