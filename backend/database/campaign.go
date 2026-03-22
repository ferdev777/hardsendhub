package database

import (
	"database/sql"
	"time"

	"hardsend/models"
)

// --- Campaign Operations ---

// CreateCampaign inserts a new campaign record.
func (db *DB) CreateCampaign(c *models.Campaign) error {
	_, err := db.conn.Exec(
		`INSERT INTO campaigns (id, name, folder_path, txt_path, due_date, template_subject, template_body, status, total_invoices, valid_count, no_email_count, skipped_count, created_at, updated_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		c.ID, c.Name, c.FolderPath, c.TxtPath, c.DueDate, c.TemplateSubject, c.TemplateBody,
		c.Status, c.TotalInvoices, c.ValidCount, c.NoEmailCount, c.SkippedCount,
		c.CreatedAt, c.UpdatedAt,
	)
	return err
}

// GetCampaign retrieves a campaign by ID.
func (db *DB) GetCampaign(campaignID string) (*models.Campaign, error) {
	row := db.conn.QueryRow(
		`SELECT id, name, folder_path, txt_path, due_date, template_subject, template_body, status,
		        total_invoices, valid_count, no_email_count, skipped_count, created_at, updated_at
		 FROM campaigns WHERE id = ?`, campaignID,
	)
	c := &models.Campaign{}
	err := row.Scan(
		&c.ID, &c.Name, &c.FolderPath, &c.TxtPath, &c.DueDate, &c.TemplateSubject, &c.TemplateBody,
		&c.Status, &c.TotalInvoices, &c.ValidCount, &c.NoEmailCount, &c.SkippedCount,
		&c.CreatedAt, &c.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}
	return c, nil
}

// GetActiveCampaign retrieves the latest campaign that is not completed or cancelled.
func (db *DB) GetActiveCampaign() (*models.Campaign, error) {
	row := db.conn.QueryRow(
		`SELECT id, name, folder_path, txt_path, due_date, template_subject, template_body, status,
		        total_invoices, valid_count, no_email_count, skipped_count, created_at, updated_at
		 FROM campaigns
		 WHERE status NOT IN ('COMPLETED', 'CANCELLED')
		 ORDER BY created_at DESC LIMIT 1`,
	)
	c := &models.Campaign{}
	err := row.Scan(
		&c.ID, &c.Name, &c.FolderPath, &c.TxtPath, &c.DueDate, &c.TemplateSubject, &c.TemplateBody,
		&c.Status, &c.TotalInvoices, &c.ValidCount, &c.NoEmailCount, &c.SkippedCount,
		&c.CreatedAt, &c.UpdatedAt,
	)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return c, nil
}

// UpdateCampaignStatus updates the status and updated_at timestamp of a campaign.
func (db *DB) UpdateCampaignStatus(campaignID, status string) error {
	_, err := db.conn.Exec(
		"UPDATE campaigns SET status = ?, updated_at = ? WHERE id = ?",
		status, time.Now(), campaignID,
	)
	return err
}

// UpdateCampaignCounts updates the invoice counts after analysis.
func (db *DB) UpdateCampaignCounts(campaignID string, total, valid, noEmail, skipped int) error {
	_, err := db.conn.Exec(
		`UPDATE campaigns SET total_invoices = ?, valid_count = ?, no_email_count = ?, skipped_count = ?, updated_at = ?
		 WHERE id = ?`,
		total, valid, noEmail, skipped, time.Now(), campaignID,
	)
	return err
}

// --- Campaign Invoice Operations ---

// CreateCampaignInvoice inserts a new campaign invoice record.
func (db *DB) CreateCampaignInvoice(ci *models.CampaignInvoice) error {
	_, err := db.conn.Exec(
		`INSERT INTO campaign_invoices (id, campaign_id, invoice_number, client_name, email, pdf_path, status, reason)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		ci.ID, ci.CampaignID, ci.InvoiceNumber, ci.ClientName, ci.Email, ci.PDFPath, ci.Status, ci.Reason,
	)
	return err
}

// CreateCampaignInvoicesBatch inserts multiple campaign invoices in a single transaction.
func (db *DB) CreateCampaignInvoicesBatch(invoices []models.CampaignInvoice) error {
	tx, err := db.conn.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	stmt, err := tx.Prepare(
		`INSERT INTO campaign_invoices (id, campaign_id, invoice_number, client_name, email, pdf_path, status, reason)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
	)
	if err != nil {
		return err
	}
	defer stmt.Close()

	for _, ci := range invoices {
		_, err := stmt.Exec(ci.ID, ci.CampaignID, ci.InvoiceNumber, ci.ClientName, ci.Email, ci.PDFPath, ci.Status, ci.Reason)
		if err != nil {
			return err
		}
	}

	return tx.Commit()
}

// GetCampaignInvoices retrieves all invoices for a campaign, optionally filtered by status.
func (db *DB) GetCampaignInvoices(campaignID, statusFilter string) ([]models.CampaignInvoice, error) {
	var query string
	var args []interface{}

	if statusFilter != "" {
		query = `SELECT id, campaign_id, invoice_number, client_name, email, pdf_path, status, reason
				 FROM campaign_invoices WHERE campaign_id = ? AND status = ?
				 ORDER BY invoice_number`
		args = []interface{}{campaignID, statusFilter}
	} else {
		query = `SELECT id, campaign_id, invoice_number, client_name, email, pdf_path, status, reason
				 FROM campaign_invoices WHERE campaign_id = ?
				 ORDER BY invoice_number`
		args = []interface{}{campaignID}
	}

	rows, err := db.conn.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var results []models.CampaignInvoice
	for rows.Next() {
		var ci models.CampaignInvoice
		if err := rows.Scan(&ci.ID, &ci.CampaignID, &ci.InvoiceNumber, &ci.ClientName, &ci.Email, &ci.PDFPath, &ci.Status, &ci.Reason); err != nil {
			return nil, err
		}
		results = append(results, ci)
	}
	return results, nil
}

// GetCampaignInvoiceByNumber checks if an invoice number already exists in a campaign.
func (db *DB) GetCampaignInvoiceByNumber(campaignID, invoiceNumber string) (*models.CampaignInvoice, error) {
	row := db.conn.QueryRow(
		`SELECT id, campaign_id, invoice_number, client_name, email, pdf_path, status, reason
		 FROM campaign_invoices WHERE campaign_id = ? AND invoice_number = ?`,
		campaignID, invoiceNumber,
	)
	ci := &models.CampaignInvoice{}
	err := row.Scan(&ci.ID, &ci.CampaignID, &ci.InvoiceNumber, &ci.ClientName, &ci.Email, &ci.PDFPath, &ci.Status, &ci.Reason)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return ci, nil
}

// UpdateCampaignInvoiceStatus updates the status of campaign invoices (e.g., QUEUED -> SENT).
func (db *DB) UpdateCampaignInvoiceStatus(id, status string) error {
	_, err := db.conn.Exec(
		"UPDATE campaign_invoices SET status = ? WHERE id = ?",
		status, id,
	)
	return err
}

// UpdateCampaignInvoicesBulkStatus updates the status of all campaign invoices with a given current status.
func (db *DB) UpdateCampaignInvoicesBulkStatus(campaignID, currentStatus, newStatus string) (int64, error) {
	result, err := db.conn.Exec(
		"UPDATE campaign_invoices SET status = ? WHERE campaign_id = ? AND status = ?",
		newStatus, campaignID, currentStatus,
	)
	if err != nil {
		return 0, err
	}
	return result.RowsAffected()
}
