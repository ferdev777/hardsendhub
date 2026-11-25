package analyzer

import (
	"archive/zip"
	"fmt"
	"io"
	"log"
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
	"hardsend/reputation"
)

// Result holds the aggregated counts from an analysis run.
// It also contains the DryRunSummary for the pre-flight semaphore in CampaignPlan.
type Result struct {
	Total       int
	Valid       int
	NoEmail     int
	Skipped     int
	NewFiles    int // Files that were not already in the campaign (for re-scan)
	ElapsedMs   int64
	// Pre-flight semaphore fields (Dry Run)
	VerifiedCount   int // GREEN  — Wave 1, reputation=verified
	UntestedCount   int // BLUE   — Wave 2, reputation=untested
	FixableCount    int // YELLOW — typo detected and auto-corrected (needs human review)
	SuppressedCount int // GREY   — bounced or cashier_only
	PDFErrorCount   int // RED    — PDF missing or > 3.4MB
}

// pdfFile holds information about a discovered PDF.
type pdfFile struct {
	filename string
	path     string
}

// Analyzer scans folders, cross-references with TXT, and saves results to the DB.
type Analyzer struct {
	db          *database.DB
	reputation  *reputation.Store
	mxCache     *reputation.MXCache
	workerCount int
}

// New creates a new Analyzer.
func New(db *database.DB, repStore *reputation.Store, mxCache *reputation.MXCache, workerCount int) *Analyzer {
	if workerCount <= 0 {
		workerCount = 4
	}
	if mxCache == nil {
		mxCache = reputation.NewMXCache(5 * time.Second)
	}
	return &Analyzer{
		db:          db,
		reputation:  repStore,
		mxCache:     mxCache,
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

	// Step 2b: Warm up MX cache for all unique domains in the TXT (Capa 2, Step 3)
	if a.mxCache != nil {
		allEmails := clientDB.AllEmails()
		go a.mxCache.WarmUp(allEmails) // non-blocking warm-up; lookups will hit cache shortly after
	}

	// Step 3: Cross-reference with goroutines
	result := a.crossReference(campaignID, pdfs, clientDB, forceResend)
	result.ElapsedMs = time.Since(start).Milliseconds()

	log.Printf("[Analyzer] Analysis complete in %dms: total=%d valid=%d no_email=%d skipped=%d suppressed=%d pdf_errors=%d",
		result.ElapsedMs, result.Total, result.Valid, result.NoEmail, result.Skipped, result.SuppressedCount, result.PDFErrorCount)

	return result, nil
}

// scanFolder walks a directory, extracts any .zip/.rar archives on disk by streaming, and returns all PDF files found.
func scanFolder(folderPath string) ([]pdfFile, error) {
	// Step 0: Check for any .zip or .rar archives in folderPath and extract them via streaming if not yet extracted
	if err := ensureArchivesExtracted(folderPath); err != nil {
		log.Printf("[Analyzer] Warning extracting archive in %s: %v", folderPath, err)
	}

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

func ensureArchivesExtracted(folderPath string) error {
	entries, err := os.ReadDir(folderPath)
	if err != nil {
		return err
	}

	extractDir := filepath.Join(folderPath, "_extracted")
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		lower := strings.ToLower(entry.Name())
		if !strings.HasSuffix(lower, ".rar") && !strings.HasSuffix(lower, ".zip") {
			continue
		}

		info, err := entry.Info()
		if err != nil {
			continue
		}

		markerFile := filepath.Join(extractDir, fmt.Sprintf(".done_%s_%d", entry.Name(), info.Size()))
		if _, err := os.Stat(markerFile); err == nil {
			// Archive already extracted previously
			continue
		}

		if err := os.MkdirAll(extractDir, 0755); err != nil {
			return err
		}

		archivePath := filepath.Join(folderPath, entry.Name())
		log.Printf("[Analyzer] Streaming archive extraction from disk: %s (%.2f MB)...", archivePath, float64(info.Size())/(1024*1024))

		if strings.HasSuffix(lower, ".rar") {
			if err := streamExtractRar(archivePath, extractDir); err != nil {
				return fmt.Errorf("error extrayendo RAR %s: %w", entry.Name(), err)
			}
		} else if strings.HasSuffix(lower, ".zip") {
			if err := streamExtractZip(archivePath, extractDir); err != nil {
				return fmt.Errorf("error extrayendo ZIP %s: %w", entry.Name(), err)
			}
		}

		_ = os.WriteFile(markerFile, []byte("ok"), 0644)
	}
	return nil
}

func streamExtractRar(archivePath, destDir string) error {
	r, err := rardecode.OpenReader(archivePath)
	if err != nil {
		return err
	}
	defer r.Close()

	for {
		header, err := r.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return err
		}
		if header.IsDir {
			continue
		}
		base := filepath.Base(header.Name)
		if !strings.HasSuffix(strings.ToLower(base), ".pdf") {
			continue
		}
		outPath := filepath.Join(destDir, base)
		outFile, err := os.Create(outPath)
		if err != nil {
			continue
		}
		_, _ = io.Copy(outFile, r)
		outFile.Close()
	}
	return nil
}

