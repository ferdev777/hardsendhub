package handlers

import (
	"archive/zip"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/google/uuid"
	"github.com/nwaples/rardecode/v2"

	"hardsend/config"
	"hardsend/database"
	"hardsend/models"
	"hardsend/parser"
	"hardsend/reputation"
	"hardsend/storage"
	"hardsend/workers"
)

const (
	defaultR2UploadWorkers = 6
	maxStreamPDFBytes      = 3_400_000
	d1FlushBatchSize       = 50
)

type r2UploadTask struct {
	campaignID   string
	invNum       string
	clientName   string
	email        string
	dueDate      string
	priorityWave int
	pdfBytes     []byte
}

type dispatchCandidate struct {
	invNum   string
	email    string
	filename string
	pdfPath  string
}

type streamSession struct {
	id            string
	jobID         string
	archiveName   string
	dueDate       string
	dailyLimit    int
	forceResend   bool
	template      *models.EmailTemplate
	clientDB      *parser.ClientDB
	pw            *io.PipeWriter
	doneChan      chan struct{}
	quotaReached  int32
	queuedCount   int32
	skippedCount  int32
	blockedCount  int32
	r2FailedCount int32
	err           error
}

// UploadHandler handles file upload and chunked stream operations.
type UploadHandler struct {
	db              *database.DB
	pool            *workers.Pool
	cfg             *config.Config
	repStore        *reputation.Store
	clientDB        *parser.ClientDB
	tempDir         string
	cfSync          *storage.CloudflareSync
	r2UploadWorkers int
	mu              sync.RWMutex
	streams         sync.Map // map[string]*streamSession
}

// NewUploadHandler creates a new upload handler with Anti-Bounce reputation integration.
func NewUploadHandler(db *database.DB, pool *workers.Pool, cfg *config.Config, repStore *reputation.Store, tempDir string) *UploadHandler {
	return &UploadHandler{
		db:              db,
		pool:            pool,
		cfg:             cfg,
		repStore:        repStore,
		clientDB:        parser.NewClientDB(),
		tempDir:         tempDir,
		cfSync:          storage.NewCloudflareSync(cfg.CFWorkerURL),
		r2UploadWorkers: defaultR2UploadWorkers,
	}
}

// SetClientDB updates the client database in a thread-safe way.
func (h *UploadHandler) SetClientDB(clientDB *parser.ClientDB) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.clientDB = clientDB
}

// getClientDB returns the current client database in a thread-safe way.
func (h *UploadHandler) getClientDB() *parser.ClientDB {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return h.clientDB
}

