package main

import (
	"database/sql"
	"encoding/csv"
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

	// Consultar las facturas de la campaña actual
	rows, err := db.Query(`
		SELECT invoice_number, COALESCE(email, ''), COALESCE(client_name, ''), status, COALESCE(reason, '') 
		FROM campaign_invoices 
	`)
	if err != nil {
		fmt.Println("Error consultando campaign_invoices:", err)
		return
	}
	defer rows.Close()

	file, err := os.Create("C:\\Users\\fer\\Downloads\\facturas_enviadas_hardsend.csv")
	if err != nil {
		fmt.Println("Error creando CSV:", err)
		return
	}
	defer file.Close()

	writer := csv.NewWriter(file)
	writer.Comma = ';' // Usar punto y coma para que Excel lo abra bien en español
	defer writer.Flush()

	// Escribir cabeceras
	writer.Write([]string{"Numero Factura", "Email", "Cliente", "Estado", "Observacion"})

	count := 0
	for rows.Next() {
		var invNum, email, clientName, status, reason string
		rows.Scan(&invNum, &email, &clientName, &status, &reason)
		
		writer.Write([]string{invNum, email, clientName, status, reason})
		count++
	}

	fmt.Printf("¡Éxito! Se ha generado el archivo CSV con %d registros.\n", count)
	fmt.Println("Ruta: C:\\Users\\fer\\Downloads\\facturas_enviadas_hardsend.csv")
}
