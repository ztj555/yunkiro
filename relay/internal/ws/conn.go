package ws

import (
	"sync"
	"time"

	"github.com/gorilla/websocket"
	"github.com/ztj555/yunkiro/relay/internal/logger"
)

const (
	// WriteWait is the time allowed to write a message to the peer.
	WriteWait = 10 * time.Second

	// MaxMessageSize is the maximum message size allowed from peer.
	MaxMessageSize = 64 * 1024

	// SendBufferSize is the channel buffer size for outbound messages.
	SendBufferSize = 256
)

// Conn wraps a gorilla/websocket connection with read/write goroutines.
type Conn struct {
	ws         *websocket.Conn
	send       chan []byte
	done       chan struct{}
	closeOnce  sync.Once
	log        *logger.Logger

	// Client identity, set after auth
	PIN        string
	Role       string
	DeviceID   string
	DeviceName string

	// Heartbeat tracking
	mu           sync.RWMutex
	lastActivity time.Time

	// OnMessage is called when a message is received.
	OnMessage func(conn *Conn, data []byte)

	// OnClose is called when the connection is closed.
	OnClose func(conn *Conn)
}

// NewConn wraps a WebSocket connection.
func NewConn(wsConn *websocket.Conn, log *logger.Logger) *Conn {
	c := &Conn{
		ws:           wsConn,
		send:         make(chan []byte, SendBufferSize),
		done:         make(chan struct{}),
		log:          log,
		lastActivity: time.Now(),
	}
	return c
}

// Start begins the read and write goroutines.
func (c *Conn) Start() {
	go c.readPump()
	go c.writePump()
}

// Send queues a message for writing. Returns false if the connection is closed.
func (c *Conn) Send(data []byte) bool {
	select {
	case c.send <- data:
		return true
	case <-c.done:
		return false
	default:
		// Buffer full, drop message
		c.log.Warn("send buffer full for device=%s, dropping message", c.DeviceID)
		return false
	}
}

// Close gracefully closes the connection.
func (c *Conn) Close() {
	c.closeOnce.Do(func() {
		close(c.done)
		if c.ws != nil {
			c.ws.Close()
		}
		if c.OnClose != nil {
			c.OnClose(c)
		}
	})
}

// LastActivity returns the last activity timestamp.
func (c *Conn) LastActivity() time.Time {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.lastActivity
}

// TouchActivity updates the last activity timestamp.
func (c *Conn) TouchActivity() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.lastActivity = time.Now()
}

func (c *Conn) readPump() {
	defer c.Close()

	c.ws.SetReadLimit(MaxMessageSize)

	for {
		_, message, err := c.ws.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseNormalClosure) {
				c.log.Debug("read error for device=%s: %v", c.DeviceID, err)
			}
			return
		}

		c.TouchActivity()

		if c.OnMessage != nil {
			c.OnMessage(c, message)
		}
	}
}

func (c *Conn) writePump() {
	defer c.Close()

	for {
		select {
		case msg, ok := <-c.send:
			if !ok {
				return
			}
			c.ws.SetWriteDeadline(time.Now().Add(WriteWait))
			if err := c.ws.WriteMessage(websocket.TextMessage, msg); err != nil {
				c.log.Debug("write error for device=%s: %v", c.DeviceID, err)
				return
			}
		case <-c.done:
			return
		}
	}
}

// ReadRaw reads a single raw message from the WebSocket (used during auth handshake).
func (c *Conn) ReadRaw(timeout time.Duration) ([]byte, error) {
	c.ws.SetReadDeadline(time.Now().Add(timeout))
	_, data, err := c.ws.ReadMessage()
	if err != nil {
		return nil, err
	}
	c.ws.SetReadDeadline(time.Time{})
	c.TouchActivity()
	return data, nil
}

// WriteRaw writes a message directly to the WebSocket (used during auth handshake before pumps start).
func (c *Conn) WriteRaw(data []byte) error {
	c.ws.SetWriteDeadline(time.Now().Add(WriteWait))
	return c.ws.WriteMessage(websocket.TextMessage, data)
}
