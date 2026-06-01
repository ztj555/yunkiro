package protocol

import "encoding/json"

// Message types
const (
	TypeAuth         = "auth"
	TypeAuthResult   = "auth_result"
	TypeDial         = "dial"
	TypeHangup       = "hangup"
	TypeSMS          = "sms"
	TypeDialResult   = "dial_result"
	TypeHangupResult = "hangup_result"
	TypeSMSResult    = "sms_result"
	TypeDeviceStatus = "device_status"
	TypePhoneOnline  = "phone_online"
	TypePhoneOffline = "phone_offline"
	TypeAck          = "ack"
	TypePing         = "ping"
	TypePong         = "pong"
)

// Roles
const (
	RolePC    = "pc"
	RolePhone = "phone"
)

// Envelope is used to peek at the message type before full deserialization.
type Envelope struct {
	Type string `json:"type"`
}

// AuthRequest is sent by clients as the first message after connecting.
type AuthRequest struct {
	Type       string `json:"type"`
	PIN        string `json:"pin"`
	Role       string `json:"role"`
	DeviceID   string `json:"device_id"`
	DeviceName string `json:"device_name"`
}

// AuthResult is sent by the server in response to auth.
type AuthResult struct {
	Type         string        `json:"type"`
	Success      bool          `json:"success"`
	Message      string        `json:"message"`
	OnlinePhones []PhoneInfo   `json:"online_phones,omitempty"`
}

// PhoneInfo describes an online phone.
type PhoneInfo struct {
	DeviceID   string `json:"device_id"`
	DeviceName string `json:"device_name"`
}

// DialMessage is sent by PC to request a phone call.
type DialMessage struct {
	Type        string `json:"type"`
	MessageID   string `json:"message_id"`
	PhoneNumber string `json:"phone_number"`
	DeviceID    string `json:"device_id"`
	SIMSlot     int    `json:"sim_slot"`
}

// HangupMessage is sent by PC to hang up a call.
type HangupMessage struct {
	Type      string `json:"type"`
	MessageID string `json:"message_id"`
	DeviceID  string `json:"device_id"`
}

// SMSMessage is sent by PC to send an SMS.
type SMSMessage struct {
	Type        string `json:"type"`
	MessageID   string `json:"message_id"`
	PhoneNumber string `json:"phone_number"`
	Content     string `json:"content"`
	DeviceID    string `json:"device_id"`
}

// DialResult is sent by phone as result of a dial command.
type DialResult struct {
	Type      string `json:"type"`
	MessageID string `json:"message_id"`
	Success   bool   `json:"success"`
	Error     string `json:"error"`
}

// HangupResult is sent as result of a hangup command.
type HangupResult struct {
	Type      string `json:"type"`
	MessageID string `json:"message_id"`
	Success   bool   `json:"success"`
	Error     string `json:"error"`
}

// SMSResult is sent by phone as result of an sms command.
type SMSResult struct {
	Type      string `json:"type"`
	MessageID string `json:"message_id"`
	Success   bool   `json:"success"`
	Error     string `json:"error"`
}

// DeviceStatus is sent by phone to report its status.
type DeviceStatus struct {
	Type     string `json:"type"`
	DeviceID string `json:"device_id"`
	Status   string `json:"status"`
}

// PhoneOnline is broadcast to PCs when a phone joins the group.
type PhoneOnline struct {
	Type       string `json:"type"`
	DeviceID   string `json:"device_id"`
	DeviceName string `json:"device_name"`
}

// PhoneOffline is broadcast to PCs when a phone leaves the group.
type PhoneOffline struct {
	Type       string `json:"type"`
	DeviceID   string `json:"device_id"`
	DeviceName string `json:"device_name"`
}

// AckMessage acknowledges receipt of a message.
type AckMessage struct {
	Type      string `json:"type"`
	MessageID string `json:"message_id"`
}

// PingMessage is a heartbeat ping.
type PingMessage struct {
	Type string `json:"type"`
}

// PongMessage is a heartbeat pong.
type PongMessage struct {
	Type string `json:"type"`
}

// ParseEnvelope extracts the message type from raw JSON.
func ParseEnvelope(data []byte) (string, error) {
	var env Envelope
	if err := json.Unmarshal(data, &env); err != nil {
		return "", err
	}
	return env.Type, nil
}

// Marshal serializes a message to JSON bytes.
func Marshal(v interface{}) ([]byte, error) {
	return json.Marshal(v)
}

// ParseAuth parses an auth request message.
func ParseAuth(data []byte) (*AuthRequest, error) {
	var msg AuthRequest
	if err := json.Unmarshal(data, &msg); err != nil {
		return nil, err
	}
	return &msg, nil
}

// ValidatePIN checks that a PIN is exactly 4 digits.
func ValidatePIN(pin string) bool {
	if len(pin) != 4 {
		return false
	}
	for _, c := range pin {
		if c < '0' || c > '9' {
			return false
		}
	}
	return true
}
