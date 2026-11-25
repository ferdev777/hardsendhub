package handlers

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"io"
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"

	"hardsend/database"
	"hardsend/models"
	"hardsend/reputation"
)

type WebhookHandler struct {
	db         *database.DB
	reputation *reputation.Store
	svixSecret string
}

func NewWebhookHandler(db *database.DB, repStore *reputation.Store, svixSecret string) *WebhookHandler {
	return &WebhookHandler{
		db:         db,
		reputation: repStore,
		svixSecret: svixSecret,
	}
}

type ResendWebhookPayload struct {
	Type      string             `json:"type"`
	CreatedAt string             `json:"created_at"`
	Data      ResendWebhookData  `json:"data"`
}

type ResendWebhookData struct {
	CreatedAt string             `json:"created_at"`
	EmailID   string             `json:"email_id"`
	From      string             `json:"from"`
	To        []string           `json:"to"`
	Subject   string             `json:"subject"`
	Tags      []ResendWebhookTag `json:"tags"`
}

type ResendWebhookTag struct {
	Name  string `json:"name"`
	Value string `json:"value"`
}

func (h *WebhookHandler) HandleResendWebhook(w http.ResponseWriter, r *http.Request) {
	body, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, "Failed to read body", http.StatusBadRequest)
		return
	}
	defer r.Body.Close()

	// Verify Svix signature if SVIX_SECRET is set
	if h.svixSecret != "" {
		if !verifySvixSignature(h.svixSecret, r.Header, body) {
			log.Println("[Webhook] Invalid Svix signature rejection")
			http.Error(w, "Invalid webhook signature", http.StatusUnauthorized)
			return
		}
	} else {
		log.Println("[Webhook] Warning: SVIX_SECRET not set, accepting webhook without signature verification")
	}

	var payload ResendWebhookPayload
	if err := json.Unmarshal(body, &payload); err != nil {
		log.Printf("[Webhook] Failed to unmarshal JSON: %v", err)
		http.Error(w, "Invalid JSON payload", http.StatusBadRequest)
		return
	}

	log.Printf("[Webhook] Received event: %s for EmailID: %s", payload.Type, payload.Data.EmailID)

	// Extract invoice_id tag if present
	var invoiceID string
	for _, tag := range payload.Data.Tags {
		if tag.Name == "invoice_id" {
			invoiceID = tag.Value
			break
		}
	}

	eventTime := time.Now()
	if t, err := time.Parse(time.RFC3339Nano, payload.CreatedAt); err == nil {
		eventTime = t
	}

	// Update engagement status in sqlite database
	err = h.db.UpdateCampaignInvoiceEngagement(payload.Data.EmailID, invoiceID, payload.Type, eventTime)
	if err != nil {
		log.Printf("[Webhook] Failed to update engagement for EmailID %s: %v", payload.Data.EmailID, err)
	}

	// If hard bounce or complaint, add to automatic suppression blacklist (Capa 1)
	if (payload.Type == "email.bounced" || payload.Type == "email.complained") && len(payload.Data.To) > 0 {
		email := payload.Data.To[0]
		reason := payload.Type
		if h.reputation != nil {
			_ = h.reputation.MarkBounced(email, reason)
		}
		// Also mark in campaign missing_emails for UX reporting
		me := &models.MissingEmail{
			ID:            "bounce-" + payload.Data.EmailID,
			JobID:         "webhook", // pseudo-job
			InvoiceNumber: invoiceID,
			ClientName:    "Unknown",
			Email:         email,
			Reason:        "bounced",
			CreatedAt:     time.Now(),
		}
		_ = h.db.CreateMissingEmail(me)

		log.Printf("[Webhook] Added %s to blacklist due to %s", email, reason)
	}

	w.WriteHeader(http.StatusOK)
	w.Write([]byte("OK"))
}

