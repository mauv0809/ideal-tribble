package main

import (
	"context"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/charmbracelet/log"
	"github.com/mauv0809/ideal-tribble/internal/club"
	"github.com/mauv0809/ideal-tribble/internal/config"
	"github.com/mauv0809/ideal-tribble/internal/database"
	server "github.com/mauv0809/ideal-tribble/internal/http"
	"github.com/mauv0809/ideal-tribble/internal/matchmaking"
	"github.com/mauv0809/ideal-tribble/internal/metrics"
	"github.com/mauv0809/ideal-tribble/internal/notifier/slack"
	"github.com/mauv0809/ideal-tribble/internal/playtomic"
	"github.com/mauv0809/ideal-tribble/internal/processor"
	"github.com/mauv0809/ideal-tribble/internal/pubsub"
)

func main() {
	// Start profiling timer
	startTime := time.Now()
	log.SetFormatter(log.JSONFormatter)
	cfg := config.Load()
	db, dbTeardown, err := database.InitDB(cfg.DBName, cfg.Turso.PrimaryURL, cfg.Turso.AuthToken, cfg.MigrationsDir)
	dbInitDuration := time.Since(startTime)
	log.Info("Database initialization time recorded", "duration_ms", dbInitDuration.Milliseconds())
	if err != nil {
		log.Fatalf("Failed to initialize database: %s", err)
	}
	defer func() {
		log.Info("Closing database connection")
		dbTeardown()
	}()
	/*dev := true
	options := inngestgo.ClientOpts{
		AppID:      cfg.Inngest.AppID,
		SigningKey: &cfg.Inngest.SingingKey,
		EventKey:   &cfg.Inngest.EventKey,
		Dev:        &dev,
	}
	inngestProvider, err := inngestgo.NewClient(options)
	if err != nil {
		log.Fatalf("Failed to initialize inngest: %s", err)
	}
	inngestClient := inngest.New(inngestProvider)
	*/
	clubStore := club.New(db)
	metricsSvc := metrics.NewService()
	metricsHandler := metrics.NewMetricsHandler()
	playtomicClient := playtomic.NewClient()
	notifier := slack.NewNotifier(cfg.Slack.Token, cfg.Slack.ChannelID, metricsSvc)
	pubsub := pubsub.New(cfg.ProjectID)
	processor := processor.New(clubStore, notifier, metricsSvc, pubsub)
	matchmakingService := matchmaking.NewStore(db)

	s := server.NewServer(
		clubStore,
		metricsSvc,
		metricsHandler,
		cfg,
		playtomicClient,
		notifier,
		processor,
		matchmakingService,
		pubsub,
		//inngestClient,
	)
	metricsSvc.SetStartupTime(float64(dbInitDuration.Milliseconds()) / 1000)

	// --- Record startup time ---
	startupDuration := time.Since(startTime)
	metricsSvc.SetStartupTime(startupDuration.Seconds())
	log.Info("Startup time recorded", "duration_ms", startupDuration.Milliseconds())

	// --- Graceful shutdown setup ---
	srv := &http.Server{
		Addr:    ":" + cfg.Port,
		Handler: s,
	}

	// Channel to listen for errors coming from the server
	serverErrors := make(chan error, 1)

	// Start the server in a goroutine
	go func() {
		log.Info("Server started", "port", cfg.Port)
		serverErrors <- srv.ListenAndServe()
	}()

	// Channel to listen for interrupt signals
	shutdown := make(chan os.Signal, 1)
	signal.Notify(shutdown, os.Interrupt, syscall.SIGTERM)

	// Block until we receive a signal or an error
	select {
	case err := <-serverErrors:
		if err != nil && err != http.ErrServerClosed {
			log.Fatalf("Server error: %v", err)
		}
	case sig := <-shutdown:
		log.Info("Shutdown signal received", "signal", sig)

		// Create a context with a timeout for the shutdown.
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		// Attempt to gracefully shut down the server.
		if err := srv.Shutdown(ctx); err != nil {
			log.Error("Server shutdown failed", "error", err)
		} else {
			log.Info("Server gracefully stopped")
		}
	}

	log.Info("Server process shutting down")
}
