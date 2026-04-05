package websocket

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"strings"

	ws "github.com/gorilla/websocket"
	"github.com/org/vital-api/observability"
)

var upgrader = ws.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	CheckOrigin: func(r *http.Request) bool {
		return true
	},
}

//encore:service
type WebSocketService struct {
	hub    *Hub
	logger *slog.Logger
}

func initWebSocketService() (*WebSocketService, error) {
	logger := slog.Default()
	hub := NewHub(logger)
	go hub.Run()
	return &WebSocketService{hub: hub, logger: logger}, nil
}

// HandleWebSocket upgrades HTTP to WebSocket. Authentication via query param token.
//
//encore:api public raw method=GET path=/ws
func (s *WebSocketService) HandleWebSocket(w http.ResponseWriter, r *http.Request) {
	token := r.URL.Query().Get("token")
	if token == "" {
		http.Error(w, `{"error":"missing token"}`, http.StatusUnauthorized)
		return
	}

	userID, err := validateWSToken(token)
	if err != nil {
		http.Error(w, `{"error":"invalid token"}`, http.StatusUnauthorized)
		return
	}

	if !s.hub.CanConnect(userID) {
		http.Error(w, `{"error":"max connections exceeded"}`, http.StatusTooManyRequests)
		return
	}

	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		s.logger.Error("websocket upgrade failed", slog.String("error", err.Error()))
		return
	}

	client := newClient(s.hub, conn, userID, s.logger)
	s.hub.register <- client
	observability.WSConnectionOpened()

	go client.writePump()
	go client.readPump()
}

// GetHub returns the hub for broadcasting from other services.
func (s *WebSocketService) GetHub() *Hub {
	return s.hub
}

// validateWSToken extracts the sub claim from a JWT for WebSocket auth.
// TODO: Share JWKS cache with authhandler for full signature validation.
func validateWSToken(token string) (string, error) {
	parts := strings.SplitN(token, ".", 3)
	if len(parts) != 3 {
		return "", fmt.Errorf("invalid token format")
	}

	payload, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return "", fmt.Errorf("decode token payload: %w", err)
	}

	var claims struct {
		Sub string `json:"sub"`
	}
	if err := json.Unmarshal(payload, &claims); err != nil {
		return "", fmt.Errorf("parse token claims: %w", err)
	}

	if claims.Sub == "" {
		return "", fmt.Errorf("missing sub claim")
	}

	return claims.Sub, nil
}
