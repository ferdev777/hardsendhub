package parser

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"hardsend/models"
)

// InvoiceNumberRegex is the exact regex for extracting invoice numbers from PDF filenames.
var InvoiceNumberRegex = regexp.MustCompile(`[A-Z]\d{4}-\d{8}`)

// EmailRegex is a basic email validation pattern.
var EmailRegex = regexp.MustCompile(`^[a-zA-Z0-9._%+\-]+@[a-zA-Z0-9.\-]+\.[a-zA-Z]{2,}$`)

// clientNameFromFilenameRegex extracts client name from real PDF filenames.
// Format: "00000149 - Factura  B0002-00338911 - ABRIGO NORMA DIANA.pdf"
var clientNameFromFilenameRegex = regexp.MustCompile(`(?i)\d+\s*-\s*Factura\s+[A-Z]\d{4}-\d{8}\s*-\s*(.+)\.pdf$`)

// ClientDB holds the parsed client database mapping invoice numbers to emails.
type ClientDB struct {
	entries map[string]string // invoice_number -> email
}

// NewClientDB creates an empty client database.
func NewClientDB() *ClientDB {
	return &ClientDB{
		entries: make(map[string]string),
	}
}

// ParseTXTFile parses the TXT file format: email;numero_factura (no headers, separated by ;).
func ParseTXTFile(filePath string) (*ClientDB, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to open TXT file: %w", err)
	}
	defer file.Close()

	db := NewClientDB()
	scanner := bufio.NewScanner(file)

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		parseLine(db, line)
	}

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("error reading TXT file: %w", err)
	}

	return db, nil
}

// ParseTXTFromBytes parses the TXT data from a byte slice.
func ParseTXTFromBytes(data []byte) (*ClientDB, error) {
	db := NewClientDB()
	scanner := bufio.NewScanner(strings.NewReader(string(data)))

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		parseLine(db, line)
	}

	return db, nil
}

// parseLine parses a single line: email;invoice_number
func parseLine(db *ClientDB, line string) {
	parts := strings.SplitN(line, ";", 2)
	if len(parts) != 2 {
		return
	}

	email := strings.TrimSpace(parts[0])
	invoiceNumber := strings.TrimSpace(parts[1])

	if email == "" || invoiceNumber == "" {
		return
	}

	db.entries[invoiceNumber] = email
}

// GetEmail looks up an email for the given invoice number.
func (c *ClientDB) GetEmail(invoiceNumber string) (string, bool) {
	email, ok := c.entries[invoiceNumber]
	return email, ok
}

// Size returns the number of entries in the client database.
func (c *ClientDB) Size() int {
	return len(c.entries)
}

// ExtractInvoiceNumber extracts the invoice number from a PDF filename.
func ExtractInvoiceNumber(filename string) (string, error) {
	match := InvoiceNumberRegex.FindString(filename)
	if match == "" {
		return "", fmt.Errorf("no invoice number found in filename: %s", filename)
	}
	return match, nil
}

// ValidateEmail checks if an email address has a valid format.
func ValidateEmail(email string) bool {
	return EmailRegex.MatchString(email) && strings.Contains(email, "@")
}

// IsTypeXInvoice checks if the invoice number is a type X (e.g. X0003-00017479).
// Type X invoices are generated but NOT sent to anyone — they are skipped silently.
func IsTypeXInvoice(invoiceNumber string) bool {
	return strings.HasPrefix(invoiceNumber, "X")
}

// ExtractClientNameFromFilename extracts the client name from the PDF filename.
// Real invoices use format: "00000149 - Factura  B0002-00338911 - ABRIGO NORMA DIANA.pdf"
// If no name is found, returns "Cliente/a" as default.
func ExtractClientNameFromFilename(pdfPath string) string {
	filename := filepath.Base(pdfPath)

	matches := clientNameFromFilenameRegex.FindStringSubmatch(filename)
	if len(matches) >= 2 {
		name := strings.TrimSpace(matches[1])
		if name != "" {
			return name
		}
	}

	return "Cliente/a"
}

// ValidateAndBuildInvoice creates an Invoice from a PDF filename using the client database.
// Returns the invoice and a validation error string (empty if valid).
// Type X invoices are silently skipped by returning a special "SKIP" reason.
func ValidateAndBuildInvoice(pdfFilename string, clientDB *ClientDB) (*models.Invoice, string) {
	// Extract invoice number from filename
	invoiceNumber, err := ExtractInvoiceNumber(pdfFilename)
	if err != nil {
		reason := fmt.Sprintf("Invalid PDF filename format. Could not extract invoice number: %s", pdfFilename)
		return nil, reason
	}

	// Skip type X invoices silently — they are generated but never sent
	if IsTypeXInvoice(invoiceNumber) {
		return nil, "SKIP_TYPE_X"
	}

	// Look up email in client database
	email, found := clientDB.GetEmail(invoiceNumber)
	if !found {
		reason := fmt.Sprintf("Invoice number %s not found in client database (TXT file)", invoiceNumber)
		return nil, reason
	}

	// Validate email format
	if !ValidateEmail(email) {
		reason := fmt.Sprintf("Invalid email format for invoice %s: %s", invoiceNumber, email)
		return nil, reason
	}

	invoice := &models.Invoice{
		InvoiceNumber:  invoiceNumber,
		RecipientEmail: email,
		Status:         models.InvoiceStatusPending,
		Attempts:       0,
	}

	return invoice, ""
}
