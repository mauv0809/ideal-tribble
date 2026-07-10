package telemetry

import (
	"os"
	"strings"

	"github.com/charmbracelet/log"
)

// getEnvOrDefault returns the value of the environment variable or a default value.
func getEnvOrDefault(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

// InitLogging configures the charmbracelet/log logger based on environment variables.
//
// Environment variables:
//   - LOG_FORMAT: "json" (default) or "text" - controls output format
//   - LOG_LEVEL: "debug", "info" (default), "warn", or "error"
//
// JSON format is recommended for production so log lines are machine-readable
// and easy to filter (e.g. via `kamal app logs`).
func InitLogging() {
	// Configure log format
	format := strings.ToLower(getEnvOrDefault("LOG_FORMAT", "json"))
	switch format {
	case "text":
		log.SetFormatter(log.TextFormatter)
	default:
		log.SetFormatter(log.JSONFormatter)
	}

	// Configure log level
	level := strings.ToLower(getEnvOrDefault("LOG_LEVEL", "info"))
	switch level {
	case "debug":
		log.SetLevel(log.DebugLevel)
	case "warn", "warning":
		log.SetLevel(log.WarnLevel)
	case "error":
		log.SetLevel(log.ErrorLevel)
	default:
		log.SetLevel(log.InfoLevel)
	}

	// Log the configuration (only visible if level allows)
	log.Debug("Logging initialized", "format", format, "level", level)
}
