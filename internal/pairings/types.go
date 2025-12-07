package pairings

// TrackedPairing represents a player pair to track analytics for.
type TrackedPairing struct {
	ID          int64  `json:"id"`
	Player1ID   string `json:"player1_id"`
	Player1Name string `json:"player1_name"`
	Player2ID   string `json:"player2_id"`
	Player2Name string `json:"player2_name"`
	CreatedAt   int64  `json:"created_at"`
	Active      bool   `json:"active"`
}

// PairingMatch represents a match involving a tracked pairing.
type PairingMatch struct {
	ID            int64  `json:"id"`
	PairingID     int64  `json:"pairing_id"`
	MatchID       string `json:"match_id"`
	MatchDate     int64  `json:"match_date"`
	DayOfWeek     int    `json:"day_of_week"`  // 0=Sunday, 6=Saturday
	HourOfDay     int    `json:"hour_of_day"`  // 0-23
	Opponent1ID   string `json:"opponent1_id"`
	Opponent1Name string `json:"opponent1_name"`
	Opponent2ID   string `json:"opponent2_id"`
	Opponent2Name string `json:"opponent2_name"`
	PairingWon    bool   `json:"pairing_won"`
	SetsWon       int    `json:"sets_won"`
	SetsLost      int    `json:"sets_lost"`
	GamesWon      int    `json:"games_won"`
	GamesLost     int    `json:"games_lost"`
	TenantID      string `json:"tenant_id"`
	TenantName    string `json:"tenant_name"`
	// Per-set scores for situational analytics
	Set1PairingScore  *int `json:"set1_pairing_score,omitempty"`
	Set1OpponentScore *int `json:"set1_opponent_score,omitempty"`
	Set2PairingScore  *int `json:"set2_pairing_score,omitempty"`
	Set2OpponentScore *int `json:"set2_opponent_score,omitempty"`
	Set3PairingScore  *int `json:"set3_pairing_score,omitempty"`
	Set3OpponentScore *int `json:"set3_opponent_score,omitempty"`
}

// PairingStats represents overall statistics for a tracked pairing.
type PairingStats struct {
	PairingID     int64   `json:"pairing_id"`
	Player1Name   string  `json:"player1_name"`
	Player2Name   string  `json:"player2_name"`
	MatchesPlayed int     `json:"matches_played"`
	MatchesWon    int     `json:"matches_won"`
	MatchesLost   int     `json:"matches_lost"`
	WinPercentage float64 `json:"win_percentage"`
	SetsWon       int     `json:"sets_won"`
	SetsLost      int     `json:"sets_lost"`
	GamesWon      int     `json:"games_won"`
	GamesLost     int     `json:"games_lost"`
	CurrentStreak int     `json:"current_streak"` // Positive = wins, negative = losses
	LongestWin    int     `json:"longest_win_streak"`
	LongestLoss   int     `json:"longest_loss_streak"`
}

// OpponentStats represents win/loss record against a specific opponent pair.
type OpponentStats struct {
	Opponent1ID   string  `json:"opponent1_id"`
	Opponent1Name string  `json:"opponent1_name"`
	Opponent2ID   string  `json:"opponent2_id"`
	Opponent2Name string  `json:"opponent2_name"`
	MatchesPlayed int     `json:"matches_played"`
	MatchesWon    int     `json:"matches_won"`
	MatchesLost   int     `json:"matches_lost"`
	WinPercentage float64 `json:"win_percentage"`
	SetsWon       int     `json:"sets_won"`
	SetsLost      int     `json:"sets_lost"`
	GamesWon      int     `json:"games_won"`
	GamesLost     int     `json:"games_lost"`
}

// TimeStats represents performance patterns by time.
type TimeStats struct {
	ByDayOfWeek map[int]*DayStats  `json:"by_day_of_week"` // 0=Sunday, 6=Saturday
	ByHourRange map[string]*DayStats `json:"by_hour_range"`  // "morning", "afternoon", "evening"
}