func streamExtractZip(archivePath, destDir string) error {
	zr, err := zip.OpenReader(archivePath)
	if err != nil {
		return err
	}
	defer zr.Close()

	for _, f := range zr.File {
		if f.FileInfo().IsDir() {
			continue
		}
		base := filepath.Base(f.Name)
		if !strings.HasSuffix(strings.ToLower(base), ".pdf") {
			continue
		}
		rc, err := f.Open()
		if err != nil {
			continue
		}
		outPath := filepath.Join(destDir, base)
		outFile, err := os.Create(outPath)
		if err != nil {
			rc.Close()
			continue
		}
		_, _ = io.Copy(outFile, rc)
		outFile.Close()
		rc.Close()
	}
	return nil
}

// dryRunBucket classifies an invoice for the pre-flight semaphore.
type dryRunBucket int

const (
	dryRunNone       dryRunBucket = iota
	dryRunVerified                // GREEN
	dryRunUntested                // BLUE
	dryRunFixable                 // YELLOW
	dryRunSuppressed              // GREY
	dryRunPDFError                // RED
)

// analysisResult is the result of analyzing a single PDF.
type analysisResult struct {
	invoice      models.CampaignInvoice
	isNew        bool         // true if this invoice wasn't already in the campaign
	dryRunBucket dryRunBucket // pre-flight semaphore classification
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
	var verified, untested, fixable, suppressed, pdfError int

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

		// Dry-run semaphore buckets
		switch r.dryRunBucket {
		case dryRunVerified:
			verified++
		case dryRunUntested:
			untested++
		case dryRunFixable:
			fixable++
		case dryRunSuppressed:
			suppressed++
		case dryRunPDFError:
			pdfError++
		}
	}

	// Batch insert new invoices
	if len(allInvoices) > 0 {
		if err := a.db.CreateCampaignInvoicesBatch(allInvoices); err != nil {
			log.Printf("[Analyzer] Error batch inserting invoices: %v", err)
		}
	}

	// Update campaign counts
	summary := models.DryRunSummary{
		VerifiedCount:   verified,
		UntestedCount:   untested,
		FixableCount:    fixable,
		SuppressedCount: suppressed,
		PDFErrorCount:   pdfError,
	}
	if err := a.db.UpdateCampaignCounts(campaignID, summary, total, valid, noEmail, skipped); err != nil {
		log.Printf("[Analyzer] Error updating campaign counts: %v", err)
	}

	return &Result{
		Total:           total,
		Valid:           valid,
		NoEmail:         noEmail,
		Skipped:         skipped,
		NewFiles:        newFiles,
		VerifiedCount:   verified,
		UntestedCount:   untested,
		FixableCount:    fixable,
		SuppressedCount: suppressed,
		PDFErrorCount:   pdfError,
	}
}

