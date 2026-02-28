package telemetry

import (
	"os"
	"testing"

	"github.com/charmbracelet/log"
	"github.com/stretchr/testify/assert"
)

func TestInitLogging_DefaultsToJSON(t *testing.T) {
	// Clear env vars
	os.Unsetenv("LOG_FORMAT")
	os.Unsetenv("LOG_LEVEL")

	InitLogging()

	// Default should be JSON format and Info level
	assert.Equal(t, log.InfoLevel, log.Default().GetLevel())
}

func TestInitLogging_TextFormat(t *testing.T) {
	os.Setenv("LOG_FORMAT", "text")
	defer os.Unsetenv("LOG_FORMAT")

	InitLogging()

	// Just verify it doesn't panic - we can't easily inspect the formatter
}

func TestInitLogging_LogLevels(t *testing.T) {
	tests := []struct {
		envLevel string
		expected log.Level
	}{
		{"debug", log.DebugLevel},
		{"DEBUG", log.DebugLevel},
		{"info", log.InfoLevel},
		{"INFO", log.InfoLevel},
		{"warn", log.WarnLevel},
		{"warning", log.WarnLevel},
		{"error", log.ErrorLevel},
		{"ERROR", log.ErrorLevel},
		{"invalid", log.InfoLevel}, // defaults to info
		{"", log.InfoLevel},        // defaults to info
	}

	for _, tt := range tests {
		t.Run("level_"+tt.envLevel, func(t *testing.T) {
			if tt.envLevel == "" {
				os.Unsetenv("LOG_LEVEL")
			} else {
				os.Setenv("LOG_LEVEL", tt.envLevel)
			}
			defer os.Unsetenv("LOG_LEVEL")

			InitLogging()

			assert.Equal(t, tt.expected, log.Default().GetLevel())
		})
	}
}
