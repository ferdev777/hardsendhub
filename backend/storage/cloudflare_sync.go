package storage

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"time"
)

// CloudflareSync manages seamless two-way synchronization with Cloudflare R2 and D1.
type CloudflareSync struct {
	workerURL      string
	httpClient     *http.Client
	maxRetries     int
	retryBaseDelay time.Duration
}

// NewCloudflareSync creates a new CloudflareSync client with connection pooling and bounded retries.
func NewCloudflareSync(workerURL string) *CloudflareSync {
	if workerURL == "" {
		workerURL = "https://app.facturasvideodigital.com"
	}
	transport := &http.Transport{
		MaxIdleConns:        16,
		MaxIdleConnsPerHost: 16,
		MaxConnsPerHost:     16,
		IdleConnTimeout:     90 * time.Second,
	}
	return &CloudflareSync{
		workerURL: workerURL,
		httpClient: &http.Client{
			Timeout:   60 * time.Second,
			Transport: transport,
		},
		maxRetries:     3,
		retryBaseDelay: 250 * time.Millisecond,
	}
}

// SetRetryConfig overrides retry count and base delay (primarily for tests).
func (s *CloudflareSync) SetRetryConfig(maxRetries int, baseDelay time.Duration) {
	if maxRetries < 1 {
		maxRetries = 1
	}
	s.maxRetries = maxRetries
	s.retryBaseDelay = baseDelay
}

// doWithRetry executes an HTTP request builder with exponential backoff on network errors, 429, or 5xx responses.
func (s *CloudflareSync) doWithRetry(buildReq func() (*http.Request, error)) (*http.Response, error) {
	attempts := s.maxRetries
	if attempts < 1 {
		attempts = 1
	}

	var lastErr error
	for attempt := 0; attempt < attempts; attempt++ {
		req, err := buildReq()
		if err != nil {
			return nil, err
		}

		resp, err := s.httpClient.Do(req)
		if err == nil && resp.StatusCode < 500 && resp.StatusCode != http.StatusTooManyRequests {
			return resp, nil
		}

		if err != nil {
			lastErr = err
		} else {
			body, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
			resp.Body.Close()
			lastErr = fmt.Errorf("HTTP %d: %s", resp.StatusCode, string(body))
		}

		if attempt+1 < attempts {
			delay := s.retryBaseDelay * time.Duration(1<<attempt)
			time.Sleep(delay)
		}
	}

	return nil, lastErr
}

// UploadPDFToR2 uploads a single PDF into R2 at invoices/{campaignID}/{invoiceNumber}.pdf
func (s *CloudflareSync) UploadPDFToR2(campaignID, invoiceNumber string, pdfBytes []byte) error {
	url := fmt.Sprintf("%s/api/storage/invoices/%s/%s", s.workerURL, campaignID, invoiceNumber)
	resp, err := s.doWithRetry(func() (*http.Request, error) {
		req, err := http.NewRequest(http.MethodPut, url, bytes.NewReader(pdfBytes))
		if err != nil {
			return nil, err
		}
		req.Header.Set("Content-Type", "application/pdf")
		return req, nil
	})
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		b, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("R2 upload error (%d): %s", resp.StatusCode, string(b))
	}
	return nil
}

// DownloadPDFFromR2 downloads a PDF from R2 and writes it to the local destination path.
func (s *CloudflareSync) DownloadPDFFromR2(campaignID, invoiceNumber, destPath string) error {
	if err := os.MkdirAll(filepath.Dir(destPath), 0755); err != nil {
		return err
	}
	url := fmt.Sprintf("%s/api/storage/invoices/%s/%s", s.workerURL, campaignID, invoiceNumber)
	resp, err := s.httpClient.Get(url)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("R2 download error (%d): %s", resp.StatusCode, string(b))
	}
	f, err := os.Create(destPath)
	if err != nil {
		return err
	}
	defer f.Close()
	_, err = io.Copy(f, resp.Body)
	return err
}

// CreateCampaignInD1 creates or sets an active campaign in D1.
func (s *CloudflareSync) CreateCampaignInD1(campaignID, name, period, archive string, total, dailyLimit int) error {
	url := fmt.Sprintf("%s/api/d1/campaigns/create", s.workerURL)
	payload := map[string]interface{}{
		"id":               campaignID,
		"name":             name,
		"period":           period,
		"archive_filename": archive,
		"total_invoices":   total,
		"daily_limit":      dailyLimit,
	}
	b, _ := json.Marshal(payload)
	resp, err := s.doWithRetry(func() (*http.Request, error) {
		req, err := http.NewRequest(http.MethodPost, url, bytes.NewReader(b))
		if err != nil {
			return nil, err
		}
		req.Header.Set("Content-Type", "application/json")
		return req, nil
	})
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("D1 create campaign error (%d): %s", resp.StatusCode, string(body))
	}
	return nil
}

// IngestInvoicesBatchToD1 flushes a batch of invoice metadata to D1.
func (s *CloudflareSync) IngestInvoicesBatchToD1(campaignID string, invoices []map[string]interface{}) error {
	if len(invoices) == 0 {
		return nil
	}
	url := fmt.Sprintf("%s/api/d1/invoices/batch", s.workerURL)
	payload := map[string]interface{}{
		"campaign_id": campaignID,
		"invoices":    invoices,
	}
	b, _ := json.Marshal(payload)
	resp, err := s.doWithRetry(func() (*http.Request, error) {
		req, err := http.NewRequest(http.MethodPost, url, bytes.NewReader(b))
		if err != nil {
			return nil, err
		}
		req.Header.Set("Content-Type", "application/json")
		return req, nil
	})
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("D1 batch insert error (%d): %s", resp.StatusCode, string(body))
	}
	return nil
}

// GetActiveCampaignFromD1 queries D1 for the active campaign and today's usage.
func (s *CloudflareSync) GetActiveCampaignFromD1() (map[string]interface{}, error) {
	url := fmt.Sprintf("%s/api/d1/campaigns/active", s.workerURL)
	resp, err := s.httpClient.Get(url)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	var res map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&res); err != nil {
		return nil, err
	}
	return res, nil
}

// GetNextQueuedBatchFromD1 retrieves up to limit invoices with status QUEUED.
func (s *CloudflareSync) GetNextQueuedBatchFromD1(campaignID string, limit int) ([]map[string]interface{}, error) {
	url := fmt.Sprintf("%s/api/d1/invoices/next-batch?campaign_id=%s&limit=%d", s.workerURL, campaignID, limit)
	resp, err := s.httpClient.Get(url)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	var invoices []map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&invoices); err != nil {
		return nil, err
	}
	return invoices, nil
}

// MarkBatchSentInD1 updates invoices to ACCEPTED and records sent count in daily_usage.
func (s *CloudflareSync) MarkBatchSentInD1(campaignID string, sentIDs []string) error {
	if len(sentIDs) == 0 {
		return nil
	}
	url := fmt.Sprintf("%s/api/d1/invoices/mark-batch-sent", s.workerURL)
	payload := map[string]interface{}{
		"campaign_id": campaignID,
		"sent_ids":    sentIDs,
	}
	b, _ := json.Marshal(payload)
	resp, err := s.doWithRetry(func() (*http.Request, error) {
		req, err := http.NewRequest(http.MethodPost, url, bytes.NewReader(b))
		if err != nil {
			return nil, err
		}
		req.Header.Set("Content-Type", "application/json")
		return req, nil
	})
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("D1 mark sent error (%d): %s", resp.StatusCode, string(body))
	}
	return nil
}

