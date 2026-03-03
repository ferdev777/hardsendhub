package models

import (
	"time"
)

// Job status constants.
const (
	JobStatusProcessing = "PROCESSING"
	JobStatusCompleted  = "COMPLETED"
	JobStatusFailed     = "FAILED"
)

// Invoice status constants.
const (
	InvoiceStatusPending         = "PENDING"
	InvoiceStatusProcessing      = "PROCESSING"
	InvoiceStatusSuccess         = "SUCCESS"
	InvoiceStatusErrorValidation = "ERROR_VALIDATION"
	InvoiceStatusErrorNetwork    = "ERROR_NETWORK"
)

// Job represents a batch upload of a ZIP/RAR file.
type Job struct {
	ID         string    `json:"id"`
	Filename   string    `json:"filename"`
	TotalFiles int       `json:"total_files"`
	Status     string    `json:"status"`
	CreatedAt  time.Time `json:"created_at"`
}

// Invoice represents an individual PDF to be sent.
type Invoice struct {
	ID             string     `json:"id"`
	JobID          string     `json:"job_id"`
	InvoiceNumber  string     `json:"invoice_number"`
	RecipientEmail string     `json:"recipient_email"`
	Status         string     `json:"status"`
	ErrorReason    *string    `json:"error_reason"`
	Attempts       int        `json:"attempts"`
	LastAttemptAt  *time.Time `json:"last_attempt_at"`
	Opened         bool       `json:"opened"`
	Bounced        bool       `json:"bounced"`
	Complained     bool       `json:"complained"`
}

// InvoiceJob is passed through worker channels for processing.
type InvoiceJob struct {
	Invoice    Invoice
	PDFPath    string
	JobID      string
	ClientName string
	DueDate    string
}

// MetricsUpdate is the WebSocket payload sent to the frontend every second.
type MetricsUpdate struct {
	JobID                string  `json:"job_id"`
	TotalFiles           int     `json:"total_files"`
	ProcessedCount       int     `json:"processed_count"`
	SuccessCount         int     `json:"success_count"`
	ErrorValidationCount int     `json:"error_validation_count"`
	ErrorNetworkCount    int     `json:"error_network_count"`
	ActiveWorkers        int     `json:"active_workers"`
	SuccessRate          float64 `json:"success_rate"`
	CircuitBreakerState  string  `json:"circuit_breaker_state"`
	OpenedCount          int     `json:"opened_count"`
	BounceCount          int     `json:"bounce_count"`
	ComplaintCount       int     `json:"complaint_count"`
}

// LoginRequest represents the login API payload.
type LoginRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

// LoginResponse is the response after successful authentication.
type LoginResponse struct {
	Token string `json:"token"`
	User  string `json:"user"`
}

// ErrorInvoiceRow is used by the error datagrid in the frontend.
type ErrorInvoiceRow struct {
	ID             string  `json:"id"`
	InvoiceNumber  string  `json:"invoice_number"`
	RecipientEmail string  `json:"recipient_email"`
	Status         string  `json:"status"`
	ErrorReason    *string `json:"error_reason"`
	Attempts       int     `json:"attempts"`
}

// JobHistory is a job with aggregated success/error counts for the history view.
type JobHistory struct {
	ID           string    `json:"id"`
	Filename     string    `json:"filename"`
	TotalFiles   int       `json:"total_files"`
	Status       string    `json:"status"`
	CreatedAt    time.Time `json:"created_at"`
	SuccessCount int       `json:"success_count"`
	ErrorCount   int       `json:"error_count"`
}

// HistorySummary is an aggregated summary for a date range.
type HistorySummary struct {
	TotalJobs     int     `json:"total_jobs"`
	TotalInvoices int     `json:"total_invoices"`
	TotalSuccess  int     `json:"total_success"`
	TotalErrors   int     `json:"total_errors"`
	SuccessRate   float64 `json:"success_rate"`
}

// HistoryResponse combines summary and detailed job list for the history API.
type HistoryResponse struct {
	Summary *HistorySummary `json:"summary"`
	Jobs    []JobHistory    `json:"jobs"`
}

// ActivityEvent represents an engagement event for the live feed.
type ActivityEvent struct {
	Type          string    `json:"type"`       // "activity_event"
	EventType     string    `json:"event_type"` // "email.opened", etc.
	InvoiceNumber string    `json:"invoice_number"`
	Recipient     string    `json:"recipient"`
	CreatedAt     time.Time `json:"created_at"`
}

// MissingEmail represents an invoice where the client had no email in the TXT database
// or where the email bounced after sending.
type MissingEmail struct {
	ID            string     `json:"id"`
	JobID         string     `json:"job_id"`
	InvoiceNumber string     `json:"invoice_number"`
	ClientName    string     `json:"client_name"`
	Email         string     `json:"email"`
	Reason        string     `json:"reason"` // "no_email" or "bounced"
	CreatedAt     time.Time  `json:"created_at"`
	Resolved      bool       `json:"resolved"`
	ResolvedAt    *time.Time `json:"resolved_at"`
}

// MissingEmailSummary holds aggregated stats for the missing emails view.
type MissingEmailSummary struct {
	Total    int `json:"total"`
	Pending  int `json:"pending"`
	Resolved int `json:"resolved"`
}

// MissingEmailResponse combines summary and list for the API response.
type MissingEmailResponse struct {
	Summary *MissingEmailSummary `json:"summary"`
	Items   []MissingEmail       `json:"items"`
}
