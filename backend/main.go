package main

import (
	"fmt"
	"io"
	"io/fs"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"github.com/go-chi/chi/v5"
	chimiddleware "github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/cors"

	"hardsend/auth"
	"hardsend/config"
	"hardsend/database"
	"hardsend/email"
	"hardsend/handlers"
	"hardsend/reputation"
	"hardsend/websocket"
	"hardsend/workers"
)

func main() {
	log.Println("=================================================================")
	log.Println("  Hardsend - Server Edition")
	log.Println("  © 2026 Fernando Hirschfeld & Devrow. All rights reserved.")
	log.Println("  Closed Source.")
	log.Println("=================================================================")

	// Load configuration
	cfg := config.Load()

	// Ensure pre-seeded reputation DB is restored on cold container start if target DB does not exist yet
	ensureSeedDatabase(cfg.DBPath, "/app/seed/hardsend_metrics.db")

	// Initialize database
	db, err := database.New(cfg.DBPath)
	if err != nil {
		log.Fatalf("Failed to initialize database: %v", err)
	}
	defer db.Close()

	// Create temp directory
	if err := os.MkdirAll(cfg.TempDir, 0755); err != nil {
		log.Fatalf("Failed to create temp directory: %v", err)
	}

	// Initialize reputation store and MX cache (Capa 1 y Capa 2, Step 3)
	repStore := reputation.New(db.RawConn())
	mxCache := reputation.NewMXCache(5 * time.Second)

	// Initialize Email Client (Cloudflare direct REST API / Worker Proxy or Resend fallback)
	var emailClient email.EmailProvider
	var errEmail error

	if (cfg.CFAccountID != "" && cfg.CFAPIToken != "") || cfg.CFWorkerURL != "" {
		emailClient, errEmail = email.NewCloudflareClient(
			cfg.CFAccountID,
			cfg.CFAPIToken,
			cfg.CFWorkerURL,
			cfg.CFWorkerToken,
			cfg.CFFromAddress,
			cfg.CFRateLimit,
		)
		if errEmail != nil {
			log.Printf("[WARNING] Failed to init Cloudflare client: %v", errEmail)
		}
	} else {
		// Fallback to Resend
		emailClient, errEmail = email.NewClient(cfg.ResendAPIKey, cfg.ResendFrom, cfg.ResendRateLimit)
		if errEmail != nil {
			log.Printf("[WARNING] Failed to init Resend client: %v", errEmail)
		}
	}

	if emailClient == nil {
		log.Println("[WARNING] Email sending will fail. Ensure Cloudflare or Resend is configured.")
	}

	// Initialize WebSocket hub
	hub := websocket.NewHub(cfg.JWTSecret)
	go hub.Run()

	// Initialize worker pool
	pool := workers.NewPool(cfg, db, emailClient, hub)
	pool.Start()

	// Start temp file cleaner (removes processed PDFs older than 20 days, checks every 24 hours)
	cleaner := workers.NewTempCleaner(cfg.TempDir, 20, 24)
	cleaner.Start()

	// Initialize handlers
	authHandler := handlers.NewAuthHandler(cfg)
	uploadHandler := handlers.NewUploadHandler(db, pool, cfg, repStore, cfg.TempDir)
	jobsHandler := handlers.NewJobsHandler(db)
	missingEmailsHandler := handlers.NewMissingEmailsHandler(db)
	campaignHandler := handlers.NewCampaignHandler(db, pool, cfg, repStore, mxCache, emailClient)
	historyHandler := handlers.NewHistoryHandler(db)
	filesystemHandler := handlers.NewFilesystemHandler()
	webhookHandler := handlers.NewWebhookHandler(db, repStore, cfg.SvixSecret)
	analyticsHandler := handlers.NewAnalyticsHandler(db, cfg)

	// Start automatic recovery for interrupted campaigns
	go campaignHandler.ResumeActiveCampaign()

	// Start daily scheduler (runs at configured SCHEDULE_TIME, default 09:00)
	scheduler := workers.NewScheduler(db, campaignHandler, cfg.ScheduleTime)
	scheduler.Start()
	defer scheduler.Stop()

	// Set up router
	r := chi.NewRouter()

	// Middleware
	r.Use(chimiddleware.Logger)
	r.Use(chimiddleware.Recoverer)
	r.Use(chimiddleware.RealIP)
	r.Use(cors.Handler(cors.Options{
		AllowedOrigins:   []string{"http://localhost:5173", "http://localhost:8080", "*"},
		AllowedMethods:   []string{"GET", "POST", "PUT", "PATCH", "DELETE", "OPTIONS"},
		AllowedHeaders:   []string{"Accept", "Authorization", "Content-Type"},
		ExposedHeaders:   []string{"Link"},
		AllowCredentials: true,
		MaxAge:           300,
	}))

	// Public routes
	r.Post("/api/login", authHandler.Login)
	r.Post("/api/auth/google", authHandler.GoogleLogin)
	r.Post("/api/webhooks/resend", webhookHandler.HandleResendWebhook)
	r.Post("/api/webhooks/cloudflare", webhookHandler.HandleCloudflareWebhook)

	// WebSocket route (auth via query param)
	r.Get("/ws/metrics", hub.HandleWebSocket)

	// Protected API routes
	r.Group(func(r chi.Router) {
		r.Use(auth.Middleware(cfg.JWTSecret))

		r.Get("/api/validate", authHandler.ValidateToken)
		r.Post("/api/upload", uploadHandler.Upload)
		r.Post("/api/upload/chunk", uploadHandler.StreamChunk)
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
		r.Post("/api/campaigns/dispatch-daily", campaignHandler.DispatchDaily)

		// Analytics dashboard endpoints
		r.Get("/api/analytics/summary", analyticsHandler.GetSummary)
		r.Get("/api/analytics/timeseries", analyticsHandler.GetTimeSeries)

		// Monthly history + manual correction routes + system
		r.Get("/api/history/monthly", historyHandler.GetMonthly)
		r.Patch("/api/invoices/{id}/status", historyHandler.UpdateInvoiceStatus)
		r.Delete("/api/system/reset", historyHandler.ResetSystem)

		// Filesystem browsing for folder picker
		r.Get("/api/filesystem/browse", filesystemHandler.Browse)
		r.Get("/api/filesystem/drives", filesystemHandler.Drives)
	})

	// Serve React static files
	staticDir := "./static"
	if _, err := os.Stat(staticDir); err != nil {
		if _, errWww := os.Stat("./www"); errWww == nil {
			staticDir = "./www"
		}
	}
	if _, err := os.Stat(staticDir); err == nil {
		log.Printf("[Server] Serving static files from %s", staticDir)
		fileServer(r, staticDir)
	} else {
		log.Println("[Server] No static directory found. Frontend must be served separately.")
	}

	// Start server with generous timeouts for large file uploads
	addr := fmt.Sprintf(":%s", cfg.ServerPort)
	log.Printf("[Server] Starting on %s", addr)
	srv := &http.Server{
		Addr:              addr,
		Handler:           r,
		ReadTimeout:       30 * time.Minute,
		ReadHeaderTimeout: 30 * time.Second,
		WriteTimeout:      30 * time.Minute,
		IdleTimeout:       30 * time.Minute,
		MaxHeaderBytes:    1 << 20, // 1MB max headers
	}
	if err := srv.ListenAndServe(); err != nil {
		log.Fatalf("Server failed: %v", err)
	}
}

