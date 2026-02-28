package main

import (
	"context"
	"encoding/json"
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
	server "github.com/mauv0809/ideal-tribble/internal/http"
	"github.com/mauv0809/ideal-tribble/internal/jobqueue"
	"github.com/mauv0809/ideal-tribble/internal/matchmaking"
	"github.com/mauv0809/ideal-tribble/internal/metrics"
	"github.com/mauv0809/ideal-tribble/internal/notifier/slack"
	"github.com/mauv0809/ideal-tribble/internal/pairings"
	"github.com/mauv0809/ideal-tribble/internal/playtomic"
	"github.com/mauv0809/ideal-tribble/internal/processor"
	"github.com/mauv0809/ideal-tribble/internal/telemetry"
	"github.com/mauv0809/ideal-tribble/internal/web"
	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
	"golang.ngrok.com/ngrok/v2"
)

func main() {
	// Start profiling timer
	startTime := time.Now()
	log.SetFormatter(log.JSONFormatter)
	
	// Initialize OpenTelemetry
	otelShutdown, err := telemetry.InitOtel(context.Background())
	if err != nil {
		log.Error("Failed to initialize OpenTelemetry", "error", err)
		os.Exit(1)
	}
	defer otelShutdown()
	
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
	jobQueue := jobqueue.New(db)
	matchmakingService := matchmaking.NewStore(db, clubStore)
	pairingsStore := pairings.New(db)
	processor := processor.New(clubStore, notifier, metricsSvc, jobQueue, matchmakingService)

	// Initialize auth components for web UI
	authStore := auth.NewStore(db)
	sessionManager := auth.NewSessionManager(db)

	// Create fetch service for manual match fetching from web UI
	fetchService := web.NewFetchService(
		clubStore,
		cfg,
		playtomicClient,
		matchmakingService,
		pairingsStore,
	)

	// Create web router
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

	s := server.NewServer(
		clubStore,
		metricsSvc,
		metricsHandler,
		cfg,
		playtomicClient,
		notifier,
		processor,
		matchmakingService,
		pairingsStore,
		jobQueue,
		//inngestClient,
	)
	metricsSvc.SetStartupTime(float64(dbInitDuration.Milliseconds()) / 1000)

	// --- Setup job queue worker ---
	worker := jobqueue.NewWorker(jobQueue, log.StandardLog())
	
	// Register job handlers
	worker.RegisterHandler(jobqueue.JobTypeAssignBallBoy, func(payload json.RawMessage) error {
		var match playtomic.PadelMatch
		if err := json.Unmarshal(payload, &match); err != nil {
			return err
		}
		processor.AssignBallBringer(&match, false)
		return nil
	})
	worker.RegisterHandler(jobqueue.JobTypeNotifyBooking, func(payload json.RawMessage) error {
		var match playtomic.PadelMatch
		if err := json.Unmarshal(payload, &match); err != nil {
			return err
		}
		return processor.NotifyBooking(&match, false)
	})
	worker.RegisterHandler(jobqueue.JobTypeNotifyResult, func(payload json.RawMessage) error {
		var match playtomic.PadelMatch
		if err := json.Unmarshal(payload, &match); err != nil {
			return err
		}
		return processor.NotifyResult(&match, false)
	})
	worker.RegisterHandler(jobqueue.JobTypeUpdatePlayerStats, func(payload json.RawMessage) error {
		var match playtomic.PadelMatch
		if err := json.Unmarshal(payload, &match); err != nil {
			return err
		}
		processor.UpdatePlayerStats(&match, false)
		return nil
	})
	worker.RegisterHandler(jobqueue.JobTypeUpdateWeeklyStats, func(payload json.RawMessage) error {
		var match playtomic.PadelMatch
		if err := json.Unmarshal(payload, &match); err != nil {
			return err
		}
		processor.UpdateWeeklyStats(&match, false)
		return nil
	})

	// --- Record startup time ---
	startupDuration := time.Since(startTime)
	metricsSvc.SetStartupTime(startupDuration.Seconds())
	log.Info("Startup time recorded", "duration_ms", startupDuration.Milliseconds())

	// --- Ngrok setup for local development ---
	var listener ngrok.EndpointListener
	if cfg.Ngrok.AuthToken != "" {
		log.Info("Creating ngrok tunnel")
		a, err := ngrok.NewAgent(ngrok.WithAuthtoken(cfg.Ngrok.AuthToken))
		if err != nil {
			log.Fatal(err)
		}
		l, err := a.Listen(context.Background(), ngrok.WithURL("dove-saving-gnu.ngrok-free.app"))
		if err != nil {
			log.Fatal(err)
		}
		log.Info("Ngrok tunnel created", "url", l.URL())
		listener = l
	} else {
		log.Info("Ngrok tunnel disabled")
	}
	defer func() {
		if listener != nil {
			log.Info("Closing ngrok tunnel")
			listener.Close()
		}
	}()

	// --- Graceful shutdown setup ---
	// Create a combined handler that routes to either API or web UI
	combinedHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Web UI routes
		switch {
		case r.URL.Path == "/" ||
			hasPrefix(r.URL.Path, "/login") ||
			r.URL.Path == "/logout" ||
			r.URL.Path == "/dashboard" ||
			r.URL.Path == "/profile" ||
			r.URL.Path == "/fetch-matches" ||
			r.URL.Path == "/api/theme" ||
			r.URL.Path == "/api/players/suggest" ||
			r.URL.Path == "/api/venues/suggest" ||
			hasPrefix(r.URL.Path, "/static/") ||
			hasPrefix(r.URL.Path, "/pairings") ||
			hasPrefix(r.URL.Path, "/matches") ||
			hasPrefix(r.URL.Path, "/players") ||
			hasPrefix(r.URL.Path, "/admin") ||
			hasPrefix(r.URL.Path, "/profile/"):
			webRouter.ServeHTTP(w, r)
		default:
			// API routes (existing server)
			s.ServeHTTP(w, r)
		}
	})

	// Wrap the combined handler with OpenTelemetry middleware
	wrappedHandler := otelhttp.NewHandler(combinedHandler, "ideal-tribble")

	// Add HTTP metrics middleware
	metricsWrappedHandler := metricsSvc.HTTPMiddleware(wrappedHandler)

	srv := &http.Server{
		Addr:    ":" + cfg.Port,
		Handler: metricsWrappedHandler,
	}

	// Channel to listen for errors coming from the server
	serverErrors := make(chan error, 1)

	// Start the job worker in a goroutine
	workerCtx, cancelWorker := context.WithCancel(context.Background())
	go func() {
		log.Info("Starting job queue worker")
		worker.StartCleanup(workerCtx, 1*time.Hour, 24*time.Hour) // Cleanup every hour, remove jobs older than 1 day
		if err := worker.Start(workerCtx); err != nil && err != context.Canceled {
			log.Error("Job worker error", "error", err)
		}
	}()

	// Start the server in a goroutine
	go func() {
		log.Info("Server started", "port", cfg.Port)
		serverErrors <- srv.ListenAndServe()
	}()

	// If ngrok is enabled, also serve through the tunnel
	if listener != nil {
		log.Info("Server also available via ngrok tunnel", "tunnel_url", listener.URL())
		go func() {
			// Create a separate server instance for ngrok to avoid port conflicts
			ngrokSrv := &http.Server{
				Handler: wrappedHandler,
			}
			if err := ngrokSrv.Serve(listener); err != nil && err != http.ErrServerClosed {
				log.Error("Ngrok server error", "error", err)
			}
		}()
	}

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

		// Stop the job worker first
		log.Info("Stopping job queue worker...")
		cancelWorker()

		// Attempt to gracefully shut down the server.
		if err := srv.Shutdown(ctx); err != nil {
			log.Error("Server shutdown failed", "error", err)
		} else {
			log.Info("Server gracefully stopped")
		}
	}

	log.Info("Server process shutting down")
}

// hasPrefix checks if s starts with prefix.
func hasPrefix(s, prefix string) bool {
	return len(s) >= len(prefix) && s[:len(prefix)] == prefix
}
