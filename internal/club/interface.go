package club

import "github.com/mauv0809/ideal-tribble/internal/playtomic"

// ClubStore defines the interface for interacting with the club's data.
type ClubStore interface {
	UpsertMatch(match *playtomic.PadelMatch) error
	UpsertMatches(matches []*playtomic.PadelMatch) error
	UpdateProcessingStatus(matchID string, status playtomic.ProcessingStatus) error
	GetMatchesForProcessing() ([]*playtomic.PadelMatch, error)
	GetPlayerStats() ([]PlayerStats, error)
	UpdatePlayerStats(match *playtomic.PadelMatch)
	AddPlayer(playerID, name string, level float64)
	UpsertPlayers(players []PlayerInfo) error
	IsKnownPlayer(playerID string) bool
	Clear()
	ClearMatch(matchID string)
	GetAllPlayers() ([]PlayerInfo, error)
	GetPlayersSortedByLevel() ([]PlayerInfo, error)
	GetAllMatches() ([]*playtomic.PadelMatch, error)
	GetPlayerStatsByName(playerName string) (*PlayerStats, error)
	GetPlayers(playerIDs []string) ([]PlayerInfo, error)
	SetBallBringer(matchID, playerID, playerName string) error // Deprecated: Use AssignBallBringerAtomically instead
	AssignBallBringerAtomically(matchID string, playerIDs []string) (string, string, error)
	UpdateNotificationTimestamp(matchID string, notificationType string) error
}
