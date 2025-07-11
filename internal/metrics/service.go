package metrics

import (
	"net/http"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

var _ Metrics = (*Service)(nil)

// NewMetricsHandler returns an http.Handler for the given Gatherer.
// If no gatherer is provided, it uses the default one.
func NewMetricsHandler(gatherer ...prometheus.Gatherer) http.Handler {
	gath := prometheus.DefaultGatherer
	if len(gatherer) > 0 {
		gath = gatherer[0]
	}
	return promhttp.HandlerFor(gath, promhttp.HandlerOpts{})
}

// NewService creates and registers the Prometheus metrics.
// If no registerer is provided, it uses the default Prometheus registerer.
func NewService(registerer ...prometheus.Registerer) *Service {
	reg := prometheus.DefaultRegisterer
	if len(registerer) > 0 {
		reg = registerer[0]
	}

	s := &Service{
		FetcherRuns: prometheus.NewCounter(prometheus.CounterOpts{
			Name: "padel_fetcher_runs_total",
			Help: "The total number of times the match fetcher has run.",
		}),
		MatchesProcessed: prometheus.NewCounter(prometheus.CounterOpts{
			Name: "padel_matches_processed_total",
			Help: "The total number of matches processed by the state machine.",
		}),
		ProcessingDuration: prometheus.NewHistogram(prometheus.HistogramOpts{
			Name:    "padel_match_processing_duration_seconds",
			Help:    "The duration of individual match processing.",
			Buckets: []float64{0.005, 0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1, 2.5, 5, 10},
		}),
		SlackNotifSent: prometheus.NewCounter(prometheus.CounterOpts{
			Name: "padel_slack_notifications_sent_total",
			Help: "The total number of Slack notifications successfully sent.",
		}),
		SlackNotifFailed: prometheus.NewCounter(prometheus.CounterOpts{
			Name: "padel_slack_notifications_failed_total",
			Help: "The total number of Slack notifications that failed to send.",
		}),
		StartupTimeSeconds: prometheus.NewGauge(prometheus.GaugeOpts{
			Name: "padel_startup_duration_seconds",
			Help: "The duration of the application startup in seconds.",
		}),
	}

	reg.MustRegister(
		s.FetcherRuns,
		s.MatchesProcessed,
		s.ProcessingDuration,
		s.SlackNotifSent,
		s.SlackNotifFailed,
		s.StartupTimeSeconds,
	)

	return s
}

func (s *Service) IncFetcherRuns() {
	s.FetcherRuns.Inc()
}

func (s *Service) IncMatchesProcessed() {
	s.MatchesProcessed.Inc()
}

func (s *Service) ObserveProcessingDuration(duration float64) {
	s.ProcessingDuration.Observe(duration)
}

func (s *Service) IncSlackNotifSent() {
	s.SlackNotifSent.Inc()
}

func (s *Service) IncSlackNotifFailed() {
	s.SlackNotifFailed.Inc()
}

func (s *Service) SetStartupTime(duration float64) {
	s.StartupTimeSeconds.Set(duration)
}
