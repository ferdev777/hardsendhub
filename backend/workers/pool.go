package workers

import (
	"context"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/google/uuid"

	"hardsend/config"
	"hardsend/database"
	"hardsend/email"
	"hardsend/models"
	"hardsend/websocket"
)

// Pool manages a pool of worker goroutines for processing invoices.
type Pool struct {
	cfg              *config.Config
	db               *database.DB
	emailClient      email.EmailSender
	hub              *websocket.Hub
	circuitBreaker   *CircuitBreaker
	bounceMonitor    *BounceRateMonitor
	jobChan          chan models.InvoiceJob
	wg               sync.WaitGroup
	activeWorkers    int64
	currentJobID     string
	paused           int32 // atomic: 1 = paused (daily limit or bounce alert)
	bounceAlertPause int32 // atomic: 1 = paused specifically due to bounce rate alert
	mu               sync.RWMutex
}

// NewPool creates a new worker pool.
func NewPool(cfg *config.Config, db *database.DB, emailClient email.EmailSender, hub *websocket.Hub) *Pool {
	p := &Pool{
		cfg:            cfg,
		db:             db,
		emailClient:    emailClient,
		hub:            hub,
		circuitBreaker: NewCircuitBreaker(cfg.CBFailureThreshold, cfg.CBRecoveryTimeout),
		jobChan:        make(chan models.InvoiceJob, 10000),
	}
	// Wire bounce monitor: on alert, pause the pool and broadcast emergency WS event
	p.bounceMonitor = NewBounceRateMonitor(func(rate float64, bounced, sent int) {
		atomic.StoreInt32(&p.bounceAlertPause, 1)
		atomic.StoreInt32(&p.paused, 1)
		log.Printf("[Pool] BOUNCE ALERT: rate=%.2f%% (%d/%d) — campaign auto-paused", rate*100, bounced, sent)
		// The broadcastMetrics goroutine will pick up bounce_alert_triggered=true on next tick
	})
	return p
}

// Start launches the worker goroutines.
func (p *Pool) Start() {
	log.Printf("[WorkerPool] Starting %d workers", p.cfg.WorkerCount)
	for i := 0; i < p.cfg.WorkerCount; i++ {
		p.wg.Add(1)
		go p.worker(i)
	}

	// Start metrics broadcaster
	go p.broadcastMetrics()
}

// Submit adds an invoice job to the processing queue.
func (p *Pool) Submit(job models.InvoiceJob) {
	p.jobChan <- job
}

// SetCurrentJobID sets the current batch job ID for metrics tracking.
func (p *Pool) SetCurrentJobID(jobID string) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.currentJobID = jobID
}

// GetCurrentJobID returns the current job ID.
func (p *Pool) GetCurrentJobID() string {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.currentJobID
}

// WaitForCompletion waits for all queued jobs to be processed.
func (p *Pool) WaitForCompletion() {
	// Wait until channel is drained
	for len(p.jobChan) > 0 {
		time.Sleep(100 * time.Millisecond)
	}
	// Wait a bit for active workers to finish
	for atomic.LoadInt64(&p.activeWorkers) > 0 {
		time.Sleep(100 * time.Millisecond)
	}
}

// GetActiveWorkers returns the number of currently active workers.
func (p *Pool) GetActiveWorkers() int {
	return int(atomic.LoadInt64(&p.activeWorkers))
}

// GetCircuitBreakerState returns the current circuit breaker state.
func (p *Pool) GetCircuitBreakerState() string {
	return string(p.circuitBreaker.GetState())
}

// worker is a goroutine that processes invoice jobs from the channel.
func (p *Pool) worker(id int) {
	defer p.wg.Done()
	log.Printf("[WorkerPool] Worker %d started", id)

	for job := range p.jobChan {
		// Check daily limit before processing
		for !p.db.CanSendToday(p.cfg.DailyLimit) {
			atomic.StoreInt32(&p.paused, 1)
			log.Printf("[Worker %d] Daily limit reached (%d). Paused until tomorrow.", id, p.cfg.DailyLimit)
			time.Sleep(5 * time.Minute)
		}

		// Check bounce alert pause (Capa 4)
		for p.IsBounceAlertActive() {
			log.Printf("[Worker %d] Bounce alert active. Waiting for operator to resume...", id)
			time.Sleep(30 * time.Second)
		}

		atomic.StoreInt32(&p.paused, 0)

		atomic.AddInt64(&p.activeWorkers, 1)
		p.processInvoice(job)
		atomic.AddInt64(&p.activeWorkers, -1)
	}

	log.Printf("[WorkerPool] Worker %d stopped", id)
}

