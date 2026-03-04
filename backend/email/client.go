package email

import (
	"context"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sync"
	"time"

	"hardsend/models"

	"github.com/resend/resend-go/v2"
)

// EmailSender defines the interface for sending emails.
// This allows for mocking in unit tests and follows
// Clean Architecture principles for high-profile Go projects.
type EmailSender interface {
	SendInvoiceEmail(ctx context.Context, recipientEmail, invoiceNumber, pdfPath, clientName, invoiceID, dueDate string, tmpl *models.EmailTemplate) error
}

// Client wraps the Resend client with rate limiting.
// Implements the EmailSender interface.
type Client struct {
	resendClient *resend.Client
	fromAddress  string
	rateLimit    int
	rateLimiter  chan struct{}
	mu           sync.Mutex
}

// Ensure Client implements EmailSender
var _ EmailSender = (*Client)(nil)

// NewClient creates a new Resend client with rate limiting.
func NewClient(apiKey, fromAddress string, rateLimit int) (*Client, error) {
	if apiKey == "" {
		return nil, fmt.Errorf("resend API key is required")
	}

	client := &Client{
		resendClient: resend.NewClient(apiKey),
		fromAddress:  fromAddress,
		rateLimit:    rateLimit,
		rateLimiter:  make(chan struct{}, rateLimit),
	}

	// Start rate limiter token refill
	go client.refillRateLimiter()

	// Pre-fill the rate limiter
	for i := 0; i < rateLimit; i++ {
		client.rateLimiter <- struct{}{}
	}

	log.Printf("[Email] Resend Client initialized. From: %s, Rate limit: %d/s",
		fromAddress, rateLimit)

	return client, nil
}

// refillRateLimiter refills the rate limiter tokens every second.
func (c *Client) refillRateLimiter() {
	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()

	for range ticker.C {
		for i := 0; i < c.rateLimit; i++ {
			select {
			case c.rateLimiter <- struct{}{}:
			default:
				// Channel is full, skip
			}
		}
	}
}

// SendInvoiceEmail sends an invoice PDF as an email attachment via Resend.
// It uses the provided context to respect timeouts and cancellations.
func (c *Client) SendInvoiceEmail(ctx context.Context, recipientEmail, invoiceNumber, pdfPath, clientName, invoiceID, dueDate string, tmpl *models.EmailTemplate) error {
	// Wait for rate limiter token or context cancellation
	select {
	case <-c.rateLimiter:
		// proceed
	case <-ctx.Done():
		return ctx.Err()
	}

	// Read the PDF file
	pdfData, err := os.ReadFile(pdfPath)
	if err != nil {
		return fmt.Errorf("failed to read PDF file at %s: %w", pdfPath, err)
	}

	// Build email content - use template if provided, otherwise defaults
	subject := "FACTURA MENSUAL VIDEO DIGITAL S.R.L"
	if tmpl != nil && tmpl.Subject != "" {
		subject = tmpl.Subject
	}
	htmlBody := buildInvoiceHTML(clientName, invoiceNumber, dueDate, tmpl)
	textBody := buildInvoiceText(clientName, invoiceNumber, dueDate, tmpl)

	// Send via Resend
	params := &resend.SendEmailRequest{
		From:    "Video Digital S.R.L <" + c.fromAddress + ">",
		To:      []string{recipientEmail},
		Subject: subject,
		Html:    htmlBody,
		Text:    textBody,
		Attachments: []*resend.Attachment{
			{
				Filename: filepath.Base(pdfPath),
				Content:  []byte(pdfData),
			},
		},
		Tags: []resend.Tag{
			{
				Name:  "invoice_id",
				Value: invoiceID,
			},
		},
	}

	// For production-grade code, wrapping the network call with context
	// ensures we don't leak goroutines if Resend hangs.
	_, err = c.resendClient.Emails.SendWithContext(ctx, params)
	if err != nil {
		return fmt.Errorf("Resend API error: %w", err)
	}

	return nil
}

