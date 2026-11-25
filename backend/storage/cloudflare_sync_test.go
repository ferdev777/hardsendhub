package storage

import (
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"
)

func TestUploadPDFToR2RetriesTransientFailures(t *testing.T) {
	tests := []struct {
		name        string
		failCount   int32
		failStatus  int
		wantErr     bool
		wantCalls   int32
	}{
		{
			name:       "succeeds on first attempt",
			failCount:  0,
			failStatus: http.StatusOK,
			wantErr:    false,
			wantCalls:  1,
		},
		{
			name:       "retries 502 twice then succeeds",
			failCount:  2,
			failStatus: http.StatusBadGateway,
			wantErr:    false,
			wantCalls:  3,
		},
		{
			name:       "retries 429 once then succeeds",
			failCount:  1,
			failStatus: http.StatusTooManyRequests,
			wantErr:    false,
			wantCalls:  2,
		},
		{
			name:       "does not retry 400 client error",
			failCount:  3,
			failStatus: http.StatusBadRequest,
			wantErr:    true,
			wantCalls:  1,
		},
		{
			name:       "fails after exhausting retries on 503",
			failCount:  5,
			failStatus: http.StatusServiceUnavailable,
			wantErr:    true,
			wantCalls:  3,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var calls int32
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				c := atomic.AddInt32(&calls, 1)
				if c <= tt.failCount {
					w.WriteHeader(tt.failStatus)
					_, _ = w.Write([]byte("transient error"))
					return
				}
				w.WriteHeader(http.StatusOK)
				_, _ = w.Write([]byte(`{"success":true}`))
			}))
			defer srv.Close()

			syncClient := NewCloudflareSync(srv.URL)
			syncClient.SetRetryConfig(3, time.Millisecond)

			err := syncClient.UploadPDFToR2("2026-11", "B0002-00000001", []byte("%PDF-1.4 test"))
			if (err != nil) != tt.wantErr {
				t.Fatalf("UploadPDFToR2() error = %v, wantErr %v", err, tt.wantErr)
			}
			if got := atomic.LoadInt32(&calls); got != tt.wantCalls {
				t.Fatalf("expected %d HTTP attempts, got %d", tt.wantCalls, got)
			}
		})
	}
}
