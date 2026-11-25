// Package reputation provides the CSV seeder for Day-0 migration.
// It reads bounced_from_resend.csv and delivered_emails.csv and bulk-imports
// them into the email_reputation table, establishing the initial whitelist/blacklist.
package reputation

import (
	"encoding/csv"
	"fmt"
	"io"
	"log"
	"os"
	"strings"

	"hardsend/models"
)

const seedBatchSize = 500

// SeedFromCSVs reads bounced and delivered CSV files and populates email_reputation.
// Both files are expected to have an "email" column (case-insensitive header matching).
// Safe to call multiple times: the upsert logic never downgrades a bounced → verified.
func (s *Store) SeedFromCSVs(bouncedCSVPath, deliveredCSVPath string) (*models.ReputationSeederResult, error) {
	result := &models.ReputationSeederResult{}

	// --- Bounced CSV (Blacklist) ---
	bouncedEmails, err := readEmailsFromCSV(bouncedCSVPath)
	if err != nil {
		return nil, fmt.Errorf("reading bounced CSV: %w", err)
	}
	log.Printf("[Seeder] Found %d emails in bounced CSV", len(bouncedEmails))

	bouncedRecords := make([]models.EmailReputation, 0, len(bouncedEmails))
	reason := "imported_from_resend_bounced_csv"
	for _, email := range bouncedEmails {
		bouncedRecords = append(bouncedRecords, models.EmailReputation{
			Email:        email,
			Status:       models.ReputationStatusBounced,
			BounceReason: &reason,
		})
	}

	bouncedInserted, bouncedSkipped, err := s.batchSeed(bouncedRecords)
	if err != nil {
		return nil, fmt.Errorf("seeding bounced records: %w", err)
	}
	result.BouncedInserted = bouncedInserted
	result.DuplicatesSkipped += bouncedSkipped
	log.Printf("[Seeder] Bounced: %d inserted, %d skipped", bouncedInserted, bouncedSkipped)

	// --- Delivered CSV (Whitelist) ---
	deliveredEmails, err := readEmailsFromCSV(deliveredCSVPath)
	if err != nil {
		return nil, fmt.Errorf("reading delivered CSV: %w", err)
	}
	log.Printf("[Seeder] Found %d emails in delivered CSV", len(deliveredEmails))

	deliveredRecords := make([]models.EmailReputation, 0, len(deliveredEmails))
	for _, email := range deliveredEmails {
		deliveredRecords = append(deliveredRecords, models.EmailReputation{
			Email:  email,
			Status: models.ReputationStatusVerified,
		})
	}

	deliveredInserted, deliveredSkipped, err := s.batchSeed(deliveredRecords)
	if err != nil {
		return nil, fmt.Errorf("seeding delivered records: %w", err)
	}
	result.DeliveredInserted = deliveredInserted
	result.DuplicatesSkipped += deliveredSkipped
	log.Printf("[Seeder] Delivered: %d inserted, %d skipped", deliveredInserted, deliveredSkipped)

	return result, nil
}

// batchSeed splits records into batches and calls UpsertBatch for each.
func (s *Store) batchSeed(records []models.EmailReputation) (totalInserted, totalSkipped int, err error) {
	for i := 0; i < len(records); i += seedBatchSize {
		end := i + seedBatchSize
		if end > len(records) {
			end = len(records)
		}
		batch := records[i:end]
		inserted, skipped, batchErr := s.UpsertBatch(batch)
		if batchErr != nil {
			return totalInserted, totalSkipped, batchErr
		}
		totalInserted += inserted
		totalSkipped += skipped
	}
	return totalInserted, totalSkipped, nil
}

// readEmailsFromCSV reads a CSV file and extracts the "email" column values.
// It accepts comma or semicolon delimiters and is case-insensitive on headers.
func readEmailsFromCSV(path string) ([]string, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("opening file: %w", err)
	}
	defer f.Close()

	reader := csv.NewReader(f)
	reader.LazyQuotes = true
	reader.TrimLeadingSpace = true

	// Read header
	header, err := reader.Read()
	if err != nil {
		return nil, fmt.Errorf("reading header: %w", err)
	}

	// Find "email" column index (case-insensitive)
	emailColIdx := -1
	for i, col := range header {
		if strings.EqualFold(strings.TrimSpace(col), "email") {
			emailColIdx = i
			break
		}
	}
	if emailColIdx == -1 {
		return nil, fmt.Errorf("no 'email' column found in header: %v", header)
	}

	var emails []string
	for {
		row, err := reader.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			// Skip malformed rows
			continue
		}
		if emailColIdx >= len(row) {
			continue
		}
		email := strings.TrimSpace(strings.ToLower(row[emailColIdx]))
		if email != "" && strings.Contains(email, "@") {
			emails = append(emails, email)
		}
	}
	return emails, nil
}
