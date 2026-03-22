package analyzer

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"

	"hardsend/database"
	"hardsend/models"
	"hardsend/parser"
)

// Result holds the aggregated counts from an analysis run.
type Result struct {
	Total       int
	Valid       int
	NoEmail     int
	Skipped     int
	NewFiles    int // Files that were not already in the campaign (for re-scan)
	ElapsedMs   int64
}

// pdfFile holds information about a discovered PDF.
type pdfFile struct {
	filename string
	path     string
}

// Analyzer scans folders, cross-references with TXT, and saves results to the DB.
type Analyzer struct {
	db          *database.DB
	workerCount int
}

// New creates a new Analyzer.
func New(db *database.DB, workerCount int) *Analyzer {
	if workerCount <= 0 {
		workerCount = 4
	}
	return &Analyzer{
		db:          db,
		workerCount: workerCount,
	}
}

// AnalyzeFolder scans a local folder for PDFs, parses the TXT, and saves analysis results.
func (a *Analyzer) AnalyzeFolder(campaignID, folderPath, txtPath string, forceResend bool) (*Result, error) {
	start := time.Now()

	// Validate paths
	folderInfo, err := os.Stat(folderPath)
	if err != nil {
		return nil, fmt.Errorf("carpeta no encontrada: %s", folderPath)
	}
	if !folderInfo.IsDir() {
		return nil, fmt.Errorf("la ruta no es una carpeta: %s", folderPath)
	}

	if _, err := os.Stat(txtPath); err != nil {
		return nil, fmt.Errorf("archivo TXT no encontrado: %s", txtPath)
	}

	// Step 1: Parse the TXT file
	clientDB, err := parser.ParseTXTFile(txtPath)
	if err != nil {
		return nil, fmt.Errorf("error al parsear el TXT: %w", err)
	}
	log.Printf("[Analyzer] TXT parsed: %d entries", clientDB.Size())

	// Step 2: Scan folder for PDFs
	pdfs, err := scanFolder(folderPath)
	if err != nil {
		return nil, fmt.Errorf("error al escanear la carpeta: %w", err)
	}
	log.Printf("[Analyzer] Found %d PDFs in folder", len(pdfs))

	if len(pdfs) == 0 {
		return &Result{ElapsedMs: time.Since(start).Milliseconds()}, nil
	}

	// Step 3: Cross-reference with goroutines
	result := a.crossReference(campaignID, pdfs, clientDB, forceResend)
	result.ElapsedMs = time.Since(start).Milliseconds()

	log.Printf("[Analyzer] Analysis complete in %dms: total=%d valid=%d no_email=%d skipped=%d new=%d",
		result.ElapsedMs, result.Total, result.Valid, result.NoEmail, result.Skipped, result.NewFiles)

	return result, nil
}

// scanFolder walks a directory and returns all PDF files found.
func scanFolder(folderPath string) ([]pdfFile, error) {
	var pdfs []pdfFile

	err := filepath.Walk(folderPath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil // skip unreadable files
		}
		if info.IsDir() {
			return nil
		}
		if strings.HasSuffix(strings.ToLower(info.Name()), ".pdf") {
			pdfs = append(pdfs, pdfFile{
				filename: info.Name(),
				path:     path,
			})
		}
		return nil
	})

	return pdfs, err
}

// analysisResult is the result of analyzing a single PDF.
type analysisResult struct {
	invoice models.CampaignInvoice
	isNew   bool // true if this invoice wasn't already in the campaign
}

// crossReference processes PDFs against the TXT database using a goroutine pool.
func (a *Analyzer) crossReference(campaignID string, pdfs []pdfFile, clientDB *parser.ClientDB, forceResend bool) *Result {
	// Channel for work distribution
	jobs := make(chan pdfFile, len(pdfs))
	results := make(chan analysisResult, len(pdfs))

	// Start worker goroutines
	var wg sync.WaitGroup
	for i := 0; i < a.workerCount; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for pdf := range jobs {
				result := a.analyzeSinglePDF(campaignID, pdf, clientDB, forceResend)
				results <- result
			}
		}()
	}

	// Feed work to workers
	for _, pdf := range pdfs {
		jobs <- pdf
	}
	close(jobs)

	// Wait for all workers to finish, then close results
	go func() {
		wg.Wait()
		close(results)
	}()

	// Collect results and batch insert
	var allInvoices []models.CampaignInvoice
	var total, valid, noEmail, skipped, newFiles int

	for r := range results {
		total++
		if r.isNew {
			newFiles++
			allInvoices = append(allInvoices, r.invoice)
		}

		switch r.invoice.Status {
		case models.CampaignInvoiceStatusQueued:
			valid++
		case models.CampaignInvoiceStatusNoEmail:
			noEmail++
		case models.CampaignInvoiceStatusSkipped:
			skipped++
		}
	}

	// Batch insert new invoices
	if len(allInvoices) > 0 {
		if err := a.db.CreateCampaignInvoicesBatch(allInvoices); err != nil {
			log.Printf("[Analyzer] Error batch inserting invoices: %v", err)
		}
	}

	// Update campaign counts
	if err := a.db.UpdateCampaignCounts(campaignID, total, valid, noEmail, skipped); err != nil {
		log.Printf("[Analyzer] Error updating campaign counts: %v", err)
	}

	return &Result{
		Total:       total,
		Valid:       valid,
		NoEmail:     noEmail,
		Skipped:     skipped,
		NewFiles:    newFiles,
	}
}

