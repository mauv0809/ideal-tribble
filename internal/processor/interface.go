package processor

import (
	"github.com/mauv0809/ideal-tribble/internal/metrics"
	"github.com/mauv0809/ideal-tribble/internal/playtomic"
	"github.com/mauv0809/ideal-tribble/internal/slack"
)

// Store defines the database operations required by the processor.
type Store interface {
	GetMatchesForProcessing() ([]*playtomic.PadelMatch, error)
	UpdateProcessingStatus(matchID string, status playtomic.ProcessingStatus) error
	UpdatePlayerStats(match *playtomic.PadelMatch)
}

// Notifier defines the notification operations required by the processor.
type Notifier interface {
	SendNotification(match *playtomic.PadelMatch, notificationType slack.NotificationType, metrics metrics.MetricsStore, dryRun bool) (string, string, error)
}
