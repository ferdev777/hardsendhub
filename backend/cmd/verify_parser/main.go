package main

import (
	"fmt"
	"hardsend/parser"
	"os"
)

func main() {
	f, _ := os.Create("../test_results_utf8.txt")
	defer f.Close()

	testFiles := []string{
		"00000149 - Factura  B0002-00338911 - ABRIGO NORMA DIANA.pdf",
		"00000118 - Factura  B0002-00338897 - CABELLO CARLOS ERNESTO.pdf",
		"00000155 - Factura  X0003-00017479 - AQUINO SILVANA MARICEL    .pdf",
		"00000130 - Factura  B0002-00338902 - GIMENEZ  LETICIA  NOELIA.pdf",
		"B0002-00000001.pdf",
	}

	fmt.Fprintln(f, "=== Invoice Number Extraction ===")
	for _, file := range testFiles {
		num, err := parser.ExtractInvoiceNumber(file)
		if err != nil {
			fmt.Fprintf(f, "  FAIL %s -> ERROR: %s\n", file, err)
		} else {
			fmt.Fprintf(f, "  OK   %s -> %s\n", file, num)
		}
	}

	fmt.Fprintln(f, "\n=== Client Name Extraction from Filename ===")
	for _, file := range testFiles {
		name := parser.ExtractClientNameFromFilename(file)
		if name == "" {
			fmt.Fprintf(f, "  WARN %s -> (no name found)\n", file)
		} else {
			fmt.Fprintf(f, "  OK   %s -> '%s'\n", file, name)
		}
	}

	fmt.Fprintln(f, "\n=== TXT Parsing ===")
	db, err := parser.ParseTXTFile("../test_data/real-data/CABLE-INTERNET_DET.TXT")
	if err != nil {
		fmt.Fprintf(f, "  FAIL Error parsing TXT: %s\n", err)
		return
	}
	fmt.Fprintf(f, "  OK   Loaded %d entries from real TXT\n", db.Size())

	lookups := []string{"B0002-00338911", "B0002-00338897", "X0003-00017479", "B0002-00338906"}
	for _, inv := range lookups {
		email, found := db.GetEmail(inv)
		if found {
			fmt.Fprintf(f, "  OK   %s -> %s\n", inv, email)
		} else {
			fmt.Fprintf(f, "  FAIL %s -> NOT FOUND\n", inv)
		}
	}

	fmt.Println("Results written to test_results_utf8.txt")
}
