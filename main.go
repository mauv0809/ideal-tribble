package main

import (
	"net/http"

	"github.com/charmbracelet/log"
	"github.com/mauv0809/ideal-tribble/internal/club"
	"github.com/mauv0809/ideal-tribble/internal/config"
	"github.com/mauv0809/ideal-tribble/internal/database"
	server "github.com/mauv0809/ideal-tribble/internal/http"
	"github.com/mauv0809/ideal-tribble/internal/metrics"
	"github.com/mauv0809/ideal-tribble/internal/playtomic"
	"github.com/mauv0809/ideal-tribble/internal/processor"
	"github.com/mauv0809/ideal-tribble/internal/slack"
)

func main() {
	log.SetFormatter(log.JSONFormatter)
	cfg := config.Load()
	db, err := database.InitDB(cfg.DBName, cfg.Turso.PrimaryURL, cfg.Turso.AuthToken)
	if err != nil {
		log.Fatalf("Failed to initialize database: %s", err)
	}
	defer func() {
		log.Info("Closing database connection")
		db.Close()
	}()

	clubStore := club.New(db)
	metricsStore := metrics.New(db)

	playtomicClient := playtomic.NewClient()
	slackClient := slack.NewClient(cfg.SlackBotToken, cfg.SlackChannelID)
	processor := processor.New(clubStore, slackClient, metricsStore)

	s := server.NewServer(
		clubStore,
		metricsStore,
		cfg,
		playtomicClient,
		slackClient,
		processor,
	)

	log.Info("Server started", "port", cfg.Port)
	if err := http.ListenAndServe(":"+cfg.Port, s); err != nil {
		log.Fatal("Could not start server", "error", err)
	}
	log.Info("Server stopped")
}
