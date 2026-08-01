package handlers

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"hardsend/analyzer"
	"hardsend/config"
	"hardsend/database"
	"hardsend/models"
	"hardsend/parser"
	"hardsend/workers"
)

// CampaignHandler handles campaign-related API endpoints.
type CampaignHandler struct {
	db       *database.DB
	pool     *workers.Pool
	cfg      *config.Config
	analyzer *analyzer.Analyzer
}

// NewCampaignHandler creates a new campaign handler.
func NewCampaignHandler(db *database.DB, pool *workers.Pool, cfg *config.Config) *CampaignHandler {
	return &CampaignHandler{
		db:       db,
		pool:     pool,
		cfg:      cfg,
		analyzer: analyzer.New(db, cfg.WorkerCount),
	}
}

// Analyze handles POST /api/campaigns/analyze.
// Scans a local folder, cross-references with TXT, and creates a persistent campaign plan.
func (h *CampaignHandler) Analyze(w http.ResponseWriter, r *http.Request) {
	var req models.AnalyzeCampaignRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, `{"error":"Cuerpo de request inválido"}`, http.StatusBadRequest)
		return
	}

	if req.FolderPath == "" {
		http.Error(w, `{"error":"folder_path es obligatorio"}`, http.StatusBadRequest)
		return
	}
	if req.TxtPath == "" {
		http.Error(w, `{"error":"txt_path es obligatorio"}`, http.StatusBadRequest)
		return
	}


	// Create the campaign
	now := time.Now()
	campaignID := uuid.New().String()
	campaignName := fmt.Sprintf("Campaña %s", now.Format("Enero 2006"))

	templateSubject := ""
	templateBody := ""
	if req.Template != nil {
		templateSubject = req.Template.Subject
		templateBody = req.Template.BodyText
	}

	campaign := &models.Campaign{
		ID:              campaignID,
		Name:            campaignName,
		FolderPath:      &req.FolderPath,
		TxtPath:         &req.TxtPath,
		DueDate:         req.DueDate,
		TemplateSubject: templateSubject,
		TemplateBody:    templateBody,
		Status:          models.CampaignStatusAnalyzing,
		CreatedAt:       now,
		UpdatedAt:       now,
	}

	if err := h.db.CreateCampaign(campaign); err != nil {
		log.Printf("[Campaign] Failed to create campaign: %v", err)
		http.Error(w, `{"error":"Error al crear la campaña"}`, http.StatusInternalServerError)
		return
	}

	// Run analysis
	result, err := h.analyzer.AnalyzeFolder(campaignID, req.FolderPath, req.TxtPath, req.ForceResend)
	if err != nil {
		_ = h.db.UpdateCampaignStatus(campaignID, models.CampaignStatusDraft)
		log.Printf("[Campaign] Analysis failed: %v", err)
		http.Error(w, fmt.Sprintf(`{"error":"%s"}`, err.Error()), http.StatusBadRequest)
		return
	}

	// Update campaign status to READY
	_ = h.db.UpdateCampaignStatus(campaignID, models.CampaignStatusReady)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(models.CampaignSummary{
		CampaignID:  campaignID,
		Total:       result.Total,
		Valid:       result.Valid,
		NoEmail:     result.NoEmail,
		Skipped:     result.Skipped,
		Status:      models.CampaignStatusReady,
		ElapsedMs:   result.ElapsedMs,
	})
}

// Rescan handles POST /api/campaigns/{id}/rescan.
// Re-scans the folder for new PDFs without re-processing existing ones.
func (h *CampaignHandler) Rescan(w http.ResponseWriter, r *http.Request) {
	campaignID := chi.URLParam(r, "id")
	if campaignID == "" {
		http.Error(w, `{"error":"Campaign ID requerido"}`, http.StatusBadRequest)
		return
	}

	campaign, err := h.db.GetCampaign(campaignID)
	if err != nil {
		http.Error(w, `{"error":"Campaña no encontrada"}`, http.StatusNotFound)
		return
	}

	if campaign.FolderPath == nil || campaign.TxtPath == nil {
		http.Error(w, `{"error":"Esta campaña no tiene carpeta local configurada"}`, http.StatusBadRequest)
		return
	}

	if campaign.Status != models.CampaignStatusReady && campaign.Status != models.CampaignStatusDraft {
		http.Error(w, `{"error":"Solo se pueden re-escanear campañas en estado DRAFT o READY"}`, http.StatusConflict)
		return
	}

	// Check if force resend was requested
	var req struct {
		ForceResend bool `json:"force_resend"`
	}
	_ = json.NewDecoder(r.Body).Decode(&req)

	// Re-run analysis (the analyzer skips already-analyzed invoices)
	result, err := h.analyzer.AnalyzeFolder(campaignID, *campaign.FolderPath, *campaign.TxtPath, req.ForceResend)
	if err != nil {
		log.Printf("[Campaign] Rescan failed: %v", err)
		http.Error(w, fmt.Sprintf(`{"error":"%s"}`, err.Error()), http.StatusBadRequest)
		return
	}

	_ = h.db.UpdateCampaignStatus(campaignID, models.CampaignStatusReady)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(models.CampaignSummary{
		CampaignID:  campaignID,
		Total:       result.Total,
		Valid:       result.Valid,
		NoEmail:     result.NoEmail,
		Skipped:     result.Skipped,
		Status:      models.CampaignStatusReady,
		ElapsedMs:   result.ElapsedMs,
	})
}

