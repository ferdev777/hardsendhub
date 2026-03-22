package main

import (
	"database/sql"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"strings"
	"time"

	_ "modernc.org/sqlite"
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

const resendAPIKey = "re_KpAqWS89_No7zyh1vkDN4bMVW9qu4MZ6J"

func fetchEmails(afterCursor string) (*ResendListResponse, error) {
	url := "https://api.resend.com/emails?limit=100"
	if afterCursor != "" {
		url += "&after=" + afterCursor
	}

	for retries := 0; retries < 3; retries++ {
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
		if resp.StatusCode == 429 {
			fmt.Printf("Rate limit hit, waiting 5 seconds... (retry %d)\n", retries+1)
			time.Sleep(5 * time.Second)
			continue
		}
		if resp.StatusCode != 200 {
			return nil, fmt.Errorf("status %d: %s", resp.StatusCode, string(body))
		}

		var result ResendListResponse
		if err := json.Unmarshal(body, &result); err != nil {
			return nil, fmt.Errorf("json parse error: %w", err)
		}
		return &result, nil
	}
	return nil, fmt.Errorf("too many retries for rate limit")
}

func main() {
	fmt.Println("1️⃣ Descargando estados desde Resend (últimas 48hs)...")
	
	limitTime := time.Now().Add(-48 * time.Hour)
	resendStatuses := make(map[string]string)
	cursor := ""
	page := 0
	stopFetching := false

	for !stopFetching {
		page++
		result, err := fetchEmails(cursor)
		if err != nil {
			log.Fatalf("Error API: %v", err)
		}
		if len(result.Data) == 0 { break }

		for _, e := range result.Data {
			parsedTime, _ := time.Parse("2006-01-02 15:04:05.999999-07", e.CreatedAt)
			if parsedTime.IsZero() {
				parsedTime, _ = time.Parse(time.RFC3339Nano, e.CreatedAt)
			}
			
			if !parsedTime.IsZero() && parsedTime.Before(limitTime) {
				stopFetching = true
				break
			}
			
			if len(e.To) > 0 {
				email := strings.ToLower(e.To[0])
				if _, exists := resendStatuses[email]; !exists {
					resendStatuses[email] = e.LastEvent
				}
			}
		}

		if !result.HasMore || stopFetching { break }
		cursor = result.Data[len(result.Data)-1].ID
		fmt.Printf("Página %d terminada, total emails en memoria: %d\n", page, len(resendStatuses))
		time.Sleep(1 * time.Second)
	}
	fmt.Printf("✔ Se obtuvieron %d estados desde Resend.\n", len(resendStatuses))

	fmt.Println("2️⃣ Consultando base de datos Hardsend...")
	dbName := "database.sqlite"
	if _, err := os.Stat("hardsend_metrics.db"); err == nil {
		dbName = "hardsend_metrics.db"
	}
	db, err := sql.Open("sqlite", dbName)
	if err != nil { log.Fatalf("Error DB: %v", err) }
	defer db.Close()

	var jobID string
	err = db.QueryRow("SELECT id FROM jobs ORDER BY created_at DESC LIMIT 1").Scan(&jobID)
	if err != nil { log.Fatalf("No se encontró ningún Job: %v", err) }

	rows, err := db.Query(`
		SELECT invoice_number, COALESCE(recipient_email, ''), status, COALESCE(error_reason, '') 
		FROM invoices 
		WHERE job_id = ?
	`, jobID)
	if err != nil { log.Fatalf("Error query: %v", err) }
	defer rows.Close()

	outPath := "C:\\Users\\fer\\Downloads\\facturas_verificadas_resend.csv"
	file, _ := os.Create(outPath)
	defer file.Close()

	writer := csv.NewWriter(file)
	writer.Comma = ';'
	defer writer.Flush()

	writer.Write([]string{"Factura", "Email", "Estado Interno", "Estado Real Resend", "Detalle"})

	total := 0
	okCount := 0
	for rows.Next() {
		var inv, email, status, reason string
		rows.Scan(&inv, &email, &status, &reason)

		realStatus := "PENDIENTE/OTRO"
		if email != "" {
			if st, ok := resendStatuses[strings.ToLower(email)]; ok {
				realStatus = strings.ToUpper(st)
				if realStatus == "DELIVERED" || realStatus == "SENT" || realStatus == "OPENED" || realStatus == "CLICKED" {
					okCount++
				}
			} else if status == "SUCCESS" {
				realStatus = "ENVIADO RECIENTE"
			}
		}

		writer.Write([]string{inv, email, status, realStatus, reason})
		total++
	}

	fmt.Println("\n=======================================")
	fmt.Printf("REPORTE GENERADO: %s\n", outPath)
	fmt.Printf("Total procesados: %d\n", total)
	fmt.Printf("Confirmados como EXITOSOS en Resend: %d\n", okCount)
	fmt.Println("=======================================")
}
