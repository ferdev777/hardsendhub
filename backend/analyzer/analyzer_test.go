package analyzer

import (
	"os"
	"path/filepath"
	"testing"

	"hardsend/database"
	"hardsend/models"
)

// setupTestDB creates an in-memory SQLite database with all migrations applied.
func setupTestDB(t *testing.T) *database.DB {
	t.Helper()
	db, err := database.New(":memory:")
	if err != nil {
		t.Fatalf("Failed to create test DB: %v", err)
	}
	t.Cleanup(func() { db.Close() })
	return db
}

// createTestFolder creates a temporary folder with fake PDF files for testing.
func createTestFolder(t *testing.T, files []string) string {
	t.Helper()
	dir := t.TempDir()
	for _, name := range files {
		path := filepath.Join(dir, name)
		if err := os.WriteFile(path, []byte("fake-pdf-content"), 0644); err != nil {
			t.Fatalf("Failed to create test file %s: %v", name, err)
		}
	}
	return dir
}

// createTestTXT creates a temporary TXT file with email;invoice_number lines.
func createTestTXT(t *testing.T, lines []string) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "clientes.txt")
	content := ""
	for _, line := range lines {
		content += line + "\n"
	}
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatalf("Failed to create test TXT: %v", err)
	}
	return path
}

// createTestCampaign creates a campaign in the DB and returns its ID.
func createTestCampaign(t *testing.T, db *database.DB) string {
	t.Helper()
	campaign := &models.Campaign{
		ID:     "test-campaign-001",
		Name:   "Test Campaign",
		Status: models.CampaignStatusAnalyzing,
	}
	if err := db.CreateCampaign(campaign); err != nil {
		t.Fatalf("Failed to create test campaign: %v", err)
	}
	return campaign.ID
}

func TestScanFolder_ValidPDFs(t *testing.T) {
	files := []string{
		"00000001 - Factura  B0002-00000001 - CLIENTE UNO.pdf",
		"00000002 - Factura  B0002-00000002 - CLIENTE DOS.pdf",
		"00000003 - Factura  B0002-00000003 - CLIENTE TRES.pdf",
	}
	dir := createTestFolder(t, files)

	pdfs, err := scanFolder(dir)
	if err != nil {
		t.Fatalf("scanFolder returned error: %v", err)
	}

	if len(pdfs) != 3 {
		t.Errorf("Expected 3 PDFs, got %d", len(pdfs))
	}
}

func TestScanFolder_EmptyFolder(t *testing.T) {
	dir := t.TempDir()

	pdfs, err := scanFolder(dir)
	if err != nil {
		t.Fatalf("scanFolder returned error: %v", err)
	}

	if len(pdfs) != 0 {
		t.Errorf("Expected 0 PDFs, got %d", len(pdfs))
	}
}

func TestScanFolder_MixedFiles(t *testing.T) {
	files := []string{
		"00000001 - Factura  B0002-00000001 - CLIENTE.pdf",
		"readme.txt",
		"image.jpg",
		"document.docx",
		"00000002 - Factura  B0002-00000002 - OTRO.pdf",
	}
	dir := createTestFolder(t, files)

	pdfs, err := scanFolder(dir)
	if err != nil {
		t.Fatalf("scanFolder returned error: %v", err)
	}

	if len(pdfs) != 2 {
		t.Errorf("Expected 2 PDFs (ignoring non-PDF files), got %d", len(pdfs))
	}
}

func TestScanFolder_InvalidPath(t *testing.T) {
	_, err := scanFolder("/nonexistent/path/that/doesnt/exist")
	// scanFolder uses filepath.Walk which doesn't error for nonexistent root on some OSes,
	// but we should at least not panic
	if err != nil {
		// This is fine — some OS implementations return an error
		t.Logf("scanFolder returned expected error: %v", err)
	}
}

func TestCrossReference_AllMatched(t *testing.T) {
	db := setupTestDB(t)
	campaignID := createTestCampaign(t, db)
	a := New(db, 2)

	files := []string{
		"00000001 - Factura  B0002-00000001 - CLIENTE UNO.pdf",
		"00000002 - Factura  B0002-00000002 - CLIENTE DOS.pdf",
	}
	dir := createTestFolder(t, files)
	txtPath := createTestTXT(t, []string{
		"cliente1@test.com;B0002-00000001",
		"cliente2@test.com;B0002-00000002",
	})

	result, err := a.AnalyzeFolder(campaignID, dir, txtPath, false)
	if err != nil {
		t.Fatalf("AnalyzeFolder returned error: %v", err)
	}

	if result.Total != 2 {
		t.Errorf("Expected total=2, got %d", result.Total)
	}
	if result.Valid != 2 {
		t.Errorf("Expected valid=2, got %d", result.Valid)
	}
	if result.NoEmail != 0 {
		t.Errorf("Expected no_email=0, got %d", result.NoEmail)
	}
}

func TestCrossReference_SomeMissing(t *testing.T) {
	db := setupTestDB(t)
	campaignID := createTestCampaign(t, db)
	a := New(db, 2)

	files := []string{
		"00000001 - Factura  B0002-00000001 - CLIENTE UNO.pdf",
		"00000002 - Factura  B0002-00000002 - CLIENTE DOS.pdf",
		"00000003 - Factura  B0002-00000003 - CLIENTE TRES.pdf",
	}
	dir := createTestFolder(t, files)
	// Only provide email for the first one
	txtPath := createTestTXT(t, []string{
		"cliente1@test.com;B0002-00000001",
	})

	result, err := a.AnalyzeFolder(campaignID, dir, txtPath, false)
	if err != nil {
		t.Fatalf("AnalyzeFolder returned error: %v", err)
	}

	if result.Valid != 1 {
		t.Errorf("Expected valid=1, got %d", result.Valid)
	}
	if result.NoEmail != 2 {
		t.Errorf("Expected no_email=2, got %d", result.NoEmail)
	}
}

