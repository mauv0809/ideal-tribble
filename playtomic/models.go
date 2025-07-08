package playtomic

// GameStatus defines the set of possible statuses for a match's game state.
type GameStatus string

const (
	// GameStatusPending means the match has not yet been played.
	GameStatusPending GameStatus = "PENDING"
	// GameStatusPlayed means the match has been completed.
	GameStatusPlayed GameStatus = "PLAYED"
	// GameStatusUnknown is for any status not explicitly handled.
	GameStatusUnknown GameStatus = "UNKNOWN"
)

type ResultsStatus string

const (
	ResultsStatusPending    ResultsStatus = "PENDING"
	ResultsStatusConfirmed  ResultsStatus = "CONFIRMED"
	ResultsStatusInvalid    ResultsStatus = "INVALID"
	ResultsStatusNotAllowed ResultsStatus = "NOT_ALLOWED"
)

// Player defines the structure for a player in a match.
type Player struct {
	UserID    string   `json:"user_id"`
	Name      string   `json:"name"`
	Level     *float64 `json:"level"`
	Paid      bool     `json:"paid"`
	CreatedAt int64    `json:"created_at"`
}

// Team defines the structure for a team in a match.
type Team struct {
	ID         string   `json:"id"`
	Players    []Player `json:"players"`
	TeamResult string   `json:"team_result"`
}

// SetResult holds the scores for each set in a match.
type SetResult struct {
	Name   string         `json:"name"`
	Scores map[string]int `json:"scores"`
}

// Tenant holds information about the facility.
type Tenant struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

// PadelMatch defines the structure for a Padel match.
type PadelMatch struct {
	MatchID         string        `json:"match_id"`
	OwnerID         string        `json:"owner_id"`
	OwnerName       string        `json:"owner_name"`
	Start           int64         `json:"start"`
	End             int64         `json:"end"`
	CreatedAt       int64         `json:"created_at"`
	Teams           []Team        `json:"teams"`
	Status          string        `json:"status"`
	GameStatus      GameStatus    `json:"game_status"`
	Results         []SetResult   `json:"results"`
	ResultsStatus   ResultsStatus `json:"results_status"`
	ResourceName    string        `json:"resource_name"`
	AccessCode      string        `json:"access_code,omitempty"`
	Price           string        `json:"price,omitempty"`
	Tenant          Tenant        `json:"tenant"`
	BallBringerID   string        `json:"ball_bringer_id,omitempty"`
	BallBringerName string        `json:"ball_bringer_name,omitempty"`
}
