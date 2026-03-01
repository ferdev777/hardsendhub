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
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/nwaples/rardecode/v2"

	"hardsend/database"
	"hardsend/models"
	"hardsend/parser"
	"hardsend/workers"
)

// UploadHandler handles file upload operations.
type UploadHandler struct {
	db       *database.DB
	pool     *workers.Pool
	clientDB *parser.ClientDB
	tempDir  string
	mu       sync.RWMutex
}

// NewUploadHandler creates a new upload handler.
func NewUploadHandler(db *database.DB, pool *workers.Pool, tempDir string) *UploadHandler {
	return &UploadHandler{
		db:       db,
		pool:     pool,
		clientDB: parser.NewClientDB(),
		tempDir:  tempDir,
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

// Upload handles POST /api/upload.
func (h *UploadHandler) Upload(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, `{"error":"Method not allowed"}`, http.StatusMethodNotAllowed)
		return
	}

	// Parse multipart form (max 500MB)
	if err := r.ParseMultipartForm(500 << 20); err != nil {
		http.Error(w, fmt.Sprintf(`{"error":"Failed to parse form: %s"}`, err.Error()), http.StatusBadRequest)
		return
	}

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

		// If only TXT was uploaded, respond immediately
		if len(r.MultipartForm.File["files"]) == 0 {
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(map[string]interface{}{
				"message":       "Base de datos de clientes actualizada exitosamente",
				"entries_count": clientDB.Size(),
			})
			return
		}
	}

	// Check if client database is loaded
	clientDB := h.getClientDB()
	if clientDB.Size() == 0 {
		http.Error(w, `{"error":"Base de datos de clientes (archivo TXT) no cargada. Por favor, suba el archivo TXT primero."}`, http.StatusBadRequest)
		return
	}

	// Process uploaded files
	files := r.MultipartForm.File["files"]
	if len(files) == 0 {
		http.Error(w, `{"error":"No files uploaded"}`, http.StatusBadRequest)
		return
	}

	// Create a new job
	jobID := uuid.New().String()
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

	// Process files in background
	go h.processFiles(jobID, files)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"job_id":  jobID,
		"message": "Carga iniciada. Procesando en segundo plano.",
		"files":   len(files),
	})
}

