// Package email provides the Cloudflare email provider.
// Architecture: Go backend POSTs a JSON payload to a Cloudflare Worker (proxy).
// The Worker uses the MailChannels binding (or send_email) to dispatch the email.
// This keeps all email-sending infrastructure on Cloudflare's edge.
//
// Worker contract (POST body):
//
//	{
//	  "to":             "recipient@example.com",
//	  "subject":        "...",
//	  "html":           "...",
//	  "text":           "...",
//	  "attachment_b64": "<base64-encoded PDF>",
//	  "attachment_name": "invoice.pdf",
//	  "idempotency_key": "<invoice_id>"  // Cloudflare Worker dedupes on this
//	}
//
// Worker response (success):
//
//	{"id": "<cloudflare-message-id>", "success": true}
package email

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"sync"
	"time"

	"hardsend/models"
)

const (
	// MaxPDFBytesForCF is 3.4 MB — Base64 overhead (~33%) puts the wire size at ~4.5 MB,
	// safely under the 5 MiB Cloudflare Email message limit.
	MaxPDFBytesForCF = 3_400_000

	// cfClientTimeout is the HTTP timeout for each Worker call.
	cfClientTimeout = 90 * time.Second
)

// cfWorkerPayload is the JSON body sent to the Cloudflare Worker proxy.
type cfWorkerPayload struct {
	To             string `json:"to"`
	From           string `json:"from"`
	FromName       string `json:"from_name"`
	Subject        string `json:"subject"`
	HTML           string `json:"html"`
	Text           string `json:"text"`
	AttachmentB64  string `json:"attachment_b64"`
	AttachmentName string `json:"attachment_name"`
	IdempotencyKey string `json:"idempotency_key"`
}

// cfWorkerResponse is the JSON response from the Cloudflare Worker.
type cfWorkerResponse struct {
	ID      string `json:"id"`
	Success bool   `json:"success"`
	Error   string `json:"error"`
}

// CloudflareClient sends emails via Cloudflare (direct REST API or Worker proxy).
// Implements the EmailProvider interface.
type CloudflareClient struct {
	accountID   string        // Cloudflare Account ID for direct REST API
	apiToken    string        // Cloudflare API Token for direct REST API
	workerURL   string        // Full URL of the Cloudflare Worker (optional if direct API used)
	workerToken string        // Bearer token for authenticating with the Worker
	fromAddress string        // Sender address (must be verified with Cloudflare)
	httpClient  *http.Client
	rateLimiter chan struct{}
	rateLimit   int
	mu          sync.Mutex
}

// Ensure CloudflareClient implements EmailProvider
var _ EmailProvider = (*CloudflareClient)(nil)

// NewCloudflareClient creates a new Cloudflare email client with rate limiting.
func NewCloudflareClient(accountID, apiToken, workerURL, workerToken, fromAddress string, rateLimit int) (*CloudflareClient, error) {
	if workerURL == "" && (accountID == "" || apiToken == "") {
		return nil, fmt.Errorf("either (CF_ACCOUNT_ID and CF_API_TOKEN) or (CF_WORKER_URL and CF_WORKER_TOKEN) must be provided")
	}
	if fromAddress == "" {
		return nil, fmt.Errorf("CF_FROM_ADDRESS is required")
	}
	if rateLimit <= 0 {
		rateLimit = 15
	}

	client := &CloudflareClient{
		accountID:   accountID,
		apiToken:    apiToken,
		workerURL:   workerURL,
		workerToken: workerToken,
		fromAddress: fromAddress,
		httpClient:  &http.Client{Timeout: cfClientTimeout},
		rateLimiter: make(chan struct{}, rateLimit),
		rateLimit:   rateLimit,
	}

	// Pre-fill rate limiter tokens
	for i := 0; i < rateLimit; i++ {
		client.rateLimiter <- struct{}{}
	}
	go client.refillRateLimiter()

	if client.isDirectAPI() {
		log.Printf("[Cloudflare] Direct REST API Client initialized. Account: %s, From: %s, Rate: %d/s",
			accountID, fromAddress, rateLimit)
	} else {
		log.Printf("[Cloudflare] Worker Proxy Client initialized. Worker: %s, From: %s, Rate: %d/s",
			workerURL, fromAddress, rateLimit)
	}

	return client, nil
}

func (c *CloudflareClient) isDirectAPI() bool {
	return c.accountID != "" && c.apiToken != ""
}

// refillRateLimiter refills tokens every second (same pattern as ResendClient).
func (c *CloudflareClient) refillRateLimiter() {
	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()
	for range ticker.C {
		for i := 0; i < c.rateLimit; i++ {
			select {
			case c.rateLimiter <- struct{}{}:
			default:
			}
		}
	}
}

// Name returns the provider identifier.
func (c *CloudflareClient) Name() string {
	return "cloudflare"
}

