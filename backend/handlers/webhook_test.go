package handlers

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"testing"
)

// Dummy db/reputation store for handler tests would normally be mocked,
// but we can test the handler parsing and auth rejection easily.

func TestHandleCloudflareWebhook_Unauthorized(t *testing.T) {
	// Create handler with a svixSecret/CF token configured
	handler := NewWebhookHandler(nil, nil, "super-secret-token")

	req, _ := http.NewRequest("POST", "/api/webhooks/cloudflare", bytes.NewBuffer([]byte(`{}`)))
	req.Header.Set("Authorization", "Bearer wrong-token")

	rr := httptest.NewRecorder()
	handler.HandleCloudflareWebhook(rr, req)

	if status := rr.Code; status != http.StatusUnauthorized {
		t.Errorf("handler returned wrong status code: got %v want %v", status, http.StatusUnauthorized)
	}
}

func TestHandleCloudflareWebhook_Authorized_BadPayload(t *testing.T) {
	handler := NewWebhookHandler(nil, nil, "super-secret-token")

	req, _ := http.NewRequest("POST", "/api/webhooks/cloudflare", bytes.NewBuffer([]byte(`{bad json`)))
	req.Header.Set("Authorization", "Bearer super-secret-token")

	rr := httptest.NewRecorder()
	handler.HandleCloudflareWebhook(rr, req)

	if status := rr.Code; status != http.StatusBadRequest {
		t.Errorf("handler returned wrong status code: got %v want %v", status, http.StatusBadRequest)
	}
}

func TestHandleResendWebhook_Unauthorized(t *testing.T) {
	handler := NewWebhookHandler(nil, nil, "whsec_abcd1234") // dummy svix secret

	req, _ := http.NewRequest("POST", "/api/webhooks/resend", bytes.NewBuffer([]byte(`{}`)))
	
	rr := httptest.NewRecorder()
	handler.HandleResendWebhook(rr, req)

	// Since there are no svix headers, it should fail
	if status := rr.Code; status != http.StatusUnauthorized {
		t.Errorf("handler returned wrong status code: got %v want %v", status, http.StatusUnauthorized)
	}
}
