package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"

	"hardsend/database"
)

// Cloudflare API doesn't expose a direct bulk endpoint for suppression lists for MailChannels through normal REST yet,
// but it is possible via the Cloudflare Email Routing API if configured, or by pushing to a KV namespace 
// that the Worker reads. We'll simulate pushing a JSON list to the Worker's suppression endpoint.

type SyncRequest struct {
	BouncedEmails []string `json:"bounced_emails"`
}

func main() {
	dbPath := flag.String("db", "./hardsend_metrics.db", "Path to SQLite database")
	workerUrl := flag.String("url", "", "Cloudflare Worker Sync URL")
	token := flag.String("token", "", "Cloudflare Worker Auth Token")
	flag.Parse()

	if *workerUrl == "" || *token == "" {
		fmt.Println("Usage: cf_sync -url <WORKER_URL> -token <TOKEN> [-db <db_path>]")
		os.Exit(1)
	}

	db, err := database.New(*dbPath)
	if err != nil {
		log.Fatalf("Failed to open DB: %v", err)
	}

	// Fetch all bounced emails (reputation layer)
	// Querying directly for simplicity in CLI
	rows, err := db.RawConn().Query("SELECT email FROM email_reputation WHERE status = 'bounced' OR status = 'complained'")
	if err != nil {
		log.Fatalf("Query failed: %v", err)
	}
	defer rows.Close()

	var bounced []string
	for rows.Next() {
		var email string
		if err := rows.Scan(&email); err == nil {
			bounced = append(bounced, email)
		}
	}

	if len(bounced) == 0 {
		log.Println("No bounced emails to sync.")
		return
	}

	log.Printf("Found %d bounced emails to sync. Uploading in batches of 1000...", len(bounced))

	batchSize := 1000
	for i := 0; i < len(bounced); i += batchSize {
		end := i + batchSize
		if end > len(bounced) {
			end = len(bounced)
		}

		batch := bounced[i:end]
		reqBody := SyncRequest{BouncedEmails: batch}
		
		jsonData, err := json.Marshal(reqBody)
		if err != nil {
			log.Fatalf("Failed to marshal batch: %v", err)
		}

		req, err := http.NewRequest("POST", *workerUrl, bytes.NewBuffer(jsonData))
		if err != nil {
			log.Fatalf("Failed to create request: %v", err)
		}
		
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer "+*token)

		client := &http.Client{}
		resp, err := client.Do(req)
		if err != nil {
			log.Fatalf("Failed to upload batch %d-%d: %v", i, end, err)
		}

		if resp.StatusCode != http.StatusOK {
			log.Printf("WARNING: Batch %d-%d returned status %d", i, end, resp.StatusCode)
		} else {
			log.Printf("Batch %d-%d synced successfully.", i, end)
		}
		resp.Body.Close()
	}

	log.Println("Sync complete.")
}
