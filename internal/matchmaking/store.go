package matchmaking

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/charmbracelet/log"
	"github.com/google/uuid"
)

// store handles database operations for matchmaking
type store struct {
	db *sql.DB
	mu sync.RWMutex
}

// NewStore creates a new matchmaking store
func NewStore(db *sql.DB) MatchmakingService {
	return &store{
		db: db,
	}
}

// CreateMatchRequest creates a new match request
func (s *store) CreateMatchRequest(requesterID, requesterName, channelID string) (*MatchRequest, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	now := time.Now()
	request := &MatchRequest{
		ID:            uuid.New().String(),
		RequesterID:   requesterID,
		RequesterName: requesterName,
		CreatedAt:     now,
		UpdatedAt:     now,
		Status:        StatusCollectingAvailability,
		ChannelID:     channelID,
	}

	query := `
		INSERT INTO match_requests (
			id, requester_id, requester_name, created_at, updated_at, status, channel_id
		) VALUES (?, ?, ?, ?, ?, ?, ?)
	`
	
	_, err := s.db.Exec(query, 
		request.ID, 
		request.RequesterID, 
		request.RequesterName, 
		request.CreatedAt.Unix(), 
		request.UpdatedAt.Unix(), 
		string(request.Status), 
		request.ChannelID,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create match request: %w", err)
	}

	log.Info("Created match request", "id", request.ID, "requester", requesterName)
	return request, nil
}

// GetMatchRequest retrieves a match request by ID
func (s *store) GetMatchRequest(requestID string) (*MatchRequest, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	query := `
		SELECT id, requester_id, requester_name, created_at, updated_at, status, channel_id,
			   thread_ts, availability_message_ts, proposed_date, proposed_start_time, proposed_end_time,
			   booking_responsible_id, booking_responsible_name, team_assignments_blob
		FROM match_requests
		WHERE id = ?
	`
	
	row := s.db.QueryRow(query, requestID)
	
	var request MatchRequest
	var createdAt, updatedAt int64
	var status string
	var teamAssignmentsBlob []byte
	
	err := row.Scan(
		&request.ID,
		&request.RequesterID,
		&request.RequesterName,
		&createdAt,
		&updatedAt,
		&status,
		&request.ChannelID,
		&request.ThreadTS,
		&request.AvailabilityMessageTS,
		&request.ProposedDate,
		&request.ProposedStartTime,
		&request.ProposedEndTime,
		&request.BookingResponsibleID,
		&request.BookingResponsibleName,
		&teamAssignmentsBlob,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("match request not found: %s", requestID)
		}
		return nil, fmt.Errorf("failed to get match request: %w", err)
	}

	request.CreatedAt = time.Unix(createdAt, 0)
	request.UpdatedAt = time.Unix(updatedAt, 0)
	request.Status = MatchRequestStatus(status)
	
	if teamAssignmentsBlob != nil {
		var teamAssignments TeamAssignments
		if err := json.Unmarshal(teamAssignmentsBlob, &teamAssignments); err != nil {
			log.Warn("Failed to unmarshal team assignments", "error", err)
		} else {
			request.TeamAssignments = &teamAssignments
		}
	}

	return &request, nil
}

// RecordPlayerAvailability records a player's availability for specific dates
func (s *store) RecordPlayerAvailability(requestID, playerID, playerName string, availableDates []string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	tx, err := s.db.Begin()
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback()

	// Delete existing availability for this player and request
	deleteQuery := `DELETE FROM match_request_availability WHERE match_request_id = ? AND player_id = ?`
	_, err = tx.Exec(deleteQuery, requestID, playerID)
	if err != nil {
		return fmt.Errorf("failed to delete existing availability: %w", err)
	}

	// Insert new availability records
	insertQuery := `
		INSERT INTO match_request_availability (match_request_id, player_id, player_name, available_date, responded_at)
		VALUES (?, ?, ?, ?, ?)
	`
	
	now := time.Now()
	for _, date := range availableDates {
		_, err = tx.Exec(insertQuery, requestID, playerID, playerName, date, now.Unix())
		if err != nil {
			return fmt.Errorf("failed to insert availability for date %s: %w", date, err)
		}
	}

	if err = tx.Commit(); err != nil {
		return fmt.Errorf("failed to commit availability transaction: %w", err)
	}

	log.Info("Recorded player availability", "request_id", requestID, "player", playerName, "dates", availableDates)
	return nil
}

