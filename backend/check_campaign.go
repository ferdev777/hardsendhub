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

	// Contar los estados de la "Campaña" actual o activa
	rows, err := db.Query(`
		SELECT status, reason, COUNT(*)
		FROM campaign_invoices
		GROUP BY status, reason
	`)
	if err == nil {
		fmt.Println("=== CAMPAIGN_INVOICES CURRENT ===")
		for rows.Next() {
			var st string
			var reason sql.NullString
			var c int
			rows.Scan(&st, &reason, &c)
			rstr := "(vacío)"
			if reason.Valid && reason.String != "" {
				rstr = reason.String
			}
			fmt.Printf("Status: %s | Reason: %s | Count: %d\n", st, rstr, c)
		}
		rows.Close()
	} else {
		fmt.Println("Error querying campaign_invoices:", err)
	}
}
