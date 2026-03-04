package database

import (
	"os"
	"testing"
	"time"

	"hardsend/models"
)

// helper: create a test DB in a temp file
func newTestDB(t *testing.T) (*DB, func()) {
	t.Helper()
	tmpFile, err := os.CreateTemp("", "hardsend_test_*.db")
	if err != nil {
		t.Fatal(err)
	}
	tmpFile.Close()

	db, err := New(tmpFile.Name())
	if err != nil {
		os.Remove(tmpFile.Name())
		t.Fatal(err)
	}

	cleanup := func() {
		db.Close()
		os.Remove(tmpFile.Name())
	}
	return db, cleanup
}

func TestNew_CreatesTables(t *testing.T) {
	db, cleanup := newTestDB(t)
	defer cleanup()

	// Should be able to query jobs and invoices after creation
	var count int
	err := db.conn.QueryRow("SELECT COUNT(*) FROM jobs").Scan(&count)
	if err != nil {
		t.Fatalf("jobs table should exist: %v", err)
	}
	err = db.conn.QueryRow("SELECT COUNT(*) FROM invoices").Scan(&count)
	if err != nil {
		t.Fatalf("invoices table should exist: %v", err)
	}
	err = db.conn.QueryRow("SELECT COUNT(*) FROM missing_emails").Scan(&count)
	if err != nil {
		t.Fatalf("missing_emails table should exist: %v", err)
	}
	err = db.conn.QueryRow("SELECT COUNT(*) FROM daily_send_counts").Scan(&count)
	if err != nil {
		t.Fatalf("daily_send_counts table should exist: %v", err)
	}
}

func TestCreateJob_And_GetJobByID(t *testing.T) {
	db, cleanup := newTestDB(t)
	defer cleanup()

	job := &models.Job{
		ID:         "job-001",
		Filename:   "test.zip",
		TotalFiles: 10,
		Status:     models.JobStatusProcessing,
		CreatedAt:  time.Now(),
	}

	if err := db.CreateJob(job); err != nil {
		t.Fatalf("CreateJob() error = %v", err)
	}

	got, err := db.GetJob("job-001")
	if err != nil {
		t.Fatalf("GetJob() error = %v", err)
	}
	if got.ID != "job-001" {
		t.Errorf("ID = %q, want job-001", got.ID)
	}
	if got.Filename != "test.zip" {
		t.Errorf("Filename = %q, want test.zip", got.Filename)
	}
	if got.Status != models.JobStatusProcessing {
		t.Errorf("Status = %q, want PROCESSING", got.Status)
	}
}

func TestCreateInvoice_And_Metrics(t *testing.T) {
	db, cleanup := newTestDB(t)
	defer cleanup()

	// Create job first
	job := &models.Job{ID: "job-001", Filename: "test.zip", Status: models.JobStatusProcessing, CreatedAt: time.Now()}
	db.CreateJob(job)

	// Create invoices
	inv1 := &models.Invoice{ID: "inv-1", JobID: "job-001", InvoiceNumber: "B0002-00000001", RecipientEmail: "a@b.com", Status: models.InvoiceStatusSuccess}
	inv2 := &models.Invoice{ID: "inv-2", JobID: "job-001", InvoiceNumber: "B0002-00000002", RecipientEmail: "c@d.com", Status: models.InvoiceStatusSuccess}
	inv3 := &models.Invoice{ID: "inv-3", JobID: "job-001", InvoiceNumber: "B0002-00000003", RecipientEmail: "", Status: models.InvoiceStatusErrorValidation}

	db.CreateInvoice(inv1)
	db.CreateInvoice(inv2)
	db.CreateInvoice(inv3)

	metrics, err := db.GetJobMetrics("job-001")
	if err != nil {
		t.Fatalf("GetJobMetrics() error = %v", err)
	}
	if metrics.SuccessCount != 2 {
		t.Errorf("SuccessCount = %d, want 2", metrics.SuccessCount)
	}
	if metrics.ErrorValidationCount != 1 {
		t.Errorf("ErrorValidationCount = %d, want 1", metrics.ErrorValidationCount)
	}
}

