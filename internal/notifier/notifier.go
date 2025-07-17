package notifier

import (
	"github.com/mauv0809/ideal-tribble/internal/club"
	"github.com/mauv0809/ideal-tribble/internal/playtomic"
)

// Notifier defines a high-level interface for sending notifications about business events.
// This decouples the rest of the application from the specific notification provider (e.g., Slack).
type Notifier interface {
	// For upcoming matches
	SendBookingNotification(match *playtomic.PadelMatch, dryRun bool) (string, error)
	// For completed matches
	SendResultNotification(match *playtomic.PadelMatch, dryRun bool) (string, error)
	// For slash commands
	SendLeaderboard(stats []club.PlayerStats, dryRun bool) error
	SendLevelLeaderboard(players []club.PlayerInfo, dryRun bool) error
	SendPlayerStats(stats *club.PlayerStats, query string, dryRun bool) error
	SendPlayerNotFound(query string, dryRun bool) error

	// For formatting responses for slash commands
	FormatLeaderboardResponse(stats []club.PlayerStats) (any, error)
	FormatLevelLeaderboardResponse(players []club.PlayerInfo) (any, error)
	FormatPlayerStatsResponse(stats *club.PlayerStats, query string) (any, error)
	FormatPlayerNotFoundResponse(query string) (any, error)
}