// IsPaused returns true if workers are paused (daily limit reached or bounce alert).
func (p *Pool) IsPaused() bool {
	return atomic.LoadInt32(&p.paused) == 1
}

// IsBounceAlertActive returns true if the pool is paused due to bounce rate threshold.
func (p *Pool) IsBounceAlertActive() bool {
	return atomic.LoadInt32(&p.bounceAlertPause) == 1
}

// ResumeCampaignAfterBounceAlert clears the bounce alert and resumes the pool.
// Called by the operator from the dashboard.
func (p *Pool) ResumeCampaignAfterBounceAlert() {
	p.bounceMonitor.ResetAlert()
	atomic.StoreInt32(&p.bounceAlertPause, 0)
	atomic.StoreInt32(&p.paused, 0)
	log.Println("[Pool] Campaign resumed by operator after bounce alert.")
}

// processInvoice handles sending a single invoice email with retry logic and context timeouts.
func (p *Pool) processInvoice(job models.InvoiceJob) {
	// Auto-clean temporary uploaded PDFs after processing to keep container disk usage minimal
	defer func() {
		if p.cfg.TempDir != "" && job.PDFPath != "" {
			absTemp, err1 := filepath.Abs(p.cfg.TempDir)
			absPDF, err2 := filepath.Abs(job.PDFPath)
			if err1 == nil && err2 == nil && strings.HasPrefix(absPDF, absTemp+string(filepath.Separator)) {
				_ = os.Remove(absPDF)
			}
		}
	}()

	invoice := job.Invoice

	// Update status to PROCESSING
	_ = p.db.UpdateInvoiceStatus(invoice.ID, models.InvoiceStatusProcessing, nil, invoice.Attempts)

	for attempt := 1; attempt <= p.cfg.MaxRetries; attempt++ {
		invoice.Attempts = attempt

		// Check circuit breaker
		for !p.circuitBreaker.AllowRequest() {
			log.Printf("[Worker] Circuit breaker OPEN. Waiting for recovery...")
			p.circuitBreaker.TryTransitionToHalfOpen()
			time.Sleep(5 * time.Second) // Check every 5 seconds
		}

		// Professional approach: Create a context with timeout for the individual request
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)

		var err error
		var resendID string
		if p.emailClient == nil {
			err = fmt.Errorf("email service is not initialized on the server (check API keys)")
		} else {
			resendID, err = p.emailClient.SendInvoiceEmail(ctx, invoice.RecipientEmail, invoice.InvoiceNumber, job.PDFPath, job.ClientName, invoice.ID, job.DueDate, job.Template)
		}
		cancel()

	if err == nil {
		// Success
		p.circuitBreaker.RecordSuccess()
		p.bounceMonitor.RecordSent() // Capa 4: track sends in bounce rate window
		_ = p.db.UpdateInvoiceStatus(invoice.ID, models.InvoiceStatusSuccess, nil, attempt)
		if job.CampaignInvoiceID != "" {
			_ = p.db.UpdateCampaignInvoiceResendID(job.CampaignInvoiceID, models.CampaignInvoiceStatusSent, resendID)
		}
		_, _ = p.db.IncrementDailySendCount(p.cfg.DailyLimit)
		log.Printf("[Worker] Successfully sent invoice %s to %s (attempt %d, ID: %s, provider: %s)",
			invoice.InvoiceNumber, invoice.RecipientEmail, attempt, resendID,
			func() string {
				if p.emailClient != nil {
					return p.emailClient.Name()
				}
				return "none"
			}())
		return
	}

		// Record failure in circuit breaker
		p.circuitBreaker.RecordFailure()
		errStr := err.Error()
		log.Printf("[Worker] Failed to send invoice %s (attempt %d/%d): %s",
			invoice.InvoiceNumber, attempt, p.cfg.MaxRetries, errStr)

		// Detect permanent errors (invalid email / hard bounce) — don't retry
		if strings.Contains(errStr, "Invalid") || strings.Contains(errStr, "permanent") ||
			strings.Contains(errStr, "does not exist") || strings.Contains(errStr, "bounced") {
			p.bounceMonitor.RecordHardBounce() // Capa 4: count hard bounce in sliding window
			reason := "Email inválido rechazado por el servidor (hard bounce)"
			_ = p.db.UpdateInvoiceStatus(invoice.ID, models.InvoiceStatusErrorValidation, &reason, attempt)
			if job.CampaignInvoiceID != "" {
				_ = p.db.UpdateCampaignInvoiceStatus(job.CampaignInvoiceID, models.CampaignInvoiceStatusError)
			}
			me := &models.MissingEmail{
				ID:            uuid.New().String(),
				JobID:         job.JobID,
				InvoiceNumber: invoice.InvoiceNumber,
				ClientName:    job.ClientName,
				Email:         invoice.RecipientEmail,
				Reason:        "invalid_email",
				CreatedAt:     time.Now(),
			}
			_ = p.db.CreateMissingEmail(me)
			log.Printf("[Worker] Hard bounce for %s (%s) — registered in missing_emails",
				invoice.InvoiceNumber, invoice.RecipientEmail)
			return
		}

		// Update status with error
		_ = p.db.UpdateInvoiceStatus(invoice.ID, models.InvoiceStatusErrorNetwork, &errStr, attempt)

		// Wait before retry (unless it's the last attempt)
		if attempt < p.cfg.MaxRetries {
			log.Printf("[Worker] Waiting %v before retry for invoice %s",
				p.cfg.RetryDelay, invoice.InvoiceNumber)
			time.Sleep(p.cfg.RetryDelay)
		}
	}

	// All retries exhausted
	reason := "Maximum retry attempts exhausted"
	_ = p.db.UpdateInvoiceStatus(invoice.ID, models.InvoiceStatusErrorNetwork, &reason, p.cfg.MaxRetries)
	if job.CampaignInvoiceID != "" {
		_ = p.db.UpdateCampaignInvoiceStatus(job.CampaignInvoiceID, models.CampaignInvoiceStatusError)
	}
	log.Printf("[Worker] All retries exhausted for invoice %s", invoice.InvoiceNumber)
}


