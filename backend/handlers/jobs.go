package handlers

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"

	"hardsend/database"
	"hardsend/models"
)

// JobsHandler handles job-related API endpoints.
type JobsHandler struct {
	db *database.DB
}

// NewJobsHandler creates a new jobs handler.
func NewJobsHandler(db *database.DB) *JobsHandler {
	return &JobsHandler{db: db}
}

// GetRecentJobs handles GET /api/jobs.
func (h *JobsHandler) GetRecentJobs(w http.ResponseWriter, r *http.Request) {
	jobs, err := h.db.GetRecentJobs(20)
	if err != nil {
		http.Error(w, `{"error":"Failed to fetch jobs"}`, http.StatusInternalServerError)
		return
	}

	if jobs == nil {
		jobs = []models.Job{}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(jobs)
}

// GetJobErrors handles GET /api/jobs/{jobID}/errors.
func (h *JobsHandler) GetJobErrors(w http.ResponseWriter, r *http.Request) {
	jobID := chi.URLParam(r, "jobID")
	if jobID == "" {
		http.Error(w, `{"error":"Job ID required"}`, http.StatusBadRequest)
		return
	}

	errors, err := h.db.GetErrorInvoices(jobID)
	if err != nil {
		http.Error(w, `{"error":"Failed to fetch errors"}`, http.StatusInternalServerError)
		return
	}

	if errors == nil {
		errors = []models.ErrorInvoiceRow{}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(errors)
}

// GetAllErrors handles GET /api/errors.
func (h *JobsHandler) GetAllErrors(w http.ResponseWriter, r *http.Request) {
	errors, err := h.db.GetAllErrorInvoices()
	if err != nil {
		http.Error(w, `{"error":"Failed to fetch errors"}`, http.StatusInternalServerError)
		return
	}

	if errors == nil {
		errors = []models.ErrorInvoiceRow{}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(errors)
}

// GetJobMetrics handles GET /api/jobs/{jobID}/metrics.
func (h *JobsHandler) GetJobMetrics(w http.ResponseWriter, r *http.Request) {
	jobID := chi.URLParam(r, "jobID")
	if jobID == "" {
		http.Error(w, `{"error":"Job ID required"}`, http.StatusBadRequest)
		return
	}

	metrics, err := h.db.GetJobMetrics(jobID)
	if err != nil {
		http.Error(w, `{"error":"Failed to fetch metrics"}`, http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(metrics)
}

// GetHistory handles GET /api/history?period=day|week|month|year|all
func (h *JobsHandler) GetHistory(w http.ResponseWriter, r *http.Request) {
	period := r.URL.Query().Get("period")
	if period == "" {
		period = "month"
	}

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

	to := now

	summary, err := h.db.GetHistorySummary(from, to)
	if err != nil {
		http.Error(w, `{"error":"Failed to fetch history summary"}`, http.StatusInternalServerError)
		return
	}

	jobs, err := h.db.GetJobsByDateRange(from, to)
	if err != nil {
		http.Error(w, `{"error":"Failed to fetch history jobs"}`, http.StatusInternalServerError)
		return
	}

	if jobs == nil {
		jobs = []models.JobHistory{}
	}

	response := models.HistoryResponse{
		Summary: summary,
		Jobs:    jobs,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}
