// Package email defines the EmailProvider interface that all email providers must implement.
// This is the seam between the worker pool and the actual email provider (Resend or Cloudflare).
// Swapping providers = change one line in main.go.
package email

import (
	"context"

	"hardsend/models"
)

// EmailProvider is the contract all email providers must fulfill.
// The worker pool accepts EmailProvider, never a concrete type.
type EmailProvider interface {
	// SendInvoiceEmail sends a single invoice PDF via email.
	// Returns the provider-assigned message ID (used for webhook correlation) and any error.
	SendInvoiceEmail(
		ctx context.Context,
		recipientEmail, invoiceNumber, pdfPath, clientName, invoiceID, dueDate string,
		tmpl *models.EmailTemplate,
	) (providerMessageID string, err error)

	// SendSystemEmail sends a text/html email without attachments (e.g. for reports).
	SendSystemEmail(ctx context.Context, to, subject, htmlBody string) error

	// Name returns the provider identifier for logging and metrics ("resend" or "cloudflare").
	Name() string
}

// Ensure the existing Client still satisfies EmailSender (backward-compat alias).
// New code should use EmailProvider instead.
type EmailSender = EmailProvider
