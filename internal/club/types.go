package club

import (
	"database/sql"
	"sync"
)

// store handles all database operations for the club.
type store struct {
	db *sql.DB
	mu sync.RWMutex
}

// PlayerStats represents a player's statistics for the leaderboard.
type PlayerStats struct {
	PlayerID      string  `json:"player_id"`
	PlayerName    string  `json:"player_name"`
	MatchesPlayed int     `json:"matches_played"`
	MatchesWon    int     `json:"matches_won"`
	MatchesLost   int     `json:"matches_lost"`
	SetsWon       int     `json:"sets_won"`
	SetsLost      int     `json:"sets_lost"`
	GamesWon      int     `json:"games_won"`
	GamesLost     int     `json:"games_lost"`
	WinPercentage float64 `json:"win_percentage"`
}

// PlayerInfo represents a player in the store.
type PlayerInfo struct {
	ID               string
	Name             string
	BallBringerCount int
	Level            float64
}
