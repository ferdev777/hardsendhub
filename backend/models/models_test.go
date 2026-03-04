package models

import (
	"encoding/json"
	"testing"
)

func TestEmailTemplate_JSON(t *testing.T) {
	template := EmailTemplate{
		Subject:     "FACTURA MENSUAL",
		BodyText:    "Se adjunta la factura.",
		ApologyText: "Disculpe las molestias.",
	}

	data, err := json.Marshal(template)
	if err != nil {
		t.Fatalf("Marshal error: %v", err)
	}

	var got EmailTemplate
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatalf("Unmarshal error: %v", err)
	}

	if got.Subject != template.Subject {
		t.Errorf("Subject = %q, want %q", got.Subject, template.Subject)
	}
	if got.BodyText != template.BodyText {
		t.Errorf("BodyText = %q, want %q", got.BodyText, template.BodyText)
	}
	if got.ApologyText != template.ApologyText {
		t.Errorf("ApologyText = %q, want %q", got.ApologyText, template.ApologyText)
	}
}

func TestEmailTemplate_JSON_EmptyApology(t *testing.T) {
	jsonStr := `{"subject":"Test","body_text":"Body","apology_text":""}`

	var tmpl EmailTemplate
	if err := json.Unmarshal([]byte(jsonStr), &tmpl); err != nil {
		t.Fatalf("Unmarshal error: %v", err)
	}
	if tmpl.ApologyText != "" {
		t.Errorf("ApologyText should be empty, got %q", tmpl.ApologyText)
	}
}

func TestMetricsUpdate_JSON(t *testing.T) {
	m := MetricsUpdate{
		JobID:          "job-001",
		TotalFiles:     100,
		SuccessCount:   80,
		ElapsedSeconds: 120,
		DailySent:      50,
		DailyMax:       1500,
		Paused:         true,
	}

	data, _ := json.Marshal(m)
	var got MetricsUpdate
	json.Unmarshal(data, &got)

	if got.ElapsedSeconds != 120 {
		t.Errorf("ElapsedSeconds = %d, want 120", got.ElapsedSeconds)
	}
	if got.DailySent != 50 {
		t.Errorf("DailySent = %d, want 50", got.DailySent)
	}
	if got.DailyMax != 1500 {
		t.Errorf("DailyMax = %d, want 1500", got.DailyMax)
	}
	if !got.Paused {
		t.Error("Paused should be true")
	}
}

func TestInvoiceJob_WithTemplate(t *testing.T) {
	tmpl := &EmailTemplate{
		Subject:  "Test Subject",
		BodyText: "Test Body",
	}

	job := InvoiceJob{
		Invoice: Invoice{
			ID:            "inv-1",
			InvoiceNumber: "B0002-00000001",
		},
		PDFPath:    "/tmp/test.pdf",
		JobID:      "job-001",
		ClientName: "PEREZ JUAN",
		DueDate:    "15/03/2026",
		Template:   tmpl,
	}

	if job.Template == nil {
		t.Error("Template should not be nil")
	}
	if job.Template.Subject != "Test Subject" {
		t.Errorf("Subject = %q, want Test Subject", job.Template.Subject)
	}
}

func TestInvoiceJob_NilTemplate(t *testing.T) {
	job := InvoiceJob{
		Invoice: Invoice{ID: "inv-1"},
		JobID:   "job-001",
	}

	if job.Template != nil {
		t.Error("Template should be nil by default")
	}
}

func TestInvoiceStatusConstants(t *testing.T) {
	// Verify all statuses are distinct
	statuses := []string{
		InvoiceStatusPending,
		InvoiceStatusProcessing,
		InvoiceStatusSuccess,
		InvoiceStatusErrorValidation,
		InvoiceStatusErrorNetwork,
	}

	seen := map[string]bool{}
	for _, s := range statuses {
		if seen[s] {
			t.Errorf("Duplicate status: %s", s)
		}
		seen[s] = true
	}

	if len(statuses) != 5 {
		t.Errorf("Expected 5 statuses, got %d", len(statuses))
	}
}
