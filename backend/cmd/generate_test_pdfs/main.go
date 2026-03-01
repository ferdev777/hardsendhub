package main

import (
	"fmt"
	"os"
	"path/filepath"
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
(Periodo: 03/2026) Tj
0 -20 Td
(Vencimiento: 31/03/2026) Tj
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
%%%%EOF`, inv.InvNumber, inv.ClientName)

	return []byte(pdf)
}

func main() {
	invoices := []testInvoice{
		{SeqNumber: "00000101", InvNumber: "B0002-00000001", ClientName: "HIRSCHFELD FERNANDO"},
		{SeqNumber: "00000102", InvNumber: "B0002-00000002", ClientName: "HIRSCHFELD FERNANDO GABRIEL"},
		{SeqNumber: "00000103", InvNumber: "B0002-00000003", ClientName: "PEREZ MARIA LAURA"},
		{SeqNumber: "00000104", InvNumber: "B0002-00000004", ClientName: "GONZALEZ SANTIAGO"},
		{SeqNumber: "00000105", InvNumber: "B0002-00000005", ClientName: "PELOZO LAUTARO"},
		{SeqNumber: "00000106", InvNumber: "B0002-00000006", ClientName: "MARTINEZ LUCAS GABRIEL"},
		{SeqNumber: "00000107", InvNumber: "B0002-00000007", ClientName: "SOLVTECH SRL"},
	}

	outputDir := filepath.Join("..", "test_data")

	// Delete old simple-named PDFs
	oldFiles, _ := filepath.Glob(filepath.Join(outputDir, "B0002-*.pdf"))
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