// GetActive handles GET /api/campaigns/active.
// Returns the latest non-completed/non-cancelled campaign, or 204 if none exists.
func (h *CampaignHandler) GetActive(w http.ResponseWriter, r *http.Request) {
	campaign, err := h.db.GetActiveCampaign()
	if err != nil {
		http.Error(w, `{"error":"Error al buscar campaña activa"}`, http.StatusInternalServerError)
		return
	}

	if campaign == nil {
		w.WriteHeader(http.StatusNoContent)
		return
	}

	// Get invoices for this campaign
	invoices, err := h.db.GetCampaignInvoices(campaign.ID, "")
	if err != nil {
		invoices = []models.CampaignInvoice{}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(models.CampaignDetailResponse{
		Campaign: campaign,
		Invoices: invoices,
	})
}

// GetCampaign handles GET /api/campaigns/{id}.
func (h *CampaignHandler) GetCampaign(w http.ResponseWriter, r *http.Request) {
	campaignID := chi.URLParam(r, "id")
	if campaignID == "" {
		http.Error(w, `{"error":"Campaign ID requerido"}`, http.StatusBadRequest)
		return
	}

	campaign, err := h.db.GetCampaign(campaignID)
	if err != nil {
		http.Error(w, `{"error":"Campaña no encontrada"}`, http.StatusNotFound)
		return
	}

	statusFilter := r.URL.Query().Get("status")
	invoices, err := h.db.GetCampaignInvoices(campaignID, statusFilter)
	if err != nil {
		invoices = []models.CampaignInvoice{}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(models.CampaignDetailResponse{
		Campaign: campaign,
		Invoices: invoices,
	})
}

// Start handles POST /api/campaigns/{id}/start.
// Takes all QUEUED invoices and dispatches them to the existing worker pool.
func (h *CampaignHandler) Start(w http.ResponseWriter, r *http.Request) {
	campaignID := chi.URLParam(r, "id")
	if campaignID == "" {
		http.Error(w, `{"error":"Campaign ID requerido"}`, http.StatusBadRequest)
		return
	}

	campaign, err := h.db.GetCampaign(campaignID)
	if err != nil {
		http.Error(w, `{"error":"Campaña no encontrada"}`, http.StatusNotFound)
		return
	}

	if campaign.Status != models.CampaignStatusReady && campaign.Status != models.CampaignStatusPaused {
		http.Error(w, fmt.Sprintf(`{"error":"No se puede iniciar una campaña en estado %s. Debe estar en READY o PAUSED."}`, campaign.Status), http.StatusConflict)
		return
	}

	// Get QUEUED invoices
	invoices, err := h.db.GetCampaignInvoices(campaignID, models.CampaignInvoiceStatusQueued)
	if err != nil || len(invoices) == 0 {
		http.Error(w, `{"error":"No hay facturas pendientes para enviar"}`, http.StatusBadRequest)
		return
	}

	// Parse template
	var template *models.EmailTemplate
	if campaign.TemplateSubject != "" || campaign.TemplateBody != "" {
		template = &models.EmailTemplate{
			Subject:  campaign.TemplateSubject,
			BodyText: campaign.TemplateBody,
		}
	}

	// Create a Job (for compatibility with existing worker pool / dashboard metrics)
	jobID := uuid.New().String()
	job := &models.Job{
		ID:         jobID,
		Filename:   fmt.Sprintf("campaign-%s", campaignID[:8]),
		TotalFiles: len(invoices),
		Status:     models.JobStatusProcessing,
		CreatedAt:  time.Now(),
	}
	if err := h.db.CreateJob(job); err != nil {
		http.Error(w, `{"error":"Error al crear el job de envío"}`, http.StatusInternalServerError)
		return
	}

	// Update campaign status
	_ = h.db.UpdateCampaignStatus(campaignID, models.CampaignStatusSending)
	h.pool.SetCurrentJobID(jobID)

	// Dispatch to worker pool in background
	go h.dispatchToWorkerPool(campaignID, jobID, invoices, campaign.DueDate, template)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"job_id":      jobID,
		"campaign_id": campaignID,
		"queued":      len(invoices),
		"message":     "Envío iniciado. Procesando en segundo plano.",
	})
}

