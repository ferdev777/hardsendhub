package main

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"time"

	_ "modernc.org/sqlite"
)

const (
	resendAPIKey = "re_KpAqWS89_No7zyh1vkDN4bMVW9qu4MZ6J"
	dbPath       = "C:/Users/fer/workspace/devrow/backend/hardsend_metrics.db"
)

type ResendEmail struct {
	ID        string   `json:"id"`
	To        []string `json:"to"`
	From      string   `json:"from"`
	Subject   string   `json:"subject"`
	CreatedAt string   `json:"created_at"`
	LastEvent string   `json:"last_event"`
}

type ResendListResponse struct {
	Object  string        `json:"object"`
	HasMore bool          `json:"has_more"`
	Data    []ResendEmail `json:"data"`
}

func fetchEmails(afterCursor string) (*ResendListResponse, error) {
	url := "https://api.resend.com/emails?limit=100"
	if afterCursor != "" {
		url += "&after=" + afterCursor
	}

	req, _ := http.NewRequest("GET", url, nil)
	req.Header.Set("Authorization", "Bearer "+resendAPIKey)
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("status %d: %s", resp.StatusCode, string(body))
	}

	var result ResendListResponse
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("json parse error: %w", err)
	}
	return &result, nil
}

func main() {
	log.Println("🔍 Consultando Resend API para obtener estado de todos los emails...")

	// Fetch all emails from Resend
	var allEmails []ResendEmail
	cursor := ""
	page := 0

	for {
		page++
		log.Printf("  📄 Página %d (cursor: %s)...", page, func() string {
			if cursor == "" {
				return "inicio"
			}
			return cursor[:20] + "..."
		}())

		result, err := fetchEmails(cursor)
		if err != nil {
			log.Fatalf("Error fetching page %d: %v", page, err)
		}

		allEmails = append(allEmails, result.Data...)
		log.Printf("  ✅ Obtenidos %d emails (total: %d)", len(result.Data), len(allEmails))

		if !result.HasMore || len(result.Data) == 0 {
			break
		}

		// Use last email ID as cursor for next page
		cursor = result.Data[len(result.Data)-1].ID

		// Rate limit: wait a bit between requests
		time.Sleep(500 * time.Millisecond)
	}

	log.Printf("\n📊 Total emails obtenidos de Resend: %d\n", len(allEmails))

	// Count by status
	statusCounts := map[string]int{}
	bouncedEmails := []ResendEmail{}
	deliveredEmails := []ResendEmail{}

	for _, e := range allEmails {
		statusCounts[e.LastEvent]++
		if e.LastEvent == "bounced" {
			bouncedEmails = append(bouncedEmails, e)
		}
		if e.LastEvent == "delivered" || e.LastEvent == "opened" || e.LastEvent == "clicked" {
			deliveredEmails = append(deliveredEmails, e)
		}
	}

	log.Println("\n📈 Desglose por estado:")
	for status, count := range statusCounts {
		log.Printf("  %s: %d", status, count)
	}

	log.Printf("\n✅ Emails entregados OK: %d", len(deliveredEmails))
	log.Printf("❌ Emails que rebotaron: %d", len(bouncedEmails))

	// Save bounced to file
	if len(bouncedEmails) > 0 {
		f, _ := os.Create("C:/Users/fer/workspace/devrow/bounced_from_resend.csv")
		defer f.Close()
		f.WriteString("email,last_event,created_at,resend_id\n")
		for _, e := range bouncedEmails {
			email := ""
			if len(e.To) > 0 {
				email = e.To[0]
			}
			f.WriteString(fmt.Sprintf("%s,%s,%s,%s\n", email, e.LastEvent, e.CreatedAt, e.ID))
		}
		log.Printf("\n📁 Lista de bounced guardada en: bounced_from_resend.csv")
	}

	// Save delivered to file (for exclusion)
	deliveredSet := map[string]bool{}
	for _, e := range deliveredEmails {
		if len(e.To) > 0 {
			deliveredSet[e.To[0]] = true
		}
	}

	f2, _ := os.Create("C:/Users/fer/workspace/devrow/delivered_emails.csv")
	defer f2.Close()
	f2.WriteString("email\n")
	for email := range deliveredSet {
		f2.WriteString(email + "\n")
	}
	log.Printf("📁 Lista de delivered guardada en: delivered_emails.csv (%d emails únicos)", len(deliveredSet))

	// Now update our DB with the bounced info
	log.Println("\n🔄 Actualizando base de datos local...")
	db, err := sql.Open("sqlite", dbPath+"?_pragma=journal_mode(WAL)&_pragma=busy_timeout(5000)")
	if err != nil {
		log.Fatalf("Failed to open DB: %v", err)
	}
	defer db.Close()

	// Update bounced column for bounced emails
	updatedBounced := 0
	for _, e := range bouncedEmails {
		if len(e.To) > 0 {
			result, err := db.Exec("UPDATE invoices SET bounced = 1 WHERE recipient_email = ? AND bounced = 0", e.To[0])
			if err == nil {
				rows, _ := result.RowsAffected()
				updatedBounced += int(rows)
			}
		}
	}
	log.Printf("✅ Marcadas %d facturas como bounced en DB local", updatedBounced)

	// Summary
	log.Println("\n" + "=================================")
	log.Println("📋 RESUMEN FINAL")
	log.Println("=================================")
	log.Printf("Total emails en Resend:    %d", len(allEmails))
	log.Printf("Entregados (delivered):    %d", len(deliveredEmails))
	log.Printf("Rebotados (bounced):       %d", len(bouncedEmails))
	log.Printf("Emails únicos entregados:  %d", len(deliveredSet))
	log.Printf("Emails a REENVIAR:         %d", len(allEmails)-len(deliveredEmails))
	log.Println("\nArchivos generados:")
	log.Println("  → bounced_from_resend.csv  (emails que rebotaron)")
	log.Println("  → delivered_emails.csv     (emails exitosos, NO reenviar)")
}
