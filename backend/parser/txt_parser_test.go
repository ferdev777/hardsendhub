package parser

import (
	"testing"
)

func TestExtractInvoiceNumber(t *testing.T) {
	tests := []struct {
		name     string
		filename string
		want     string
		wantErr  bool
	}{
		{"simple B format", "B0002-00000001.pdf", "B0002-00000001", false},
		{"real format", "00000149 - Factura  B0002-00338911 - ABRIGO NORMA DIANA.pdf", "B0002-00338911", false},
		{"type A", "A0002-00010043.pdf", "A0002-00010043", false},
		{"type X", "00000155 - Factura  X0003-00017479 - AQUINO SILVANA MARICEL    .pdf", "X0003-00017479", false},
		{"extra zero POS format", "00000149 - Factura  B00002-00338911 - ABRIGO NORMA DIANA.pdf", "B0002-00338911", false},
		{"extra zero sequence format", "00000149 - Factura  B0002-000338911 - ABRIGO NORMA DIANA.pdf", "B0002-00338911", false},
		{"extra zero before letter", "00000149 - Factura  0B00002-000338911 - ABRIGO NORMA DIANA.pdf", "B0002-00338911", false},
		{"no match", "random_file.pdf", "", true},
		{"empty", "", "", true},
		{"partial match", "B0002.pdf", "", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ExtractInvoiceNumber(tt.filename)
			if (err != nil) != tt.wantErr {
				t.Errorf("ExtractInvoiceNumber(%q) error = %v, wantErr %v", tt.filename, err, tt.wantErr)
				return
			}
			if got != tt.want {
				t.Errorf("ExtractInvoiceNumber(%q) = %q, want %q", tt.filename, got, tt.want)
			}
		})
	}
}

func TestValidateEmail(t *testing.T) {
	tests := []struct {
		email string
		valid bool
	}{
		{"test@example.com", true},
		{"user.name@domain.com.ar", true},
		{"USER@DOMAIN.COM", true},
		{"a@b.co", true},
		{"invalid", false},
		{"@domain.com", false},
		{"user@", false},
		{"", false},
		{"no spaces@email.com", false},
	}

	for _, tt := range tests {
		t.Run(tt.email, func(t *testing.T) {
			if got := ValidateEmail(tt.email); got != tt.valid {
				t.Errorf("ValidateEmail(%q) = %v, want %v", tt.email, got, tt.valid)
			}
		})
	}
}

func TestExtractClientNameFromFilename(t *testing.T) {
	tests := []struct {
		name     string
		filename string
		want     string
	}{
		{"real format", "00000149 - Factura  B0002-00338911 - ABRIGO NORMA DIANA.pdf", "ABRIGO NORMA DIANA"},
		{"with extra spaces", "00000155 - Factura  X0003-00017479 - AQUINO SILVANA MARICEL    .pdf", "AQUINO SILVANA MARICEL"},
		{"simple format no name", "B0002-00000001.pdf", "Cliente/a"},
		{"empty", "", "Cliente/a"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ExtractClientNameFromFilename(tt.filename)
			if got != tt.want {
				t.Errorf("ExtractClientNameFromFilename(%q) = %q, want %q", tt.filename, got, tt.want)
			}
		})
	}
}

func TestIsTypeXInvoice(t *testing.T) {
	tests := []struct {
		invoiceNumber string
		want          bool
	}{
		{"X0003-00017479", true},
		{"0X0003-00017479", true},
		{"X00003-000017479", true},
		{"0B00002-00338911", false},
		{"B0002-00338911", false},
		{"A0002-00010043", false},
		{"X0001-00000001", true},
	}

	for _, tt := range tests {
		t.Run(tt.invoiceNumber, func(t *testing.T) {
			if got := IsTypeXInvoice(tt.invoiceNumber); got != tt.want {
				t.Errorf("IsTypeXInvoice(%q) = %v, want %v", tt.invoiceNumber, got, tt.want)
			}
		})
	}
}

func TestParseTXTFromBytes(t *testing.T) {
	input := []byte("test@example.com;B0002-00000001\nuser@domain.com;B0002-00000002\n")

	db, err := ParseTXTFromBytes(input)
	if err != nil {
		t.Fatalf("ParseTXTFromBytes() error = %v", err)
	}

	if db.Size() != 2 {
		t.Errorf("db.Size() = %d, want 2", db.Size())
	}

	email, found := db.GetEmail("B0002-00000001")
	if !found || email != "test@example.com" {
		t.Errorf("GetEmail(B0002-00000001) = (%q, %v), want (test@example.com, true)", email, found)
	}

	_, found = db.GetEmail("B0002-99999999")
	if found {
		t.Error("GetEmail(B0002-99999999) should return false for non-existent entry")
	}
}

