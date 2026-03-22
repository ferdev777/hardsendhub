package database

import (
	"database/sql"
	"fmt"
	"log"
	"strings"
	"time"

	_ "modernc.org/sqlite"

	"hardsend/models"
)

// DB wraps the SQLite database connection.
type DB struct {
	conn *sql.DB
}

// New creates a new database connection and runs migrations.
func New(dbPath string) (*DB, error) {
	conn, err := sql.Open("sqlite", dbPath+"?_pragma=journal_mode(WAL)&_pragma=busy_timeout(5000)")
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}

	// Enable WAL mode for better concurrent read/write performance
	conn.SetMaxOpenConns(1) // SQLite only supports one writer
	conn.SetMaxIdleConns(1)

	db := &DB{conn: conn}
	if err := db.migrate(); err != nil {
		return nil, fmt.Errorf("failed to run migrations: %w", err)
	}

	return db, nil
}

// migrate runs the database schema creation.
func (db *DB) migrate() error {
	migrations := []string{
		`CREATE TABLE IF NOT EXISTS jobs (
			id TEXT PRIMARY KEY,
			filename TEXT NOT NULL,
			total_files INTEGER NOT NULL DEFAULT 0,
			status TEXT NOT NULL DEFAULT 'PROCESSING',
			created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
		)`,
		`CREATE TABLE IF NOT EXISTS invoices (
			id TEXT PRIMARY KEY,
			job_id TEXT NOT NULL,
			invoice_number TEXT NOT NULL,
			recipient_email TEXT NOT NULL,
			status TEXT NOT NULL DEFAULT 'PENDING',
			error_reason TEXT,
			attempts INTEGER NOT NULL DEFAULT 0,
			last_attempt_at TIMESTAMP,
			FOREIGN KEY (job_id) REFERENCES jobs(id) ON DELETE CASCADE
		)`,
		`CREATE INDEX IF NOT EXISTS idx_invoices_job_id ON invoices(job_id)`,
		`CREATE INDEX IF NOT EXISTS idx_invoices_status ON invoices(status)`,
		`CREATE INDEX IF NOT EXISTS idx_invoices_invoice_number ON invoices(invoice_number)`,
		`CREATE TABLE IF NOT EXISTS missing_emails (
			id TEXT PRIMARY KEY,
			job_id TEXT NOT NULL,
			invoice_number TEXT NOT NULL,
			client_name TEXT NOT NULL DEFAULT '',
			created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
			resolved INTEGER NOT NULL DEFAULT 0,
			resolved_at TIMESTAMP,
			FOREIGN KEY (job_id) REFERENCES jobs(id) ON DELETE CASCADE
		)`,
		`CREATE INDEX IF NOT EXISTS idx_missing_emails_resolved ON missing_emails(resolved)`,
		`CREATE INDEX IF NOT EXISTS idx_missing_emails_created_at ON missing_emails(created_at)`,
	}

	for _, m := range migrations {
		if _, err := db.conn.Exec(m); err != nil {
			return fmt.Errorf("migration failed: %s: %w", m, err)
		}
	}

	// Add engagement columns if they don't exist (SQLite doesn't support IF NOT EXISTS in ALTER TABLE)
	alterations := []string{
		"ALTER TABLE invoices ADD COLUMN opened INTEGER NOT NULL DEFAULT 0",
		"ALTER TABLE invoices ADD COLUMN bounced INTEGER NOT NULL DEFAULT 0",
		"ALTER TABLE invoices ADD COLUMN complained INTEGER NOT NULL DEFAULT 0",
		"ALTER TABLE invoices ADD COLUMN delivered INTEGER NOT NULL DEFAULT 0",
	}

	for _, m := range alterations {
		_, err := db.conn.Exec(m)
		if err != nil {
			// Check if it's a "duplicate column name" error and ignore it
			if strings.Contains(err.Error(), "duplicate column name") {
				continue
			}
			return fmt.Errorf("alteration failed: %s: %w", m, err)
		}
	}

	// Add email and reason columns to missing_emails if they don't exist
	missingEmailAlterations := []string{
		"ALTER TABLE missing_emails ADD COLUMN email TEXT NOT NULL DEFAULT ''",
		"ALTER TABLE missing_emails ADD COLUMN reason TEXT NOT NULL DEFAULT 'no_email'",
	}
	for _, m := range missingEmailAlterations {
		_, err := db.conn.Exec(m)
		if err != nil {
			if strings.Contains(err.Error(), "duplicate column name") {
				continue
			}
			return fmt.Errorf("alteration failed: %s: %w", m, err)
		}
	}

	// Daily send counts table
	_, err := db.conn.Exec(`CREATE TABLE IF NOT EXISTS daily_send_counts (
		date TEXT PRIMARY KEY,
		sent_count INTEGER NOT NULL DEFAULT 0,
		daily_limit INTEGER NOT NULL DEFAULT 1500
	)`)
	if err != nil {
		return fmt.Errorf("failed to create daily_send_counts: %w", err)
	}

	// Campaign tables for persistent analysis plans
	campaignTables := []string{
		`CREATE TABLE IF NOT EXISTS campaigns (
			id TEXT PRIMARY KEY,
			name TEXT NOT NULL DEFAULT '',
			folder_path TEXT,
			txt_path TEXT,
			due_date TEXT NOT NULL DEFAULT '',
			template_subject TEXT NOT NULL DEFAULT '',
			template_body TEXT NOT NULL DEFAULT '',
			status TEXT NOT NULL DEFAULT 'DRAFT',
			total_invoices INTEGER NOT NULL DEFAULT 0,
			valid_count INTEGER NOT NULL DEFAULT 0,
			no_email_count INTEGER NOT NULL DEFAULT 0,
			blacklisted_count INTEGER NOT NULL DEFAULT 0,
			skipped_count INTEGER NOT NULL DEFAULT 0,
			created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
			updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
		)`,
		`CREATE INDEX IF NOT EXISTS idx_campaigns_status ON campaigns(status)`,
		`CREATE INDEX IF NOT EXISTS idx_campaigns_created_at ON campaigns(created_at)`,
		`CREATE TABLE IF NOT EXISTS campaign_invoices (
			id TEXT PRIMARY KEY,
			campaign_id TEXT NOT NULL,
			invoice_number TEXT NOT NULL,
			client_name TEXT NOT NULL DEFAULT '',
			email TEXT NOT NULL DEFAULT '',
			pdf_path TEXT NOT NULL DEFAULT '',
			status TEXT NOT NULL DEFAULT 'QUEUED',
			reason TEXT,
			FOREIGN KEY (campaign_id) REFERENCES campaigns(id) ON DELETE CASCADE
		)`,
		`CREATE INDEX IF NOT EXISTS idx_campaign_invoices_campaign_id ON campaign_invoices(campaign_id)`,
		`CREATE INDEX IF NOT EXISTS idx_campaign_invoices_status ON campaign_invoices(status)`,
	}
	for _, m := range campaignTables {
		if _, err := db.conn.Exec(m); err != nil {
			return fmt.Errorf("campaign migration failed: %s: %w", m, err)
		}
	}

	// Blacklist table for auto-blocked emails
	_, err = db.conn.Exec(`CREATE TABLE IF NOT EXISTS blacklist (
		id TEXT PRIMARY KEY,
		email TEXT NOT NULL UNIQUE,
		reason TEXT NOT NULL DEFAULT 'hard_bounce',
		original_invoice TEXT NOT NULL DEFAULT '',
		created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
	)`)
	if err != nil {
		return fmt.Errorf("blacklist migration failed: %w", err)
	}

	log.Println("[DB] Migrations completed successfully")
	return nil
}

