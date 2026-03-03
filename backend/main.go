package main

import (
	"fmt"
	"io/fs"
	"log"
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

func main() {
	log.Println("=================================================================")
	log.Println("  Hardsend - Server Edition")
	log.Println("  © 2026 Fernando Hirschfeld & Devrow. All rights reserved.")
	log.Println("  Closed Source.")
	log.Println("=================================================================")

	// Load configuration
	cfg := config.Load()

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

	// Initialize Resend email client
	emailClient, err := email.NewClient(cfg.ResendAPIKey, cfg.ResendFrom, cfg.ResendRateLimit)
	if err != nil {
		log.Printf("[WARNING] Failed to initialize email client: %v", err)
		log.Println("[WARNING] Email sending will fail. Ensure RESEND_API_KEY is configured.")
		// Continue running - the worker pool will handle errors via circuit breaker
		emailClient = nil
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
	uploadHandler := handlers.NewUploadHandler(db, pool, cfg.TempDir)
	jobsHandler := handlers.NewJobsHandler(db)
	webhookHandler := handlers.NewWebhookHandler(db, hub)
	missingEmailsHandler := handlers.NewMissingEmailsHandler(db)

	// Set up router
	r := chi.NewRouter()

	// Middleware
	r.Use(chimiddleware.Logger)
	r.Use(chimiddleware.Recoverer)
	r.Use(chimiddleware.RealIP)
	r.Use(cors.Handler(cors.Options{
		AllowedOrigins:   []string{"http://localhost:5173", "http://localhost:8080", "*"},
		AllowedMethods:   []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
		AllowedHeaders:   []string{"Accept", "Authorization", "Content-Type"},
		ExposedHeaders:   []string{"Link"},
		AllowCredentials: true,
		MaxAge:           300,
	}))

	// Public routes
	r.Post("/api/login", authHandler.Login)
	r.Post("/api/webhooks/resend", webhookHandler.ResendWebhook)

	// WebSocket route (auth via query param)
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
	})

	// Serve React static files
	staticDir := "./static"
	if _, err := os.Stat(staticDir); err == nil {
		log.Println("[Server] Serving static files from ./static")
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
		ReadTimeout:       5 * time.Minute,
		ReadHeaderTimeout: 30 * time.Second,
		WriteTimeout:      10 * time.Minute,
		IdleTimeout:       30 * time.Minute,
		MaxHeaderBytes:    1 << 20, // 1MB max headers
	}
	if err := srv.ListenAndServe(); err != nil {
		log.Fatalf("Server failed: %v", err)
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
