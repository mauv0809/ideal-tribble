package club

import (
	"database/sql"
	"sync"
	"time"

	"github.com/mauv0809/ideal-tribble/internal/playtomic"
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
	ID                       string
	Name                     string
	BallBringerCountSingles  int `json:"ball_bringer_count_singles"`
	BallBringerCountDoubles  int `json:"ball_bringer_count_doubles"`
	LastBallBoyDateSingles   *int64 `json:"last_ball_boy_date_singles,omitempty"`
	LastBallBoyDateDoubles   *int64 `json:"last_ball_boy_date_doubles,omitempty"`
	BookingCountSingles      int `json:"booking_count_singles"`
	BookingCountDoubles      int `json:"booking_count_doubles"`
	Level                    float64
	SlackUserID              *string
	SlackUsername            *string
	SlackDisplayName         *string
	MappingStatus            *string
	MappingConfidence        *float64
	MappingUpdatedAt         *int64
}

// PlayerAlias links a manual player to a Playtomic player
type PlayerAlias struct {
	ID                  int64    `json:"id"`
	ManualPlayerID      string   `json:"manual_player_id"`
	ManualPlayerName    string   `json:"manual_player_name"`
	PlaytomicPlayerID   *string  `json:"playtomic_player_id,omitempty"`
	PlaytomicPlayerName *string  `json:"playtomic_player_name,omitempty"`
	Confirmed           bool     `json:"confirmed"`
	Confidence          *float64 `json:"confidence,omitempty"`
	CreatedAt           int64    `json:"created_at"`
	UpdatedAt           int64    `json:"updated_at"`
}

// ManualMatchInput holds form data for manual match entry
type ManualMatchInput struct {
	MatchDate       time.Time
	VenueName       string
	MatchTypeEnum   playtomic.MatchTypeEnum
	CompetitionMode string
	Team1Players    []ManualPlayerInput
	Team2Players    []ManualPlayerInput
	Sets            []SetScoreInput
}

// ManualPlayerInput represents a player input from the form
type ManualPlayerInput struct {
	ID   string // Existing ID or empty for new player
	Name string
}

// SetScoreInput represents a single set score
type SetScoreInput struct {
	Team1Games int
	Team2Games int
}
