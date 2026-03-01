package ses

import (
	"context"
	"encoding/base64"
	"fmt"
	"log"
	"mime"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/ses"
	"github.com/aws/aws-sdk-go-v2/service/ses/types"
)

// Client wraps the AWS SES client with rate limiting.
type Client struct {
	sesClient   *ses.Client
	fromAddress string
	rateLimit   int
	rateLimiter chan struct{}
	mu          sync.Mutex
}

// NewClient creates a new SES client with rate limiting.
func NewClient(region, fromAddress string, rateLimit int) (*Client, error) {
	cfg, err := awsconfig.LoadDefaultConfig(context.Background(),
		awsconfig.WithRegion(region),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to load AWS config: %w", err)
	}

	client := &Client{
		sesClient:   ses.NewFromConfig(cfg),
		fromAddress: fromAddress,
		rateLimit:   rateLimit,
		rateLimiter: make(chan struct{}, rateLimit),
	}

	// Start rate limiter token refill
	go client.refillRateLimiter()

	// Pre-fill the rate limiter
	for i := 0; i < rateLimit; i++ {
		client.rateLimiter <- struct{}{}
	}

	log.Printf("[SES] Client initialized. Region: %s, From: %s, Rate limit: %d/s",
		region, fromAddress, rateLimit)

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

// SendInvoiceEmail sends an invoice PDF as an email attachment via SES.
func (c *Client) SendInvoiceEmail(recipientEmail, invoiceNumber, pdfPath, clientName string) error {
	// Wait for rate limiter token
	<-c.rateLimiter

	// Read the PDF file
	pdfData, err := os.ReadFile(pdfPath)
	if err != nil {
		return fmt.Errorf("failed to read PDF file: %w", err)
	}

	// Build the email subject
	subject := "FACTURA MENSUAL VIDEO DIGITAL S.R.L"

	// Build the HTML body matching the company template
	htmlBody := buildInvoiceHTML(clientName, invoiceNumber)

	// Build plain text fallback
	textBody := buildInvoiceText(clientName, invoiceNumber)

	// Build raw MIME email with HTML body and PDF attachment
	rawMessage := buildRawEmail(
		"Video Digital S.R.L <"+c.fromAddress+">",
		recipientEmail,
		subject,
		textBody,
		htmlBody,
		filepath.Base(pdfPath),
		pdfData,
	)

	// Send via SES
	input := &ses.SendRawEmailInput{
		RawMessage: &types.RawMessage{
			Data: []byte(rawMessage),
		},
		Source:       &c.fromAddress,
		Destinations: []string{recipientEmail},
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	_, err = c.sesClient.SendRawEmail(ctx, input)
	if err != nil {
		return fmt.Errorf("SES send failed: %w", err)
	}

	return nil
}

// buildInvoiceHTML generates the HTML email body matching the Video Digital template.
func buildInvoiceHTML(clientName, invoiceNumber string) string {
	// Format current date for due date display
	now := time.Now()
	// Due date: last day of current month
	firstOfNextMonth := time.Date(now.Year(), now.Month()+1, 1, 0, 0, 0, 0, time.Local)
	dueDate := firstOfNextMonth.AddDate(0, 0, -1)
	dueDateStr := fmt.Sprintf("%02d/%02d/%d", dueDate.Day(), dueDate.Month(), dueDate.Year())

	// Client greeting
	greeting := "Estimado Sr/a."
	if clientName != "" {
		greeting = fmt.Sprintf("Estimado Sr/a. %s", clientName)
	}

	html := fmt.Sprintf(`<!DOCTYPE html>
<html lang="es">
<head>
<meta charset="UTF-8">
<meta name="viewport" content="width=device-width, initial-scale=1.0">
</head>
<body style="margin:0;padding:0;background-color:#f4f4f4;font-family:Arial,Helvetica,sans-serif;">
<table role="presentation" width="100%%" cellpadding="0" cellspacing="0" style="background-color:#f4f4f4;padding:20px 0;">
<tr>
<td align="center">
<table role="presentation" width="600" cellpadding="0" cellspacing="0" style="background-color:#1a1a1a;border-radius:12px;overflow:hidden;box-shadow:0 4px 20px rgba(0,0,0,0.3);">

<!-- Header -->
<tr>
<td style="background-color:#111111;padding:30px 40px;text-align:center;border-bottom:3px solid #3b82f6;">
<h1 style="color:#ffffff;margin:0;font-size:22px;font-weight:bold;letter-spacing:1px;">
FACTURA MENSUAL VIDEO DIGITAL S.R.L
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
A continuaci&oacute;n le adjuntamos la factura del servicio CABLE/INTERNET,
con vencimiento el d&iacute;a : <strong style="color:#ffffff;">%s</strong>
</p>
</td>
</tr>

<!-- Invoice Number -->
<tr>
<td style="padding:5px 40px 20px 40px;">
<table role="presentation" cellpadding="0" cellspacing="0" style="background-color:#252525;border-radius:8px;border-left:4px solid #3b82f6;width:100%%;">
<tr>
<td style="padding:15px 20px;">
<p style="color:#9ca3af;font-size:12px;margin:0 0 4px 0;text-transform:uppercase;letter-spacing:1px;">N&uacute;mero de Factura</p>
<p style="color:#ffffff;font-size:18px;font-weight:bold;margin:0;font-family:monospace;">%s</p>
</td>
</tr>
</table>
</td>
</tr>

<!-- Contact -->
<tr>
<td style="padding:10px 40px 25px 40px;">
<p style="color:#d1d5db;font-size:14px;line-height:1.7;margin:0;">
Ante cualquier consulta puede escribirnos a :<br>
<a href="mailto:clientes@videodigital.com.ar" style="color:#3b82f6;text-decoration:none;font-weight:bold;">clientes@videodigital.com.ar</a>
 o
<a href="mailto:ventas@videodigital.com.ar" style="color:#3b82f6;text-decoration:none;font-weight:bold;">ventas@videodigital.com.ar</a>
</p>
</td>
</tr>

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
&copy; 2026 Fernando Hirschfeld &amp; Devrow. Todos los derechos reservados.<br>
Este es un env&iacute;o autom&aacute;tico. Por favor no responda este correo.
</p>
</td>
</tr>

</table>
</td>
</tr>
</table>
</body>
</html>`, greeting, dueDateStr, invoiceNumber)

	return html
}

// buildInvoiceText generates the plain text fallback body.
func buildInvoiceText(clientName, invoiceNumber string) string {
	now := time.Now()
	firstOfNextMonth := time.Date(now.Year(), now.Month()+1, 1, 0, 0, 0, 0, time.Local)
	dueDate := firstOfNextMonth.AddDate(0, 0, -1)
	dueDateStr := fmt.Sprintf("%02d/%02d/%d", dueDate.Day(), dueDate.Month(), dueDate.Year())

	greeting := "Estimado Sr/a."
	if clientName != "" {
		greeting = fmt.Sprintf("Estimado Sr/a. %s", clientName)
	}

	return fmt.Sprintf(`FACTURA MENSUAL VIDEO DIGITAL S.R.L

POR FAVOR NO RESPONDA ESTE MAIL

%s

A continuacion le adjuntamos la factura del servicio CABLE/INTERNET,
con vencimiento el dia : %s

Numero de Factura: %s

Ante cualquier consulta puede escribirnos a :
clientes@videodigital.com.ar o ventas@videodigital.com.ar

Saludos Cordiales

Video Digital S.R.L

---
(c) 2026 Fernando Hirschfeld & Devrow. Todos los derechos reservados.
Este es un envio automatico. Por favor no responda este correo.`, greeting, dueDateStr, invoiceNumber)
}

// buildRawEmail constructs a MIME multipart email with HTML body and PDF attachment.
func buildRawEmail(from, to, subject, textBody, htmlBody, attachmentName string, attachmentData []byte) string {
	mixedBoundary := "NextPart_Mixed_Hardsend_2026"
	altBoundary := "NextPart_Alt_Hardsend_2026"

	var msg strings.Builder

	// Headers
	msg.WriteString(fmt.Sprintf("From: %s\r\n", from))
	msg.WriteString(fmt.Sprintf("To: %s\r\n", to))
	msg.WriteString(fmt.Sprintf("Subject: %s\r\n", mime.QEncoding.Encode("UTF-8", subject)))
	msg.WriteString("MIME-Version: 1.0\r\n")
	msg.WriteString(fmt.Sprintf("Content-Type: multipart/mixed; boundary=\"%s\"\r\n", mixedBoundary))
	msg.WriteString("\r\n")

	// --- Alternative part (text + HTML) ---
	msg.WriteString(fmt.Sprintf("--%s\r\n", mixedBoundary))
	msg.WriteString(fmt.Sprintf("Content-Type: multipart/alternative; boundary=\"%s\"\r\n", altBoundary))
	msg.WriteString("\r\n")

	// Plain text version
	msg.WriteString(fmt.Sprintf("--%s\r\n", altBoundary))
	msg.WriteString("Content-Type: text/plain; charset=UTF-8\r\n")
	msg.WriteString("Content-Transfer-Encoding: quoted-printable\r\n")
	msg.WriteString("\r\n")
	msg.WriteString(textBody)
	msg.WriteString("\r\n\r\n")

	// HTML version
	msg.WriteString(fmt.Sprintf("--%s\r\n", altBoundary))
	msg.WriteString("Content-Type: text/html; charset=UTF-8\r\n")
	msg.WriteString("Content-Transfer-Encoding: quoted-printable\r\n")
	msg.WriteString("\r\n")
	msg.WriteString(htmlBody)
	msg.WriteString("\r\n\r\n")

	// End alternative
	msg.WriteString(fmt.Sprintf("--%s--\r\n", altBoundary))

	// --- PDF attachment ---
	msg.WriteString(fmt.Sprintf("--%s\r\n", mixedBoundary))
	msg.WriteString(fmt.Sprintf("Content-Type: application/pdf; name=\"%s\"\r\n", attachmentName))
	msg.WriteString("Content-Transfer-Encoding: base64\r\n")
	msg.WriteString(fmt.Sprintf("Content-Disposition: attachment; filename=\"%s\"\r\n", attachmentName))
	msg.WriteString("\r\n")

	// Encode PDF in base64 with line wrapping
	encoded := base64.StdEncoding.EncodeToString(attachmentData)
	for i := 0; i < len(encoded); i += 76 {
		end := i + 76
		if end > len(encoded) {
			end = len(encoded)
		}
		msg.WriteString(encoded[i:end])
		msg.WriteString("\r\n")
	}

	// Final boundary
	msg.WriteString(fmt.Sprintf("--%s--\r\n", mixedBoundary))

	return msg.String()
}