// Upload handles POST /api/upload (supports both standard small uploads and initializing a chunked stream).
func (h *UploadHandler) Upload(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, `{"error":"Method not allowed"}`, http.StatusMethodNotAllowed)
		return
	}

	// Parse multipart form (32MB max memory buffer; larger parts spill to temp disk)
	if err := r.ParseMultipartForm(32 << 20); err != nil {
		http.Error(w, fmt.Sprintf(`{"error":"Failed to parse form: %s"}`, err.Error()), http.StatusBadRequest)
		return
	}

	// Read due date from form (OPTIONAL)
	dueDate := r.FormValue("due_date")

	// Parse email template from form (optional JSON)
	var emailTemplate *models.EmailTemplate
	templateJSON := r.FormValue("email_template")
	if templateJSON != "" {
		emailTemplate = &models.EmailTemplate{}
		if err := json.Unmarshal([]byte(templateJSON), emailTemplate); err != nil {
			log.Printf("[Upload] Invalid email_template JSON: %v", err)
			emailTemplate = nil
		}
	}

	// Parse daily limit (optional, default from config)
	dailyLimitStr := r.FormValue("daily_limit")
	if dailyLimitStr != "" {
		if dl, err := strconv.Atoi(dailyLimitStr); err == nil && dl > 0 {
			h.cfg.DailyLimit = dl
			log.Printf("[Upload] Daily limit set to %d from frontend", dl)
		}
	}
	if h.cfg.DailyLimit <= 0 {
		h.cfg.DailyLimit = 1800
	}

	// Parse force resend flag (optional)
	forceResend := r.FormValue("force_resend") == "true"

	// Check for TXT file upload (client database)
	txtFile, txtHeader, txtErr := r.FormFile("txt_file")
	if txtErr == nil {
		defer txtFile.Close()
		log.Printf("[Upload] Received TXT file: %s", txtHeader.Filename)

		txtData, err := io.ReadAll(txtFile)
		if err != nil {
			http.Error(w, `{"error":"Failed to read TXT file"}`, http.StatusInternalServerError)
			return
		}

		clientDB, err := parser.ParseTXTFromBytes(txtData)
		if err != nil {
			http.Error(w, fmt.Sprintf(`{"error":"Failed to parse TXT file: %s"}`, err.Error()), http.StatusBadRequest)
			return
		}

		h.SetClientDB(clientDB)
		log.Printf("[Upload] Client database loaded: %d entries", clientDB.Size())
	}

	clientDB := h.getClientDB()
	if clientDB.Size() == 0 {
		http.Error(w, `{"error":"Base de datos de clientes (archivo TXT) no cargada. Por favor, suba el archivo TXT primero."}`, http.StatusBadRequest)
		return
	}

	// Check if frontend is initializing a chunked stream for a large archive (>25MB RAR/ZIP)
	if r.FormValue("init_stream") == "true" {
		archiveName := r.FormValue("archive_name")
		if archiveName == "" {
			archiveName = "archive.rar"
		}

		jobID := uuid.New().String()
		job := &models.Job{
			ID:        jobID,
			Filename:  archiveName,
			Status:    models.JobStatusProcessing,
			CreatedAt: time.Now(),
		}
		if err := h.db.CreateJob(job); err != nil {
			http.Error(w, `{"error":"Failed to create job"}`, http.StatusInternalServerError)
			return
		}
		h.pool.SetCurrentJobID(jobID)

		pr, pw := io.Pipe()
		streamID := uuid.New().String()
		sess := &streamSession{
			id:          streamID,
			jobID:       jobID,
			archiveName: archiveName,
			dueDate:     dueDate,
			dailyLimit:  h.cfg.DailyLimit,
			forceResend: forceResend,
			template:    emailTemplate,
			clientDB:    clientDB,
			pw:          pw,
			doneChan:    make(chan struct{}),
		}
		h.streams.Store(streamID, sess)

		go h.consumeArchiveStream(sess, pr)

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"stream_id":     streamID,
			"job_id":        jobID,
			"entries_count": clientDB.Size(),
			"message":       "Stream inicializado. Enviando fragmentos...",
		})
		return
	}

	// If only TXT was uploaded, respond immediately
	files := r.MultipartForm.File["files"]
	if len(files) == 0 {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"message":       "Base de datos de clientes actualizada exitosamente",
			"entries_count": clientDB.Size(),
		})
		return
	}

	// Check if appending to an existing job_id (micro-batch mode) or creating a new job
	jobID := r.FormValue("job_id")
	if jobID == "" {
		jobID = uuid.New().String()
		job := &models.Job{
			ID:        jobID,
			Filename:  files[0].Filename,
			Status:    models.JobStatusProcessing,
			CreatedAt: time.Now(),
		}
		if err := h.db.CreateJob(job); err != nil {
			http.Error(w, `{"error":"Failed to create job"}`, http.StatusInternalServerError)
			return
		}
		h.pool.SetCurrentJobID(jobID)
	}

	// Process uploaded files in background
	go h.processFiles(jobID, files, dueDate, emailTemplate, forceResend)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"job_id":  jobID,
		"message": "Carga iniciada con filtro Anti-Bounce de 4 capas.",
		"files":   len(files),
	})
}