func TestCheckInvoiceSentThisMonth_WithBounced(t *testing.T) {
	db, cleanup := newTestDB(t)
	defer cleanup()

	job := &models.Job{ID: "job-001", Filename: "test.zip", Status: models.JobStatusProcessing, CreatedAt: time.Now()}
	db.CreateJob(job)

	now := time.Now()

	// Create a successful invoice (not bounced)
	inv := &models.Invoice{
		ID: "inv-1", JobID: "job-001", InvoiceNumber: "B0002-00000001",
		RecipientEmail: "a@b.com", Status: models.InvoiceStatusSuccess,
	}
	db.CreateInvoice(inv)
	db.UpdateInvoiceStatus("inv-1", models.InvoiceStatusSuccess, nil, 1)
	// Set last_attempt_at manually
	db.conn.Exec("UPDATE invoices SET last_attempt_at = ? WHERE id = ?", now, "inv-1")

	// Should be found (sent this month, not bounced)
	sent, _ := db.CheckInvoiceSentThisMonth("B0002-00000001")
	if !sent {
		t.Error("CheckInvoiceSentThisMonth() should return true for delivered invoice")
	}

	// Mark as bounced
	db.conn.Exec("UPDATE invoices SET bounced = 1 WHERE id = ?", "inv-1")

	// Should NOT be found (bounced = allowed to resend)
	sent, _ = db.CheckInvoiceSentThisMonth("B0002-00000001")
	if sent {
		t.Error("CheckInvoiceSentThisMonth() should return false for bounced invoice (allow resend)")
	}
}

func TestDailySendCount(t *testing.T) {
	db, cleanup := newTestDB(t)
	defer cleanup()

	// Initially zero
	count, _, _ := db.GetDailySendCount()
	if count != 0 {
		t.Errorf("Initial daily count = %d, want 0", count)
	}

	// Can send (limit not reached)
	if !db.CanSendToday(1500) {
		t.Error("CanSendToday(1500) should be true when count is 0")
	}

	// Increment
	newCount, err := db.IncrementDailySendCount(1500)
	if err != nil {
		t.Fatalf("IncrementDailySendCount() error = %v", err)
	}
	if newCount != 1 {
		t.Errorf("After increment, count = %d, want 1", newCount)
	}

	// Increment more
	db.IncrementDailySendCount(1500)
	db.IncrementDailySendCount(1500)

	count, _, _ = db.GetDailySendCount()
	if count != 3 {
		t.Errorf("After 3 increments, count = %d, want 3", count)
	}

	// Can NOT send if limit is 3
	if db.CanSendToday(3) {
		t.Error("CanSendToday(3) should be false when count is 3")
	}

	// CAN send if limit is 10
	if !db.CanSendToday(10) {
		t.Error("CanSendToday(10) should be true when count is 3")
	}
}

func TestMissingEmails(t *testing.T) {
	db, cleanup := newTestDB(t)
	defer cleanup()

	job := &models.Job{ID: "job-001", Filename: "test.zip", Status: models.JobStatusProcessing, CreatedAt: time.Now()}
	db.CreateJob(job)

	me := &models.MissingEmail{
		ID:            "me-1",
		JobID:         "job-001",
		InvoiceNumber: "B0002-00000001",
		ClientName:    "PEREZ JUAN",
		Email:         "",
		Reason:        "no_email",
		CreatedAt:     time.Now(),
	}

	if err := db.CreateMissingEmail(me); err != nil {
		t.Fatalf("CreateMissingEmail() error = %v", err)
	}

	// Verify it was created
	var count int
	db.conn.QueryRow("SELECT COUNT(*) FROM missing_emails WHERE id = ?", "me-1").Scan(&count)
	if count != 1 {
		t.Errorf("Missing email count = %d, want 1", count)
	}
}

func TestUpdateJobTotalFiles(t *testing.T) {
	db, cleanup := newTestDB(t)
	defer cleanup()

	job := &models.Job{ID: "job-001", Filename: "test.zip", TotalFiles: 0, Status: models.JobStatusProcessing, CreatedAt: time.Now()}
	db.CreateJob(job)

	db.UpdateJobTotalFiles("job-001", 150)

	got, _ := db.GetJob("job-001")
	if got.TotalFiles != 150 {
		t.Errorf("TotalFiles = %d, want 150", got.TotalFiles)
	}
}