// DayStats represents stats for a specific time period.
type DayStats struct {
	MatchesPlayed int     `json:"matches_played"`
	MatchesWon    int     `json:"matches_won"`
	MatchesLost   int     `json:"matches_lost"`
	WinPercentage float64 `json:"win_percentage"`
}

// VenueStats represents performance at a specific venue.
type VenueStats struct {
	TenantID      string  `json:"tenant_id"`
	TenantName    string  `json:"tenant_name"`
	MatchesPlayed int     `json:"matches_played"`
	MatchesWon    int     `json:"matches_won"`
	MatchesLost   int     `json:"matches_lost"`
	WinPercentage float64 `json:"win_percentage"`
}

// RecentForm represents recent match results.
type RecentForm struct {
	Last5Wins     int     `json:"last_5_wins"`
	Last5Losses   int     `json:"last_5_losses"`
	Last10Wins    int     `json:"last_10_wins"`
	Last10Losses  int     `json:"last_10_losses"`
	Last5WinPct   float64 `json:"last_5_win_pct"`
	Last10WinPct  float64 `json:"last_10_win_pct"`
}

// IndividualPlayerStats represents stats against a single opponent player (regardless of partner).
type IndividualPlayerStats struct {
	PlayerID      string  `json:"player_id"`
	PlayerName    string  `json:"player_name"`
	MatchesPlayed int     `json:"matches_played"`
	MatchesWon    int     `json:"matches_won"`
	MatchesLost   int     `json:"matches_lost"`
	WinPercentage float64 `json:"win_percentage"`
	SetsWon       int     `json:"sets_won"`
	SetsLost      int     `json:"sets_lost"`
	GamesWon      int     `json:"games_won"`
	GamesLost     int     `json:"games_lost"`
}

// PlayerPartnerStats represents stats against a player when paired with a specific partner.
type PlayerPartnerStats struct {
	PartnerID     string  `json:"partner_id"`
	PartnerName   string  `json:"partner_name"`
	MatchesPlayed int     `json:"matches_played"`
	MatchesWon    int     `json:"matches_won"`
	MatchesLost   int     `json:"matches_lost"`
	WinPercentage float64 `json:"win_percentage"`
}

// KnownPlayer represents a player we've seen in matches (for autocomplete).
type KnownPlayer struct {
	PlayerID   string `json:"player_id"`
	PlayerName string `json:"player_name"`
}

// PairingHighlight represents a pairing with summary stats for dashboard display.
type PairingHighlight struct {
	PairingID     int64   `json:"pairing_id"`
	PairingName   string  `json:"pairing_name"`
	MatchesPlayed int     `json:"matches_played"`
	MatchesWon    int     `json:"matches_won"`
	WinPercentage float64 `json:"win_percentage"`
}

// SituationalStats represents performance in different match situations.
type SituationalStats struct {
	// After winning first set
	WonFirstSetMatches    int     `json:"won_first_set_matches"`
	WonFirstSetWins       int     `json:"won_first_set_wins"`
	WonFirstSetWinPct     float64 `json:"won_first_set_win_pct"`

	// After losing first set (comeback ability)
	LostFirstSetMatches   int     `json:"lost_first_set_matches"`
	LostFirstSetWins      int     `json:"lost_first_set_wins"`
	LostFirstSetWinPct    float64 `json:"lost_first_set_win_pct"`

	// Three-set matches (decider performance)
	ThreeSetMatches       int     `json:"three_set_matches"`
	ThreeSetWins          int     `json:"three_set_wins"`
	ThreeSetWinPct        float64 `json:"three_set_win_pct"`

	// Dominant wins (2-0)
	TwoZeroWins           int     `json:"two_zero_wins"`
	TwoOneWins            int     `json:"two_one_wins"`

	// Loss breakdown
	ZeroTwoLosses         int     `json:"zero_two_losses"`
	OneTwoLosses          int     `json:"one_two_losses"`

	// Tiebreak sets (7-6 or 6-7)
	TiebreakSetsPlayed    int     `json:"tiebreak_sets_played"`
	TiebreakSetsWon       int     `json:"tiebreak_sets_won"`
	TiebreakWinPct        float64 `json:"tiebreak_win_pct"`
}
