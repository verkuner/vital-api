package websocket

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestHub_CanConnect_UnderLimit(t *testing.T) {
	hub := NewHub(nil)
	assert.True(t, hub.CanConnect("user-1"))
}

func TestHub_CanConnect_AtLimit(t *testing.T) {
	hub := NewHub(nil)
	hub.mu.Lock()
	hub.userConns["user-1"] = maxConnectionsPerUser
	hub.mu.Unlock()

	assert.False(t, hub.CanConnect("user-1"))
}

func TestHub_UserConnectionCount_NoConnections(t *testing.T) {
	hub := NewHub(nil)
	assert.Equal(t, 0, hub.UserConnectionCount("user-1"))
}

func TestValidateWSToken_ValidToken(t *testing.T) {
	// Minimal JWT with sub claim (not signed, just for parsing test)
	// Header: {"alg":"RS256","typ":"JWT"} -> eyJhbGciOiJSUzI1NiIsInR5cCI6IkpXVCJ9
	// Payload: {"sub":"user-123"} -> eyJzdWIiOiJ1c2VyLTEyMyJ9
	// Signature: test
	token := "eyJhbGciOiJSUzI1NiIsInR5cCI6IkpXVCJ9.eyJzdWIiOiJ1c2VyLTEyMyJ9.test"

	userID, err := validateWSToken(token)
	assert.NoError(t, err)
	assert.Equal(t, "user-123", userID)
}

func TestValidateWSToken_InvalidFormat(t *testing.T) {
	_, err := validateWSToken("not-a-jwt")
	assert.Error(t, err)
}

func TestValidateWSToken_EmptySub(t *testing.T) {
	// Payload: {"sub":""} -> eyJzdWIiOiIifQ
	token := "eyJhbGciOiJSUzI1NiJ9.eyJzdWIiOiIifQ.test"

	_, err := validateWSToken(token)
	assert.Error(t, err)
}
