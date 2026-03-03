package main

import (
	"database/sql"
	"fmt"

	_ "modernc.org/sqlite"
)

func main() {
	db, err := sql.Open("sqlite", "./hardsend_metrics.db?_pragma=journal_mode(WAL)&_pragma=busy_timeout(5000)")
	if err != nil {
		fmt.Println("Error:", err)
		return
	}
	defer db.Close()

	fmt.Println("=== LIMPIEZA COMPLETA DE DB ===")

	// 1. Delete all invoices
	r1, _ := db.Exec("DELETE FROM invoices")
	n1, _ := r1.RowsAffected()
	fmt.Printf("Invoices eliminadas: %d\n", n1)

	// 2. Delete all jobs
	r2, _ := db.Exec("DELETE FROM jobs")
	n2, _ := r2.RowsAffected()
	fmt.Printf("Jobs eliminados: %d\n", n2)

	// 3. Delete PENDING missing_emails (keep resolved ones as history)
	r3, _ := db.Exec("DELETE FROM missing_emails WHERE resolved = 0")
	n3, _ := r3.RowsAffected()
	fmt.Printf("Missing emails pendientes eliminados: %d\n", n3)

	// 4. Show remaining resolved missing_emails
	var resolved int
	db.QueryRow("SELECT COUNT(*) FROM missing_emails WHERE resolved = 1").Scan(&resolved)
	fmt.Printf("Missing emails resueltos conservados: %d\n", resolved)

	fmt.Println("\n=== DB LISTA PARA EMPEZAR DE CERO ===")
}
