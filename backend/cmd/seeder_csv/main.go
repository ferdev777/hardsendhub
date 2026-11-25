package main

import (
	"database/sql"
	"encoding/csv"
	"io"
	"log"
	"os"
	"strings"
	"time"

	_ "github.com/mattn/go-sqlite3"
)

func main() {
	db, err := sql.Open("sqlite3", "../hardsend_metrics.db")
	if err != nil {
		log.Fatalf("Failed to open DB: %v", err)
	}
	defer db.Close()

	file, err := os.Open("../emails/emails-sent-1791139968833.csv")
	if err != nil {
		log.Fatalf("Failed to open CSV: %v", err)
	}
	defer file.Close()

	reader := csv.NewReader(file)
	// Skip header
	_, err = reader.Read()
	if err != nil {
		log.Fatalf("Failed to read header: %v", err)
	}

	tx, err := db.Begin()
	if err != nil {
		log.Fatalf("Failed to begin tx: %v", err)
	}

	stmt, err := tx.Prepare(`
		INSERT INTO email_reputation (email, status, updated_at) 
		VALUES (?, ?, ?) 
		ON CONFLICT(email) DO UPDATE SET 
			status = CASE 
				WHEN excluded.status = 'bounced' THEN 'bounced' 
				ELSE email_reputation.status 
			END,
			updated_at = excluded.updated_at
	`)
	if err != nil {
		log.Fatalf("Failed to prepare statement: %v", err)
	}
	defer stmt.Close()

	countDelivered := 0
	countBounced := 0

	for {
		record, err := reader.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			log.Printf("Error reading record: %v", err)
			continue
		}

		if len(record) < 9 {
			continue
		}

		email := strings.ToLower(strings.TrimSpace(record[4]))
		lastEvent := strings.ToLower(strings.TrimSpace(record[8]))

		if email == "" {
			continue
		}

		status := "untested"
		if lastEvent == "delivered" {
			status = "verified"
			countDelivered++
		} else if lastEvent == "bounced" || lastEvent == "suppressed" || lastEvent == "complained" {
			status = "bounced"
			countBounced++
		} else {
			continue // Skip other statuses if any
		}

		_, err = stmt.Exec(email, status, time.Now())
		if err != nil {
			log.Printf("Error inserting %s: %v", email, err)
		}
	}

	if err := tx.Commit(); err != nil {
		log.Fatalf("Failed to commit: %v", err)
	}

	log.Printf("Seeding complete. Delivered (Verified): %d, Bounced/Suppressed: %d", countDelivered, countBounced)
}
