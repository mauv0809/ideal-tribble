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
	UpdatePlayerStats(match *playtomic.PadelMatch)
	GetPlayers(playerIDs []string) ([]club.PlayerInfo, error)
	SetBallBringer(matchID, playerID, playerName string) error
	AddPlayer(playerID, name string, level float64)
	UpsertPlayers(players []club.PlayerInfo) error
}

// Notifier defines the notification operations required by the processor.
// This is now an alias for the main notifier interface for decoupling.
type Notifier interface {
	notifier.Notifier
}
