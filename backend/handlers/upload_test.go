package handlers

import (
	"archive/zip"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"hardsend/config"
	"hardsend/database"
	"hardsend/models"
	"hardsend/parser"
	"hardsend/reputation"
	"hardsend/storage"
)

func buildTestZIP(t *testing.T, files map[string][]byte) []byte {
	t.Helper()
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	for name, content := range files {
		w, err := zw.Create(name)
		if err != nil {
			t.Fatalf("create zip entry %s: %v", name, err)
		}
		if _, err := w.Write(content); err != nil {
			t.Fatalf("write zip entry %s: %v", name, err)
		}
	}
	if err := zw.Close(); err != nil {
		t.Fatalf("close zip: %v", err)
	}
	return buf.Bytes()
}

func TestConsumeArchiveStreamBoundedR2AndWavePersistence(t *testing.T) {
	tmpDir := t.TempDir()
	db, err := database.New(filepath.Join(tmpDir, "test.db"))
	if err != nil {
		t.Fatalf("init db: %v", err)
	}
	defer db.Close()

	repStore := reputation.New(db.RawConn())
	if err := repStore.MarkVerified("verified@example.com"); err != nil {
		t.Fatalf("mark verified: %v", err)
	}
	if err := repStore.MarkBounced("bounced@example.com", "550 hard bounce"); err != nil {
		t.Fatalf("mark bounced: %v", err)
	}

	var (
		inFlightR2   int32
		maxInFlight  int32
		r2Uploaded   sync.Map
		d1Mu         sync.Mutex
		d1Invoices   = make(map[string]map[string]interface{})
	)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/api/d1/campaigns/active":
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"active":false}`))
		case r.Method == http.MethodPost && r.URL.Path == "/api/d1/campaigns/create":
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"success":true}`))
		case r.Method == http.MethodPut && strings.HasPrefix(r.URL.Path, "/api/storage/invoices/"):
			cur := atomic.AddInt32(&inFlightR2, 1)
			for {
				prev := atomic.LoadInt32(&maxInFlight)
				if cur <= prev || atomic.CompareAndSwapInt32(&maxInFlight, prev, cur) {
					break
				}
			}
			time.Sleep(15 * time.Millisecond)
			invNum := filepath.Base(r.URL.Path)
			_, _ = io.Copy(io.Discard, r.Body)
			atomic.AddInt32(&inFlightR2, -1)

			if invNum == "B0002-00000099" {
				w.WriteHeader(http.StatusBadRequest)
				_, _ = w.Write([]byte(`{"error":"forced failure"}`))
				return
			}
			r2Uploaded.Store(invNum, true)
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"success":true}`))
		case r.Method == http.MethodPost && r.URL.Path == "/api/d1/invoices/batch":
			var body struct {
				Invoices []map[string]interface{} `json:"invoices"`
			}
			if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
				w.WriteHeader(http.StatusBadRequest)
				return
			}
			d1Mu.Lock()
			for _, inv := range body.Invoices {
				num, _ := inv["invoice_number"].(string)
				d1Invoices[num] = inv
			}
			d1Mu.Unlock()
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"success":true}`))
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer srv.Close()

	// Build TXT mapping verified, untested, bounced, and one R2-failure invoice
	var txtLines []string
	zipFiles := make(map[string][]byte)
	for i := 1; i <= 12; i++ {
		inv := fmt.Sprintf("B0002-%08d", i)
		email := "untested@example.com"
		if i%2 == 0 {
			email = "verified@example.com"
		}
		txtLines = append(txtLines, fmt.Sprintf("%s;%s", email, inv))
		zipFiles[fmt.Sprintf("00000001 - Factura %s - CLIENTE %d.pdf", inv, i)] = []byte("%PDF-1.4 sample")
	}
	txtLines = append(txtLines, "bounced@example.com;B0002-00000050")
	zipFiles["00000001 - Factura B0002-00000050 - REBOTADO.pdf"] = []byte("%PDF-1.4 bounced")

	txtLines = append(txtLines, "verified@example.com;B0002-00000099")
	zipFiles["00000001 - Factura B0002-00000099 - FALLA R2.pdf"] = []byte("%PDF-1.4 fail")

	clientDB, err := parser.ParseTXTFromBytes([]byte(strings.Join(txtLines, "\n")))
	if err != nil {
		t.Fatalf("parse txt: %v", err)
	}

	cfg := &config.Config{
		DailyLimit:  200,
		CFWorkerURL: srv.URL,
	}
	handler := NewUploadHandler(db, nil, cfg, repStore, filepath.Join(tmpDir, "uploads"))
	handler.r2UploadWorkers = 3
	handler.cfSync = storage.NewCloudflareSync(srv.URL)
	handler.cfSync.SetRetryConfig(2, time.Millisecond)

	jobID := "test-job-1"
	if err := db.CreateJob(&models.Job{
		ID:        jobID,
		Filename:  "batch.zip",
		Status:    models.JobStatusProcessing,
		CreatedAt: time.Now(),
	}); err != nil {
		t.Fatalf("create job: %v", err)
	}

	pr, pw := io.Pipe()
	sess := &streamSession{
		id:          "stream-1",
		jobID:       jobID,
		archiveName: "batch.zip",
		dueDate:     "10/11/2026",
		dailyLimit:  4, // Warmup limit <= 2500
		clientDB:    clientDB,
		pw:          pw,
		doneChan:    make(chan struct{}),
	}

	zipBytes := buildTestZIP(t, zipFiles)
	go func() {
		_, _ = pw.Write(zipBytes)
		_ = pw.Close()
	}()

	handler.consumeArchiveStream(sess, pr)

	// 1. Concurrency must stay bounded by r2UploadWorkers (3)
	if got := atomic.LoadInt32(&maxInFlight); got > 3 {
		t.Fatalf("R2 concurrency exceeded worker limit: maxInFlight=%d, want <= 3", got)
	}

	// 2. All 12 valid invoices (6 verified + 6 untested) persisted to R2 & D1, NOT dropped by warmup
	d1Mu.Lock()
	defer d1Mu.Unlock()
	if len(d1Invoices) != 12 {
		t.Fatalf("expected 12 persisted invoices in D1 (both verified and untested), got %d", len(d1Invoices))
	}
	if _, exists := d1Invoices["B0002-00000050"]; exists {
		t.Fatalf("bounced invoice must not be persisted to D1")
	}
	if _, exists := d1Invoices["B0002-00000099"]; exists {
		t.Fatalf("invoice whose R2 upload failed must not be persisted to D1")
	}
	if got := atomic.LoadInt32(&sess.r2FailedCount); got != 1 {
		t.Fatalf("expected r2FailedCount=1, got %d", got)
	}
	if got := atomic.LoadInt32(&sess.blockedCount); got != 1 {
		t.Fatalf("expected blockedCount=1, got %d", got)
	}

	// 3. Wave priorities & client_name must be preserved in D1
	if wave := d1Invoices["B0002-00000002"]["priority_wave"]; wave != float64(1) {
		t.Fatalf("verified invoice expected priority_wave=1, got %v", wave)
	}
	if wave := d1Invoices["B0002-00000001"]["priority_wave"]; wave != float64(2) {
		t.Fatalf("untested invoice expected priority_wave=2, got %v", wave)
	}
	if name := d1Invoices["B0002-00000002"]["client_name"]; name != "CLIENTE 2" {
		t.Fatalf("expected client_name='CLIENTE 2', got %v", name)
	}

	// 4. Today's staged dispatch (dailyLimit=4) must select only Wave 1 verified invoices (since 6 verified exist)
	var stagedCount int
	var verifiedStaged int
	rows, err := db.RawConn().Query("SELECT recipient_email FROM invoices WHERE job_id = ?", jobID)
	if err != nil {
		t.Fatalf("query staged invoices: %v", err)
	}
	defer rows.Close()
	for rows.Next() {
		var em string
		if err := rows.Scan(&em); err != nil {
			t.Fatalf("scan: %v", err)
		}
		stagedCount++
		if em == "verified@example.com" {
			verifiedStaged++
		}
	}
	if err := rows.Err(); err != nil {
		t.Fatalf("rows iteration error: %v", err)
	}
	if stagedCount != 4 || verifiedStaged != 4 {
		t.Fatalf("expected 4 Wave-1 verified invoices staged for today, got total=%d verified=%d", stagedCount, verifiedStaged)
	}
}
