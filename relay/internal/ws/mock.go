package ws

import (
	"time"

	"github.com/ztj555/yunkiro/relay/internal/logger"
)

// NewMockConn creates a Conn suitable for unit testing (no real WebSocket).
// It has initialized send/done channels so Send() works without panicking.
func NewMockConn(pin, role, deviceID, deviceName string, log *logger.Logger) *Conn {
	c := &Conn{
		send:         make(chan []byte, SendBufferSize),
		done:         make(chan struct{}),
		log:          log,
		PIN:          pin,
		Role:         role,
		DeviceID:     deviceID,
		DeviceName:   deviceName,
		lastActivity: time.Now(),
	}
	return c
}

// ReadSent reads one message from the send buffer (for test assertions).
// Returns nil if no message is available.
func (c *Conn) ReadSent() []byte {
	select {
	case msg := <-c.send:
		return msg
	default:
		return nil
	}
}
