package main

import (
	"database/sql"
	"fmt"
	"os"

	_ "modernc.org/sqlite"
)

func main() {
	dbName := "database.sqlite"
	if _, err := os.Stat("hardsend_metrics.db"); err == nil {
		dbName = "hardsend_metrics.db"
	}

	db, err := sql.Open("sqlite", dbName)
	if err != nil {
		fmt.Println("Error abriendo DB:", err)
		return
	}
	defer db.Close()

	var jobID string
	err = db.QueryRow("SELECT id FROM jobs ORDER BY created_at DESC LIMIT 1").Scan(&jobID)
	if err != nil {
		fmt.Println("Error getting latest job:", err)
		return
	}
	fmt.Printf("=== RESULTADOS DEL ÚLTIMO TRABAJO (Job %s) ===\n", jobID)

	rows, err := db.Query("SELECT status, COALESCE(error_reason, ''), COUNT(*) FROM invoices WHERE job_id = ? GROUP BY status, error_reason", jobID)
	if err == nil {
		for rows.Next() {
			var st, reason string
			var c int
			rows.Scan(&st, &reason, &c)
			if reason == "" { reason = "[SIN RAZON ESPECIFICA]" }
			fmt.Printf("Estado '%s' | Motivo '%s' | %d facturas\n", st, reason, c)
		}
		rows.Close()
	}
}
