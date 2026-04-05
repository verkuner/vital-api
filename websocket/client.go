package websocket

import (
	"encoding/json"
	"log/slog"
	"sync"
	"time"

	"github.com/gorilla/websocket"
	"github.com/org/vital-api/observability"
)

const (
	writeWait      = 10 * time.Second
	pongWait       = 60 * time.Second
	pingPeriod     = 30 * time.Second
	maxMessageSize = 4096
)

// Client represents a single WebSocket connection.
type Client struct {
	hub     *Hub
	conn    *websocket.Conn
	userID  string
	send    chan []byte
	subs    map[string]bool
	subsMu  sync.RWMutex
	logger  *slog.Logger
}

func newClient(hub *Hub, conn *websocket.Conn, userID string, logger *slog.Logger) *Client {
	return &Client{
		hub:    hub,
		conn:   conn,
		userID: userID,
		send:   make(chan []byte, 256),
		subs:   make(map[string]bool),
		logger: logger,
	}
}

func (c *Client) readPump() {
	defer func() {
		c.hub.unregister <- c
		c.conn.Close()
	}()

	c.conn.SetReadLimit(maxMessageSize)
	if err := c.conn.SetReadDeadline(time.Now().Add(pongWait)); err != nil {
		return
	}
	c.conn.SetPongHandler(func(string) error {
		return c.conn.SetReadDeadline(time.Now().Add(pongWait))
	})

	for {
		_, data, err := c.conn.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseNormalClosure) {
				c.logger.Error("websocket read error", slog.String("error", err.Error()), slog.String("user_id", c.userID))
			}
			return
		}
		c.handleMessage(data)
	}
}

func (c *Client) writePump() {
	ticker := time.NewTicker(pingPeriod)
	defer func() {
		ticker.Stop()
		c.conn.Close()
	}()

	for {
		select {
		case msg, ok := <-c.send:
			if err := c.conn.SetWriteDeadline(time.Now().Add(writeWait)); err != nil {
				return
			}
			if !ok {
				c.conn.WriteMessage(websocket.CloseMessage, []byte{})
				return
			}
			if err := c.conn.WriteMessage(websocket.TextMessage, msg); err != nil {
				return
			}
		case <-ticker.C:
			if err := c.conn.SetWriteDeadline(time.Now().Add(writeWait)); err != nil {
				return
			}
			if err := c.conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}
		}
	}
}

func (c *Client) handleMessage(data []byte) {
	var msg Message
	if err := json.Unmarshal(data, &msg); err != nil {
		c.sendError("INVALID_MESSAGE", "malformed JSON")
		return
	}

	switch msg.Type {
	case MsgSubscribe:
		var payload SubscribePayload
		if err := json.Unmarshal(msg.Payload, &payload); err != nil {
			c.sendError("INVALID_PAYLOAD", "invalid subscribe payload")
			return
		}
		c.subsMu.Lock()
		c.subs[payload.VitalType] = true
		c.subsMu.Unlock()

	case MsgUnsubscribe:
		var payload SubscribePayload
		if err := json.Unmarshal(msg.Payload, &payload); err != nil {
			c.sendError("INVALID_PAYLOAD", "invalid unsubscribe payload")
			return
		}
		c.subsMu.Lock()
		delete(c.subs, payload.VitalType)
		c.subsMu.Unlock()

	case MsgPing:
		pong, _ := json.Marshal(Message{Type: MsgPong})
		c.send <- pong

	default:
		c.sendError("INVALID_MESSAGE", "unknown message type: "+msg.Type)
	}
}

func (c *Client) isSubscribed(vitalType string) bool {
	c.subsMu.RLock()
	defer c.subsMu.RUnlock()
	if len(c.subs) == 0 {
		return true
	}
	return c.subs[vitalType]
}

func (c *Client) sendError(code, message string) {
	payload, _ := json.Marshal(ErrorPayload{Code: code, Message: message})
	msg, _ := json.Marshal(Message{Type: MsgError, Payload: payload})
	select {
	case c.send <- msg:
	default:
	}
}

func (c *Client) close() {
	close(c.send)
	observability.WSConnectionClosed()
}