func TestCrossReference_Idempotency(t *testing.T) {
	db := setupTestDB(t)
	campaignID := createTestCampaign(t, db)
	a := New(db, 2)

	// Simulate that B0002-00000001 was already sent this month by creating a SUCCESS invoice
	inv := &models.Invoice{
		ID:             "existing-inv-001",
		JobID:          "existing-job-001",
		InvoiceNumber:  "B0002-00000001",
		RecipientEmail: "old@test.com",
		Status:         models.InvoiceStatusPending,
		Attempts:       0,
	}
	// Need to create the job first due to FK constraint
	job := &models.Job{ID: "existing-job-001", Filename: "test", Status: "COMPLETED"}
	_ = db.CreateJob(job)
	_ = db.CreateInvoice(inv)
	// Update to SUCCESS so last_attempt_at gets set (required by CheckInvoiceSentThisMonth)
	_ = db.UpdateInvoiceStatus(inv.ID, models.InvoiceStatusSuccess, nil, 1)

	files := []string{
		"00000001 - Factura  B0002-00000001 - CLIENTE UNO.pdf",
		"00000002 - Factura  B0002-00000002 - CLIENTE DOS.pdf",
	}
	dir := createTestFolder(t, files)
	txtPath := createTestTXT(t, []string{
		"cliente1@test.com;B0002-00000001",
		"cliente2@test.com;B0002-00000002",
	})

	result, err := a.AnalyzeFolder(campaignID, dir, txtPath, false)
	if err != nil {
		t.Fatalf("AnalyzeFolder returned error: %v", err)
	}

	// B0002-00000001 should be SKIPPED (already sent), B0002-00000002 should be QUEUED
	if result.Valid != 1 {
		t.Errorf("Expected valid=1 (only the new one), got %d", result.Valid)
	}
	if result.Skipped != 1 {
		t.Errorf("Expected skipped=1 (already sent this month), got %d", result.Skipped)
	}
}

func TestRescan_DetectsNewFiles(t *testing.T) {
	db := setupTestDB(t)
	campaignID := createTestCampaign(t, db)
	a := New(db, 2)

	dir := createTestFolder(t, []string{
		"00000001 - Factura  B0002-00000001 - CLIENTE UNO.pdf",
	})
	txtPath := createTestTXT(t, []string{
		"cliente1@test.com;B0002-00000001",
		"cliente2@test.com;B0002-00000002",
	})

	// First scan
	result1, err := a.AnalyzeFolder(campaignID, dir, txtPath, false)
	if err != nil {
		t.Fatalf("First AnalyzeFolder returned error: %v", err)
	}
	if result1.NewFiles != 1 {
		t.Errorf("First scan: expected new_files=1, got %d", result1.NewFiles)
	}

	// Add a new PDF to the folder
	newPDF := filepath.Join(dir, "00000002 - Factura  B0002-00000002 - CLIENTE DOS.pdf")
	if err := os.WriteFile(newPDF, []byte("fake-pdf"), 0644); err != nil {
		t.Fatalf("Failed to create new PDF: %v", err)
	}

	// Re-scan
	result2, err := a.AnalyzeFolder(campaignID, dir, txtPath, false)
	if err != nil {
		t.Fatalf("Rescan AnalyzeFolder returned error: %v", err)
	}

	if result2.Total != 2 {
		t.Errorf("Rescan: expected total=2, got %d", result2.Total)
	}
	if result2.NewFiles != 1 {
		t.Errorf("Rescan: expected new_files=1 (only the new one), got %d", result2.NewFiles)
	}
}

func TestRescan_NoDuplicates(t *testing.T) {
	db := setupTestDB(t)
	campaignID := createTestCampaign(t, db)
	a := New(db, 2)

	dir := createTestFolder(t, []string{
		"00000001 - Factura  B0002-00000001 - CLIENTE.pdf",
	})
	txtPath := createTestTXT(t, []string{
		"cliente1@test.com;B0002-00000001",
	})

	// Scan twice
	_, _ = a.AnalyzeFolder(campaignID, dir, txtPath, false)
	result2, _ := a.AnalyzeFolder(campaignID, dir, txtPath, false)

	if result2.NewFiles != 0 {
		t.Errorf("Second scan should have new_files=0, got %d", result2.NewFiles)
	}

	// Verify DB doesn't have duplicates
	invoices, _ := db.GetCampaignInvoices(campaignID, "")
	if len(invoices) != 1 {
		t.Errorf("Expected 1 invoice in DB (no duplicates), got %d", len(invoices))
	}
}

func TestAnalyzeFolder_TypeXInvoices(t *testing.T) {
	db := setupTestDB(t)
	campaignID := createTestCampaign(t, db)
	a := New(db, 2)

	files := []string{
		"00000001 - Factura  B0002-00000001 - CLIENTE UNO.pdf",
		"00000002 - Factura  X0003-00000002 - TIPO X.pdf",
	}
	dir := createTestFolder(t, files)
	txtPath := createTestTXT(t, []string{
		"cliente1@test.com;B0002-00000001",
	})

	result, err := a.AnalyzeFolder(campaignID, dir, txtPath, false)
	if err != nil {
		t.Fatalf("AnalyzeFolder returned error: %v", err)
	}

	if result.Valid != 1 {
		t.Errorf("Expected valid=1, got %d", result.Valid)
	}
	if result.Skipped != 1 {
		t.Errorf("Expected skipped=1 (type X), got %d", result.Skipped)
	}
}