// StreamChunk handles POST /api/upload/chunk?stream_id=...&eof=true|false
// Receives a 20MB slice of a huge RAR/ZIP file and pipes it directly into the streaming extractor.
func (h *UploadHandler) StreamChunk(w http.ResponseWriter, r *http.Request) {
	streamID := r.URL.Query().Get("stream_id")
	isEOF := r.URL.Query().Get("eof") == "true"

	val, ok := h.streams.Load(streamID)
	if !ok {
		http.Error(w, `{"error":"Sesión de stream no encontrada"}`, http.StatusNotFound)
		return
	}
	sess := val.(*streamSession)

	if atomic.LoadInt32(&sess.quotaReached) == 1 {
		_, _ = io.Copy(io.Discard, r.Body)
		h.streams.Delete(streamID)
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"job_id":        sess.jobID,
			"quota_reached": true,
			"queued":        atomic.LoadInt32(&sess.queuedCount),
			"skipped":       atomic.LoadInt32(&sess.skippedCount),
			"blocked":       atomic.LoadInt32(&sess.blockedCount),
			"message":       fmt.Sprintf("Cupo diario alcanzado (%d facturas verificadas encoladas). Deteniendo subida para ahorrar ancho de banda.", atomic.LoadInt32(&sess.queuedCount)),
		})
		return
	}

	var src io.Reader = r.Body
	if r.ContentLength > 0 {
		src = io.LimitReader(r.Body, r.ContentLength)
	}
	_, writeErr := io.Copy(sess.pw, src)
	if writeErr != nil {
		if atomic.LoadInt32(&sess.quotaReached) == 0 {
			log.Printf("[StreamChunk] Pipe write ended: %v", writeErr)
		}
		// Drain remainder of request body so Go/Cloudflare HTTP doesn't abruptly reset the connection
		_, _ = io.Copy(io.Discard, src)
	}

	if isEOF || atomic.LoadInt32(&sess.quotaReached) == 1 {
		_ = sess.pw.Close()
		select {
		case <-sess.doneChan:
		case <-time.After(15 * time.Second):
		}
		h.streams.Delete(streamID)
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"job_id":        sess.jobID,
		"quota_reached": atomic.LoadInt32(&sess.quotaReached) == 1,
		"queued":        atomic.LoadInt32(&sess.queuedCount),
		"skipped":       atomic.LoadInt32(&sess.skippedCount),
		"blocked":       atomic.LoadInt32(&sess.blockedCount),
		"r2_failed":     atomic.LoadInt32(&sess.r2FailedCount),
		"message":       fmt.Sprintf("Procesando lote: %d facturas en cola", atomic.LoadInt32(&sess.queuedCount)),
	})
}

