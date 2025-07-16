package config

import (
	"os"

	"github.com/charmbracelet/log"
	"github.com/joho/godotenv"
)

// Load reads configuration from environment variables and .env file.
func Load() Config {
	err := godotenv.Load()
	if err != nil {
		log.Info("No .env file found, reading from environment variables")
	}

	// A helper function to get a required env var. It will fail if the env var is not set.
	getEnv := func(key string) string {
		if value, ok := os.LookupEnv(key); ok {
			return value
		}
		log.Fatalf("Error: Required environment variable %s is not set.", key)
		return "" // This line is never reached
	}

	cfg := Config{
		DBName:        getEnv("DB_NAME"),
		MigrationsDir: "./migrations",
		Slack: SlackConfig{
			Token:         getEnv("SLACK_BOT_TOKEN"),
			ChannelID:     getEnv("SLACK_CHANNEL_ID"),
			SigningSecret: getEnv("SLACK_SIGNING_SECRET"),
		},
		TenantID: getEnv("TENANT_ID"),
		Port:     getEnv("PORT"),
		Turso: TursoConfig{
			PrimaryURL: getEnv("TURSO_PRIMARY_URL"),
			AuthToken:  getEnv("TURSO_AUTH_TOKEN"),
		},
		/*Inngest: InngestConfig{
			AppID:      getEnv("INNGEST_APP_ID"),
			SingingKey: getEnv("INNGEST_SIGNING_KEY"),
			EventKey:   getEnv("INNGEST_EVENT_KEY"),
		},*/
		ProjectID: getEnv("GCP_PROJECT"),
	}
	return cfg
}
