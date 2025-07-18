package playtomic

// SearchMatchesParams defines the parameters for searching for matches.
type SearchMatchesParams struct {
	SportID       string
	HasPlayers    bool
	Sort          string
	TenantIDs     []string
	FromStartDate string
}

// MatchSummary contains the essential details of a match from a search result.
type MatchSummary struct {
	MatchID string
	OwnerID *string
}

// MatchType represents the type of a padel match.
type MatchType string

const (
	// MatchTypeAll represents all match types combined.
	MatchTypeAll MatchType = "ALL"
	// MatchTypeSingles represents a singles match.
	MatchTypeSingles MatchType = "SINGLES"
	// MatchTypeDoubles represents a doubles match.
	MatchTypeDoubles MatchType = "DOUBLES"
)

// PadelMatch represents a single padel match with all its details.
type PadelMatch struct {
	MatchID           string
	OwnerID           string
	OwnerName         string
	Start             int64
	End               int64
	CreatedAt         int64
	Status            string
	GameStatus        GameStatus
	Teams             []Team
	Results           []SetResult
	ResultsStatus     ResultsStatus
	ResourceName      string
	AccessCode        string
	Price             string
	Tenant            Tenant
	BallBringerID     string
	BallBringerName   string
	BookingNotifiedTs *int64 // Unix timestamp when booking notification was sent
	ResultNotifiedTs  *int64 // Unix timestamp when result notification was sent
	CompetitionType   CompetitionType
	MatchType         MatchType
	ProcessingStatus  ProcessingStatus
}

// ProcessingStatus defines the internal processing state of a match.
type ProcessingStatus string

const (
	StatusNew                  ProcessingStatus = "NEW"
	StatusAssigningBallBringer ProcessingStatus = "ASSIGNING_BALL_BRINGER"
	StatusBallBoyAssigned      ProcessingStatus = "BALL_BOY_ASSIGNED"
	StatusBookingNotified      ProcessingStatus = "BOOKING_NOTIFIED"
	StatusResultAvailable      ProcessingStatus = "RESULT_AVAILABLE"
	StatusResultNotified       ProcessingStatus = "RESULT_NOTIFIED"
	StatusUpdatingPlayerStats  ProcessingStatus = "UPDATING_PLAYER_STATS"
	StatusPlayerStatsUpdated   ProcessingStatus = "PLAYER_STATS_UPDATED"
	StatusUpdatingWeeklyStats  ProcessingStatus = "UPDATING_WEEKLY_STATS"
	StatusStatsUpdated         ProcessingStatus = "STATS_UPDATED"
	StatusCompleted            ProcessingStatus = "COMPLETED"
)

// CompetitionType defines the type of match.
type CompetitionType string

const (
	Competition CompetitionType = "COMPETITIVE"
	Practice    CompetitionType = "FRIENDLY"
)

// GameStatus defines the status of a game.
type GameStatus string

const (
	GameStatusPending    GameStatus = "PENDING"
	GameStatusPlayed     GameStatus = "PLAYED"
	GameStatusUnknown    GameStatus = "UNKNOWN"
	GameStatusCanceled   GameStatus = "CANCELED"
	GameStatusWaitingFor GameStatus = "WAITING_FOR"
	GameStatusExpired    GameStatus = "EXPIRED"
	GameStatusInProgress GameStatus = "IN_PROGRESS"
)

// ResultsStatus defines the status of the match results.
type ResultsStatus string

const (
	ResultsStatusPending    ResultsStatus = "PENDING"
	ResultsStatusConfirmed  ResultsStatus = "CONFIRMED"
	ResultsStatusInvalid    ResultsStatus = "INVALID"
	ResultsStatusNotAllowed ResultsStatus = "NOT_ALLOWED"
	ResultsStatusExpired    ResultsStatus = "EXPIRED"
	ResultsStatusCanceled   ResultsStatus = "CANCELED"
	ResultsStatusWaitingFor ResultsStatus = "WAITING_FOR"
	ResultsStatusValidating ResultsStatus = "VALIDATING"
)

// Team represents a team in a match.
type Team struct {
	ID         string
	Players    []Player
	TeamResult string
}

// Player represents a player in a match.
type Player struct {
	UserID string
	Name   string
	Level  float64
	Paid   bool
}

// SetResult represents the result of a single set.
type SetResult struct {
	Name   string
	Scores map[string]int
}

// Tenant represents a Playtomic tenant (club).
type Tenant struct {
	ID   string
	Name string
}

// playtomicMatchResponse defines the structure for the JSON response from the Playtomic API for a single match.
type playtomicMatchResponse struct {
	OwnerID            string                       `json:"owner_id"`
	StartDate          string                       `json:"start_date"`
	EndDate            string                       `json:"end_date"`
	CreatedAt          string                       `json:"created_at"`
	Status             string                       `json:"status"`
	GameStatus         string                       `json:"game_status"`
	Teams              []playtomicTeamResponse      `json:"teams"`
	Results            []playtomicResult            `json:"results"`
	ResultsStatus      string                       `json:"results_status"`
	RegistrationInfo   playtomicRegistrationInfo    `json:"registration_info"`
	ResourceName       string                       `json:"resource_name"`
	MerchantAccessCode *playtomicMerchantAccessCode `json:"merchant_access_code"`
	Price              string                       `json:"price"`
	Tenant             playtomicTenant              `json:"tenant"`
	CompetitionType    string                       `json:"competition_mode"`
}

// playtomicResult defines a set result.
type playtomicResult struct {
	Name   string               `json:"name"`
	Scores []playtomicTeamScore `json:"scores"`
}

// playtomicTeamScore defines the score for a team in a set.
type playtomicTeamScore struct {
	TeamID string `json:"team_id"`
	Score  int    `json:"score"`
}

// playtomicRegistrationInfo defines the registration details.
type playtomicRegistrationInfo struct {
	Registrations []playtomicRegistration `json:"registrations"`
}

// playtomicRegistration defines a single player registration.
type playtomicRegistration struct {
	UserID  string `json:"user_id"`
	Payable bool   `json:"payable"`
}

// playtomicTenant defines the structure for the tenant information in the response.
type playtomicTenant struct {
	ID   string `json:"tenant_id"`
	Name string `json:"tenant_name"`
}

// playtomicMerchantAccessCode defines the structure for the merchant access code.
type playtomicMerchantAccessCode struct {
	Code string `json:"code"`
}

// playtomicTeamResponse defines the structure for a team within the match response.
type playtomicTeamResponse struct {
	TeamID     string                    `json:"team_id"`
	Players    []playtomicPlayerResponse `json:"players"`
	TeamResult *string                   `json:"team_result"`
}

// playtomicPlayerResponse defines the structure for a player within a team.
type playtomicPlayerResponse struct {
	UserID     string   `json:"user_id"`
	Name       string   `json:"name"`
	LevelValue *float64 `json:"level_value"`
}
