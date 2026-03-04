package websocket

import (
	"testing"
)

func TestNewHub(t *testing.T) {
	hub := NewHub("test-secret")
	if hub == nil {
		t.Fatal("NewHub() returned nil")
	}
	if hub.clients == nil {
		t.Error("Hub clients map should be initialized")
	}
	if hub.jwtSecret != "test-secret" {
		t.Errorf("jwtSecret = %q, want test-secret", hub.jwtSecret)
	}
}

func TestHub_Broadcast_NoClients(t *testing.T) {
	hub := NewHub("test-secret")
	go hub.Run()

	// Should not panic when broadcasting with no clients
	hub.Broadcast(map[string]string{"test": "data"})
}