// buildInvoiceHTML generates the HTML email body matching the Video Digital template.
func buildInvoiceHTML(clientName, invoiceNumber, dueDate string, tmpl *models.EmailTemplate) string {
	dueDateStr := dueDate

	greeting := "Estimado Sr/a."
	if clientName != "" {
		greeting = fmt.Sprintf("Estimado Sr/a. %s", clientName)
	}

	// Use template body or default
	bodyText := fmt.Sprintf("A continuaci&oacute;n le adjuntamos la factura del servicio CABLE/INTERNET,\ncon vencimiento el d&iacute;a : <strong style=\"color:#ffffff;\">%s</strong>", dueDateStr)
	if tmpl != nil && tmpl.BodyText != "" {
		bodyText = fmt.Sprintf("%s<br><br>Vencimiento: <strong style=\"color:#ffffff;\">%s</strong>", tmpl.BodyText, dueDateStr)
	}

	headerTitle := "FACTURA MENSUAL VIDEO DIGITAL S.R.L"
	if tmpl != nil && tmpl.Subject != "" {
		headerTitle = tmpl.Subject
	}

	// Build apology section only if provided
	apologyHTML := ""
	if tmpl != nil && tmpl.ApologyText != "" {
		apologyHTML = fmt.Sprintf(`
<!-- Apology Notice -->
<tr>
<td style="padding:5px 40px 25px 40px;">
<table role="presentation" cellpadding="0" cellspacing="0" style="background-color:#fef3c7;border-radius:8px;border-left:4px solid #f59e0b;width:100%%;">
<tr>
<td style="padding:15px 20px;">
<p style="color:#92400e;font-size:13px;font-weight:bold;margin:0 0 6px 0;">AVISO IMPORTANTE</p>
<p style="color:#78350f;font-size:13px;line-height:1.6;margin:0;">%s</p>
</td>
</tr>
</table>
</td>
</tr>`, tmpl.ApologyText)
	}

	return fmt.Sprintf(`<!DOCTYPE html>
<html lang="es">
<head>
<meta charset="UTF-8">
<meta name="viewport" content="width=device-width, initial-scale=1.0">
</head>
<body style="margin:0;padding:0;background-color:#f4f4f4;font-family:Arial,Helvetica,sans-serif;">
<table role="presentation" width="100%%%%" cellpadding="0" cellspacing="0" style="background-color:#f4f4f4;padding:20px 0;">
<tr>
<td align="center">
<table role="presentation" width="600" cellpadding="0" cellspacing="0" style="background-color:#1a1a1a;border-radius:12px;overflow:hidden;box-shadow:0 4px 20px rgba(0,0,0,0.3);">

<!-- Header -->
<tr>
<td style="background-color:#111111;padding:30px 40px;text-align:center;border-bottom:3px solid #3b82f6;">
<h1 style="color:#ffffff;margin:0;font-size:22px;font-weight:bold;letter-spacing:1px;">
%s
</h1>
</td>
</tr>

<!-- Warning -->
<tr>
<td style="padding:25px 40px 5px 40px;">
<p style="color:#ef4444;font-size:14px;font-weight:bold;margin:0;">
POR FAVOR NO RESPONDA ESTE MAIL
</p>
</td>
</tr>

<!-- Greeting -->
<tr>
<td style="padding:20px 40px 5px 40px;">
<p style="color:#ffffff;font-size:16px;font-weight:bold;margin:0;">
%s
</p>
</td>
</tr>

<!-- Body -->
<tr>
<td style="padding:20px 40px;">
<p style="color:#d1d5db;font-size:15px;line-height:1.7;margin:0;">
%s
</p>
</td>
</tr>

<!-- Invoice Number -->
<tr>
<td style="padding:5px 40px 20px 40px;">
<table role="presentation" cellpadding="0" cellspacing="0" style="background-color:#252525;border-radius:8px;border-left:4px solid #3b82f6;width:100%%%%;"><tr>
<td style="padding:15px 20px;">
<p style="color:#9ca3af;font-size:12px;margin:0 0 4px 0;text-transform:uppercase;letter-spacing:1px;">N&uacute;mero de Factura</p>
<p style="color:#ffffff;font-size:18px;font-weight:bold;margin:0;font-family:monospace;">%s</p>
</td>
</tr></table>
</td>
</tr>

<!-- Contact -->
<tr>
<td style="padding:10px 40px 15px 40px;">
<p style="color:#d1d5db;font-size:14px;line-height:1.7;margin:0;">
Ante cualquier consulta puede escribirnos a :<br>
<a href="mailto:clientes@videodigital.com.ar" style="color:#3b82f6;text-decoration:none;font-weight:bold;">clientes@videodigital.com.ar</a>
 o
<a href="mailto:ventas@videodigital.com.ar" style="color:#3b82f6;text-decoration:none;font-weight:bold;">ventas@videodigital.com.ar</a>
</p>
</td>
</tr>

%s

<!-- Closing -->
<tr>
<td style="padding:10px 40px;">
<p style="color:#ffffff;font-size:15px;font-weight:500;margin:0;">Saludos Cordiales</p>
</td>
</tr>

<tr>
<td style="padding:5px 40px 30px 40px;">
<p style="color:#9ca3af;font-size:15px;font-weight:bold;margin:0;">Video Digital S.R.L</p>
</td>
</tr>

<!-- Footer -->
<tr>
<td style="background-color:#111111;padding:20px 40px;text-align:center;border-top:1px solid #333333;">
<p style="color:#6b7280;font-size:11px;margin:0;">
Este es un env&iacute;o autom&aacute;tico. Por favor no responda este correo.
</p>
</td>
</tr>

</table>
</td>
</tr>
</table>
</body>
</html>`, headerTitle, greeting, bodyText, invoiceNumber, apologyHTML)
}

// buildInvoiceText generates the plain text fallback body.
func buildInvoiceText(clientName, invoiceNumber, dueDate string, tmpl *models.EmailTemplate) string {
	dueDateStr := dueDate

	greeting := "Estimado Sr/a."
	if clientName != "" {
		greeting = fmt.Sprintf("Estimado Sr/a. %s", clientName)
	}

	header := "FACTURA MENSUAL VIDEO DIGITAL S.R.L"
	if tmpl != nil && tmpl.Subject != "" {
		header = tmpl.Subject
	}

	body := fmt.Sprintf("A continuacion le adjuntamos la factura del servicio CABLE/INTERNET,\ncon vencimiento el dia : %s", dueDateStr)
	if tmpl != nil && tmpl.BodyText != "" {
		body = fmt.Sprintf("%s\nVencimiento: %s", tmpl.BodyText, dueDateStr)
	}

	apology := ""
	if tmpl != nil && tmpl.ApologyText != "" {
		apology = fmt.Sprintf("\nAVISO IMPORTANTE:\n%s\n", tmpl.ApologyText)
	}

	return fmt.Sprintf(`%s

POR FAVOR NO RESPONDA ESTE MAIL

%s

%s

Numero de Factura: %s

Ante cualquier consulta puede escribirnos a :
clientes@videodigital.com.ar o ventas@videodigital.com.ar
%s
Saludos Cordiales

Video Digital S.R.L

---
Este es un envio automatico. Por favor no responda este correo.`, header, greeting, body, invoiceNumber, apology)
}
