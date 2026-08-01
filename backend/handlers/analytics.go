package handlers

import (
	"encoding/json"
	"net/http"

	"hardsend/config"
	"hardsend/database"
)

type AnalyticsHandler struct {
	db  *database.DB
	cfg *config.Config
}

func NewAnalyticsHandler(db *database.DB, cfg *config.Config) *AnalyticsHandler {
	return &AnalyticsHandler{db: db, cfg: cfg}
}

// GetSummary returns funnel summary statistics for a given period (realtime, today, week, month, year).
func (h *AnalyticsHandler) GetSummary(w http.ResponseWriter, r *http.Request) {
	period := r.URL.Query().Get("period")
	if period == "" {
		period = "today"
	}

	summary, err := h.db.GetAnalyticsSummary(period)
	if err != nil {
		http.Error(w, "Failed to get analytics summary", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(summary)
}

// GetTimeSeries returns aggregated time points for charts.
func (h *AnalyticsHandler) GetTimeSeries(w http.ResponseWriter, r *http.Request) {
	period := r.URL.Query().Get("period")
	if period == "" {
		period = "today"
	}

	points, err := h.db.GetAnalyticsTimeSeries(period)
	if err != nil {
		http.Error(w, "Failed to get analytics time series", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(points)
}

// SyncResend manually fetches email statuses from Resend API and syncs with SQLite.
func (h *AnalyticsHandler) SyncResend(w http.ResponseWriter, r *http.Request) {
	result, err := h.db.SyncResendWithAPI(h.cfg.ResendAPIKey)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(result)
}

