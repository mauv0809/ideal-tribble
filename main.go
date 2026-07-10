package main

import (
	"context"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/charmbracelet/log"
	"github.com/mauv0809/ideal-tribble/internal/auth"
	"github.com/mauv0809/ideal-tribble/internal/club"
	"github.com/mauv0809/ideal-tribble/internal/config"
	"github.com/mauv0809/ideal-tribble/internal/database"
	"github.com/mauv0809/ideal-tribble/internal/pairings"
	"github.com/mauv0809/ideal-tribble/internal/playtomic"
	"github.com/mauv0809/ideal-tribble/internal/scheduler"
	"github.com/mauv0809/ideal-tribble/internal/telemetry"
	"github.com/mauv0809/ideal-tribble/internal/web"
)

func main() {
	startTime := time.Now()

	// Logging (JSON by default; configurable via LOG_FORMAT and LOG_LEVEL).
	telemetry.InitLogging()

	cfg := config.Load()

	db, dbTeardown, err := database.InitDB(cfg.DBName, cfg.Turso.PrimaryURL, cfg.Turso.AuthToken, cfg.MigrationsDir)
	if err != nil {
		log.Fatalf("Failed to initialize database: %s", err)
	}
	defer func() {
		log.Info("Closing database connection")
		dbTeardown()
	}()

	// --- Stores & services ---
	clubStore := club.New(db)
	pairingsStore := pairings.New(db)
	authStore := auth.NewStore(db)
	sessionManager := auth.NewSessionManager(db)
	playtomicClient := playtomic.NewClient()

	// FetchService pulls matches from Playtomic and populates the pairing/club
	// data the web dashboard reads. Used by both the manual "Fetch matches"
	// button and the scheduler below.
	fetchService := web.NewFetchService(
		clubStore,
		cfg,
		playtomicClient,
		pairingsStore,
	)

	webRouter, err := web.NewRouter(
		web.Config{
			SessionSecret:     []byte(cfg.Web.SessionSecret),
			TOTPEncryptionKey: []byte(cfg.Web.TOTPEncryptionKey),
		},
		authStore,
		sessionManager,
		pairingsStore,
		clubStore,
		fetchService,
	)
	if err != nil {
		log.Fatalf("Failed to create web router: %s", err)
	}

	// --- In-app scheduler (replaces the host crontab) ---
	// Hourly fetch keeps dashboard data fresh; SkipIfStillRunning guards
	// against a slow run stacking up against the next tick.
	sched := scheduler.New(log.Default())
	if err := sched.Register("0 * * * *", "fetch-matches", func() {
		if _, _, err := fetchService.FetchMatches(1); err != nil {
			log.Error("Scheduled fetch failed", "error", err)
		}
	}); err != nil {
		log.Fatalf("Failed to register fetch job: %s", err)
	}
	sched.Start()
	log.Info("In-app scheduler started")

	// --- HTTP server ---
	srv := &http.Server{
		Addr:    ":" + cfg.Port,
		Handler: webRouter,
	}

	log.Info("Startup complete", "duration_ms", time.Since(startTime).Milliseconds())

	serverErrors := make(chan error, 1)
	go func() {
		log.Info("Server started", "port", cfg.Port)
		serverErrors <- srv.ListenAndServe()
	}()

	// --- Graceful shutdown ---
	shutdown := make(chan os.Signal, 1)
	signal.Notify(shutdown, os.Interrupt, syscall.SIGTERM)

	select {
	case err := <-serverErrors:
		if err != nil && err != http.ErrServerClosed {
			log.Fatalf("Server error: %v", err)
		}
	case sig := <-shutdown:
		log.Info("Shutdown signal received", "signal", sig)

		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		// Stop the scheduler first so no new fetch runs start during teardown.
		log.Info("Stopping scheduler...")
		sched.Stop(ctx)

		if err := srv.Shutdown(ctx); err != nil {
			log.Error("Server shutdown failed", "error", err)
		} else {
			log.Info("Server gracefully stopped")
		}
	}

	log.Info("Server process shutting down")
}
