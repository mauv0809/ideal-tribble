package main

import (
	"net/http"

	"github.com/charmbracelet/log"
	"github.com/mauv0809/ideal-tribble/api"
	"github.com/mauv0809/ideal-tribble/club"
	"github.com/mauv0809/ideal-tribble/config"
	"github.com/mauv0809/ideal-tribble/database"
)

func main() {
	// Load configuration from environment variables
	cfg, initialPlayerIDs, storeFile := config.Load()

	db, err := database.InitDB(storeFile)
	if err != nil {
		log.Fatalf("Failed to initialize database: %s", err)
	}
	defer db.Close()

	clubStore := club.New(db)
	clubStore.AddInitialPlayers(initialPlayerIDs)

	server := api.NewServer(clubStore, cfg)

	// Set up the HTTP handler
	http.HandleFunc("/check", server.CheckAndNotifyHandler())
	http.HandleFunc("/health", server.HealthCheckHandler())
	http.HandleFunc("/clear", server.ClearStoreHandler())
	http.HandleFunc("/members", server.ListMembersHandler())
	http.HandleFunc("/matches", server.ListMatchesHandler())

	// Start the server
	log.Info("Starting server", "port", cfg.Port)
	if err := http.ListenAndServe(":"+cfg.Port, nil); err != nil {
		log.Fatalf("failed to start server: %s\n", err)
	}
}
