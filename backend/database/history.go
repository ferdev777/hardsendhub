package database

import (
	"time"

	"hardsend/models"
)

// GetInvoicesByDateRange retrieves all invoices from jobs created within the given date range,
// optionally filtered by status. Used for the monthly history view.
func (db *DB) GetInvoicesByDateRange(from, to time.Time, statusFilter string) ([]models.InvoiceHistoryRow, error) {
	var query string
	var args []interface{}

	if statusFilter != "" {
		query = `SELECT i.id, i.invoice_number, i.recipient_email, i.status, i.error_reason,
				        i.opened, i.bounced, i.complained, i.delivered, i.last_attempt_at, j.filename
				 FROM invoices i
				 JOIN jobs j ON i.job_id = j.id
				 WHERE j.created_at >= ? AND j.created_at <= ? AND i.status = ?
				 ORDER BY i.invoice_number`
		args = []interface{}{from, to, statusFilter}
	} else {
		query = `SELECT i.id, i.invoice_number, i.recipient_email, i.status, i.error_reason,
				        i.opened, i.bounced, i.complained, i.delivered, i.last_attempt_at, j.filename
				 FROM invoices i
				 JOIN jobs j ON i.job_id = j.id
				 WHERE j.created_at >= ? AND j.created_at <= ?
				 ORDER BY i.invoice_number`
		args = []interface{}{from, to}
	}

	rows, err := db.conn.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var results []models.InvoiceHistoryRow
	for rows.Next() {
		var r models.InvoiceHistoryRow
		if err := rows.Scan(&r.ID, &r.InvoiceNumber, &r.RecipientEmail, &r.Status, &r.ErrorReason,
			&r.Opened, &r.Bounced, &r.Complained, &r.Delivered, &r.LastAttemptAt, &r.JobFilename); err != nil {
			return nil, err
		}
		results = append(results, r)
	}
	return results, nil
}
