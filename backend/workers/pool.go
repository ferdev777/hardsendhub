package workers

import (
	"context"
	"fmt"
	"log"
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
	cfg            *config.Config
	db             *database.DB
	emailClient    email.EmailSender
	hub            *websocket.Hub
	circuitBreaker *CircuitBreaker
	jobChan        chan models.InvoiceJob
	wg             sync.WaitGroup
	activeWorkers  int64
	currentJobID   string
	paused         int32 // atomic: 1 = paused
	mu             sync.RWMutex
}

// NewPool creates a new worker pool.
func NewPool(cfg *config.Config, db *database.DB, emailClient email.EmailSender, hub *websocket.Hub) *Pool {
	return &Pool{
		cfg:            cfg,
		db:             db,
		emailClient:    emailClient,
		hub:            hub,
		circuitBreaker: NewCircuitBreaker(cfg.CBFailureThreshold, cfg.CBRecoveryTimeout),
		jobChan:        make(chan models.InvoiceJob, cfg.WorkerCount*2),
	}
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
			// Sleep 5 minutes and recheck (date change resets count)
			time.Sleep(5 * time.Minute)
		}
		atomic.StoreInt32(&p.paused, 0)

		atomic.AddInt64(&p.activeWorkers, 1)
		p.processInvoice(job)
		atomic.AddInt64(&p.activeWorkers, -1)
	}

	log.Printf("[WorkerPool] Worker %d stopped", id)
}

// IsPaused returns true if workers are paused (daily limit reached).
func (p *Pool) IsPaused() bool {
	return atomic.LoadInt32(&p.paused) == 1
}

// processInvoice handles sending a single invoice email with retry logic and context timeouts.
func (p *Pool) processInvoice(job models.InvoiceJob) {
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
			_ = p.db.UpdateInvoiceStatus(invoice.ID, models.InvoiceStatusSuccess, nil, attempt)
			if job.CampaignInvoiceID != "" {
				_ = p.db.UpdateCampaignInvoiceResendID(job.CampaignInvoiceID, models.CampaignInvoiceStatusSent, resendID)
			}
			_, _ = p.db.IncrementDailySendCount(p.cfg.DailyLimit)
			log.Printf("[Worker] Successfully sent invoice %s to %s (attempt %d, ResendID: %s)",
				invoice.InvoiceNumber, invoice.RecipientEmail, attempt, resendID)
			return
		}

		// Record failure in circuit breaker
		p.circuitBreaker.RecordFailure()
		errStr := err.Error()
		log.Printf("[Worker] Failed to send invoice %s (attempt %d/%d): %s",
			invoice.InvoiceNumber, attempt, p.cfg.MaxRetries, errStr)

		// Detect permanent errors (invalid email) — don't retry
		if strings.Contains(errStr, "Invalid") {
			reason := "Email inválido rechazado por el servidor"
			_ = p.db.UpdateInvoiceStatus(invoice.ID, models.InvoiceStatusErrorValidation, &reason, attempt)
			if job.CampaignInvoiceID != "" {
				_ = p.db.UpdateCampaignInvoiceStatus(job.CampaignInvoiceID, models.CampaignInvoiceStatusError)
			}
			// Register in missing_emails as invalid_email
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
			log.Printf("[Worker] Invalid email detected for %s (%s) — registered in missing_emails",
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
