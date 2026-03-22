package handlers

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"

	"hardsend/database"
	"hardsend/models"
)

// HistoryHandler handles monthly history and invoice correction endpoints.
type HistoryHandler struct {
	db *database.DB
}

// NewHistoryHandler creates a new history handler.
func NewHistoryHandler(db *database.DB) *HistoryHandler {
	return &HistoryHandler{db: db}
}

// MonthlyHistoryResponse is the response for GET /api/history/monthly.
type MonthlyHistoryResponse struct {
	Year     int                        `json:"year"`
	Month    int                        `json:"month"`
	Summary  *models.HistorySummary     `json:"summary"`
	Jobs     []models.JobHistory        `json:"jobs"`
	Invoices []models.InvoiceHistoryRow `json:"invoices"`
}

// GetMonthly handles GET /api/history/monthly?year=2026&month=3&status=SUCCESS.
func (h *HistoryHandler) GetMonthly(w http.ResponseWriter, r *http.Request) {
	yearStr := r.URL.Query().Get("year")
	monthStr := r.URL.Query().Get("month")
	statusFilter := r.URL.Query().Get("status")

	now := time.Now()
	year := now.Year()
	month := int(now.Month())

	if yearStr != "" {
		fmt.Sscanf(yearStr, "%d", &year)
	}
	if monthStr != "" {
		fmt.Sscanf(monthStr, "%d", &month)
	}

	from := time.Date(year, time.Month(month), 1, 0, 0, 0, 0, time.Local)
	to := from.AddDate(0, 1, 0).Add(-time.Second)

	// Get summary
	summary, err := h.db.GetHistorySummary(from, to)
	if err != nil {
		http.Error(w, `{"error":"Error al obtener resumen"}`, http.StatusInternalServerError)
		return
	}

	// Get jobs
	jobs, err := h.db.GetJobsByDateRange(from, to)
	if err != nil {
		jobs = []models.JobHistory{}
	}

	// Get invoices for this month with optional status filter
	invoices, err := h.db.GetInvoicesByDateRange(from, to, statusFilter)
	if err != nil {
		invoices = []models.InvoiceHistoryRow{}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(MonthlyHistoryResponse{
		Year:     year,
		Month:    month,
		Summary:  summary,
		Jobs:     jobs,
		Invoices: invoices,
	})
}

// UpdateInvoiceStatusRequest is the request body for PATCH /api/invoices/{id}/status.
type UpdateInvoiceStatusRequest struct {
	Status         string `json:"status"`          // e.g. "MANUAL_SUCCESS"
	Note           string `json:"note"`            // Optional correction note
	CorrectedEmail string `json:"corrected_email"` // If email was wrong, the new one
}

// UpdateInvoiceStatus handles PATCH /api/invoices/{id}/status.
// Allows manual correction of invoice statuses (e.g., marking a bounced invoice as manually resolved).
func (h *HistoryHandler) UpdateInvoiceStatus(w http.ResponseWriter, r *http.Request) {
	invoiceID := chi.URLParam(r, "id")
	if invoiceID == "" {
		http.Error(w, `{"error":"Invoice ID requerido"}`, http.StatusBadRequest)
		return
	}

	var req UpdateInvoiceStatusRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, `{"error":"Cuerpo de request inválido"}`, http.StatusBadRequest)
		return
	}

	// Validate allowed status transitions
	allowedStatuses := map[string]bool{
		models.InvoiceStatusManualSuccess: true,
		models.InvoiceStatusSuccess:       true,
	}
	if !allowedStatuses[req.Status] {
		http.Error(w, fmt.Sprintf(`{"error":"Estado '%s' no permitido para corrección manual"}`, req.Status), http.StatusBadRequest)
		return
	}

	// Build note/reason
	var reason *string
	if req.Note != "" {
		note := fmt.Sprintf("[Corrección manual] %s", req.Note)
		reason = &note
	} else {
		note := "[Corrección manual] Marcada como resuelta por el operador"
		reason = &note
	}

	// Update invoice status
	if err := h.db.UpdateInvoiceStatus(invoiceID, req.Status, reason, 0); err != nil {
		log.Printf("[History] Failed to update invoice %s: %v", invoiceID, err)
		http.Error(w, `{"error":"Error al actualizar la factura"}`, http.StatusInternalServerError)
		return
	}

	log.Printf("[History] Invoice %s manually updated to %s. Note: %s", invoiceID, req.Status, req.Note)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"message":    "Estado actualizado exitosamente",
		"invoice_id": invoiceID,
		"new_status": req.Status,
	})
}

// ResetSystem handles DELETE /api/system/reset.
// Deletes all campaigns, jobs, invoices, missing emails, and daily limits to start fresh.
func (h *HistoryHandler) ResetSystem(w http.ResponseWriter, r *http.Request) {
	if err := h.db.PurgeAllData(); err != nil {
		log.Printf("[System] Error purging database: %v", err)
		http.Error(w, `{"error":"Error al limpiar la base de datos"}`, http.StatusInternalServerError)
		return
	}

	log.Println("[System] Database fully purged by user request.")
	w.WriteHeader(http.StatusOK)
	w.Write([]byte(`{"message": "Sistema reiniciado exitosamente. Todos los datos han sido borrados."}`))
}
