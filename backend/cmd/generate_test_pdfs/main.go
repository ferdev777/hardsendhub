package main

import (
	"fmt"
	"os"
	"path/filepath"
	"time"
)

type testInvoice struct {
	SeqNumber  string
	InvNumber  string
	ClientName string
}

func generateTestPDF(inv testInvoice) []byte {
	pdf := fmt.Sprintf(`%%PDF-1.4
1 0 obj
<< /Type /Catalog /Pages 2 0 R >>
endobj

2 0 obj
<< /Type /Pages /Kids [3 0 R] /Count 1 >>
endobj

3 0 obj
<< /Type /Page /Parent 2 0 R /MediaBox [0 0 612 792]
   /Contents 4 0 R /Resources << /Font << /F1 5 0 R >> >> >>
endobj

4 0 obj
<< /Length 600 >>
stream
BT
/F1 16 Tf
50 750 Td
(VIDEO DIGITAL) Tj
0 -20 Td
/F1 10 Tf
(Factura) Tj
200 0 Td
(Punto de Venta: 0002  Comp. Nro: %s) Tj
-200 -30 Td
(Nombre y Apellido: %s) Tj
0 -20 Td
(CUIT: 30714696587) Tj
0 -20 Td
(I.V.A. Responsable Inscripto) Tj
0 -20 Td
(Fecha de Inicio de Actividades: 09/01/15) Tj
0 -40 Td
/F1 12 Tf
(Detalle del Servicio: CABLE/INTERNET) Tj
0 -30 Td
(Periodo: %s) Tj
0 -20 Td
(Vencimiento: %s) Tj
ET
endstream
endobj

5 0 obj
<< /Type /Font /Subtype /Type1 /BaseFont /Helvetica >>
endobj

xref
0 6
0000000000 65535 f 
0000000009 00000 n 
0000000058 00000 n 
0000000115 00000 n 
0000000266 00000 n 
trailer
<< /Size 6 /Root 1 0 R >>
startxref
0
%%%%EOF`, inv.InvNumber, inv.ClientName, time.Now().Format("01/2006"), time.Now().AddDate(0, 0, 30).Format("02/01/2006"))

	return []byte(pdf)
}

func main() {
	invoices := []testInvoice{
		{SeqNumber: "00001101", InvNumber: "B0002-00001901", ClientName: "CLIENTE DE PRUEBA UNO"},
		{SeqNumber: "00001102", InvNumber: "B0002-00001902", ClientName: "CLIENTE DE PRUEBA DOS"},
		{SeqNumber: "00001103", InvNumber: "B0002-00001903", ClientName: "PEREZ MARIA LAURA"},
		{SeqNumber: "00001104", InvNumber: "B0002-00001904", ClientName: "GONZALEZ SANTIAGO"},
		{SeqNumber: "00001105", InvNumber: "B0002-00001905", ClientName: "PELOZO LAUTARO"},
		{SeqNumber: "00001106", InvNumber: "B0002-00001906", ClientName: "MARTINEZ LUCAS GABRIEL"},
		{SeqNumber: "00001107", InvNumber: "B0002-00001907", ClientName: "SOLVTECH SRL"},
	}

	outputDir := filepath.Join("..", "..", "..", "test_data")

	// Delete old PDFs in test_data
	oldFiles, _ := filepath.Glob(filepath.Join(outputDir, "*.pdf"))
	for _, f := range oldFiles {
		os.Remove(f)
		fmt.Printf("Deleted old: %s\n", filepath.Base(f))
	}

	// Create new PDFs with real filename format
	os.MkdirAll(outputDir, 0755)
	for _, inv := range invoices {
		filename := fmt.Sprintf("%s - Factura  %s - %s.pdf", inv.SeqNumber, inv.InvNumber, inv.ClientName)
		fullPath := filepath.Join(outputDir, filename)
		data := generateTestPDF(inv)
		if err := os.WriteFile(fullPath, data, 0644); err != nil {
			fmt.Printf("Error: %s: %v\n", filename, err)
			continue
		}
		fmt.Printf("Created: %s\n", filename)
	}

	fmt.Println("\nAll test PDFs generated with real filename format!")
}
