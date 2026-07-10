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

	// Get web config (optional - will use defaults if not set)
	sessionSecret := os.Getenv("WEB_SESSION_SECRET")
	if sessionSecret == "" {
		sessionSecret = "change-me-in-production-32chars!" // Default for dev
		log.Warn("WEB_SESSION_SECRET not set, using insecure default")
	}
	totpKey := os.Getenv("WEB_TOTP_ENCRYPTION_KEY")
	if totpKey == "" {
		totpKey = "change-me-32-chars-for-aes-256!" // Default for dev
		log.Warn("WEB_TOTP_ENCRYPTION_KEY not set, using insecure default")
	}

	cfg := Config{
		DBName:        getEnv("DB_NAME"),
		MigrationsDir: "./migrations",
		TenantID:      getEnv("TENANT_ID"),
		Port:          getEnv("PORT"),
		Turso: TursoConfig{
			PrimaryURL: getEnv("TURSO_PRIMARY_URL"),
			AuthToken:  getEnv("TURSO_AUTH_TOKEN"),
		},
		Web: WebConfig{
			SessionSecret:     sessionSecret,
			TOTPEncryptionKey: totpKey,
		},
	}
	return cfg
}
