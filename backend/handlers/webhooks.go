package handlers

import (
	"encoding/json"
	"log"
	"net/http"
	"time"

	"hardsend/database"
	"hardsend/models"
	"hardsend/websocket"
)

type ResendWebhookPayload struct {
	Type      string    `json:"type"`
	CreatedAt time.Time `json:"created_at"`
	Data      struct {
		Tags []struct {
			Name  string `json:"name"`
			Value string `json:"value"`
		} `json:"tags"`
		Email string `json:"email"`
	} `json:"data"`
}

type WebhookHandler struct {
	db  *database.DB
	hub *websocket.Hub
}

func NewWebhookHandler(db *database.DB, hub *websocket.Hub) *WebhookHandler {
	return &WebhookHandler{
		db:  db,
		hub: hub,
	}
}

func (h *WebhookHandler) ResendWebhook(w http.ResponseWriter, r *http.Request) {
	var payload ResendWebhookPayload
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		log.Printf("[Webhook] Failed to decode payload: %v", err)
		http.Error(w, "Bad request", http.StatusBadRequest)
		return
	}

	var invoiceID string
	for _, tag := range payload.Data.Tags {
		if tag.Name == "invoice_id" {
			invoiceID = tag.Value
			break
		}
	}

	if invoiceID == "" {
		log.Printf("[Webhook] No invoice_id tag found in event: %s", payload.Type)
		w.WriteHeader(http.StatusOK) // Still return OK to avoid retries
		return
	}

	log.Printf("[Webhook] Received %s for invoice %s", payload.Type, invoiceID)

	// Update database
	err := h.db.UpdateEngagementStatus(invoiceID, payload.Type)
	if err != nil {
		log.Printf("[Webhook] Failed to update DB for invoice %s: %v", invoiceID, err)
		// Internal error is fine, but we usually return 200 to webhooks to avoid retries if the event was processed
	}

	// Get invoice details specifically for the live feed
	inv, err := h.db.GetInvoice(invoiceID)
	if err == nil {
		// Broadcast activity event to WebSocket
		event := models.ActivityEvent{
			Type:          "activity_event",
			EventType:     payload.Type,
			InvoiceNumber: inv.InvoiceNumber,
			Recipient:     inv.RecipientEmail,
			CreatedAt:     time.Now(),
		}
		h.hub.Broadcast(event)
	}

	w.WriteHeader(http.StatusOK)
}