// processFiles processes uploaded files (ZIP or PDFs).
func (h *UploadHandler) processFiles(jobID string, files []*multipart.FileHeader) {
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
			// Extract ZIP file
			extracted, err := h.extractZip(data, jobID)
			if err != nil {
				log.Printf("[Upload] Failed to extract ZIP %s: %v", fileHeader.Filename, err)
				continue
			}
			pdfFiles = append(pdfFiles, extracted...)
		} else if strings.HasSuffix(lowerName, ".rar") {
			// Extract RAR file
			extracted, err := h.extractRar(data, jobID)
			if err != nil {
				log.Printf("[Upload] Failed to extract RAR %s: %v", fileHeader.Filename, err)
				continue
			}
			pdfFiles = append(pdfFiles, extracted...)
		} else if strings.HasSuffix(lowerName, ".pdf") {
			// Save PDF to temp directory
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
			// TXT file - update client database
			newClientDB, err := parser.ParseTXTFromBytes(data)
			if err == nil {
				h.SetClientDB(newClientDB)
				clientDB = newClientDB
				log.Printf("[Upload] Client database updated from %s: %d entries", fileHeader.Filename, newClientDB.Size())
			}
		}
	}

	// Update total files count
	_ = h.db.UpdateJobTotalFiles(jobID, len(pdfFiles))
	log.Printf("[Upload] Job %s: %d PDF files to process", jobID, len(pdfFiles))

	// Queue PDFs for processing
	var queued int
	for _, pdf := range pdfFiles {
		invoice, validationError := parser.ValidateAndBuildInvoice(pdf.filename, clientDB)

		invoiceID := uuid.New().String()

		if validationError != "" {
			// Type X invoices are silently skipped — not sent, not counted as errors
			if validationError == "SKIP_TYPE_X" {
				log.Printf("[Upload] Skipping type X invoice: %s", pdf.filename)
				continue
			}

			// Validation error - mark immediately, do not retry
			inv := &models.Invoice{
				ID:             invoiceID,
				JobID:          jobID,
				InvoiceNumber:  extractInvoiceNumberSafe(pdf.filename),
				RecipientEmail: "",
				Status:         models.InvoiceStatusErrorValidation,
				ErrorReason:    &validationError,
				Attempts:       0,
			}
			_ = h.db.CreateInvoice(inv)
			log.Printf("[Upload] Validation error for %s: %s", pdf.filename, validationError)
			continue
		}

		// Check idempotency - skip if already sent this month
		alreadySent, err := h.db.CheckInvoiceSentThisMonth(invoice.InvoiceNumber)
		if err == nil && alreadySent {
			reason := fmt.Sprintf("Factura %s ya enviada exitosamente este mes (omitida)", invoice.InvoiceNumber)
			inv := &models.Invoice{
				ID:             invoiceID,
				JobID:          jobID,
				InvoiceNumber:  invoice.InvoiceNumber,
				RecipientEmail: invoice.RecipientEmail,
				Status:         models.InvoiceStatusSuccess,
				ErrorReason:    &reason,
				Attempts:       0,
			}
			_ = h.db.CreateInvoice(inv)
			log.Printf("[Upload] Skipped duplicate: %s", invoice.InvoiceNumber)
			continue
		}

		// Extract client name from the PDF filename
		clientName := parser.ExtractClientNameFromFilename(pdf.path)
		log.Printf("[Upload] Extracted client name from filename %s: '%s'", pdf.filename, clientName)

		// Create invoice record and submit to worker pool
		invoice.ID = invoiceID
		invoice.JobID = jobID
		_ = h.db.CreateInvoice(invoice)

		h.pool.Submit(models.InvoiceJob{
			Invoice:    *invoice,
			PDFPath:    pdf.path,
			JobID:      jobID,
			ClientName: clientName,
		})
		queued++
	}

	log.Printf("[Upload] Job %s: %d invoices queued for sending", jobID, queued)

	// Wait for completion in a separate goroutine
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
				log.Printf("[Upload] Failed to open file in ZIP: %s: %v", f.Name, err)
				continue
			}

			fileData, err := io.ReadAll(rc)
			rc.Close()
			if err != nil {
				log.Printf("[Upload] Failed to read file in ZIP: %s: %v", f.Name, err)
				continue
			}

			filename := filepath.Base(f.Name)
			pdfPath := filepath.Join(h.tempDir, jobID, filename)
			if err := h.saveTempFile(pdfPath, fileData); err != nil {
				log.Printf("[Upload] Failed to save extracted PDF %s: %v", filename, err)
				continue
			}

			pdfs = append(pdfs, pdfFileInfo{
				filename: filename,
				path:     pdfPath,
			})
		}
	}

	return pdfs, nil
}

// saveTempFile saves data to a temporary file, creating directories as needed.
func (h *UploadHandler) saveTempFile(path string, data []byte) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create directory: %w", err)
	}
	return os.WriteFile(path, data, 0644)
}

// pdfFileInfo holds information about an extracted PDF.
type pdfFileInfo struct {
	filename string
	path     string
}

// extractInvoiceNumberSafe extracts the invoice number from a filename, returning the filename if extraction fails.
func extractInvoiceNumberSafe(filename string) string {
	num, err := parser.ExtractInvoiceNumber(filename)
	if err != nil {
		return filename
	}
	return num
}

// extractRar extracts PDF files from a RAR archive.
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
			log.Printf("[Upload] Error reading RAR entry: %v", err)
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
			log.Printf("[Upload] Failed to read file in RAR: %s: %v", name, err)
			continue
		}

		pdfPath := filepath.Join(h.tempDir, jobID, name)
		if err := h.saveTempFile(pdfPath, fileData); err != nil {
			log.Printf("[Upload] Failed to save extracted PDF %s: %v", name, err)
			continue
		}

		pdfs = append(pdfs, pdfFileInfo{
			filename: name,
			path:     pdfPath,
		})
	}

	return pdfs, nil
}