// GetPlayerAvailability gets all availability responses for a match request
func (s *store) GetPlayerAvailability(requestID string) ([]PlayerAvailability, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	query := `
		SELECT id, match_request_id, player_id, player_name, available_date, responded_at
		FROM match_request_availability
		WHERE match_request_id = ?
		ORDER BY responded_at ASC
	`
	
	rows, err := s.db.Query(query, requestID)
	if err != nil {
		return nil, fmt.Errorf("failed to query availability: %w", err)
	}
	defer rows.Close()

	var availabilities []PlayerAvailability
	for rows.Next() {
		var availability PlayerAvailability
		var respondedAt int64
		
		err := rows.Scan(
			&availability.ID,
			&availability.MatchRequestID,
			&availability.PlayerID,
			&availability.PlayerName,
			&availability.AvailableDate,
			&respondedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan availability row: %w", err)
		}
		
		availability.RespondedAt = time.Unix(respondedAt, 0)
		availabilities = append(availabilities, availability)
	}

	return availabilities, nil
}

// AnalyzeAvailability analyzes responses to find the best match dates
func (s *store) AnalyzeAvailability(requestID string) ([]AvailabilityResult, error) {
	availabilities, err := s.GetPlayerAvailability(requestID)
	if err != nil {
		return nil, fmt.Errorf("failed to get availabilities: %w", err)
	}

	// Group by date
	dateGroups := make(map[string][]Player)
	for _, availability := range availabilities {
		player := Player{
			ID:   availability.PlayerID,
			Name: availability.PlayerName,
		}
		dateGroups[availability.AvailableDate] = append(dateGroups[availability.AvailableDate], player)
	}

	// Convert to results and sort by player count (descending)
	var results []AvailabilityResult
	for date, players := range dateGroups {
		results = append(results, AvailabilityResult{
			Date:             date,
			AvailablePlayers: players,
			PlayerCount:      len(players),
		})
	}

	// Sort by player count (descending), then by date (ascending)
	for i := 0; i < len(results)-1; i++ {
		for j := i + 1; j < len(results); j++ {
			if results[i].PlayerCount < results[j].PlayerCount ||
				(results[i].PlayerCount == results[j].PlayerCount && results[i].Date > results[j].Date) {
				results[i], results[j] = results[j], results[i]
			}
		}
	}

	return results, nil
}

// ProposeMatch proposes a match with team assignments and booking responsibility
func (s *store) ProposeMatch(requestID, date, startTime, endTime string) (*MatchProposal, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Get available players for the date
	availabilities, err := s.GetPlayerAvailability(requestID)
	if err != nil {
		return nil, fmt.Errorf("failed to get availabilities: %w", err)
	}

	var availablePlayers []Player
	for _, availability := range availabilities {
		if availability.AvailableDate == date {
			availablePlayers = append(availablePlayers, Player{
				ID:   availability.PlayerID,
				Name: availability.PlayerName,
			})
		}
	}

	if len(availablePlayers) < 4 {
		return nil, fmt.Errorf("not enough players available for date %s: %d", date, len(availablePlayers))
	}

	// Create team assignments (simple alternating assignment)
	teamAssignments := TeamAssignments{
		Team1: []Player{},
		Team2: []Player{},
	}

	for i, player := range availablePlayers[:4] { // Take first 4 players
		if i%2 == 0 {
			teamAssignments.Team1 = append(teamAssignments.Team1, player)
		} else {
			teamAssignments.Team2 = append(teamAssignments.Team2, player)
		}
	}

	// Assign booking responsibility to first player
	bookingResponsible := availablePlayers[0]

	// Update match request with proposal
	teamAssignmentsBlob, err := json.Marshal(teamAssignments)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal team assignments: %w", err)
	}

	updateQuery := `
		UPDATE match_requests 
		SET proposed_date = ?, proposed_start_time = ?, proposed_end_time = ?,
			booking_responsible_id = ?, booking_responsible_name = ?,
			team_assignments_blob = ?, status = ?, updated_at = ?
		WHERE id = ?
	`
	
	now := time.Now()
	_, err = s.db.Exec(updateQuery, 
		date, startTime, endTime,
		bookingResponsible.ID, bookingResponsible.Name,
		teamAssignmentsBlob, string(StatusProposingMatch), now.Unix(),
		requestID,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to update match request with proposal: %w", err)
	}

	proposal := &MatchProposal{
		Date:                   date,
		StartTime:              startTime,
		EndTime:                endTime,
		AvailablePlayers:       availablePlayers,
		TeamAssignments:        teamAssignments,
		BookingResponsibleID:   bookingResponsible.ID,
		BookingResponsibleName: bookingResponsible.Name,
	}

	log.Info("Proposed match", "request_id", requestID, "date", date, "players", len(availablePlayers))
	return proposal, nil
}

// ConfirmMatch confirms a proposed match
func (s *store) ConfirmMatch(requestID string) error {
	return s.UpdateMatchRequestStatus(requestID, StatusConfirmed)
}

// CancelMatchRequest cancels a match request
func (s *store) CancelMatchRequest(requestID string) error {
	return s.UpdateMatchRequestStatus(requestID, StatusCancelled)
}

