package handlers

import (
	"encoding/csv"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"hardsend/database"
	"hardsend/models"
)

// MissingEmailsHandler handles missing email API endpoints.
type MissingEmailsHandler struct {
	db *database.DB
}

// NewMissingEmailsHandler creates a new missing emails handler.
func NewMissingEmailsHandler(db *database.DB) *MissingEmailsHandler {
	return &MissingEmailsHandler{db: db}
}

// parsePeriod converts a period string to a from/to time range.
func parsePeriod(period string) (time.Time, time.Time) {
	now := time.Now()
	var from time.Time

	switch period {
	case "day":
		from = time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.Local)
	case "week":
		weekday := int(now.Weekday())
		if weekday == 0 {
			weekday = 7
		}
		from = time.Date(now.Year(), now.Month(), now.Day()-weekday+1, 0, 0, 0, 0, time.Local)
	case "month":
		from = time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, time.Local)
	case "year":
		from = time.Date(now.Year(), 1, 1, 0, 0, 0, 0, time.Local)
	case "all":
		from = time.Date(2020, 1, 1, 0, 0, 0, 0, time.Local)
	default:
		from = time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, time.Local)
	}

	return from, now
}

// GetMissingEmails handles GET /api/missing-emails?period=day|week|month|year|all&show_resolved=true|false
func (h *MissingEmailsHandler) GetMissingEmails(w http.ResponseWriter, r *http.Request) {
	period := r.URL.Query().Get("period")
	if period == "" {
		period = "month"
	}
	showResolved := r.URL.Query().Get("show_resolved") == "true"

	from, to := parsePeriod(period)

	items, err := h.db.GetMissingEmails(from, to, showResolved)
	if err != nil {
		http.Error(w, `{"error":"Failed to fetch missing emails"}`, http.StatusInternalServerError)
		return
	}
	if items == nil {
		items = []models.MissingEmail{}
	}

	summary, err := h.db.GetMissingEmailSummary(from, to)
	if err != nil {
		http.Error(w, `{"error":"Failed to fetch summary"}`, http.StatusInternalServerError)
		return
	}

	response := models.MissingEmailResponse{
		Summary: summary,
		Items:   items,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// ExportMissingEmails handles GET /api/missing-emails/export?period=...&show_resolved=...
func (h *MissingEmailsHandler) ExportMissingEmails(w http.ResponseWriter, r *http.Request) {
	period := r.URL.Query().Get("period")
	if period == "" {
		period = "month"
	}
	showResolved := r.URL.Query().Get("show_resolved") == "true"

	from, to := parsePeriod(period)

	items, err := h.db.GetMissingEmails(from, to, showResolved)
	if err != nil {
		http.Error(w, `{"error":"Failed to fetch missing emails"}`, http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "text/csv; charset=utf-8")
	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=emails_faltantes_%s.csv", period))
	// UTF-8 BOM for Excel compatibility
	w.Write([]byte{0xEF, 0xBB, 0xBF})

	writer := csv.NewWriter(w)
	writer.Comma = ';'
	defer writer.Flush()

	// Header
	writer.Write([]string{"Nro Factura", "Cliente", "Email", "Razón", "Fecha", "Estado"})

	for _, item := range items {
		estado := "Pendiente"
		if item.Resolved {
			estado = "Resuelto"
		}
		razon := "Sin email en TXT"
		switch item.Reason {
		case "bounced":
			razon = "Email rebotó"
		case "invalid_email":
			razon = "Email inválido"
		}
		writer.Write([]string{
			item.InvoiceNumber,
			item.ClientName,
			item.Email,
			razon,
			item.CreatedAt.Format("02/01/2006 15:04"),
			estado,
		})
	}
}

// ResolveRequest is the payload for resolve operations.
type ResolveRequest struct {
	IDs    []string `json:"ids"`
	All    bool     `json:"all"`
	Period string   `json:"period"`
}

// ResolveMissingEmails handles POST /api/missing-emails/resolve
func (h *MissingEmailsHandler) ResolveMissingEmails(w http.ResponseWriter, r *http.Request) {
	var req ResolveRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, `{"error":"Invalid request body"}`, http.StatusBadRequest)
		return
	}

	if req.All {
		period := req.Period
		if period == "" {
			period = "month"
		}
		from, to := parsePeriod(period)

		affected, err := h.db.ResolveAllMissingEmails(from, to)
		if err != nil {
			http.Error(w, `{"error":"Failed to resolve emails"}`, http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"message":  fmt.Sprintf("%d registros marcados como resueltos", affected),
			"resolved": affected,
		})
		return
	}

	if len(req.IDs) == 0 {
		http.Error(w, `{"error":"No IDs provided"}`, http.StatusBadRequest)
		return
	}

	if len(req.IDs) == 1 {
		if err := h.db.ResolveMissingEmail(req.IDs[0]); err != nil {
			http.Error(w, `{"error":"Failed to resolve"}`, http.StatusInternalServerError)
			return
		}
	} else {
		if err := h.db.ResolveMissingEmailsBulk(req.IDs); err != nil {
			http.Error(w, `{"error":"Failed to resolve"}`, http.StatusInternalServerError)
			return
		}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"message":  fmt.Sprintf("%d registros marcados como resueltos", len(req.IDs)),
		"resolved": len(req.IDs),
	})
}
