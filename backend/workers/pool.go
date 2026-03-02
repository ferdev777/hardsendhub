package workers

import (
	"context"
	"fmt"
	"log"
	"sync"
	"sync/atomic"
	"time"

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
	emailClient    email.EmailSender // Changed to interface for professionalism
	hub            *websocket.Hub
	circuitBreaker *CircuitBreaker
	jobChan        chan models.InvoiceJob
	wg             sync.WaitGroup
	activeWorkers  int64
	currentJobID   string
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
		atomic.AddInt64(&p.activeWorkers, 1)
		p.processInvoice(job)
		atomic.AddInt64(&p.activeWorkers, -1)
	}

	log.Printf("[WorkerPool] Worker %d stopped", id)
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

		// Attempt to send via EmailSender
		var err error
		if p.emailClient == nil {
			err = fmt.Errorf("email service is not initialized on the server (check API keys)")
		} else {
			err = p.emailClient.SendInvoiceEmail(ctx, invoice.RecipientEmail, invoice.InvoiceNumber, job.PDFPath, job.ClientName, invoice.ID)
		}
		cancel()

		if err == nil {
			// Success
			p.circuitBreaker.RecordSuccess()
			_ = p.db.UpdateInvoiceStatus(invoice.ID, models.InvoiceStatusSuccess, nil, attempt)
			log.Printf("[Worker] Successfully sent invoice %s to %s (attempt %d)",
				invoice.InvoiceNumber, invoice.RecipientEmail, attempt)
			return
		}

		// Record failure in circuit breaker
		p.circuitBreaker.RecordFailure()
		errStr := err.Error()
		log.Printf("[Worker] Failed to send invoice %s (attempt %d/%d): %s",
			invoice.InvoiceNumber, attempt, p.cfg.MaxRetries, errStr)

		// Update status with error
		_ = p.db.UpdateInvoiceStatus(invoice.ID, models.InvoiceStatusErrorNetwork, &errStr, attempt)

		// Wait before retry (unless it's the last attempt)
		if attempt < p.cfg.MaxRetries {
			log.Printf("[Worker] Waiting %v before retry for invoice %s",
				p.cfg.RetryDelay, invoice.InvoiceNumber)

			// Use a context-aware sleep or just time.Sleep in this worker loop
			time.Sleep(p.cfg.RetryDelay)
		}
	}

	// All retries exhausted
	reason := "Maximum retry attempts exhausted"
	_ = p.db.UpdateInvoiceStatus(invoice.ID, models.InvoiceStatusErrorNetwork, &reason, p.cfg.MaxRetries)
	log.Printf("[Worker] All retries exhausted for invoice %s", invoice.InvoiceNumber)
}

// broadcastMetrics sends real-time metrics to connected WebSocket clients every second.
func (p *Pool) broadcastMetrics() {
	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()

	for range ticker.C {
		jobID := p.GetCurrentJobID()
		if jobID == "" {
			continue
		}

		metrics, err := p.db.GetJobMetrics(jobID)
		if err != nil {
			continue
		}

		metrics.ActiveWorkers = p.GetActiveWorkers()
		metrics.CircuitBreakerState = p.GetCircuitBreakerState()

		p.hub.Broadcast(metrics)
	}
}
