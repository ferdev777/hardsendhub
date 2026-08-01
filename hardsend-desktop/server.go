package main

import (
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"time"

	"github.com/go-chi/chi/v5"
	chimiddleware "github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/cors"

	"hardsend/auth"
	"hardsend/config"
	"hardsend/database"
	"hardsend/email"
	"hardsend/handlers"
	"hardsend/websocket"
	"hardsend/workers"
)

func startBackendServer() {
	log.Println("[Desktop] Starting embedded Hardsend API server on port 8088...")

	// Load configuration
	cfg := config.Load()

	addr := fmt.Sprintf("127.0.0.1:%s", cfg.ServerPort)
	ln, err := net.Listen("tcp", addr)
	if err != nil {
		log.Printf("[Desktop] Notice: Port %s is already in use by another Hardsend instance. Using active server without duplicate workers.", cfg.ServerPort)
		return
	}

	// Initialize database
	db, err := database.New(cfg.DBPath)
	if err != nil {
		log.Fatalf("Failed to initialize database: %v", err)
	}

	// Create temp directory
	if err := os.MkdirAll(cfg.TempDir, 0755); err != nil {
		log.Fatalf("Failed to create temp directory: %v", err)
	}

	// Initialize Resend email client
	emailClient, err := email.NewClient(cfg.ResendAPIKey, cfg.ResendFrom, cfg.ResendRateLimit)
	if err != nil {
		log.Printf("[WARNING] Failed to initialize email client: %v", err)
		emailClient = nil
	}

	// Initialize WebSocket hub
	hub := websocket.NewHub(cfg.JWTSecret)
	go hub.Run()

	// Initialize worker pool
	pool := workers.NewPool(cfg, db, emailClient, hub)
	pool.Start()

	// Start temp file cleaner
	cleaner := workers.NewTempCleaner(cfg.TempDir, 20, 24)
	cleaner.Start()

	// Initialize handlers
	authHandler := handlers.NewAuthHandler(cfg)
	uploadHandler := handlers.NewUploadHandler(db, pool, cfg, cfg.TempDir)
	jobsHandler := handlers.NewJobsHandler(db)
	missingEmailsHandler := handlers.NewMissingEmailsHandler(db)
	campaignHandler := handlers.NewCampaignHandler(db, pool, cfg)
	historyHandler := handlers.NewHistoryHandler(db)
	filesystemHandler := handlers.NewFilesystemHandler()
	webhookHandler := handlers.NewWebhookHandler(db, cfg.SvixSecret)
	analyticsHandler := handlers.NewAnalyticsHandler(db, cfg)

	// Start automatic recovery for interrupted campaigns
	go campaignHandler.ResumeActiveCampaign()

	// Start daily scheduler
	scheduler := workers.NewScheduler(db, campaignHandler, cfg.ScheduleTime)
	scheduler.Start()

	// Set up router
	r := chi.NewRouter()

	// Middleware
	r.Use(chimiddleware.Logger)
	r.Use(chimiddleware.Recoverer)
	r.Use(chimiddleware.RealIP)
	r.Use(cors.Handler(cors.Options{
		AllowedOrigins:   []string{"http://localhost:5173", "http://localhost:8088", "http://wails.localhost", "http://wails.localhost:34115", "*"},
		AllowedMethods:   []string{"GET", "POST", "PUT", "PATCH", "DELETE", "OPTIONS"},
		AllowedHeaders:   []string{"Accept", "Authorization", "Content-Type"},
		ExposedHeaders:   []string{"Link"},
		AllowCredentials: true,
		MaxAge:           300,
	}))

	// Public routes
	r.Post("/api/login", authHandler.Login)
	r.Post("/api/webhooks/resend", webhookHandler.HandleResendWebhook)

	// WebSocket route
	r.Get("/ws/metrics", hub.HandleWebSocket)

	// Protected API routes
	r.Group(func(r chi.Router) {
		r.Use(auth.Middleware(cfg.JWTSecret))

		r.Get("/api/validate", authHandler.ValidateToken)
		r.Post("/api/upload", uploadHandler.Upload)
		r.Get("/api/jobs", jobsHandler.GetRecentJobs)
		r.Get("/api/jobs/{jobID}/errors", jobsHandler.GetJobErrors)
		r.Get("/api/jobs/{jobID}/metrics", jobsHandler.GetJobMetrics)
		r.Get("/api/errors", jobsHandler.GetAllErrors)
		r.Get("/api/history", jobsHandler.GetHistory)
		r.Get("/api/missing-emails", missingEmailsHandler.GetMissingEmails)
		r.Get("/api/missing-emails/export", missingEmailsHandler.ExportMissingEmails)
		r.Post("/api/missing-emails/resolve", missingEmailsHandler.ResolveMissingEmails)

		// Campaign routes
		r.Post("/api/campaigns/analyze", campaignHandler.Analyze)
		r.Get("/api/campaigns/active", campaignHandler.GetActive)
		r.Get("/api/campaigns/{id}", campaignHandler.GetCampaign)
		r.Post("/api/campaigns/{id}/rescan", campaignHandler.Rescan)
		r.Post("/api/campaigns/{id}/start", campaignHandler.Start)
		r.Post("/api/campaigns/{id}/cancel", campaignHandler.Cancel)

		// Analytics dashboard endpoints
		r.Get("/api/analytics/summary", analyticsHandler.GetSummary)
		r.Get("/api/analytics/timeseries", analyticsHandler.GetTimeSeries)
		r.Post("/api/resend/sync", analyticsHandler.SyncResend)

		// Monthly history + manual correction routes + system
		r.Get("/api/history/monthly", historyHandler.GetMonthly)
		r.Patch("/api/invoices/{id}/status", historyHandler.UpdateInvoiceStatus)
		r.Delete("/api/system/reset", historyHandler.ResetSystem)

		// Filesystem browsing for folder picker
		r.Get("/api/filesystem/browse", filesystemHandler.Browse)
		r.Get("/api/filesystem/drives", filesystemHandler.Drives)
	})

	log.Printf("[Desktop] Embedded API server listening on %s", addr)
	srv := &http.Server{
		Handler:           r,
		ReadTimeout:       5 * time.Minute,
		ReadHeaderTimeout: 30 * time.Second,
		WriteTimeout:      10 * time.Minute,
		IdleTimeout:       30 * time.Minute,
		MaxHeaderBytes:    1 << 20,
	}
	if err := srv.Serve(ln); err != nil && err != http.ErrServerClosed {
		log.Printf("[Desktop] Notice: Embedded API server stopped: %v", err)
	}
}
