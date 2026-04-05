package websocket

import (
	"encoding/json"
	"log/slog"
	"sync"
)

const maxConnectionsPerUser = 5

// Hub manages all active WebSocket clients.
type Hub struct {
	clients    map[*Client]bool
	userConns  map[string]int
	register   chan *Client
	unregister chan *Client
	broadcast  chan broadcastMsg
	mu         sync.RWMutex
	logger     *slog.Logger
}

type broadcastMsg struct {
	userID    string
	vitalType string
	data      []byte
}

// NewHub creates a new WebSocket hub.
func NewHub(logger *slog.Logger) *Hub {
	return &Hub{
		clients:    make(map[*Client]bool),
		userConns:  make(map[string]int),
		register:   make(chan *Client),
		unregister: make(chan *Client),
		broadcast:  make(chan broadcastMsg, 256),
		logger:     logger,
	}
}

// Run starts the hub's event loop. Call in a goroutine.
func (h *Hub) Run() {
	for {
		select {
		case client := <-h.register:
			h.mu.Lock()
			h.clients[client] = true
			h.userConns[client.userID]++
			h.mu.Unlock()

		case client := <-h.unregister:
			h.mu.Lock()
			if _, ok := h.clients[client]; ok {
				delete(h.clients, client)
				h.userConns[client.userID]--
				if h.userConns[client.userID] <= 0 {
					delete(h.userConns, client.userID)
				}
				client.close()
			}
			h.mu.Unlock()

		case msg := <-h.broadcast:
			h.mu.RLock()
			for client := range h.clients {
				if client.userID == msg.userID && client.isSubscribed(msg.vitalType) {
					select {
					case client.send <- msg.data:
					default:
						go func(c *Client) { h.unregister <- c }(client)
					}
				}
			}
			h.mu.RUnlock()
		}
	}
}

// BroadcastVitalReading sends a vital reading to all subscribed clients for a user.
func (h *Hub) BroadcastVitalReading(userID string, payload interface{}) {
	data, err := json.Marshal(payload)
	if err != nil {
		h.logger.Error("marshal vital reading broadcast", slog.String("error", err.Error()))
		return
	}
	msg, _ := json.Marshal(Message{Type: MsgVitalReading, Payload: data})
	h.broadcast <- broadcastMsg{userID: userID, data: msg}
}

// BroadcastAlert sends an alert to all connected clients for a user.
func (h *Hub) BroadcastAlert(userID string, payload interface{}) {
	data, err := json.Marshal(payload)
	if err != nil {
		h.logger.Error("marshal alert broadcast", slog.String("error", err.Error()))
		return
	}
	msg, _ := json.Marshal(Message{Type: MsgAlert, Payload: data})
	h.broadcast <- broadcastMsg{userID: userID, data: msg}
}

// UserConnectionCount returns the number of active connections for a user.
func (h *Hub) UserConnectionCount(userID string) int {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return h.userConns[userID]
}

// CanConnect checks if a user can open another connection.
func (h *Hub) CanConnect(userID string) bool {
	return h.UserConnectionCount(userID) < maxConnectionsPerUser
}