func TestParseTXTFromBytes_MalformedLines(t *testing.T) {
	input := []byte("valid@email.com;B0002-00000001\nmalformed_no_semicolon\n;B0002-00000002\nemail@test.com;\n\nnormal@email.com;B0002-00000003\n")

	db, err := ParseTXTFromBytes(input)
	if err != nil {
		t.Fatalf("ParseTXTFromBytes() error = %v", err)
	}

	// Only valid lines should be parsed: line 1 and line 6
	if db.Size() != 2 {
		t.Errorf("db.Size() = %d, want 2 (only valid lines)", db.Size())
	}
}

func TestValidateAndBuildInvoice(t *testing.T) {
	db := NewClientDB()
	db.entries["B0002-00000001"] = "test@example.com"
	db.entries["B0002-00000002"] = "invalid-email"

	// Valid invoice
	inv, errMsg := ValidateAndBuildInvoice("B0002-00000001.pdf", db)
	if errMsg != "" {
		t.Errorf("ValidateAndBuildInvoice() unexpected error: %s", errMsg)
	}
	if inv == nil || inv.InvoiceNumber != "B0002-00000001" {
		t.Error("ValidateAndBuildInvoice() should return valid invoice")
	}

	// Invoice not found
	inv, errMsg = ValidateAndBuildInvoice("B0002-99999999.pdf", db)
	if errMsg == "" {
		t.Error("ValidateAndBuildInvoice() should return error for unknown invoice")
	}

	// Invalid email
	inv, errMsg = ValidateAndBuildInvoice("B0002-00000002.pdf", db)
	if errMsg == "" {
		t.Error("ValidateAndBuildInvoice() should return error for invalid email")
	}

	// Type X invoice (should be skipped)
	inv, errMsg = ValidateAndBuildInvoice("00000155 - Factura  X0003-00017479 - TEST.pdf", db)
	if errMsg != "SKIP_TYPE_X" {
		t.Errorf("ValidateAndBuildInvoice() for type X should return SKIP_TYPE_X, got: %s", errMsg)
	}

	// Type X invoice with extra zero (should also be skipped)
	inv, errMsg = ValidateAndBuildInvoice("00000155 - Factura  0X00003-00017479 - TEST.pdf", db)
	if errMsg != "SKIP_TYPE_X" {
		t.Errorf("ValidateAndBuildInvoice() for type X with extra zero should return SKIP_TYPE_X, got: %s", errMsg)
	}

	// Valid invoice with extra zero in filename matching TXT entry without extra zero
	inv, errMsg = ValidateAndBuildInvoice("00000149 - Factura  B00002-00000001 - TEST.pdf", db)
	if errMsg != "" {
		t.Errorf("ValidateAndBuildInvoice() with extra zero unexpected error: %s", errMsg)
	}
	if inv == nil || inv.InvoiceNumber != "B0002-00000001" {
		t.Error("ValidateAndBuildInvoice() should normalize invoice number from filename")
	}

	// Invalid filename
	inv, errMsg = ValidateAndBuildInvoice("random.pdf", db)
	if errMsg == "" {
		t.Error("ValidateAndBuildInvoice() should return error for invalid filename")
	}
	_ = inv // prevent unused warning
}

func TestNormalizeInvoiceNumber(t *testing.T) {
	tests := []struct {
		name    string
		raw     string
		want    string
		wantErr bool
	}{
		{"4 and 8 digits", "B0002-00000001", "B0002-00000001", false},
		{"5 and 9 digits with leading zeros", "0B00002-000000001", "B0002-00000001", false},
		{"type X with extra zeros", "0X00003-000017479", "X0003-00017479", false},
		{"real filename with extra zeroes", "00000149 - Factura  B00002-000338911 - ABRIGO NORMA DIANA.pdf", "B0002-00338911", false},
		{"invalid format", "random_string", "", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := NormalizeInvoiceNumber(tt.raw)
			if (err != nil) != tt.wantErr {
				t.Errorf("NormalizeInvoiceNumber(%q) error = %v, wantErr %v", tt.raw, err, tt.wantErr)
				return
			}
			if got != tt.want {
				t.Errorf("NormalizeInvoiceNumber(%q) = %q, want %q", tt.raw, got, tt.want)
			}
		})
	}
}

func TestParseTXTFromBytes_NormalizedLookup(t *testing.T) {
	input := []byte("test@example.com;B0002-00000001\nuser@domain.com;X00003-00017479\n")
	db, err := ParseTXTFromBytes(input)
	if err != nil {
		t.Fatalf("ParseTXTFromBytes() error = %v", err)
	}

	email, found := db.GetEmail("B00002-00000001")
	if !found || email != "test@example.com" {
		t.Errorf("GetEmail(B00002-00000001) = (%q, %v), want (test@example.com, true)", email, found)
	}

	email, found = db.GetEmail("X0003-00017479")
	if !found || email != "user@domain.com" {
		t.Errorf("GetEmail(X0003-00017479) = (%q, %v), want (user@domain.com, true)", email, found)
	}
}
