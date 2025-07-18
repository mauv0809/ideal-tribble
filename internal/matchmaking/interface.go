package matchmaking

// MatchmakingService handles match requests and availability collection
type MatchmakingService interface {
	// CreateMatchRequest creates a new match request and initiates availability collection
	CreateMatchRequest(requesterID, requesterName, channelID string) (*MatchRequest, error)
	
	// GetMatchRequest retrieves a match request by ID
	GetMatchRequest(requestID string) (*MatchRequest, error)
	
	// RecordPlayerAvailability records a player's availability for specific dates
	RecordPlayerAvailability(requestID, playerID, playerName string, availableDates []string) error
	
	// GetPlayerAvailability gets all availability responses for a match request
	GetPlayerAvailability(requestID string) ([]PlayerAvailability, error)
	
	// AnalyzeAvailability analyzes responses to find the best match dates
	AnalyzeAvailability(requestID string) ([]AvailabilityResult, error)
	
	// ProposeMatch proposes a match with team assignments and booking responsibility
	ProposeMatch(requestID, date, startTime, endTime string) (*MatchProposal, error)
	
	// ConfirmMatch confirms a proposed match
	ConfirmMatch(requestID string) error
	
	// CancelMatchRequest cancels a match request
	CancelMatchRequest(requestID string) error
	
	// UpdateMatchRequestStatus updates the status of a match request
	UpdateMatchRequestStatus(requestID string, status MatchRequestStatus) error
	
	// GetActiveMatchRequests gets all active match requests
	GetActiveMatchRequests() ([]MatchRequest, error)
	
	// IsActiveMatchRequestMessage checks if a message timestamp belongs to an active match request
	IsActiveMatchRequestMessage(messageTimestamp string) (string, bool, error)
	
	// AddPlayerAvailability adds a day to a player's availability
	AddPlayerAvailability(requestID, playerID, playerName, day string) error
	
	// RemovePlayerAvailability removes a day from a player's availability  
	RemovePlayerAvailability(requestID, playerID, day string) error
}

// Notifier defines the notification operations required by matchmaking.
// This keeps the matchmaking package decoupled from the main notifier interface.
type Notifier interface {
	// SendMatchAvailabilityRequest sends initial availability request message
	// Returns channel ID and timestamp for thread tracking
	SendMatchAvailabilityRequest(request *MatchRequest, dryRun bool) (string, string, error)
	
	// SendMatchProposal sends a reply with the proposed match in the thread
	SendMatchProposal(request *MatchRequest, proposal *MatchProposal, dryRun bool) error
	
	// SendMatchConfirmation sends a reply with confirmation in the thread
	SendMatchConfirmation(request *MatchRequest, dryRun bool) error
	
	// FormatMatchRequestResponse formats a response for the /match command
	FormatMatchRequestResponse(request *MatchRequest) (interface{}, error)
}