// consumeArchiveStream reads RAR or ZIP entries directly from the pipe,
// persists all valid invoice PDFs to Cloudflare R2 and D1 with bounded concurrency,
// and dispatches today's quota batch prioritized by reputation wave.
func (h *UploadHandler) consumeArchiveStream(sess *streamSession, pr *io.PipeReader) {
	defer close(sess.doneChan)
	defer pr.Close()

	lower := strings.ToLower(sess.archiveName)
	sentToday, _, _ := h.db.GetDailySendCount()
	if activeData, err := h.cfSync.GetActiveCampaignFromD1(); err == nil && activeData != nil {
		if todayInfo, ok := activeData["today"].(map[string]interface{}); ok {
			if d1Sent, ok := todayInfo["sent_count"].(float64); ok && int(d1Sent) > sentToday {
				sentToday = int(d1Sent)
			}
		}
	}
	remainingQuota := sess.dailyLimit - sentToday
	if remainingQuota < 0 {
		remainingQuota = 0
	}

	campaignID := time.Now().Format("2006-01")
	if sess.dueDate != "" {
		parts := strings.Split(sess.dueDate, "/")
		if len(parts) == 3 {
			campaignID = fmt.Sprintf("%s-%s", parts[2], parts[1])
		}
	}
	campaignName := fmt.Sprintf("Facturación %s", campaignID)
	if err := h.cfSync.CreateCampaignInD1(campaignID, campaignName, campaignID, sess.archiveName, 0, sess.dailyLimit); err != nil {
		log.Printf("[Stream] D1 create campaign warning (%s): %v", campaignID, err)
	}

	r2Workers := h.r2UploadWorkers
	if r2Workers <= 0 {
		r2Workers = defaultR2UploadWorkers
	}
	r2Ch := make(chan r2UploadTask, r2Workers*2)
	var r2Wg sync.WaitGroup
	var d1Mu sync.Mutex
	d1Batch := make([]map[string]interface{}, 0, d1FlushBatchSize)

	flushBatch := func(batch []map[string]interface{}) {
		if len(batch) == 0 {
			return
		}
		if err := h.cfSync.IngestInvoicesBatchToD1(campaignID, batch); err != nil {
			log.Printf("[Stream] D1 batch insert warning (%d items): %v", len(batch), err)
		}
	}

	for i := 0; i < r2Workers; i++ {
		r2Wg.Add(1)
		go func() {
			defer r2Wg.Done()
			for task := range r2Ch {
				if err := h.cfSync.UploadPDFToR2(task.campaignID, task.invNum, task.pdfBytes); err != nil {
					atomic.AddInt32(&sess.r2FailedCount, 1)
					log.Printf("[Stream] R2 upload failed permanently for invoice %s: %v", task.invNum, err)
					continue
				}

				entry := map[string]interface{}{
					"invoice_number":  task.invNum,
					"client_code":     "",
					"client_name":     task.clientName,
					"recipient_email": task.email,
					"due_date":        task.dueDate,
					"pdf_r2_key":      fmt.Sprintf("invoices/%s/%s.pdf", task.campaignID, task.invNum),
					"pdf_size":        len(task.pdfBytes),
					"priority_wave":   task.priorityWave,
				}

				var toFlush []map[string]interface{}
				d1Mu.Lock()
				d1Batch = append(d1Batch, entry)
				if len(d1Batch) >= d1FlushBatchSize {
					toFlush = d1Batch
					d1Batch = make([]map[string]interface{}, 0, d1FlushBatchSize)
				}
				d1Mu.Unlock()

				if len(toFlush) > 0 {
					flushBatch(toFlush)
				}

				curQueued := int(atomic.AddInt32(&sess.queuedCount, 1))
				if curQueued%50 == 0 {
					_ = h.db.UpdateJobTotalFiles(sess.jobID, curQueued)
				}
			}
		}()
	}

	var wave1Candidates []dispatchCandidate
	var wave2Candidates []dispatchCandidate

	stageDispatchCandidate := func(priorityWave int, invNum, email, name string, pdfBytes []byte) {
		if remainingQuota <= 0 {
			return
		}
		if priorityWave == 1 {
			if len(wave1Candidates) >= remainingQuota {
				return
			}
		} else {
			if len(wave1Candidates)+len(wave2Candidates) >= remainingQuota {
				return
			}
		}

		pdfPath := filepath.Join(h.tempDir, sess.jobID, name)
		if err := os.MkdirAll(filepath.Dir(pdfPath), 0755); err != nil {
			log.Printf("[Stream] Failed to create temp dir for %s: %v", name, err)
			return
		}
		if err := os.WriteFile(pdfPath, pdfBytes, 0644); err != nil {
			log.Printf("[Stream] Failed to stage temp PDF %s: %v", name, err)
			return
		}

		cand := dispatchCandidate{
			invNum:   invNum,
			email:    email,
			filename: name,
			pdfPath:  pdfPath,
		}

		if priorityWave == 1 {
			wave1Candidates = append(wave1Candidates, cand)
			for len(wave1Candidates)+len(wave2Candidates) > remainingQuota && len(wave2Candidates) > 0 {
				evicted := wave2Candidates[len(wave2Candidates)-1]
				wave2Candidates = wave2Candidates[:len(wave2Candidates)-1]
				_ = os.Remove(evicted.pdfPath)
			}
		} else {
			wave2Candidates = append(wave2Candidates, cand)
		}
	}

	handleExtractedPDF := func(name string, unpackedSize int64, r io.Reader, drainOnSkip bool) {
		email, invNum, priorityWave, shouldPersist := h.evaluateStreamEntry(sess, name, unpackedSize)
		if !shouldPersist {
			if drainOnSkip {
				_, _ = io.Copy(io.Discard, r)
			}
			return
		}

		var pdfBuf bytes.Buffer
		lr := io.LimitReader(r, maxStreamPDFBytes+1)
		if _, err := io.Copy(&pdfBuf, lr); err != nil {
			log.Printf("[Stream] Failed to read PDF %s from archive: %v", name, err)
			atomic.AddInt32(&sess.skippedCount, 1)
			return
		}
		if pdfBuf.Len() > maxStreamPDFBytes {
			atomic.AddInt32(&sess.skippedCount, 1)
			return
		}
		pdfBytes := pdfBuf.Bytes()
		clientName := parser.ExtractClientNameFromFilename(name)

		stageDispatchCandidate(priorityWave, invNum, email, name, pdfBytes)

		r2Ch <- r2UploadTask{
			campaignID:   campaignID,
			invNum:       invNum,
			clientName:   clientName,
			email:        email,
			dueDate:      sess.dueDate,
			priorityWave: priorityWave,
			pdfBytes:     pdfBytes,
		}
	}

	if strings.HasSuffix(lower, ".rar") {
		rr, err := rardecode.NewReader(pr)
		if err != nil {
			log.Printf("[Stream] Failed to init RAR stream reader: %v", err)
			sess.err = err
			close(r2Ch)
			r2Wg.Wait()
			_ = pr.Close()
			return
		}

		for {
			header, err := rr.Next()
			if err == io.EOF {
				break
			}
			if err != nil {
				log.Printf("[Stream] RAR stream ended: %v", err)
				break
			}
			if header.IsDir {
				continue
			}

			name := filepath.Base(header.Name)
			if !strings.HasSuffix(strings.ToLower(name), ".pdf") {
				_, _ = io.Copy(io.Discard, rr)
				continue
			}

			handleExtractedPDF(name, header.UnPackedSize, rr, true)
		}
	} else {
		// Fallback for streamed ZIP: spool to a single temp file on disk
		tmpZip := filepath.Join(h.tempDir, sess.jobID+"_upload.zip")
		_ = os.MkdirAll(h.tempDir, 0755)
		f, err := os.Create(tmpZip)
		if err != nil {
			close(r2Ch)
			r2Wg.Wait()
			_ = pr.Close()
			return
		}
		_, _ = io.Copy(f, pr)
		f.Close()
		defer os.Remove(tmpZip)

		zr, err := zip.OpenReader(tmpZip)
		if err == nil {
			for _, zf := range zr.File {
				if zf.FileInfo().IsDir() {
					continue
				}
				name := filepath.Base(zf.Name)
				if !strings.HasSuffix(strings.ToLower(name), ".pdf") {
					continue
				}
				rc, err := zf.Open()
				if err != nil {
					continue
				}
				handleExtractedPDF(name, int64(zf.UncompressedSize64), rc, false)
				rc.Close()
			}
			zr.Close()
		}
	}

	// Wait for all bounded R2 uploads to complete before final D1 flush and dispatch
	close(r2Ch)
	r2Wg.Wait()

	d1Mu.Lock()
	remainingBatch := d1Batch
	d1Batch = nil
	d1Mu.Unlock()
	flushBatch(remainingBatch)

	finalQueued := int(atomic.LoadInt32(&sess.queuedCount))
	_ = h.db.UpdateJobTotalFiles(sess.jobID, finalQueued)

	log.Printf("[Stream] Ingestion finished for campaign %s (job %s): persisted=%d, r2_failed=%d, skipped=%d, blocked=%d",
		campaignID, sess.jobID, finalQueued, atomic.LoadInt32(&sess.r2FailedCount),
		atomic.LoadInt32(&sess.skippedCount), atomic.LoadInt32(&sess.blockedCount))

	selected := append(wave1Candidates, wave2Candidates...)
	jobsToDispatch := make([]models.InvoiceJob, 0, len(selected))
	for _, cand := range selected {
		jobsToDispatch = append(jobsToDispatch, h.createValidatedInvoiceJob(sess, cand.invNum, cand.email, cand.filename, cand.pdfPath))
	}

	if len(jobsToDispatch) > 0 && h.pool != nil {
		log.Printf("[Stream] Dispatching today's quota batch (%d invoices; wave1=%d, wave2=%d) for job %s...",
			len(jobsToDispatch), len(wave1Candidates), len(wave2Candidates), sess.jobID)
		go func(jobs []models.InvoiceJob, jobID string, cID string) {
			time.Sleep(2 * time.Second)
			for _, j := range jobs {
				h.pool.Submit(j)
			}
			h.pool.WaitForCompletion()
			_ = h.db.UpdateJobStatus(jobID, models.JobStatusCompleted)

			sentIDs := h.collectSucceededD1IDs(jobID, cID)
			if len(sentIDs) > 0 {
				if err := h.cfSync.MarkBatchSentInD1(cID, sentIDs); err != nil {
					log.Printf("[Stream] Failed to mark batch sent in D1 for campaign %s: %v", cID, err)
				}
			}
			_ = os.RemoveAll(filepath.Join(h.tempDir, jobID))
			log.Printf("[Stream] Job %s completed (%d/%d succeeded, synced to D1)", jobID, len(sentIDs), len(jobs))
		}(jobsToDispatch, sess.jobID, campaignID)
	} else {
		_ = h.db.UpdateJobStatus(sess.jobID, models.JobStatusCompleted)
	}
}

