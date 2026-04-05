package websocket

import "encoding/json"

// Message is the envelope for all WebSocket messages.
type Message struct {
	Type    string          `json:"type"`
	Payload json.RawMessage `json:"payload,omitempty"`
}

// Client → Server message types
const (
	MsgSubscribe   = "subscribe"
	MsgUnsubscribe = "unsubscribe"
	MsgPing        = "ping"
)

// Server → Client message types
const (
	MsgVitalReading = "vital_reading"
	MsgAlert        = "alert"
	MsgPong         = "pong"
	MsgError        = "error"
)

// SubscribePayload is the payload for subscribe/unsubscribe messages.
type SubscribePayload struct {
	VitalType string `json:"vital_type"`
}

// ErrorPayload is the payload for error messages.
type ErrorPayload struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}
