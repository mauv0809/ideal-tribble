package metrics

import "github.com/prometheus/client_golang/prometheus"

// Service holds all the Prometheus metrics for the application.
// By defining them all in one place, we ensure consistency in naming and labeling.
type Service struct {
	FetcherRuns        prometheus.Counter
	MatchesProcessed   prometheus.Counter
	ProcessingDuration prometheus.Histogram
	SlackNotifSent     prometheus.Counter
	SlackNotifFailed   prometheus.Counter
	StartupTimeSeconds prometheus.Gauge
}
