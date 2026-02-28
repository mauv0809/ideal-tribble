package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/charmbracelet/log"
	"github.com/mauv0809/ideal-tribble/internal/club"
	"github.com/mauv0809/ideal-tribble/internal/notifier"
	"github.com/mauv0809/ideal-tribble/internal/telemetry"
)

// HealthChecker defines a component that can report its health status.
type HealthChecker interface {
	Check(ctx context.Context) error
}

// HealthCheckResult represents the result of a single health check.
type HealthCheckResult struct {
	Name      string `json:"name"`
	Status    string `json:"status"` // "healthy" or "unhealthy"
	LatencyMs int64  `json:"latency_ms"`
	Error     string `json:"error,omitempty"`
}

// HealthResponse represents the overall health check response.
type HealthResponse struct {
	Status    string              `json:"status"` // "healthy" or "unhealthy"
	Timestamp string              `json:"timestamp"`
	Checks    []HealthCheckResult `json:"checks"`
}

// DatabaseHealthChecker wraps a ClubStore for health checking.
type DatabaseHealthChecker struct {
	store club.ClubStore
}

func (d *DatabaseHealthChecker) Check(ctx context.Context) error {
	return d.store.Ping()
}

// SlackHealthChecker wraps a Notifier for health checking.
type SlackHealthChecker struct {
	notifier notifier.Notifier
}

func (s *SlackHealthChecker) Check(ctx context.Context) error {
	return s.notifier.Ping()
}

// runCheck executes a health check and returns the result.
func runCheck(ctx context.Context, name string, checker HealthChecker) HealthCheckResult {
	start := time.Now()
	err := checker.Check(ctx)
	latency := time.Since(start).Milliseconds()

	result := HealthCheckResult{
		Name:      name,
		LatencyMs: latency,
	}

	if err != nil {
		result.Status = "unhealthy"
		result.Error = err.Error()
	} else {
		result.Status = "healthy"
	}

	return result
}

// DeepHealthCheckHandler performs health checks on all dependencies.
func DeepHealthCheckHandler(store club.ClubStore, n notifier.Notifier) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		logger := telemetry.LogWithTrace(ctx)
		logger.Info("Deep health check request received")

		checkers := map[string]HealthChecker{
			"database": &DatabaseHealthChecker{store: store},
			"slack":    &SlackHealthChecker{notifier: n},
		}

		response := HealthResponse{
			Status:    "healthy",
			Timestamp: time.Now().UTC().Format(time.RFC3339),
			Checks:    make([]HealthCheckResult, 0, len(checkers)),
		}

		for name, checker := range checkers {
			result := runCheck(ctx, name, checker)
			response.Checks = append(response.Checks, result)

			if result.Status == "unhealthy" {
				response.Status = "unhealthy"
				logger.Warn("Health check failed", "check", name, "error", result.Error, "latency_ms", result.LatencyMs)
			} else {
				logger.Debug("Health check passed", "check", name, "latency_ms", result.LatencyMs)
			}
		}

		w.Header().Set("Content-Type", "application/json")
		if response.Status == "unhealthy" {
			w.WriteHeader(http.StatusServiceUnavailable)
		} else {
			w.WriteHeader(http.StatusOK)
		}

		if err := json.NewEncoder(w).Encode(response); err != nil {
			log.Error("Failed to encode health response", "error", err)
		}
	}
}

// HealthCheckHandler provides a simple liveness check (no dependencies).
func HealthCheckHandler(store club.ClubStore) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		logger := telemetry.LogWithTrace(r.Context())
		logger.Info("Health check request received with trace correlation")

		log.Debug("Received health check request")
		w.WriteHeader(http.StatusOK)
		fmt.Fprintf(w, "OK!")
	}
}

func ClearStoreHandler(store club.ClubStore) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		matchID := r.URL.Query().Get("matchID")
		if matchID != "" {
			log.Info("Received request to clear a specific match", "matchID", matchID)
			store.ClearMatch(matchID)
			w.WriteHeader(http.StatusOK)
			fmt.Fprintf(w, "Cleared match %s from store!", matchID)
			log.Info("Successfully cleared match from store", "matchID", matchID)
		} else {
			log.Info("Received request to clear entire store")
			store.Clear()
			w.WriteHeader(http.StatusOK)
			fmt.Fprint(w, "Store cleared!")
			log.Info("Store cleared successfully")
		}
	}
}