func ensureSeedDatabase(targetDBPath, seedDBPath string) {
	if _, err := os.Stat(targetDBPath); err == nil {
		return // Target DB already exists
	}
	if _, err := os.Stat(seedDBPath); err != nil {
		return // No seed DB available
	}
	_ = os.MkdirAll(filepath.Dir(targetDBPath), 0755)
	in, err := os.Open(seedDBPath)
	if err != nil {
		return
	}
	defer in.Close()
	out, err := os.Create(targetDBPath)
	if err != nil {
		return
	}
	defer out.Close()
	if _, err := io.Copy(out, in); err == nil {
		log.Printf("[Seed] Restored pre-seeded reputation database from %s to %s", seedDBPath, targetDBPath)
	}
}

// fileServer serves static files from the given directory and falls back to index.html for SPA routing.
func fileServer(r chi.Router, staticDir string) {
	root := http.Dir(staticDir)

	r.Get("/*", func(w http.ResponseWriter, r *http.Request) {
		// Try to serve the file from the static directory
		path := r.URL.Path

		// Check if the file exists
		f, err := root.Open(path)
		if err != nil {
			// File doesn't exist, serve index.html for SPA routing
			indexFile, err := os.ReadFile(staticDir + "/index.html")
			if err != nil {
				http.NotFound(w, r)
				return
			}
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			w.Write(indexFile)
			return
		}

		// Check if it's a directory
		stat, err := f.Stat()
		f.Close()
		if err != nil || stat.IsDir() {
			// Try to serve index.html from the directory
			indexPath := path + "/index.html"
			if _, err := fs.Stat(os.DirFS(staticDir), indexPath[1:]); err != nil {
				indexFile, err := os.ReadFile(staticDir + "/index.html")
				if err != nil {
					http.NotFound(w, r)
					return
				}
				w.Header().Set("Content-Type", "text/html; charset=utf-8")
				w.Write(indexFile)
				return
			}
		}

		// Serve the file
		http.FileServer(root).ServeHTTP(w, r)
	})
}
