package main

import (
	"bufio"
	"encoding/csv"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

func main() {
	fmt.Println("===============================================================")
	fmt.Println("  Hardsend - Generador Inteligente de Lista Restante")
	fmt.Println("===============================================================")

	procesados := make(map[string]bool)
	baseDir := "C:/Users/fer/workspace/devrow"

	// 1. Cargar Emails Entregados (Exitosos)
	deliveredPath := filepath.Join(baseDir, "delivered_emails.csv")
	if f, err := os.Open(deliveredPath); err == nil {
		r := csv.NewReader(f)
		records, _ := r.ReadAll()
		// Ignoramos la cabecera (email)
		for i, row := range records {
			if i > 0 && len(row) > 0 {
				email := strings.ToLower(strings.TrimSpace(row[0]))
				procesados[email] = true
			}
		}
		f.Close()
	}

	// 2. Cargar Emails Rebotados (Suprimidos)
	bouncedPath := filepath.Join(baseDir, "bounced_from_resend.csv")
	if f, err := os.Open(bouncedPath); err == nil {
		r := csv.NewReader(f)
		records, _ := r.ReadAll()
		for i, row := range records {
			if i > 0 && len(row) > 0 {
				email := strings.ToLower(strings.TrimSpace(row[0]))
				procesados[email] = true
			}
		}
		f.Close()
	}

	fmt.Printf("[+] Base de datos conectada. Hay %d clientes que ya no debemos contactar.\n\n", len(procesados))

	// 3. Pedir la lista original a filtrar
	fmt.Print("Arrastra a esta ventana o escribe la ruta de tu .TXT original con TODOS los clientes:\n> ")
	reader := bufio.NewReader(os.Stdin)
	inputPath, _ := reader.ReadString('\n')
	inputPath = strings.TrimSpace(inputPath)

	// Limpiar comillas si se arrastró en Windows
	inputPath = strings.Trim(inputPath, "\"")
	inputPath = strings.Trim(inputPath, "'")

	if inputPath == "" {
		fmt.Println("\n[!] Debes ingresar la ruta al archivo. Cierra y vuelve a intentar.")
		return
	}

	inFile, err := os.Open(inputPath)
	if err != nil {
		fmt.Printf("\n[!] No se pudo encontrar o abrir el archivo: %s\nError: %v\n", inputPath, err)
		return
	}
	defer inFile.Close()

	// 4. Escribir archivo filtrado
	outPath := filepath.Join(filepath.Dir(inputPath), "clientes_faltantes_para_manana.txt")
	outFile, err := os.Create(outPath)
	if err != nil {
		fmt.Printf("\n[!] Error creando el nuevo archivo: %v\n", err)
		return
	}
	defer outFile.Close()

	scanner := bufio.NewScanner(inFile)
	totalOriginal := 0
	totalRestantes := 0

	for scanner.Scan() {
		line := scanner.Text()
		lineStr := strings.TrimSpace(line)

		if lineStr == "" {
			continue // skip vacías
		}

		totalOriginal++

		// El formato es email;num_factura
		parts := strings.Split(lineStr, ";")
		email := ""
		if len(parts) > 0 {
			email = strings.ToLower(strings.TrimSpace(parts[0]))
		}

		if !procesados[email] {
			// No está procesado, guardamos.
			outFile.WriteString(lineStr + "\n")
			totalRestantes++
		}
	}

	fmt.Println("\n===============================================================")
	fmt.Printf("✔ PROCESO TERMINADO\n\n")
	fmt.Printf("Total de clientes en tu lista original: %d\n", totalOriginal)
	fmt.Printf("Total omitidos (ya enviados/rebotados):   %d\n", totalOriginal-totalRestantes)
	fmt.Printf("TOTAL LIMPIO QUE FALTA ENVIAR:            %d\n", totalRestantes)
	fmt.Println("===============================================================")
	fmt.Printf("\nSe ha guardado tu nueva lista reducida exactamente en:\n%s\n", outPath)
	fmt.Println("\nPara mañana, usa ESTE NUEVO ARCHIVO al subir tu ZIP para enviar las que faltan sin repetidos.")
}