// broadcastMetrics sends real-time metrics to connected WebSocket clients every second.
func (p *Pool) broadcastMetrics() {
	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()

	var currentActiveSeconds int
	var lastKnownJobID string

	for range ticker.C {
		jobID := p.GetCurrentJobID()
		if jobID == "" {
			continue
		}

		if jobID != lastKnownJobID {
			currentActiveSeconds = 0
			lastKnownJobID = jobID
		}

		metrics, err := p.db.GetJobMetrics(jobID)
		if err != nil {
			continue
		}

		metrics.ActiveWorkers = p.GetActiveWorkers()
		metrics.CircuitBreakerState = p.GetCircuitBreakerState()
		metrics.Paused = p.IsPaused()
		metrics.BounceAlertTriggered = p.IsBounceAlertActive()

		// Bounce rate from monitor
		_, _, _, _, bounceRate := p.bounceMonitor.GetStats()
		metrics.BounceRate = bounceRate

		// Email provider name
		if p.emailClient != nil {
			metrics.EmailProvider = p.emailClient.Name()
		}

		jobFinished := metrics.ProcessedCount > 0 && metrics.ProcessedCount >= metrics.TotalFiles && metrics.ActiveWorkers == 0

		// Timer — only ticks if actively processing and NOT paused
		if !jobFinished && !metrics.Paused {
			currentActiveSeconds++
		}
		metrics.ElapsedSeconds = currentActiveSeconds

		// Daily tracking
		dailySent, dailyMax, _ := p.db.GetDailySendCount()
		metrics.DailySent = dailySent

		// Prioritizar el límite explícito configurado por el usuario en esta corrida.
		// Solo usar el de la BD si por alguna razón no se mandó config.
		metrics.DailyMax = p.cfg.DailyLimit
		if metrics.DailyMax <= 0 && dailyMax > 0 {
			metrics.DailyMax = dailyMax
		}

		p.hub.Broadcast(metrics)
	}
}