// analyzeSinglePDF processes one PDF: extracts invoice number, cross-references with TXT.
func (a *Analyzer) analyzeSinglePDF(campaignID string, pdf pdfFile, clientDB *parser.ClientDB, forceResend bool) analysisResult {
	// Check if already analyzed in this campaign (for re-scan)
	invoiceNumber := extractInvoiceNumberSafe(pdf.filename)

	existing, _ := a.db.GetCampaignInvoiceByNumber(campaignID, invoiceNumber)
	if existing != nil {
		// Already analyzed — return existing result but don't mark as new
		return analysisResult{invoice: *existing, isNew: false}
	}

	// Extract invoice number
	invNum, err := parser.ExtractInvoiceNumber(pdf.filename)
	if err != nil {
		reason := fmt.Sprintf("Nombre de archivo inválido: %s", pdf.filename)
		return analysisResult{
			invoice: models.CampaignInvoice{
				ID:            uuid.New().String(),
				CampaignID:    campaignID,
				InvoiceNumber: invoiceNumber,
				ClientName:    parser.ExtractClientNameFromFilename(pdf.path),
				Email:         "",
				PDFPath:       pdf.path,
				Status:        models.CampaignInvoiceStatusNoEmail,
				Reason:        &reason,
			},
			isNew: true,
		}
	}

	// Check if type X (skip silently)
	if parser.IsTypeXInvoice(invNum) {
		reason := "Factura tipo X omitida (no requiere envío)"
		return analysisResult{
			invoice: models.CampaignInvoice{
				ID:            uuid.New().String(),
				CampaignID:    campaignID,
				InvoiceNumber: invNum,
				ClientName:    parser.ExtractClientNameFromFilename(pdf.path),
				Email:         "",
				PDFPath:       pdf.path,
				Status:        models.CampaignInvoiceStatusSkipped,
				Reason:        &reason,
			},
			isNew: true,
		}
	}

	// Check idempotency — was it already sent this month?
	if !forceResend {
		alreadySent, err := a.db.CheckInvoiceSentThisMonth(invNum)
		if err == nil && alreadySent {
			reason := fmt.Sprintf("Factura %s ya enviada exitosamente este mes", invNum)
			return analysisResult{
				invoice: models.CampaignInvoice{
					ID:            uuid.New().String(),
					CampaignID:    campaignID,
					InvoiceNumber: invNum,
					ClientName:    parser.ExtractClientNameFromFilename(pdf.path),
					Email:         "",
					PDFPath:       pdf.path,
					Status:        models.CampaignInvoiceStatusSkipped,
					Reason:        &reason,
				},
				isNew: true,
			}
		}
	}

	// Look up email in TXT
	email, found := clientDB.GetEmail(invNum)
	clientName := parser.ExtractClientNameFromFilename(pdf.path)

	if !found {
		reason := fmt.Sprintf("Factura %s no encontrada en la base de datos de clientes (TXT)", invNum)
		return analysisResult{
			invoice: models.CampaignInvoice{
				ID:            uuid.New().String(),
				CampaignID:    campaignID,
				InvoiceNumber: invNum,
				ClientName:    clientName,
				Email:         "",
				PDFPath:       pdf.path,
				Status:        models.CampaignInvoiceStatusNoEmail,
				Reason:        &reason,
			},
			isNew: true,
		}
	}

	// Validate email format
	if !parser.ValidateEmail(email) {
		reason := fmt.Sprintf("Email inválido para factura %s: %s", invNum, email)
		return analysisResult{
			invoice: models.CampaignInvoice{
				ID:            uuid.New().String(),
				CampaignID:    campaignID,
				InvoiceNumber: invNum,
				ClientName:    clientName,
				Email:         email,
				PDFPath:       pdf.path,
				Status:        models.CampaignInvoiceStatusNoEmail,
				Reason:        &reason,
			},
			isNew: true,
		}
	}



	// Valid — ready to send
	return analysisResult{
		invoice: models.CampaignInvoice{
			ID:            uuid.New().String(),
			CampaignID:    campaignID,
			InvoiceNumber: invNum,
			ClientName:    clientName,
			Email:         email,
			PDFPath:       pdf.path,
			Status:        models.CampaignInvoiceStatusQueued,
			Reason:        nil,
		},
		isNew: true,
	}
}

// extractInvoiceNumberSafe extracts the invoice number from a filename, returning the filename if extraction fails.
func extractInvoiceNumberSafe(filename string) string {
	num, err := parser.ExtractInvoiceNumber(filename)
	if err != nil {
		return filename
	}
	return num
}