// collectSucceededD1IDs queries local SQLite for invoices in jobID that reached SUCCESS
// so failed sends are never falsely marked ACCEPTED in Cloudflare D1.
func (h *UploadHandler) collectSucceededD1IDs(jobID, campaignID string) []string {
	rows, err := h.db.RawConn().Query(
		"SELECT invoice_number FROM invoices WHERE job_id = ? AND status = ?",
		jobID, models.InvoiceStatusSuccess,
	)
	if err != nil {
		log.Printf("[Stream] Failed to query succeeded invoices for job %s: %v", jobID, err)
		return nil
	}
	defer rows.Close()

	var sentIDs []string
	for rows.Next() {
		var invNum string
		if err := rows.Scan(&invNum); err == nil {
			sentIDs = append(sentIDs, fmt.Sprintf("%s_%s", campaignID, invNum))
		}
	}
	if err := rows.Err(); err != nil {
		log.Printf("[Stream] Row iteration error for job %s: %v", jobID, err)
	}
	return sentIDs
}

// evaluateStreamEntry applies Anti-Bounce Layers 1 and 2 on a filename before reading the PDF
// and assigns its D1 priority wave (1 = verified reputation, 2 = untested/standard).
func (h *UploadHandler) evaluateStreamEntry(sess *streamSession, filename string, unpackedSize int64) (string, string, int, bool) {
	lower := strings.ToLower(filename)
	if strings.Contains(lower, "nota de cr") {
		atomic.AddInt32(&sess.skippedCount, 1)
		return "", "", 0, false
	}

	invNum, err := parser.ExtractInvoiceNumber(filename)
	if err != nil || parser.IsTypeXInvoice(invNum) {
		atomic.AddInt32(&sess.skippedCount, 1)
		return "", "", 0, false
	}

	if unpackedSize > maxStreamPDFBytes {
		atomic.AddInt32(&sess.skippedCount, 1)
		return "", "", 0, false
	}

	rawEmail, found := sess.clientDB.GetEmail(invNum)
	if !found {
		atomic.AddInt32(&sess.skippedCount, 1)
		return "", "", 0, false
	}

	// Capa 2: Sanitize
	san := reputation.Sanitize(rawEmail)
	if san.IsInvalid || !parser.ValidateEmail(san.Fixed) {
		atomic.AddInt32(&sess.blockedCount, 1)
		return "", "", 0, false
	}
	email := san.Fixed

	// Capa 1: Reputation & Blacklist check
	if h.repStore != nil && h.repStore.IsBlocked(email) {
		atomic.AddInt32(&sess.blockedCount, 1)
		return "", "", 0, false
	}
	if h.db.IsBlacklisted(email) {
		atomic.AddInt32(&sess.blockedCount, 1)
		return "", "", 0, false
	}

	// Idempotency check
	if !sess.forceResend {
		if sent, err := h.db.CheckInvoiceSentThisMonth(invNum); err == nil && sent {
			atomic.AddInt32(&sess.skippedCount, 1)
			return "", "", 0, false
		}
	}

	// Capa 3: Assign wave priority instead of discarding unverified emails from R2/D1 persistence.
	priorityWave := 2
	if h.repStore != nil && h.repStore.GetStatus(email) == models.ReputationStatusVerified {
		priorityWave = 1
	}

	return email, invNum, priorityWave, true
}

