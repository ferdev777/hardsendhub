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
		fmt.Println("Error:", err)
		return
	}
	defer db.Close()

	rows, _ := db.Query("PRAGMA table_info(invoices)")
	fmt.Println("Columns in 'invoices':")
	for rows.Next() {
		var id int
		var name, dtype string
		var notnull, pk int
		var dflt interface{}
		rows.Scan(&id, &name, &dtype, &notnull, &dflt, &pk)
		fmt.Printf("- %s (%s)\n", name, dtype)
	}
}