// verifySvixSignature checks signature headers from Resend / Svix.
func verifySvixSignature(secret string, headers http.Header, body []byte) bool {
	id := headers.Get("svix-id")
	timestamp := headers.Get("svix-timestamp")
	sigHeader := headers.Get("svix-signature")

	if id == "" || timestamp == "" || sigHeader == "" {
		return false
	}

	// Check timestamp freshness (optional, prevent replay attacks > 5 mins)
	if tsSec, err := strconv.ParseInt(timestamp, 10, 64); err == nil {
		if time.Since(time.Unix(tsSec, 0)) > 5*time.Minute {
			return false
		}
	}

	// Decode whsec_ secret from base64
	cleanSecret := strings.TrimPrefix(secret, "whsec_")
	key, err := base64.StdEncoding.DecodeString(cleanSecret)
	if err != nil {
		return false
	}

	signedPayload := id + "." + timestamp + "." + string(body)

	mac := hmac.New(sha256.New, key)
	mac.Write([]byte(signedPayload))
	expectedSig := base64.StdEncoding.EncodeToString(mac.Sum(nil))

	// Check against any signature in header (v1,base64hash...)
	sigs := strings.Split(sigHeader, " ")
	for _, s := range sigs {
		parts := strings.SplitN(s, ",", 2)
		if len(parts) == 2 && parts[0] == "v1" {
			if hmac.Equal([]byte(parts[1]), []byte(expectedSig)) {
				return true
			}
		}
	}

	return false
}

// CFWebhookPayload is the payload expected from the Cloudflare Worker when MailChannels reports a bounce/delivery.
type CFWebhookPayload struct {
	EmailID   string `json:"email_id"`
	InvoiceID string `json:"invoice_id"`
	Email     string `json:"email"`
	Status    string `json:"status"` // "delivered", "bounced", "complained"
	Reason    string `json:"reason"` // detail if bounced
	Timestamp string `json:"timestamp"`
}

// HandleCloudflareWebhook handles engagement events from the CF Worker.
func (h *WebhookHandler) HandleCloudflareWebhook(w http.ResponseWriter, r *http.Request) {
	// Simple auth via Bearer token (should match CF_WORKER_TOKEN)
	// For production, the worker sends back the same token it requires.
	authHeader := r.Header.Get("Authorization")
	if h.svixSecret != "" && authHeader != "Bearer "+h.svixSecret {
		// Just reusing svixSecret as the shared secret for simplicity, or we could use a new CF_WEBHOOK_SECRET
		log.Println("[Webhook] Invalid CF webhook token")
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	var payload CFWebhookPayload
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}
	defer r.Body.Close()

	log.Printf("[Webhook] CF event: %s for Email: %s (Invoice: %s)", payload.Status, payload.Email, payload.InvoiceID)

	eventTime := time.Now()
	if payload.Timestamp != "" {
		if t, err := time.Parse(time.RFC3339, payload.Timestamp); err == nil {
			eventTime = t
		}
	}

	// Map CF status to internal status format
	eventType := "email." + payload.Status // email.delivered, email.bounced, etc.

	_ = h.db.UpdateCampaignInvoiceEngagement(payload.EmailID, payload.InvoiceID, eventType, eventTime)

	if payload.Status == "bounced" || payload.Status == "complained" {
		if h.reputation != nil {
			_ = h.reputation.MarkBounced(payload.Email, payload.Status)
		}
		me := &models.MissingEmail{
			ID:            "bounce-" + payload.EmailID,
			JobID:         "webhook",
			InvoiceNumber: payload.InvoiceID,
			ClientName:    "Unknown",
			Email:         payload.Email,
			Reason:        payload.Status,
			CreatedAt:     time.Now(),
		}
		_ = h.db.CreateMissingEmail(me)
		log.Printf("[Webhook] Added %s to blacklist due to CF %s", payload.Email, payload.Status)
	}

	w.WriteHeader(http.StatusOK)
}
