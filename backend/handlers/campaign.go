package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"hardsend/analyzer"
	"hardsend/config"
	"hardsend/database"
	"hardsend/email"
	"hardsend/models"
	"hardsend/parser"
	"hardsend/reputation"
	"hardsend/storage"
	"hardsend/workers"
)

// CampaignHandler handles campaign-related API endpoints.
type CampaignHandler struct {
	db       *database.DB
	pool     *workers.Pool
	cfg      *config.Config
	analyzer *analyzer.Analyzer
	email    email.EmailProvider
}

// NewCampaignHandler creates a new campaign handler.
func NewCampaignHandler(db *database.DB, pool *workers.Pool, cfg *config.Config, repStore *reputation.Store, mxCache *reputation.MXCache, emailProvider email.EmailProvider) *CampaignHandler {
	return &CampaignHandler{
		db:       db,
		pool:     pool,
		cfg:      cfg,
		analyzer: analyzer.New(db, repStore, mxCache, cfg.WorkerCount),
		email:    emailProvider,
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
		CampaignID:      campaignID,
		Total:           result.Total,
		Valid:           result.Valid,
		NoEmail:         result.NoEmail,
		Skipped:         result.Skipped,
		VerifiedCount:   result.VerifiedCount,
		UntestedCount:   result.UntestedCount,
		FixableCount:    result.FixableCount,
		SuppressedCount: result.SuppressedCount,
		PDFErrorCount:   result.PDFErrorCount,
		Status:          models.CampaignStatusReady,
		ElapsedMs:       result.ElapsedMs,
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
		CampaignID:      campaignID,
		Total:           result.Total,
		Valid:           result.Valid,
		NoEmail:         result.NoEmail,
		Skipped:         result.Skipped,
		VerifiedCount:   result.VerifiedCount,
		UntestedCount:   result.UntestedCount,
		FixableCount:    result.FixableCount,
		SuppressedCount: result.SuppressedCount,
		PDFErrorCount:   result.PDFErrorCount,
		Status:          models.CampaignStatusReady,
		ElapsedMs:       result.ElapsedMs,
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

	if campaign.Status != models.CampaignStatusReady && campaign.Status != models.CampaignStatusPaused && campaign.Status != models.CampaignStatusInProgress {
		http.Error(w, fmt.Sprintf(`{"error":"No se puede iniciar una campaña en estado %s. Debe estar en READY, PAUSED o IN_PROGRESS."}`, campaign.Status), http.StatusConflict)
		return
	}

	// Read optional daily_limit and apology_text from request body
	var startReq struct {
		DailyLimit  int    `json:"daily_limit"`
		ApologyText string `json:"apology_text"`
	}
	_ = json.NewDecoder(r.Body).Decode(&startReq)
	if startReq.DailyLimit > 0 {
		h.cfg.DailyLimit = startReq.DailyLimit
		log.Printf("[Campaign] Daily limit set to %d from frontend", startReq.DailyLimit)
	} else if h.cfg.DailyLimit <= 0 {
		h.cfg.DailyLimit = 1800
		log.Printf("[Campaign] Defaulting daily limit to %d (warm-up)", h.cfg.DailyLimit)
	}

	// Get QUEUED invoices ordered by Capa 3 Wave Priority (Wave 1: verified first, Wave 2: untested second)
	invoices, err := h.db.GetPendingCampaignInvoicesWaveOrdered(campaignID)
	if err != nil || len(invoices) == 0 {
		http.Error(w, `{"error":"No hay facturas pendientes para enviar"}`, http.StatusBadRequest)
		return
	}

	// Parse template
	var template *models.EmailTemplate
	if campaign.TemplateSubject != "" || campaign.TemplateBody != "" || startReq.ApologyText != "" {
		template = &models.EmailTemplate{
			Subject:     campaign.TemplateSubject,
			BodyText:    campaign.TemplateBody,
			ApologyText: startReq.ApologyText,
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

// dispatchToWorkerPool sends QUEUED campaign invoices to the existing worker pool,
// respecting the daily warm-up quota so remaining invoices stay QUEUED in SQLite for the next day.
func (h *CampaignHandler) dispatchToWorkerPool(campaignID, jobID string, campaignInvoices []models.CampaignInvoice, dueDate string, template *models.EmailTemplate) {
	sentToday, savedLimit, _ := h.db.GetDailySendCount()
	effectiveLimit := h.cfg.DailyLimit
	if effectiveLimit <= 0 {
		if savedLimit > 0 {
			effectiveLimit = savedLimit
			h.cfg.DailyLimit = savedLimit
		} else {
			effectiveLimit = 1800
			h.cfg.DailyLimit = 1800
		}
	}

	remainingToday := effectiveLimit - sentToday
	if remainingToday <= 0 {
		log.Printf("[Campaign] Daily quota already reached today (%d/%d). Campaign %s stays IN_PROGRESS for tomorrow's schedule.", sentToday, effectiveLimit, campaignID)
		_ = h.db.UpdateCampaignStatus(campaignID, models.CampaignStatusInProgress)
		_ = h.db.UpdateJobStatus(jobID, models.JobStatusCompleted)
		return
	}

	batch := campaignInvoices
	if len(batch) > remainingToday {
		batch = batch[:remainingToday]
	}
	_ = h.db.UpdateJobTotalFiles(jobID, len(batch))

	var queued int
	for _, ci := range batch {
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

	log.Printf("[Campaign] Dispatched %d/%d wave-ordered invoices from campaign %s to job %s (daily limit: %d)",
		queued, len(campaignInvoices), campaignID, jobID, effectiveLimit)

	// Wait for the worker pool to finish today's batch and update statuses
	go func() {
		h.pool.WaitForCompletion()
		_ = h.db.UpdateJobStatus(jobID, models.JobStatusCompleted)

		stillPending, _ := h.db.GetPendingCampaignInvoicesWaveOrdered(campaignID)
		if len(stillPending) > 0 {
			_ = h.db.UpdateCampaignStatus(campaignID, models.CampaignStatusInProgress)
			log.Printf("[Campaign] Daily batch finished for campaign %s. %d invoices remain QUEUED for next warm-up window.", campaignID, len(stillPending))
			return
		}

		_ = h.db.UpdateCampaignStatus(campaignID, models.CampaignStatusCompleted)
		log.Printf("[Campaign] Campaign %s completed (job %s)", campaignID, jobID)

		// Enviar reporte final
		reportHTML := fmt.Sprintf(`
			<h2>Reporte de Envío de Campaña</h2>
			<p>La campaña <strong>%s</strong> ha finalizado.</p>
			<p>Revisar panel web para ver métricas finales.</p>
		`, campaignID)
		if h.email != nil {
			err := h.email.SendSystemEmail(context.Background(), "admin@videodigital.com.ar", "Campaña Finalizada", reportHTML)
			if err != nil {
				log.Printf("[Campaign] Error sending report email: %v", err)
			} else {
				log.Printf("[Campaign] Report email sent to admin.")
			}
		}
	}()
}

// ParseTXTFile is a helper used by the TXT file parsing route.
var ParseTXTFile = parser.ParseTXTFile

// ResumeActiveCampaign checks for an interrupted or daily-paused campaign and resumes sending QUEUED invoices in wave order.
func (h *CampaignHandler) ResumeActiveCampaign() {
	c, err := h.db.GetActiveCampaign()
	if err != nil || c == nil {
		return
	}
	if c.Status != models.CampaignStatusInProgress && c.Status != models.CampaignStatusSending && c.Status != models.CampaignStatusPaused {
		return
	}

	pending, err := h.db.GetPendingCampaignInvoicesWaveOrdered(c.ID)
	if err != nil {
		log.Printf("[Recovery] Failed to fetch pending invoices for campaign %s: %v", c.ID, err)
		return
	}

	if len(pending) == 0 {
		log.Printf("[Recovery] Campaign %s has no pending QUEUED invoices, marking as COMPLETED", c.ID)
		_ = h.db.UpdateCampaignStatus(c.ID, models.CampaignStatusCompleted)
		return
	}

	log.Printf("[Recovery] Found active campaign %s ('%s') with %d pending wave-ordered invoices. Resuming...", c.ID, c.Name, len(pending))

	jobID := uuid.New().String()
	job := &models.Job{
		ID:         jobID,
		Filename:   fmt.Sprintf("campaign-%s", c.ID[:8]),
		TotalFiles: len(pending),
		Status:     models.JobStatusProcessing,
		CreatedAt:  time.Now(),
	}
	_ = h.db.CreateJob(job)
	_ = h.db.UpdateCampaignStatus(c.ID, models.CampaignStatusSending)
	h.pool.SetCurrentJobID(jobID)

	var tmpl *models.EmailTemplate
	if c.TemplateSubject != "" || c.TemplateBody != "" {
		tmpl = &models.EmailTemplate{
			Subject:  c.TemplateSubject,
			BodyText: c.TemplateBody,
		}
	}

	go h.dispatchToWorkerPool(c.ID, jobID, pending, c.DueDate, tmpl)
}

// DispatchDaily handles POST /api/campaigns/dispatch-daily.
// Fetches the next batch of QUEUED invoices from D1, downloads their PDFs from R2,
// and submits them to the worker pool for today's sending window.
func (h *CampaignHandler) DispatchDaily(w http.ResponseWriter, r *http.Request) {
	cfSync := storage.NewCloudflareSync(h.cfg.CFWorkerURL)
	activeData, err := cfSync.GetActiveCampaignFromD1()
	if err != nil || activeData == nil || activeData["active"] != true {
		http.Error(w, `{"error":"No hay ninguna campaña activa en Cloudflare D1."}`, http.StatusBadRequest)
		return
	}

	campaignInfo, _ := activeData["campaign"].(map[string]interface{})
	todayInfo, _ := activeData["today"].(map[string]interface{})
	campaignID, _ := campaignInfo["id"].(string)

	remainingQuota := 200
	if rq, ok := todayInfo["remaining_quota"].(float64); ok {
		remainingQuota = int(rq)
	}
	dailyLimit := remainingQuota
	if dl, ok := todayInfo["daily_limit"].(float64); ok && int(dl) > 0 {
		dailyLimit = int(dl)
	}

	if remainingQuota <= 0 {
		http.Error(w, `{"error":"El cupo de envíos de hoy ya fue alcanzado. Se restablece a las 00:00 UTC."}`, http.StatusBadRequest)
		return
	}

	// Ensure worker pool daily limit is hydrated from D1 after cold container restarts
	if dailyLimit > h.cfg.DailyLimit {
		h.cfg.DailyLimit = dailyLimit
	}

	// Parse requested limit from body if present
	var req struct {
		Limit int `json:"limit"`
	}
	_ = json.NewDecoder(r.Body).Decode(&req)
	batchSize := remainingQuota
	if req.Limit > 0 && req.Limit < batchSize {
		batchSize = req.Limit
	}

	// Fetch next QUEUED invoices from D1
	invoices, err := cfSync.GetNextQueuedBatchFromD1(campaignID, batchSize)
	if err != nil {
		http.Error(w, fmt.Sprintf(`{"error":"Error al consultar D1: %v"}`, err), http.StatusInternalServerError)
		return
	}

	if len(invoices) == 0 {
		http.Error(w, `{"error":"No hay más facturas en cola para esta campaña. ¡Campaña completada!"}`, http.StatusOK)
		return
	}

	jobID := uuid.New().String()
	job := &models.Job{
		ID:         jobID,
		Filename:   fmt.Sprintf("lote-diario-%s", campaignID),
		TotalFiles: len(invoices),
		Status:     models.JobStatusProcessing,
		CreatedAt:  time.Now(),
	}
	_ = h.db.CreateJob(job)
	h.pool.SetCurrentJobID(jobID)

	// Download PDFs from R2 and submit to pool asynchronously so large daily batches never hit HTTP timeouts
	go func(batch []map[string]interface{}, jobID, cID string) {
		submitted := 0
		for _, inv := range batch {
			invNum, _ := inv["invoice_number"].(string)
			email, _ := inv["recipient_email"].(string)
			clientName, _ := inv["client_name"].(string)
			dueDate, _ := inv["due_date"].(string)

			pdfPath := filepath.Join(h.cfg.TempDir, jobID, fmt.Sprintf("%s.pdf", invNum))
			if err := cfSync.DownloadPDFFromR2(cID, invNum, pdfPath); err != nil {
				log.Printf("[DispatchDaily] Failed to download PDF for invoice %s from R2: %v", invNum, err)
				continue
			}

			invoiceID := uuid.New().String()
			invModel := models.Invoice{
				ID:             invoiceID,
				JobID:          jobID,
				InvoiceNumber:  invNum,
				RecipientEmail: email,
				Status:         models.InvoiceStatusPending,
			}
			_ = h.db.CreateInvoice(&invModel)

			h.pool.Submit(models.InvoiceJob{
				Invoice:    invModel,
				PDFPath:    pdfPath,
				JobID:      jobID,
				ClientName: clientName,
				DueDate:    dueDate,
			})
			submitted++
		}

		if submitted == 0 {
			_ = h.db.UpdateJobStatus(jobID, models.JobStatusFailed)
			log.Printf("[DispatchDaily] Job %s failed: no PDFs could be downloaded from R2", jobID)
			return
		}

		h.pool.WaitForCompletion()
		_ = h.db.UpdateJobStatus(jobID, models.JobStatusCompleted)

		// Mark ONLY actually succeeded invoices as ACCEPTED in D1
		var succeededIDs []string
		rows, err := h.db.RawConn().Query(
			"SELECT invoice_number FROM invoices WHERE job_id = ? AND status = ?",
			jobID, models.InvoiceStatusSuccess,
		)
		if err == nil {
			defer rows.Close()
			for rows.Next() {
				var invNum string
				if err := rows.Scan(&invNum); err == nil {
					succeededIDs = append(succeededIDs, fmt.Sprintf("%s_%s", cID, invNum))
				}
			}
			if err := rows.Err(); err != nil {
				log.Printf("[DispatchDaily] Row iteration error for job %s: %v", jobID, err)
			}
		}

		if len(succeededIDs) > 0 {
			if err := cfSync.MarkBatchSentInD1(cID, succeededIDs); err != nil {
				log.Printf("[DispatchDaily] Failed to mark batch sent in D1 for campaign %s: %v", cID, err)
			}
		}
		_ = os.RemoveAll(filepath.Join(h.cfg.TempDir, jobID))
		log.Printf("[DispatchDaily] Completed dispatch of %d/%d invoices for campaign %s (job %s)", len(succeededIDs), submitted, cID, jobID)
	}(invoices, jobID, campaignID)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"success":     true,
		"job_id":      jobID,
		"campaign_id": campaignID,
		"dispatched":  len(invoices),
		"message":     fmt.Sprintf("Lote diario de %d facturas iniciado en segundo plano.", len(invoices)),
	})
}

