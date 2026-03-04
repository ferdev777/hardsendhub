package main

import (
	"bufio"
	"encoding/csv"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"strings"

	"golang.org/x/text/encoding/charmap"
)

const (
	bouncedCSV   = "C:/Users/fer/workspace/devrow/bounced_from_resend.csv"
	deliveredCSV = "C:/Users/fer/workspace/devrow/delivered_emails.csv"
	txtFile      = "C:/Users/fer/workspace/devrow/test_data/produccion/CABLE-INTERNET_DET.TXT"
	pdfDir       = "C:/Users/fer/workspace/devrow/test_data/produccion"
	outputDir    = "C:/Users/fer/workspace/devrow/reenvio_pendientes_v2"
)

func main() {
	log.Println("🔄 Creando carpeta de reenvío inteligente...")

	// 1. Load delivered emails (exclusion list)
	deliveredSet := loadEmailSet(deliveredCSV)
	log.Printf("✅ %d emails entregados cargados (serán excluidos)", len(deliveredSet))

	// 2. Load bounced emails
	bouncedSet := loadEmailSet(bouncedCSV)
	log.Printf("❌ %d emails rebotados cargados", len(bouncedSet))

	// 3. Parse TXT to map email -> invoice numbers
	emailToInvoices := parseTXT(txtFile)
	log.Printf("📋 %d entradas en el TXT parseadas", len(emailToInvoices))

	// 4. Find invoices that need resending (bounced AND NOT delivered)
	toResend := map[string]string{} // invoiceNumber -> email
	for email := range bouncedSet {
		if deliveredSet[email] {
			continue // Already delivered, skip
		}
		invoices, ok := emailToInvoices[email]
		if !ok {
			continue // Not in TXT
		}
		for _, inv := range invoices {
			toResend[inv] = email
		}
	}
	log.Printf("📧 %d facturas necesitan reenvío", len(toResend))

	// 5. Create output directory
	os.MkdirAll(outputDir, 0755)

	// 6. Find and copy PDFs
	copied := 0
	notFound := 0
	var notFoundList []string

	for invoiceNum, email := range toResend {
		// Search for matching PDF
		pdfName := findPDF(pdfDir, invoiceNum)
		if pdfName == "" {
			notFound++
			notFoundList = append(notFoundList, fmt.Sprintf("%s (%s)", invoiceNum, email))
			continue
		}

		// Copy PDF
		src := filepath.Join(pdfDir, pdfName)
		dst := filepath.Join(outputDir, pdfName)
		if err := copyFile(src, dst); err != nil {
			log.Printf("Error copying %s: %v", pdfName, err)
			continue
		}
		copied++
	}

	// 7. Copy TXT file
	copyFile(txtFile, filepath.Join(outputDir, "CABLE-INTERNET_DET.TXT"))

	// 8. Save delivered exclusion list for the system
	saveExclusionList(deliveredSet, filepath.Join(outputDir, "excluir_delivered.csv"))

	// 9. Summary
	log.Println("\n=================================")
	log.Println("📋 RESUMEN")
	log.Println("=================================")
	log.Printf("📁 Carpeta: %s", outputDir)
	log.Printf("✅ PDFs copiados:     %d", copied)
	log.Printf("❌ PDFs no encontrados: %d", notFound)
	log.Printf("📄 TXT incluido:       sí")
	log.Printf("📊 Exclusión CSV:      excluir_delivered.csv")

	if notFound > 0 && notFound <= 20 {
		log.Println("\n⚠️ Facturas sin PDF encontrado:")
		for _, nf := range notFoundList {
			log.Printf("  - %s", nf)
		}
	}
}

func loadEmailSet(path string) map[string]bool {
	set := map[string]bool{}
	f, err := os.Open(path)
	if err != nil {
		log.Printf("Warning: cannot open %s: %v", path, err)
		return set
	}
	defer f.Close()

	reader := csv.NewReader(f)
	reader.Read() // skip header
	for {
		record, err := reader.Read()
		if err != nil {
			break
		}
		if len(record) > 0 {
			set[strings.ToLower(strings.TrimSpace(record[0]))] = true
		}
	}
	return set
}

func parseTXT(path string) map[string][]string {
	result := map[string][]string{} // email -> []invoiceNumber

	f, err := os.Open(path)
	if err != nil {
		log.Fatalf("Cannot open TXT: %v", err)
	}
	defer f.Close()

	decoder := charmap.Windows1252.NewDecoder()
	reader := decoder.Reader(f)
	scanner := bufio.NewScanner(reader)

	for scanner.Scan() {
		line := scanner.Text()
		parts := strings.SplitN(line, ";", 2)
		if len(parts) != 2 {
			continue
		}
		email := strings.ToLower(strings.TrimSpace(parts[0]))
		invoiceNum := strings.TrimSpace(parts[1])
		if email != "" && invoiceNum != "" {
			result[email] = append(result[email], invoiceNum)
		}
	}
	return result
}

func findPDF(dir, invoiceNum string) string {
	// Try to find a PDF with matching invoice number
	entries, err := os.ReadDir(dir)
	if err != nil {
		return ""
	}
	for _, entry := range entries {
		name := entry.Name()
		if !strings.HasSuffix(strings.ToLower(name), ".pdf") {
			continue
		}
		if strings.Contains(name, invoiceNum) {
			return name
		}
	}
	return ""
}

func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()

	os.MkdirAll(filepath.Dir(dst), 0755)
	out, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer out.Close()

	_, err = io.Copy(out, in)
	return err
}

func saveExclusionList(delivered map[string]bool, path string) {
	f, _ := os.Create(path)
	defer f.Close()
	f.WriteString("email\n")
	for email := range delivered {
		f.WriteString(email + "\n")
	}
}