func (h *UploadHandler) createValidatedInvoiceJob(sess *streamSession, invNum, email, filename, pdfPath string) models.InvoiceJob {
	invoiceID := uuid.New().String()
	clientName := parser.ExtractClientNameFromFilename(filename)
	inv := &models.Invoice{
		ID:             invoiceID,
		JobID:          sess.jobID,
		InvoiceNumber:  invNum,
		RecipientEmail: email,
		Status:         models.InvoiceStatusPending,
		Attempts:       0,
	}
	_ = h.db.CreateInvoice(inv)

	return models.InvoiceJob{
		Invoice:    *inv,
		PDFPath:    pdfPath,
		JobID:      sess.jobID,
		ClientName: clientName,
		DueDate:    sess.dueDate,
		Template:   sess.template,
	}
}

// processFiles processes standard (<25MB) uploaded files with the 4-layer Anti-Bounce filter and Wave ordering.
func (h *UploadHandler) processFiles(jobID string, files []*multipart.FileHeader, dueDate string, template *models.EmailTemplate, forceResend bool) {
	var pdfFiles []pdfFileInfo
	clientDB := h.getClientDB()

	for _, fileHeader := range files {
		file, err := fileHeader.Open()
		if err != nil {
			log.Printf("[Upload] Failed to open file %s: %v", fileHeader.Filename, err)
			continue
		}

		data, err := io.ReadAll(file)
		file.Close()
		if err != nil {
			log.Printf("[Upload] Failed to read file %s: %v", fileHeader.Filename, err)
			continue
		}

		lowerName := strings.ToLower(fileHeader.Filename)

		if strings.HasSuffix(lowerName, ".zip") {
			extracted, err := h.extractZip(data, jobID)
			if err != nil {
				log.Printf("[Upload] Failed to extract ZIP %s: %v", fileHeader.Filename, err)
				continue
			}
			pdfFiles = append(pdfFiles, extracted...)
		} else if strings.HasSuffix(lowerName, ".rar") {
			extracted, err := h.extractRar(data, jobID)
			if err != nil {
				log.Printf("[Upload] Failed to extract RAR %s: %v", fileHeader.Filename, err)
				continue
			}
			pdfFiles = append(pdfFiles, extracted...)
		} else if strings.HasSuffix(lowerName, ".pdf") {
			pdfPath := filepath.Join(h.tempDir, jobID, fileHeader.Filename)
			if err := h.saveTempFile(pdfPath, data); err != nil {
				log.Printf("[Upload] Failed to save PDF %s: %v", fileHeader.Filename, err)
				continue
			}
			pdfFiles = append(pdfFiles, pdfFileInfo{
				filename: fileHeader.Filename,
				path:     pdfPath,
			})
		} else if strings.HasSuffix(lowerName, ".txt") {
			newClientDB, err := parser.ParseTXTFromBytes(data)
			if err == nil {
				h.SetClientDB(newClientDB)
				clientDB = newClientDB
				log.Printf("[Upload] Client database updated from %s: %d entries", fileHeader.Filename, newClientDB.Size())
			}
		}
	}

	// Capa 3: Sort pdfFiles by Wave Priority (Wave 1: verified in repStore first)
	if h.repStore != nil {
		sort.SliceStable(pdfFiles, func(i, j int) bool {
			invI, errI := parser.ExtractInvoiceNumber(pdfFiles[i].filename)
			invJ, errJ := parser.ExtractInvoiceNumber(pdfFiles[j].filename)
			var verI, verJ bool
			if errI == nil {
				if em, ok := clientDB.GetEmail(invI); ok {
					verI = h.repStore.GetStatus(reputation.Sanitize(em).Fixed) == models.ReputationStatusVerified
				}
			}
			if errJ == nil {
				if em, ok := clientDB.GetEmail(invJ); ok {
					verJ = h.repStore.GetStatus(reputation.Sanitize(em).Fixed) == models.ReputationStatusVerified
				}
			}
			if verI != verJ {
				return verI
			}
			return pdfFiles[i].filename < pdfFiles[j].filename
		})
	}

	sentToday, _, _ := h.db.GetDailySendCount()
	remainingQuota := h.cfg.DailyLimit - sentToday
	if remainingQuota < 0 {
		remainingQuota = 0
	}

	var queued int
	for _, pdf := range pdfFiles {
		if queued >= remainingQuota {
			_ = os.Remove(pdf.path)
			continue
		}

		lowerPdf := strings.ToLower(pdf.filename)
		if strings.Contains(lowerPdf, "nota de cr") {
			_ = os.Remove(pdf.path)
			continue
		}

		invoice, validationError := parser.ValidateAndBuildInvoice(pdf.filename, clientDB)
		invoiceID := uuid.New().String()

		if validationError != "" {
			_ = os.Remove(pdf.path)
			if validationError == "SKIP_TYPE_X" {
				continue
			}
			continue
		}

		// Capa 2: Sanitize email
		san := reputation.Sanitize(invoice.RecipientEmail)
		if san.IsInvalid {
			_ = os.Remove(pdf.path)
			continue
		}
		invoice.RecipientEmail = san.Fixed

		// Capa 1: Reputation suppression check
		if (h.repStore != nil && h.repStore.IsBlocked(invoice.RecipientEmail)) || h.db.IsBlacklisted(invoice.RecipientEmail) {
			_ = os.Remove(pdf.path)
			log.Printf("[Upload] Blocked suppressed email %s for invoice %s", invoice.RecipientEmail, invoice.InvoiceNumber)
			continue
		}

		if !forceResend {
			alreadySent, err := h.db.CheckInvoiceSentThisMonth(invoice.InvoiceNumber)
			if err == nil && alreadySent {
				_ = os.Remove(pdf.path)
				continue
			}
		}

		clientName := parser.ExtractClientNameFromFilename(pdf.path)
		invoice.ID = invoiceID
		invoice.JobID = jobID
		_ = h.db.CreateInvoice(invoice)

		h.pool.Submit(models.InvoiceJob{
			Invoice:    *invoice,
			PDFPath:    pdf.path,
			JobID:      jobID,
			ClientName: clientName,
			DueDate:    dueDate,
			Template:   template,
		})
		queued++
	}

	_ = h.db.UpdateJobTotalFiles(jobID, queued)
	log.Printf("[Upload] Job %s: %d invoices queued for sending", jobID, queued)

	go func() {
		h.pool.WaitForCompletion()
		_ = h.db.UpdateJobStatus(jobID, models.JobStatusCompleted)
		log.Printf("[Upload] Job %s completed", jobID)
	}()
}

