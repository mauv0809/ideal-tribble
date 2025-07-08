package main

import (
	"net/http"
	"os"
	"strings"

	"github.com/charmbracelet/log"
	"github.com/mauv0809/ideal-tribble/database"
)

// Config stores the application configuration.
// It's populated from environment variables.
type Config struct {
	PlaytomicUser  string
	PlaytomicPass  string
	SlackBotToken  string
	SlackChannelID string
	BookingFilter  string // A string to identify the specific booking
	TenantID       string
	Port           string
	StoreFile      string
}

var store *ClubStore

func main() {
	// Load configuration from environment variables
	cfg, initialPlayerIDs := loadConfig()

	db, err := database.InitDB(cfg.StoreFile)
	if err != nil {
		log.Fatalf("Failed to initialize database: %s", err)
	}
	defer db.Close()

	store = NewClubStore(db)
	store.AddInitialPlayers(initialPlayerIDs)

	// Set up the HTTP handler
	http.HandleFunc("/check", checkAndNotifyHandler(cfg))
	http.HandleFunc("/health", healthCheckHandler())
	http.HandleFunc("/clear", clearStoreHandler())
	http.HandleFunc("/members", listMembersHandler())
	http.HandleFunc("/matches", listMatchesHandler())
	// Start the server
	log.Printf("Starting server on port %s", cfg.Port)
	if err := http.ListenAndServe(":"+cfg.Port, nil); err != nil {
		log.Fatalf("failed to start server: %s\n", err)
	}
}

// loadConfig loads configuration from environment variables.
func loadConfig() (Config, []string) {
	// A helper function to get an env var or return a default
	getEnv := func(key, fallback string) string {
		if value, ok := os.LookupEnv(key); ok {
			return value
		}
		if fallback == "" {
			log.Fatalf("Error: Environment variable %s is not set.", key)
		}
		return fallback
	}

	playerIDsStr := getEnv("PLAYER_IDS", "9759891")
	playerIDs := strings.Split(playerIDsStr, ",")

	cfg := Config{
		SlackBotToken:  getEnv("SLACK_BOT_TOKEN", "EMPTY"),
		SlackChannelID: getEnv("SLACK_CHANNEL_ID", "EMPTY"),
		BookingFilter:  getEnv("BOOKING_FILTER", "PADEL"),
		TenantID:       getEnv("TENANT_ID", "b8fe7430-f819-4413-b402-a008f94fc2b5"),
		Port:           getEnv("PORT", "8080"),
		StoreFile:      getEnv("STORE_FILE", "club.db"),
	}
	return cfg, playerIDs
}
