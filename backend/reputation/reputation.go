package reputation

import (
	"database/sql"
	"fmt"
	"time"

	"hardsend/models"
)

// Store wraps the SQLite connection for email_reputation operations.
type Store struct {
	conn *sql.DB
}

// New creates a new reputation Store.
func New(conn *sql.DB) *Store {
	return &Store{conn: conn}
}

// GetStatus returns the reputation status for an email, or "untested" if not found.
func (s *Store) GetStatus(email string) string {
	var status string
	err := s.conn.QueryRow(
		"SELECT status FROM email_reputation WHERE email = ?", email,
	).Scan(&status)
	if err == sql.ErrNoRows {
		return models.ReputationStatusUntested
	}
	if err != nil {
		return models.ReputationStatusUntested
	}
	return status
}

// IsBlocked returns true if the email is bounced or cashier_only (should not be emailed).
func (s *Store) IsBlocked(email string) bool {
	status := s.GetStatus(email)
	return status == models.ReputationStatusBounced || status == models.ReputationStatusCashierOnly
}

// Upsert inserts or updates an email's reputation record.
func (s *Store) Upsert(email, status string, bounceReason *string) error {
	_, err := s.conn.Exec(
		`INSERT INTO email_reputation (email, status, bounce_reason, updated_at)
		 VALUES (?, ?, ?, ?)
		 ON CONFLICT(email) DO UPDATE SET
		   status = excluded.status,
		   bounce_reason = excluded.bounce_reason,
		   updated_at = excluded.updated_at`,
		email, status, bounceReason, time.Now(),
	)
	return err
}

// MarkBounced marks an email as hard-bounced with the given reason.
func (s *Store) MarkBounced(email, reason string) error {
	return s.Upsert(email, models.ReputationStatusBounced, &reason)
}

// MarkVerified marks an email as successfully delivered.
func (s *Store) MarkVerified(email string) error {
	return s.Upsert(email, models.ReputationStatusVerified, nil)
}

// MarkCashierOnly marks a client as voluntarily excluded from email campaigns.
func (s *Store) MarkCashierOnly(email string) error {
	return s.Upsert(email, models.ReputationStatusCashierOnly, nil)
}

// GetAll returns all reputation records, ordered by status and email.
func (s *Store) GetAll() ([]models.EmailReputation, error) {
	rows, err := s.conn.Query(
		"SELECT email, status, bounce_reason, updated_at FROM email_reputation ORDER BY status, email",
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var results []models.EmailReputation
	for rows.Next() {
		var r models.EmailReputation
		if err := rows.Scan(&r.Email, &r.Status, &r.BounceReason, &r.UpdatedAt); err != nil {
			return nil, err
		}
		results = append(results, r)
	}
	return results, nil
}

// GetByStatus returns all emails with a given reputation status.
func (s *Store) GetByStatus(status string) ([]string, error) {
	rows, err := s.conn.Query(
		"SELECT email FROM email_reputation WHERE status = ? ORDER BY email", status,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var emails []string
	for rows.Next() {
		var email string
		if err := rows.Scan(&email); err != nil {
			return nil, err
		}
		emails = append(emails, email)
	}
	return emails, nil
}

// UpsertBatch inserts or updates multiple reputation records in a single transaction.
// Used by the CSV seeder for fast bulk import.
func (s *Store) UpsertBatch(records []models.EmailReputation) (inserted int, skipped int, err error) {
	if len(records) == 0 {
		return 0, 0, nil
	}

	tx, txErr := s.conn.Begin()
	if txErr != nil {
		return 0, 0, txErr
	}
	defer tx.Rollback()

	stmt, prepErr := tx.Prepare(
		`INSERT INTO email_reputation (email, status, bounce_reason, updated_at)
		 VALUES (?, ?, ?, ?)
		 ON CONFLICT(email) DO UPDATE SET
		   status = CASE
		     -- Never downgrade a bounced email to verified
		     WHEN email_reputation.status = 'bounced' AND excluded.status = 'verified' THEN 'bounced'
		     -- Never overwrite cashier_only
		     WHEN email_reputation.status = 'cashier_only' THEN 'cashier_only'
		     ELSE excluded.status
		   END,
		   bounce_reason = COALESCE(excluded.bounce_reason, email_reputation.bounce_reason),
		   updated_at = excluded.updated_at`,
	)
	if prepErr != nil {
		return 0, 0, fmt.Errorf("preparing batch upsert: %w", prepErr)
	}
	defer stmt.Close()

	now := time.Now()
	for _, r := range records {
		result, execErr := stmt.Exec(r.Email, r.Status, r.BounceReason, now)
		if execErr != nil {
			skipped++
			continue
		}
		rowsAffected, _ := result.RowsAffected()
		if rowsAffected > 0 {
			inserted++
		} else {
			skipped++
		}
	}

	if commitErr := tx.Commit(); commitErr != nil {
		return 0, 0, fmt.Errorf("committing batch upsert: %w", commitErr)
	}
	return inserted, skipped, nil
}
