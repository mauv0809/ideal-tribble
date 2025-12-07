package metrics

import "net/http"

// Metrics defines the interface for collecting application metrics.
// This decouples the application from the specific metrics implementation (e.g., Prometheus).
type Metrics interface {
	IncFetcherRuns()
	IncMatchesProcessed()
	ObserveProcessingDuration(duration float64)
	IncSlackNotifSent()
	IncSlackNotifFailed()
	SetStartupTime(duration float64)

	// HTTP metrics
	RecordHTTPRequest(method, path string, statusCode int, duration float64)
	HTTPMiddleware(next http.Handler) http.Handler
}
