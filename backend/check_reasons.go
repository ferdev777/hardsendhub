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

	var c int
	err = db.QueryRow("SELECT COUNT(*) FROM invoices WHERE error_reason LIKE '%not found in client database%'").Scan(&c)
	if err == nil {
		fmt.Printf("Facturas no encontradas en el TXT (tabla invoices antigua): %d\n", c)
	}
	
	err = db.QueryRow("SELECT COUNT(*) FROM invoices WHERE status = 'ERROR_VALIDATION' AND error_reason LIKE '%not found in client database%'").Scan(&c)
	if err == nil {
		fmt.Printf("Facturas no encontradas en el TXT marcadas ERROR_VALIDATION (tabla invoices antigua): %d\n", c)
	}

	err = db.QueryRow("SELECT COUNT(*) FROM invoices WHERE status = 'ERROR_VALIDATION' AND error_reason NOT LIKE '%not found in client database%'").Scan(&c)
	if err == nil {
		fmt.Printf("Otras con ERROR_VALIDATION (tabla invoices antigua): %d\n", c)
	}
}