// Close closes the database connection.
func (db *DB) Close() error {
	return db.conn.Close()
}

// PurgeAllData deletes all data from all tracking tables to start fresh.
func (db *DB) PurgeAllData() error {
	tables := []string{
		"campaign_invoices",
		"campaigns",
		"missing_emails",
		"invoices",
		"jobs",
		"daily_send_counts",
	}

	tx, err := db.conn.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	for _, table := range tables {
		if _, err := tx.Exec("DELETE FROM " + table); err != nil {
			return fmt.Errorf("failed to clear table %s: %w", table, err)
		}
	}

	return tx.Commit()
}

// --- Job Operations ---

// CreateJob inserts a new job record.
func (db *DB) CreateJob(job *models.Job) error {
	_, err := db.conn.Exec(
		"INSERT INTO jobs (id, filename, total_files, status, created_at) VALUES (?, ?, ?, ?, ?)",
		job.ID, job.Filename, job.TotalFiles, job.Status, job.CreatedAt,
	)
	return err
}

// UpdateJobStatus updates the status of a job.
func (db *DB) UpdateJobStatus(jobID, status string) error {
	_, err := db.conn.Exec("UPDATE jobs SET status = ? WHERE id = ?", status, jobID)
	return err
}

// UpdateJobTotalFiles updates the total file count for a job.
func (db *DB) UpdateJobTotalFiles(jobID string, total int) error {
	_, err := db.conn.Exec("UPDATE jobs SET total_files = ? WHERE id = ?", total, jobID)
	return err
}