// extractZip extracts PDF files from a ZIP archive.
func (h *UploadHandler) extractZip(data []byte, jobID string) ([]pdfFileInfo, error) {
	reader, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		return nil, fmt.Errorf("failed to open ZIP: %w", err)
	}

	var pdfs []pdfFileInfo
	for _, f := range reader.File {
		if strings.HasSuffix(strings.ToLower(f.Name), ".pdf") {
			rc, err := f.Open()
			if err != nil {
				continue
			}
			fileData, err := io.ReadAll(rc)
			rc.Close()
			if err != nil {
				continue
			}
			filename := filepath.Base(f.Name)
			pdfPath := filepath.Join(h.tempDir, jobID, filename)
			if err := h.saveTempFile(pdfPath, fileData); err != nil {
				continue
			}
			pdfs = append(pdfs, pdfFileInfo{filename: filename, path: pdfPath})
		}
	}
	return pdfs, nil
}

func (h *UploadHandler) saveTempFile(path string, data []byte) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create directory: %w", err)
	}
	return os.WriteFile(path, data, 0644)
}

type pdfFileInfo struct {
	filename string
	path     string
}

func (h *UploadHandler) extractRar(data []byte, jobID string) ([]pdfFileInfo, error) {
	reader, err := rardecode.NewReader(bytes.NewReader(data))
	if err != nil {
		return nil, fmt.Errorf("failed to open RAR: %w", err)
	}

	var pdfs []pdfFileInfo
	for {
		header, err := reader.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			break
		}
		if header.IsDir {
			continue
		}
		name := filepath.Base(header.Name)
		if !strings.HasSuffix(strings.ToLower(name), ".pdf") {
			continue
		}
		fileData, err := io.ReadAll(reader)
		if err != nil {
			continue
		}
		pdfPath := filepath.Join(h.tempDir, jobID, name)
		if err := h.saveTempFile(pdfPath, fileData); err != nil {
			continue
		}
		pdfs = append(pdfs, pdfFileInfo{filename: name, path: pdfPath})
	}
	return pdfs, nil
}
