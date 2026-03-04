package email

import (
	"hardsend/models"
	"strings"
	"testing"
)

func TestBuildInvoiceHTML_ContainsClientName(t *testing.T) {
	html := buildInvoiceHTML("PEREZ JUAN", "B0002-00000001", "15/03/2026", nil)

	if !strings.Contains(html, "PEREZ JUAN") {
		t.Error("HTML should contain client name")
	}
	if !strings.Contains(html, "B0002-00000001") {
		t.Error("HTML should contain invoice number")
	}
	if !strings.Contains(html, "15/03/2026") {
		t.Error("HTML should contain due date")
	}
}

func TestBuildInvoiceHTML_DefaultTemplate(t *testing.T) {
	html := buildInvoiceHTML("TEST CLIENT", "B0002-00000001", "01/01/2026", nil)

	if !strings.Contains(html, "FACTURA MENSUAL VIDEO DIGITAL S.R.L") {
		t.Error("HTML should contain default header when no template")
	}
	if !strings.Contains(html, "CABLE/INTERNET") {
		t.Error("HTML should contain default body text when no template")
	}
	// No apology section when template is nil
	if strings.Contains(html, "AVISO IMPORTANTE") {
		t.Error("HTML should NOT contain apology section when template is nil")
	}
}

func TestBuildInvoiceHTML_WithTemplate(t *testing.T) {
	tmpl := &models.EmailTemplate{
		Subject:     "FACTURA CUSTOM TEST",
		BodyText:    "Este es un texto personalizado desde el frontend",
		ApologyText: "Disculpe las molestias por el reenvío",
	}
	html := buildInvoiceHTML("GARCIA MARIA", "A0002-00010043", "20/03/2026", tmpl)

	if !strings.Contains(html, "FACTURA CUSTOM TEST") {
		t.Error("HTML should contain custom subject from template")
	}
	if !strings.Contains(html, "Este es un texto personalizado desde el frontend") {
		t.Error("HTML should contain custom body text from template")
	}
	if !strings.Contains(html, "Disculpe las molestias por el reenvío") {
		t.Error("HTML should contain apology text from template")
	}
	if !strings.Contains(html, "AVISO IMPORTANTE") {
		t.Error("HTML should contain apology notice when apology text is set")
	}
}

func TestBuildInvoiceHTML_WithTemplate_NoApology(t *testing.T) {
	tmpl := &models.EmailTemplate{
		Subject:  "FACTURA TEST",
		BodyText: "Texto custom",
	}
	html := buildInvoiceHTML("TEST", "B0002-00000001", "01/01/2026", tmpl)

	if strings.Contains(html, "AVISO IMPORTANTE") {
		t.Error("HTML should NOT contain apology section when ApologyText is empty")
	}
}

func TestBuildInvoiceText_ContainsInfo(t *testing.T) {
	text := buildInvoiceText("GARCIA MARIA", "A0002-00010043", "20/03/2026", nil)

	if !strings.Contains(text, "GARCIA MARIA") {
		t.Error("Text should contain client name")
	}
	if !strings.Contains(text, "A0002-00010043") {
		t.Error("Text should contain invoice number")
	}
	if !strings.Contains(text, "20/03/2026") {
		t.Error("Text should contain due date")
	}
}

func TestBuildInvoiceText_WithTemplate(t *testing.T) {
	tmpl := &models.EmailTemplate{
		Subject:     "CUSTOM SUBJECT",
		BodyText:    "Custom body text",
		ApologyText: "Sorry for the inconvenience",
	}
	text := buildInvoiceText("TEST", "B0002-00000001", "01/01/2026", tmpl)

	if !strings.Contains(text, "CUSTOM SUBJECT") {
		t.Error("Text should contain custom subject")
	}
	if !strings.Contains(text, "Custom body text") {
		t.Error("Text should contain custom body")
	}
	if !strings.Contains(text, "Sorry for the inconvenience") {
		t.Error("Text should contain apology")
	}
}

func TestBuildInvoiceText_NotEmpty(t *testing.T) {
	text := buildInvoiceText("TEST", "B0002-00000001", "01/01/2026", nil)
	if len(text) < 50 {
		t.Errorf("Text body too short: %d chars", len(text))
	}
}

func TestBuildInvoiceHTML_SpecialCharacters(t *testing.T) {
	html := buildInvoiceHTML("LÓPEZ MARÍA", "B0002-00000001", "15/03/2026", nil)
	if !strings.Contains(html, "LÓPEZ MARÍA") {
		t.Error("HTML should handle special characters in names")
	}
}