// SendInvoiceEmail sends an invoice PDF via the Cloudflare Worker proxy.
func (c *CloudflareClient) SendInvoiceEmail(
	ctx context.Context,
	recipientEmail, invoiceNumber, pdfPath, clientName, invoiceID, dueDate string,
	tmpl *models.EmailTemplate,
) (string, error) {
	// Acquire rate limiter token
	select {
	case <-c.rateLimiter:
		// proceed
	case <-ctx.Done():
		return "", ctx.Err()
	}

	// Read and validate PDF size
	pdfData, err := os.ReadFile(pdfPath)
	if err != nil {
		return "", fmt.Errorf("failed to read PDF %s: %w", pdfPath, err)
	}
	if len(pdfData) > MaxPDFBytesForCF {
		return "", fmt.Errorf("PDF exceeds 3.4MB limit for Cloudflare: %s (%.2f MB)",
			pdfPath, float64(len(pdfData))/1_000_000)
	}

	// Build email content (reuse same templates as ResendClient)
	subject := "FACTURA MENSUAL VIDEO DIGITAL S.R.L"
	if tmpl != nil && tmpl.Subject != "" {
		subject = tmpl.Subject
	}
	htmlBody := buildInvoiceHTML(clientName, invoiceNumber, dueDate, tmpl)
	textBody := buildInvoiceText(clientName, invoiceNumber, dueDate, tmpl)

	// Encode PDF as base64
	pdfB64 := base64.StdEncoding.EncodeToString(pdfData)

	var reqURL string
	var authHeader string
	var body []byte

	if c.isDirectAPI() {
		reqURL = fmt.Sprintf("https://api.cloudflare.com/client/v4/accounts/%s/email/sending/send", c.accountID)
		authHeader = "Bearer " + c.apiToken

		directPayload := map[string]interface{}{
			"to":      recipientEmail,
			"from":    c.fromAddress,
			"subject": subject,
			"html":    htmlBody,
			"text":    textBody,
			"attachments": []map[string]string{
				{
					"filename":    filepath.Base(pdfPath),
					"content":     pdfB64,
					"type":        "application/pdf",
					"disposition": "attachment",
				},
			},
		}
		var err error
		body, err = json.Marshal(directPayload)
		if err != nil {
			return "", fmt.Errorf("marshaling CF direct payload: %w", err)
		}
	} else {
		reqURL = c.workerURL
		authHeader = "Bearer " + c.workerToken

		payload := cfWorkerPayload{
			To:             recipientEmail,
			From:           c.fromAddress,
			FromName:       "Video Digital S.R.L",
			Subject:        subject,
			HTML:           htmlBody,
			Text:           textBody,
			AttachmentB64:  pdfB64,
			AttachmentName: filepath.Base(pdfPath),
			IdempotencyKey: invoiceID,
		}
		var err error
		body, err = json.Marshal(payload)
		if err != nil {
			return "", fmt.Errorf("marshaling CF worker payload: %w", err)
		}
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, reqURL, bytes.NewReader(body))
	if err != nil {
		return "", fmt.Errorf("creating CF request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", authHeader)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("CF request failed: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("reading CF response: %w", err)
	}

	// Handle non-2xx: treat 429 as a retriable rate-limit error
	if resp.StatusCode == http.StatusTooManyRequests {
		return "", fmt.Errorf("cloudflare rate limit (429): back off and retry")
	}
	if resp.StatusCode >= 400 {
		return "", fmt.Errorf("CF error %d: %s", resp.StatusCode, string(respBody))
	}

	if c.isDirectAPI() {
		var directResp struct {
			Success bool `json:"success"`
			Errors  []struct {
				Message string `json:"message"`
			} `json:"errors"`
			Result struct {
				MessageID string `json:"message_id"`
			} `json:"result"`
		}
		if err := json.Unmarshal(respBody, &directResp); err == nil && directResp.Success {
			if directResp.Result.MessageID != "" {
				return directResp.Result.MessageID, nil
			}
			return "cf-" + invoiceID, nil
		}
		return "", fmt.Errorf("CF direct API error: %s", string(respBody))
	}

	var workerResp cfWorkerResponse
	if err := json.Unmarshal(respBody, &workerResp); err != nil {
		// Non-JSON success response is still OK if status was 2xx
		return "cf-" + invoiceID, nil
	}

	if !workerResp.Success {
		return "", fmt.Errorf("CF worker reported failure: %s", workerResp.Error)
	}

	messageID := workerResp.ID
	if messageID == "" {
		messageID = "cf-" + invoiceID
	}

	return messageID, nil
}

// SendSystemEmail sends a text/html email without attachments (e.g. for reports).
func (c *CloudflareClient) SendSystemEmail(ctx context.Context, to, subject, htmlBody string) error {
	var reqURL string
	var authHeader string
	var body []byte

	if c.isDirectAPI() {
		reqURL = fmt.Sprintf("https://api.cloudflare.com/client/v4/accounts/%s/email/sending/send", c.accountID)
		authHeader = "Bearer " + c.apiToken

		directPayload := map[string]interface{}{
			"to":      to,
			"from":    c.fromAddress,
			"subject": subject,
			"html":    htmlBody,
			"text":    "Report from Hardsend System",
		}
		var err error
		body, err = json.Marshal(directPayload)
		if err != nil {
			return fmt.Errorf("marshaling CF direct payload: %w", err)
		}
	} else {
		reqURL = c.workerURL
		authHeader = "Bearer " + c.workerToken

		payload := cfWorkerPayload{
			To:             to,
			From:           c.fromAddress,
			FromName:       "Hardsend System",
			Subject:        subject,
			HTML:           htmlBody,
			IdempotencyKey: fmt.Sprintf("report-%d", time.Now().UnixNano()),
		}
		var err error
		body, err = json.Marshal(payload)
		if err != nil {
			return fmt.Errorf("marshaling CF payload: %w", err)
		}
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, reqURL, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("creating CF request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", authHeader)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("CF request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		respBody, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("CF error %d: %s", resp.StatusCode, string(respBody))
	}
	return nil
}