// Cancel handles POST /api/campaigns/{id}/cancel.
func (h *CampaignHandler) Cancel(w http.ResponseWriter, r *http.Request) {
	campaignID := chi.URLParam(r, "id")
	if campaignID == "" {
		http.Error(w, `{"error":"Campaign ID requerido"}`, http.StatusBadRequest)
		return
	}

	campaign, err := h.db.GetCampaign(campaignID)
	if err != nil {
		http.Error(w, `{"error":"Campaña no encontrada"}`, http.StatusNotFound)
		return
	}

	if campaign.Status == models.CampaignStatusCompleted || campaign.Status == models.CampaignStatusCancelled {
		http.Error(w, `{"error":"La campaña ya está finalizada"}`, http.StatusConflict)
		return
	}

	// Mark remaining QUEUED invoices as CANCELLED
	affected, _ := h.db.UpdateCampaignInvoicesBulkStatus(campaignID, models.CampaignInvoiceStatusQueued, models.CampaignInvoiceStatusCancelled)
	_ = h.db.UpdateCampaignStatus(campaignID, models.CampaignStatusCancelled)

	log.Printf("[Campaign] Campaign %s cancelled. %d invoices cancelled.", campaignID, affected)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"message":   "Campaña cancelada",
		"cancelled": affected,
	})
}

// dispatchToWorkerPool sends QUEUED campaign invoices to the existing worker pool.
func (h *CampaignHandler) dispatchToWorkerPool(campaignID, jobID string, campaignInvoices []models.CampaignInvoice, dueDate string, template *models.EmailTemplate) {
	var queued int

	for _, ci := range campaignInvoices {
		// Create the invoice record in the existing invoices table (for worker compatibility)
		invoiceID := uuid.New().String()
		invoice := &models.Invoice{
			ID:             invoiceID,
			JobID:          jobID,
			InvoiceNumber:  ci.InvoiceNumber,
			RecipientEmail: ci.Email,
			Status:         models.InvoiceStatusPending,
			Attempts:       0,
		}
		_ = h.db.CreateInvoice(invoice)

		// Submit to worker pool without marking SENT prematurely (for resilience/auto-recovery)
		h.pool.Submit(models.InvoiceJob{
			Invoice:           *invoice,
			PDFPath:           ci.PDFPath,
			JobID:             jobID,
			ClientName:        ci.ClientName,
			DueDate:           dueDate,
			Template:          template,
			CampaignInvoiceID: ci.ID,
		})
		queued++
	}

	log.Printf("[Campaign] Dispatched %d invoices from campaign %s to job %s", queued, campaignID, jobID)

	// Wait for the worker pool to finish and update statuses
	go func() {
		h.pool.WaitForCompletion()
		_ = h.db.UpdateJobStatus(jobID, models.JobStatusCompleted)
		_ = h.db.UpdateCampaignStatus(campaignID, models.CampaignStatusCompleted)
		log.Printf("[Campaign] Campaign %s completed (job %s)", campaignID, jobID)
	}()
}

// ParseTXTFile is a helper used by the TXT file parsing route.
var ParseTXTFile = parser.ParseTXTFile

// ResumeActiveCampaign checks for an interrupted campaign and resumes sending QUEUED invoices.
func (h *CampaignHandler) ResumeActiveCampaign() {
	c, err := h.db.GetActiveCampaign()
	if err != nil || c == nil {
		return
	}
	if c.Status != models.CampaignStatusInProgress {
		return
	}

	pending, err := h.db.GetPendingCampaignInvoices(c.ID)
	if err != nil {
		log.Printf("[Recovery] Failed to fetch pending invoices for campaign %s: %v", c.ID, err)
		return
	}

	if len(pending) == 0 {
		log.Printf("[Recovery] Campaign %s has no pending QUEUED invoices, marking as COMPLETED", c.ID)
		_ = h.db.UpdateCampaignStatus(c.ID, models.CampaignStatusCompleted)
		return
	}

	log.Printf("[Recovery] Found interrupted campaign %s ('%s') with %d pending invoices. Resuming...", c.ID, c.Name, len(pending))

	jobID := uuid.New().String()
	job := &models.Job{
		ID:        jobID,
		Status:    models.JobStatusInProgress,
		StartedAt: time.Now(),
	}
	_ = h.db.CreateJob(job)

	var tmpl *models.EmailTemplate
	if c.TemplateSubject != "" {
		tmpl = &models.EmailTemplate{
			Subject:  c.TemplateSubject,
			BodyText: c.TemplateBody,
		}
	}

	h.dispatchToWorkerPool(c.ID, jobID, pending, c.DueDate, tmpl)
}