// analyzeSinglePDF processes one PDF: extracts invoice number, cross-references with TXT.
// Applies the 4-layer Anti-Bounce Engine in pre-flight order.
func (a *Analyzer) analyzeSinglePDF(campaignID string, pdf pdfFile, clientDB *parser.ClientDB, forceResend bool) analysisResult {
	// Check if already analyzed in this campaign (for re-scan)
	invoiceNumber := extractInvoiceNumberSafe(pdf.filename)

	existing, _ := a.db.GetCampaignInvoiceByNumber(campaignID, invoiceNumber)
	if existing != nil {
		return analysisResult{invoice: *existing, isNew: false}
	}

	// Extract invoice number
	invNum, err := parser.ExtractInvoiceNumber(pdf.filename)
	if err != nil {
		reason := fmt.Sprintf("Nombre de archivo inválido: %s", pdf.filename)
		return analysisResult{
			invoice: models.CampaignInvoice{
				ID: uuid.New().String(), CampaignID: campaignID, InvoiceNumber: invoiceNumber,
				ClientName: parser.ExtractClientNameFromFilename(pdf.path),
				Email: "", PDFPath: pdf.path, Status: models.CampaignInvoiceStatusNoEmail, Reason: &reason,
			},
			isNew: true, dryRunBucket: dryRunNone,
		}
	}

	// Check if type X (skip silently)
	if parser.IsTypeXInvoice(invNum) {
		reason := "Factura tipo X omitida (no requiere envío)"
		return analysisResult{
			invoice: models.CampaignInvoice{
				ID: uuid.New().String(), CampaignID: campaignID, InvoiceNumber: invNum,
				ClientName: parser.ExtractClientNameFromFilename(pdf.path),
				Email: "", PDFPath: pdf.path, Status: models.CampaignInvoiceStatusSkipped, Reason: &reason,
			},
			isNew: true, dryRunBucket: dryRunNone,
		}
	}

	// Check idempotency — was it already sent this month?
	if !forceResend {
		alreadySent, err := a.db.CheckInvoiceSentThisMonth(invNum)
		if err == nil && alreadySent {
			reason := fmt.Sprintf("Factura %s ya enviada exitosamente este mes", invNum)
			return analysisResult{
				invoice: models.CampaignInvoice{
					ID: uuid.New().String(), CampaignID: campaignID, InvoiceNumber: invNum,
					ClientName: parser.ExtractClientNameFromFilename(pdf.path),
					Email: "", PDFPath: pdf.path, Status: models.CampaignInvoiceStatusSkipped, Reason: &reason,
				},
				isNew: true, dryRunBucket: dryRunNone,
			}
		}
	}

	// Look up email in TXT
	rawEmail, found := clientDB.GetEmail(invNum)
	clientName := parser.ExtractClientNameFromFilename(pdf.path)

	if !found {
		lowerFilename := strings.ToLower(pdf.filename)
		if strings.Contains(lowerFilename, "nota de cr") {
			reason := fmt.Sprintf("Nota de Crédito %s omitida (no incluida en padrón TXT)", invNum)
			return analysisResult{
				invoice: models.CampaignInvoice{
					ID: uuid.New().String(), CampaignID: campaignID, InvoiceNumber: invNum,
					ClientName: clientName, Email: "", PDFPath: pdf.path,
					Status: models.CampaignInvoiceStatusSkipped, Reason: &reason,
				},
				isNew: true, dryRunBucket: dryRunNone,
			}
		}
		reason := fmt.Sprintf("Factura %s no encontrada en TXT (cobro por ventanilla)", invNum)
		return analysisResult{
			invoice: models.CampaignInvoice{
				ID: uuid.New().String(), CampaignID: campaignID, InvoiceNumber: invNum,
				ClientName: clientName, Email: "", PDFPath: pdf.path,
				Status: models.CampaignInvoiceStatusNoEmail, Reason: &reason,
			},
			isNew: true, dryRunBucket: dryRunNone,
		}
	}

	// =====================================================================
	// CAPA 2: Sanitizer + MX Check
	// =====================================================================
	sanitized := reputation.Sanitize(rawEmail)
	email := sanitized.Fixed

	if sanitized.IsInvalid {
		reason := fmt.Sprintf("Email con formato inválido: %s", rawEmail)
		return analysisResult{
			invoice: models.CampaignInvoice{
				ID: uuid.New().String(), CampaignID: campaignID, InvoiceNumber: invNum,
				ClientName: clientName, Email: rawEmail, PDFPath: pdf.path,
				Status: models.CampaignInvoiceStatusNoEmail, Reason: &reason,
			},
			isNew: true, dryRunBucket: dryRunFixable,
		}
	}

	// =====================================================================
	// CAPA 1: Blacklist (email_reputation) check
	// =====================================================================
	if a.reputation != nil && a.reputation.IsBlocked(email) {
		status := a.reputation.GetStatus(email)
		reason := fmt.Sprintf("Email suprimido (reputación: %s): %s", status, email)
		return analysisResult{
			invoice: models.CampaignInvoice{
				ID: uuid.New().String(), CampaignID: campaignID, InvoiceNumber: invNum,
				ClientName: clientName, Email: email, PDFPath: pdf.path,
				Status: models.CampaignInvoiceStatusSkipped, Reason: &reason,
			},
			isNew: true, dryRunBucket: dryRunSuppressed,
		}
	}

	// MX validation (Capa 2, Step 3) — skip for well-known domains to save time
	if a.mxCache != nil && !isKnownGoodDomain(email) {
		if !a.mxCache.HasMXDomain(extractDomainFromEmail(email)) {
			reason := fmt.Sprintf("Dominio sin registros MX (caducado?): %s", email)
			return analysisResult{
				invoice: models.CampaignInvoice{
					ID: uuid.New().String(), CampaignID: campaignID, InvoiceNumber: invNum,
					ClientName: clientName, Email: email, PDFPath: pdf.path,
					Status: models.CampaignInvoiceStatusNoEmail, Reason: &reason,
				},
				isNew: true, dryRunBucket: dryRunNone,
			}
		}
	}

	// Old format validation (kept for compat)
	if !parser.ValidateEmail(email) {
		reason := fmt.Sprintf("Email inválido para factura %s: %s", invNum, email)
		return analysisResult{
			invoice: models.CampaignInvoice{
				ID: uuid.New().String(), CampaignID: campaignID, InvoiceNumber: invNum,
				ClientName: clientName, Email: email, PDFPath: pdf.path,
				Status: models.CampaignInvoiceStatusNoEmail, Reason: &reason,
			},
			isNew: true, dryRunBucket: dryRunFixable,
		}
	}

	// =====================================================================
	// CAPA 2, Step 4: PDF Size Validation (≤ 3.4 MB for Cloudflare)
	// =====================================================================
	pdfInfo, statErr := os.Stat(pdf.path)
	if statErr != nil || pdfInfo == nil {
		reason := fmt.Sprintf("PDF no encontrado: %s", pdf.path)
		return analysisResult{
			invoice: models.CampaignInvoice{
				ID: uuid.New().String(), CampaignID: campaignID, InvoiceNumber: invNum,
				ClientName: clientName, Email: email, PDFPath: pdf.path,
				Status: models.CampaignInvoiceStatusNoEmail, Reason: &reason,
			},
			isNew: true, dryRunBucket: dryRunPDFError,
		}
	}
	if pdfInfo.Size() > 3_400_000 {
		reason := fmt.Sprintf("PDF demasiado grande: %.2f MB (máximo 3.4 MB)", float64(pdfInfo.Size())/1_000_000)
		return analysisResult{
			invoice: models.CampaignInvoice{
				ID: uuid.New().String(), CampaignID: campaignID, InvoiceNumber: invNum,
				ClientName: clientName, Email: email, PDFPath: pdf.path,
				Status: models.CampaignInvoiceStatusNoEmail, Reason: &reason,
			},
			isNew: true, dryRunBucket: dryRunPDFError,
		}
	}

	// =====================================================================
	// Determine wave (reputation lookup for ordering)
	// =====================================================================
	bucket := dryRunUntested
	if a.reputation != nil {
		if a.reputation.GetStatus(email) == "verified" {
			bucket = dryRunVerified
		}
	}
	if sanitized.WasFixed {
		bucket = dryRunFixable // Fixable but still sendable
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
		isNew:        true,
		dryRunBucket: bucket,
	}
}

// isKnownGoodDomain returns true for high-traffic domains that are always valid.
// Skipping MX lookup for these saves time and avoids unnecessary DNS load.
func isKnownGoodDomain(email string) bool {
	known := []string{"gmail.com", "hotmail.com", "yahoo.com", "outlook.com",
		"live.com", "icloud.com", "fibertel.com.ar", "arnet.com.ar"}
	lower := strings.ToLower(email)
	for _, d := range known {
		if strings.HasSuffix(lower, "@"+d) {
			return true
		}
	}
	return false
}

// extractDomainFromEmail extracts the domain part from an email string.
func extractDomainFromEmail(email string) string {
	parts := strings.SplitN(email, "@", 2)
	if len(parts) == 2 {
		return parts[1]
	}
	return ""
}

// extractInvoiceNumberSafe extracts the invoice number from a filename, returning the filename if extraction fails.
func extractInvoiceNumberSafe(filename string) string {
	num, err := parser.ExtractInvoiceNumber(filename)
	if err != nil {
		return filename
	}
	return num
}