// GetJob retrieves a job by ID.
func (db *DB) GetJob(jobID string) (*models.Job, error) {
	row := db.conn.QueryRow("SELECT id, filename, total_files, status, created_at FROM jobs WHERE id = ?", jobID)
	job := &models.Job{}
	err := row.Scan(&job.ID, &job.Filename, &job.TotalFiles, &job.Status, &job.CreatedAt)
	if err != nil {
		return nil, err
	}
	return job, nil
}

// GetRecentJobs retrieves the most recent jobs.
func (db *DB) GetRecentJobs(limit int) ([]models.Job, error) {
	rows, err := db.conn.Query(
		"SELECT id, filename, total_files, status, created_at FROM jobs ORDER BY created_at DESC LIMIT ?",
		limit,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var jobs []models.Job
	for rows.Next() {
		var j models.Job
		if err := rows.Scan(&j.ID, &j.Filename, &j.TotalFiles, &j.Status, &j.CreatedAt); err != nil {
			return nil, err
		}
		jobs = append(jobs, j)
	}
	return jobs, nil
}

// --- Invoice Operations ---

// CreateInvoice inserts a new invoice record.
func (db *DB) CreateInvoice(inv *models.Invoice) error {
	_, err := db.conn.Exec(
		`INSERT INTO invoices (id, job_id, invoice_number, recipient_email, status, error_reason, attempts, last_attempt_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		inv.ID, inv.JobID, inv.InvoiceNumber, inv.RecipientEmail, inv.Status, inv.ErrorReason, inv.Attempts, inv.LastAttemptAt,
	)
	return err
}

// UpdateInvoiceStatus updates the status, error reason, and attempt count of an invoice.
func (db *DB) UpdateInvoiceStatus(invoiceID, status string, errorReason *string, attempts int) error {
	now := time.Now()
	_, err := db.conn.Exec(
		"UPDATE invoices SET status = ?, error_reason = ?, attempts = ?, last_attempt_at = ? WHERE id = ?",
		status, errorReason, attempts, now, invoiceID,
	)
	return err
}

// CheckInvoiceSentThisMonth checks if an invoice number already has SUCCESS status this month
// AND has NOT bounced. Bounced invoices are allowed to be resent.
func (db *DB) CheckInvoiceSentThisMonth(invoiceNumber string) (bool, error) {
	now := time.Now()
	startOfMonth := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, time.Local)

	var count int
	err := db.conn.QueryRow(
		`SELECT COUNT(*) FROM invoices
		 WHERE invoice_number = ? AND status = 'SUCCESS' AND bounced = 0 AND last_attempt_at >= ?`,
		invoiceNumber, startOfMonth,
	).Scan(&count)
	if err != nil {
		return false, err
	}
	return count > 0, nil
}

// GetErrorInvoices retrieves invoices with error statuses for a specific job.
func (db *DB) GetErrorInvoices(jobID string) ([]models.ErrorInvoiceRow, error) {
	query := `SELECT id, invoice_number, recipient_email, status, error_reason, attempts
			  FROM invoices WHERE job_id = ? AND status IN ('ERROR_VALIDATION', 'ERROR_NETWORK')
			  ORDER BY invoice_number`

	rows, err := db.conn.Query(query, jobID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var results []models.ErrorInvoiceRow
	for rows.Next() {
		var r models.ErrorInvoiceRow
		if err := rows.Scan(&r.ID, &r.InvoiceNumber, &r.RecipientEmail, &r.Status, &r.ErrorReason, &r.Attempts); err != nil {
			return nil, err
		}
		results = append(results, r)
	}
	return results, nil
}

// GetAllErrorInvoices retrieves all invoices with error statuses across all jobs.
func (db *DB) GetAllErrorInvoices() ([]models.ErrorInvoiceRow, error) {
	query := `SELECT id, invoice_number, recipient_email, status, error_reason, attempts
			  FROM invoices WHERE status IN ('ERROR_VALIDATION', 'ERROR_NETWORK')
			  ORDER BY invoice_number`

	rows, err := db.conn.Query(query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var results []models.ErrorInvoiceRow
	for rows.Next() {
		var r models.ErrorInvoiceRow
		if err := rows.Scan(&r.ID, &r.InvoiceNumber, &r.RecipientEmail, &r.Status, &r.ErrorReason, &r.Attempts); err != nil {
			return nil, err
		}
		results = append(results, r)
	}
	return results, nil
}

// GetJobMetrics retrieves real-time metrics for a job.
func (db *DB) GetJobMetrics(jobID string) (*models.MetricsUpdate, error) {
	metrics := &models.MetricsUpdate{JobID: jobID}

	// Get total files from job
	err := db.conn.QueryRow("SELECT total_files FROM jobs WHERE id = ?", jobID).Scan(&metrics.TotalFiles)
	if err != nil {
		return nil, err
	}

	// Get counts by status
	rows, err := db.conn.Query(
		"SELECT status, COUNT(*) FROM invoices WHERE job_id = ? GROUP BY status",
		jobID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	for rows.Next() {
		var status string
		var count int
		if err := rows.Scan(&status, &count); err != nil {
			return nil, err
		}
		switch status {
		case models.InvoiceStatusSuccess:
			metrics.SuccessCount = count
		case models.InvoiceStatusErrorValidation:
			metrics.ErrorValidationCount = count
		case models.InvoiceStatusErrorNetwork:
			metrics.ErrorNetworkCount = count
		}
		if status != models.InvoiceStatusPending {
			metrics.ProcessedCount += count
		}
	}

	// Get engagement metrics (opened, bounced, complained are counts across all success emails)
	_ = db.conn.QueryRow("SELECT COUNT(*) FROM invoices WHERE job_id = ? AND opened = 1", jobID).Scan(&metrics.OpenedCount)
	_ = db.conn.QueryRow("SELECT COUNT(*) FROM invoices WHERE job_id = ? AND bounced = 1", jobID).Scan(&metrics.BounceCount)
	_ = db.conn.QueryRow("SELECT COUNT(*) FROM invoices WHERE job_id = ? AND complained = 1", jobID).Scan(&metrics.ComplaintCount)

	if metrics.ProcessedCount > 0 {
		metrics.SuccessRate = float64(metrics.SuccessCount) / float64(metrics.ProcessedCount) * 100
	}

	return metrics, nil
}

// --- Engagement Operations ---

// UpdateEngagementStatus marks an invoice as opened, bounced, or complained.
func (db *DB) UpdateEngagementStatus(invoiceID, eventType string) error {
	column := ""
	switch eventType {
	case "email.opened":
		column = "opened"
	case "email.bounced":
		column = "bounced"
	case "email.complained":
		column = "complained"
	case "email.delivered":
		column = "delivered"
	default:
		return fmt.Errorf("invalid engagement event type: %s", eventType)
	}

	query := fmt.Sprintf("UPDATE invoices SET %s = 1 WHERE id = ?", column)
	_, err := db.conn.Exec(query, invoiceID)
	return err
}

// GetInvoice retrieves an invoice by its ID.
func (db *DB) GetInvoice(invoiceID string) (*models.Invoice, error) {
	row := db.conn.QueryRow(`SELECT id, job_id, invoice_number, recipient_email, status, error_reason, attempts, last_attempt_at, opened, bounced, complained, delivered
						      FROM invoices WHERE id = ?`, invoiceID)
	inv := &models.Invoice{}
	err := row.Scan(&inv.ID, &inv.JobID, &inv.InvoiceNumber, &inv.RecipientEmail, &inv.Status, &inv.ErrorReason, &inv.Attempts, &inv.LastAttemptAt, &inv.Opened, &inv.Bounced, &inv.Complained, &inv.Delivered)
	if err != nil {
		return nil, err
	}
	return inv, nil
}

// GetLatestActiveJobID returns the ID of the latest active (PROCESSING) job, or the latest completed job.
func (db *DB) GetLatestActiveJobID() (string, error) {
	var jobID string
	err := db.conn.QueryRow(
		"SELECT id FROM jobs WHERE status = 'PROCESSING' ORDER BY created_at DESC LIMIT 1",
	).Scan(&jobID)
	if err == sql.ErrNoRows {
		// Get the latest completed job instead
		err = db.conn.QueryRow(
			"SELECT id FROM jobs ORDER BY created_at DESC LIMIT 1",
		).Scan(&jobID)
	}
	if err == sql.ErrNoRows {
		return "", nil
	}
	return jobID, err
}

// GetJobsByDateRange retrieves jobs within a date range.
func (db *DB) GetJobsByDateRange(from, to time.Time) ([]models.JobHistory, error) {
	query := `
		SELECT j.id, j.filename, j.total_files, j.status, j.created_at,
			COALESCE(SUM(CASE WHEN i.status = 'SUCCESS' THEN 1 ELSE 0 END), 0) AS success_count,
			COALESCE(SUM(CASE WHEN i.status IN ('ERROR_VALIDATION', 'ERROR_NETWORK') THEN 1 ELSE 0 END), 0) AS error_count
		FROM jobs j
		LEFT JOIN invoices i ON j.id = i.job_id
		WHERE j.created_at >= ? AND j.created_at <= ?
		GROUP BY j.id
		ORDER BY j.created_at DESC`

	rows, err := db.conn.Query(query, from, to)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var jobs []models.JobHistory
	for rows.Next() {
		var j models.JobHistory
		if err := rows.Scan(&j.ID, &j.Filename, &j.TotalFiles, &j.Status, &j.CreatedAt, &j.SuccessCount, &j.ErrorCount); err != nil {
			return nil, err
		}
		jobs = append(jobs, j)
	}
	return jobs, nil
}

// GetHistorySummary retrieves aggregated stats for a date range.
func (db *DB) GetHistorySummary(from, to time.Time) (*models.HistorySummary, error) {
	summary := &models.HistorySummary{}

	err := db.conn.QueryRow(`
		SELECT
			COALESCE(COUNT(DISTINCT j.id), 0),
			COALESCE(SUM(j.total_files), 0)
		FROM jobs j
		WHERE j.created_at >= ? AND j.created_at <= ?`,
		from, to,
	).Scan(&summary.TotalJobs, &summary.TotalInvoices)
	if err != nil {
		return nil, err
	}

	err = db.conn.QueryRow(`
		SELECT
			COALESCE(SUM(CASE WHEN i.status = 'SUCCESS' THEN 1 ELSE 0 END), 0),
			COALESCE(SUM(CASE WHEN i.status IN ('ERROR_VALIDATION', 'ERROR_NETWORK') THEN 1 ELSE 0 END), 0)
		FROM invoices i
		JOIN jobs j ON i.job_id = j.id
		WHERE j.created_at >= ? AND j.created_at <= ?`,
		from, to,
	).Scan(&summary.TotalSuccess, &summary.TotalErrors)
	if err != nil {
		return nil, err
	}

	if summary.TotalInvoices > 0 {
		summary.SuccessRate = float64(summary.TotalSuccess) / float64(summary.TotalInvoices) * 100
	}

	return summary, nil
}

// --- Missing Email Operations ---

// CreateMissingEmail inserts a new missing email record.
// If the same invoice_number was already resolved, it skips the insert to avoid duplicates.
func (db *DB) CreateMissingEmail(me *models.MissingEmail) error {
	// Check if this invoice was already resolved before — don't re-register it
	var count int
	err := db.conn.QueryRow(
		"SELECT COUNT(*) FROM missing_emails WHERE invoice_number = ? AND resolved = 1",
		me.InvoiceNumber,
	).Scan(&count)
	if err == nil && count > 0 {
		return nil // Already resolved before, skip
	}

	// Also check if it's already pending (same invoice in current batch)
	err = db.conn.QueryRow(
		"SELECT COUNT(*) FROM missing_emails WHERE invoice_number = ? AND resolved = 0",
		me.InvoiceNumber,
	).Scan(&count)
	if err == nil && count > 0 {
		return nil // Already registered as pending, skip duplicate
	}

	_, err = db.conn.Exec(
		`INSERT INTO missing_emails (id, job_id, invoice_number, client_name, email, reason, created_at, resolved)
		 VALUES (?, ?, ?, ?, ?, ?, ?, 0)`,
		me.ID, me.JobID, me.InvoiceNumber, me.ClientName, me.Email, me.Reason, me.CreatedAt,
	)
	return err
}

// GetMissingEmails retrieves missing emails within a date range, optionally filtering by resolved status.
func (db *DB) GetMissingEmails(from, to time.Time, showResolved bool) ([]models.MissingEmail, error) {
	var query string
	var args []interface{}

	if showResolved {
		query = `SELECT id, job_id, invoice_number, client_name, email, reason, created_at, resolved, resolved_at
				 FROM missing_emails
				 WHERE created_at >= ? AND created_at <= ?
				 ORDER BY created_at DESC`
		args = []interface{}{from, to}
	} else {
		query = `SELECT id, job_id, invoice_number, client_name, email, reason, created_at, resolved, resolved_at
				 FROM missing_emails
				 WHERE created_at >= ? AND created_at <= ? AND resolved = 0
				 ORDER BY created_at DESC`
		args = []interface{}{from, to}
	}

	rows, err := db.conn.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var results []models.MissingEmail
	for rows.Next() {
		var me models.MissingEmail
		if err := rows.Scan(&me.ID, &me.JobID, &me.InvoiceNumber, &me.ClientName, &me.Email, &me.Reason, &me.CreatedAt, &me.Resolved, &me.ResolvedAt); err != nil {
			return nil, err
		}
		results = append(results, me)
	}
	return results, nil
}

// GetMissingEmailSummary retrieves aggregated stats for missing emails in a date range.
func (db *DB) GetMissingEmailSummary(from, to time.Time) (*models.MissingEmailSummary, error) {
	summary := &models.MissingEmailSummary{}

	err := db.conn.QueryRow(`
		SELECT
			COALESCE(COUNT(*), 0),
			COALESCE(SUM(CASE WHEN resolved = 0 THEN 1 ELSE 0 END), 0),
			COALESCE(SUM(CASE WHEN resolved = 1 THEN 1 ELSE 0 END), 0)
		FROM missing_emails
		WHERE created_at >= ? AND created_at <= ?`,
		from, to,
	).Scan(&summary.Total, &summary.Pending, &summary.Resolved)
	if err != nil {
		return nil, err
	}
	return summary, nil
}

// ResolveMissingEmail marks a single missing email as resolved.
func (db *DB) ResolveMissingEmail(id string) error {
	now := time.Now()
	_, err := db.conn.Exec(
		"UPDATE missing_emails SET resolved = 1, resolved_at = ? WHERE id = ?",
		now, id,
	)
	return err
}

// ResolveMissingEmailsBulk marks multiple missing emails as resolved.
func (db *DB) ResolveMissingEmailsBulk(ids []string) error {
	if len(ids) == 0 {
		return nil
	}
	now := time.Now()
	placeholders := make([]string, len(ids))
	args := make([]interface{}, 0, len(ids)+1)
	args = append(args, now)
	for i, id := range ids {
		placeholders[i] = "?"
		args = append(args, id)
	}
	query := fmt.Sprintf(
		"UPDATE missing_emails SET resolved = 1, resolved_at = ? WHERE id IN (%s)",
		strings.Join(placeholders, ","),
	)
	_, err := db.conn.Exec(query, args...)
	return err
}

// ResolveAllMissingEmails marks all pending missing emails in a date range as resolved.
func (db *DB) ResolveAllMissingEmails(from, to time.Time) (int64, error) {
	now := time.Now()
	result, err := db.conn.Exec(
		"UPDATE missing_emails SET resolved = 1, resolved_at = ? WHERE resolved = 0 AND created_at >= ? AND created_at <= ?",
		now, from, to,
	)
	if err != nil {
		return 0, err
	}
	return result.RowsAffected()
}

// --- Daily Send Count Operations ---

// GetDailySendCount returns how many emails were sent today.
func (db *DB) GetDailySendCount() (int, int, error) {
	today := time.Now().Format("2006-01-02")
	var count, limit int
	err := db.conn.QueryRow("SELECT sent_count, daily_limit FROM daily_send_counts WHERE date = ?", today).Scan(&count, &limit)
	if err != nil {
		return 0, 0, nil // no record yet = 0 sent
	}
	return count, limit, nil
}

// IncrementDailySendCount atomically increments today's send count.
func (db *DB) IncrementDailySendCount(dailyLimit int) (int, error) {
	today := time.Now().Format("2006-01-02")
	_, err := db.conn.Exec(
		`INSERT INTO daily_send_counts (date, sent_count, daily_limit)
		 VALUES (?, 1, ?)
		 ON CONFLICT(date) DO UPDATE SET sent_count = sent_count + 1`,
		today, dailyLimit,
	)
	if err != nil {
		return 0, err
	}
	var count int
	db.conn.QueryRow("SELECT sent_count FROM daily_send_counts WHERE date = ?", today).Scan(&count)
	return count, nil
}

// CanSendToday checks if we haven't hit the daily limit.
func (db *DB) CanSendToday(dailyLimit int) bool {
	count, _, _ := db.GetDailySendCount()
	return count < dailyLimit
}
