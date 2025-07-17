package processor

import (
	"github.com/mauv0809/ideal-tribble/internal/club"
	"github.com/mauv0809/ideal-tribble/internal/notifier"
	"github.com/mauv0809/ideal-tribble/internal/playtomic"
)

// Store defines the database operations required by the processor.
type Store interface {
	GetMatchesForProcessing() ([]*playtomic.PadelMatch, error)
	UpdateProcessingStatus(matchID string, status playtomic.ProcessingStatus) error
	UpsertPlayers(players []club.PlayerInfo) error
	AssignBallBringerAtomically(matchID string, playerIDs []string) (string, string, error)
	UpdateNotificationTimestamp(matchID string, notificationType string, messageId string) error
	UpdatePlayerStats(match *playtomic.PadelMatch)
}

// Notifier defines the notification operations required by the processor.
// This is now an alias for the main notifier interface for decoupling.
type Notifier interface {
	notifier.Notifier
}