// UpdateMatchRequestStatus updates the status of a match request
func (s *store) UpdateMatchRequestStatus(requestID string, status MatchRequestStatus) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	query := `UPDATE match_requests SET status = ?, updated_at = ? WHERE id = ?`
	now := time.Now()
	
	_, err := s.db.Exec(query, string(status), now.Unix(), requestID)
	if err != nil {
		return fmt.Errorf("failed to update match request status: %w", err)
	}

	log.Info("Updated match request status", "id", requestID, "status", status)
	return nil
}

// GetActiveMatchRequests gets all active match requests
func (s *store) GetActiveMatchRequests() ([]MatchRequest, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	query := `
		SELECT id, requester_id, requester_name, created_at, updated_at, status, channel_id,
			   thread_ts, availability_message_ts, proposed_date, proposed_start_time, proposed_end_time,
			   booking_responsible_id, booking_responsible_name, team_assignments_blob
		FROM match_requests
		WHERE status IN (?, ?)
		ORDER BY created_at DESC
	`
	
	rows, err := s.db.Query(query, string(StatusCollectingAvailability), string(StatusProposingMatch))
	if err != nil {
		return nil, fmt.Errorf("failed to query active match requests: %w", err)
	}
	defer rows.Close()

	var requests []MatchRequest
	for rows.Next() {
		var request MatchRequest
		var createdAt, updatedAt int64
		var status string
		var teamAssignmentsBlob []byte
		
		err := rows.Scan(
			&request.ID,
			&request.RequesterID,
			&request.RequesterName,
			&createdAt,
			&updatedAt,
			&status,
			&request.ChannelID,
			&request.ThreadTS,
			&request.AvailabilityMessageTS,
			&request.ProposedDate,
			&request.ProposedStartTime,
			&request.ProposedEndTime,
			&request.BookingResponsibleID,
			&request.BookingResponsibleName,
			&teamAssignmentsBlob,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan match request row: %w", err)
		}

		request.CreatedAt = time.Unix(createdAt, 0)
		request.UpdatedAt = time.Unix(updatedAt, 0)
		request.Status = MatchRequestStatus(status)
		
		if teamAssignmentsBlob != nil {
			var teamAssignments TeamAssignments
			if err := json.Unmarshal(teamAssignmentsBlob, &teamAssignments); err != nil {
				log.Warn("Failed to unmarshal team assignments", "error", err)
			} else {
				request.TeamAssignments = &teamAssignments
			}
		}

		requests = append(requests, request)
	}

	return requests, nil
}

// IsActiveMatchRequestMessage checks if a message timestamp belongs to an active match request
func (s *store) IsActiveMatchRequestMessage(messageTimestamp string) (string, bool, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	query := `
		SELECT id 
		FROM match_requests 
		WHERE availability_message_ts = ? AND status = ?
	`
	
	var requestID string
	err := s.db.QueryRow(query, messageTimestamp, string(StatusCollectingAvailability)).Scan(&requestID)
	if err != nil {
		if err == sql.ErrNoRows {
			return "", false, nil // Not an active match request message
		}
		return "", false, fmt.Errorf("failed to check match request message: %w", err)
	}

	return requestID, true, nil
}

// AddPlayerAvailability adds a day to a player's availability
func (s *store) AddPlayerAvailability(requestID, playerID, playerName, day string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Check if this availability already exists
	var existingID string
	checkQuery := `
		SELECT id FROM player_availability 
		WHERE match_request_id = ? AND player_id = ? AND available_date = ?
	`
	err := s.db.QueryRow(checkQuery, requestID, playerID, day).Scan(&existingID)
	if err == nil {
		log.Debug("Player availability already exists", "requestID", requestID, "playerID", playerID, "day", day)
		return nil // Already exists, no need to add again
	} else if err != sql.ErrNoRows {
		return fmt.Errorf("failed to check existing availability: %w", err)
	}

	// Insert new availability
	insertQuery := `
		INSERT INTO player_availability (id, match_request_id, player_id, player_name, available_date, responded_at)
		VALUES (?, ?, ?, ?, ?, ?)
	`
	
	availabilityID := uuid.New().String()
	now := time.Now().Unix()
	
	_, err = s.db.Exec(insertQuery, availabilityID, requestID, playerID, playerName, day, now)
	if err != nil {
		return fmt.Errorf("failed to insert player availability: %w", err)
	}

	log.Info("Added player availability", "requestID", requestID, "playerID", playerID, "playerName", playerName, "day", day)
	return nil
}

// RemovePlayerAvailability removes a day from a player's availability
func (s *store) RemovePlayerAvailability(requestID, playerID, day string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	query := `
		DELETE FROM player_availability 
		WHERE match_request_id = ? AND player_id = ? AND available_date = ?
	`
	
	result, err := s.db.Exec(query, requestID, playerID, day)
	if err != nil {
		return fmt.Errorf("failed to remove player availability: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}

	if rowsAffected == 0 {
		log.Debug("No availability found to remove", "requestID", requestID, "playerID", playerID, "day", day)
	} else {
		log.Info("Removed player availability", "requestID", requestID, "playerID", playerID, "day", day)
	}

	return nil
}