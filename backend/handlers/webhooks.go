package handlers

import (
	"encoding/json"
	"log"
	"net/http"
	"time"

	"github.com/google/uuid"

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
		w.WriteHeader(http.StatusOK)
		return
	}

	log.Printf("[Webhook] Received %s for invoice %s", payload.Type, invoiceID)

	// Update database
	err := h.db.UpdateEngagementStatus(invoiceID, payload.Type)
	if err != nil {
		log.Printf("[Webhook] Failed to update DB for invoice %s: %v", invoiceID, err)
	}

	// Get invoice details for live feed and bounce tracking
	inv, err := h.db.GetInvoice(invoiceID)
	if err == nil {
		// If it's a bounce, register in missing_emails so it appears in Faltantes
		if payload.Type == "email.bounced" {
			me := &models.MissingEmail{
				ID:            uuid.New().String(),
				JobID:         inv.JobID,
				InvoiceNumber: inv.InvoiceNumber,
				ClientName:    "",
				Email:         payload.Data.Email,
				Reason:        "bounced",
				CreatedAt:     time.Now(),
			}
			_ = h.db.CreateMissingEmail(me)
			log.Printf("[Webhook] Bounce registered for %s (%s) in missing_emails", inv.InvoiceNumber, payload.Data.Email)
		}

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
