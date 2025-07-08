package config

import (
	"os"
	"strings"

	"github.com/charmbracelet/log"
	"github.com/joho/godotenv"
)

// Config stores the application configuration.
// It's populated from environment variables.
type Config struct {
	SlackBotToken  string
	SlackChannelID string
	BookingFilter  string // A string to identify the specific booking
	TenantID       string
	Port           string
}

// Load loads configuration from environment variables.
// It returns the main config, a slice of initial player IDs, and the path to the store file.
func Load() (Config, []string, string) {
	// For local development, load variables from .env file.
	// This will be ignored in a cloud environment where the file doesn't exist.
	_ = godotenv.Load()

	// A helper function to get a required env var. It will fail if the env var is not set.
	getEnv := func(key string) string {
		if value, ok := os.LookupEnv(key); ok {
			return value
		}
		log.Fatalf("Error: Required environment variable %s is not set.", key)
		return "" // This line is never reached
	}

	playerIDsStr := getEnv("PLAYER_IDS")
	playerIDs := strings.Split(playerIDsStr, ",")
	storeFile := getEnv("STORE_FILE")

	cfg := Config{
		SlackBotToken:  getEnv("SLACK_BOT_TOKEN"),
		SlackChannelID: getEnv("SLACK_CHANNEL_ID"),
		BookingFilter:  getEnv("BOOKING_FILTER"),
		TenantID:       getEnv("TENANT_ID"),
		Port:           getEnv("PORT"),
	}
	return cfg, playerIDs, storeFile
}
