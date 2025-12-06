package matchmaking

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/charmbracelet/log"
	"github.com/google/uuid"
	"github.com/mauv0809/ideal-tribble/internal/club"
	"github.com/mauv0809/ideal-tribble/internal/playtomic"
)

// store handles database operations for matchmaking
type store struct {
	db        *sql.DB
	clubStore club.ClubStore
	mu        sync.RWMutex
}

// NewStore creates a new matchmaking store
func NewStore(db *sql.DB, clubStore club.ClubStore) MatchmakingService {
	return &store{
		db:        db,
		clubStore: clubStore,
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
	log.Info("ProposeMatch called", "requestID", requestID, "date", date, "startTime", startTime, "endTime", endTime)

	// Get available players for the date (GetPlayerAvailability handles its own locking)
	log.Info("Getting player availability", "requestID", requestID)
	availabilities, err := s.GetPlayerAvailability(requestID)
	if err != nil {
		return nil, fmt.Errorf("failed to get availabilities: %w", err)
	}
	log.Info("Got player availability", "count", len(availabilities))

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

	// Assign booking responsibility atomically based on booking counts
	log.Info("Creating team assignments for 4 players")
	playerIDs := make([]string, 4)
	for i, player := range availablePlayers[:4] {
		playerIDs[i] = player.ID
	}
	log.Info("Calling AssignBookingResponsibleAtomically", "playerIDs", playerIDs)

	bookingResponsibleID, bookingResponsibleName, err := s.clubStore.AssignBookingResponsibleAtomically(playerIDs)
	log.Info("AssignBookingResponsibleAtomically returned", "error", err, "id", bookingResponsibleID, "name", bookingResponsibleName)
	
	var bookingResponsible Player
	if err != nil {
		log.Warn("Failed to assign booking responsible player atomically, using first player", "error", err)
		bookingResponsible = availablePlayers[0]
	} else {
		bookingResponsible = Player{
			ID:   bookingResponsibleID,
			Name: bookingResponsibleName,
		}
	}
	log.Info("Selected booking responsible player", "id", bookingResponsible.ID, "name", bookingResponsible.Name)

	// Update match request with proposal (need to lock for database write)
	s.mu.Lock()
	defer s.mu.Unlock()
	log.Info("Acquired lock for database update")
	
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

	log.Info("Executing database update")
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
	log.Info("Database update completed")

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

// UpdateMatchRequestMessageTimestamps updates thread and availability message timestamps
func (s *store) UpdateMatchRequestMessageTimestamps(requestID, threadTS, availabilityMessageTS string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	query := `UPDATE match_requests SET thread_ts = ?, availability_message_ts = ?, updated_at = ? WHERE id = ?`
	now := time.Now()

	_, err := s.db.Exec(query, threadTS, availabilityMessageTS, now.Unix(), requestID)
	if err != nil {
		return fmt.Errorf("failed to update match request message timestamps: %w", err)
	}

	log.Info("Updated match request message timestamps", "id", requestID, "threadTS", threadTS, "availabilityMessageTS", availabilityMessageTS)
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
		SELECT id FROM match_request_availability 
		WHERE match_request_id = ? AND player_id = ? AND available_date = ?
	`
	err := s.db.QueryRow(checkQuery, requestID, playerID, day).Scan(&existingID)
	if err == nil {
		log.Debug("Player availability already exists", "requestID", requestID, "playerID", playerID, "day", day)
		return nil // Already exists, no need to add again
	} else if err != sql.ErrNoRows {
		return fmt.Errorf("failed to check existing availability: %w", err)
	}

	// Insert new availability (id is auto-increment, so we don't specify it)
	insertQuery := `
		INSERT INTO match_request_availability (match_request_id, player_id, player_name, available_date, responded_at)
		VALUES (?, ?, ?, ?, ?)
	`

	now := time.Now().Unix()

	_, err = s.db.Exec(insertQuery, requestID, playerID, playerName, day, now)
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
		DELETE FROM match_request_availability 
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

// CanProposeMatch checks if there are enough players available to propose a match
func (s *store) CanProposeMatch(requestID string) (bool, *AvailabilityResult, error) {
	results, err := s.AnalyzeAvailability(requestID)
	if err != nil {
		return false, nil, fmt.Errorf("failed to analyze availability: %w", err)
	}

	// Check if any date has 4 or more players (minimum for a match)
	for _, result := range results {
		if result.PlayerCount >= 4 {
			return true, &result, nil
		}
	}

	// Return the best available option even if not enough players
	if len(results) > 0 {
		return false, &results[0], nil
	}

	return false, nil, nil
}

// GetMatchRequestsInProposingStatus gets all match requests that are in PROPOSING_MATCH status
func (s *store) GetMatchRequestsInProposingStatus() ([]MatchRequest, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	query := `
		SELECT id, requester_id, requester_name, created_at, updated_at, status, channel_id,
			   thread_ts, availability_message_ts, proposed_date, proposed_start_time, proposed_end_time,
			   booking_responsible_id, booking_responsible_name, team_assignments_blob
		FROM match_requests
		WHERE status = ?
		ORDER BY created_at DESC
	`

	rows, err := s.db.Query(query, string(StatusProposingMatch))
	if err != nil {
		return nil, fmt.Errorf("failed to query proposing match requests: %w", err)
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

// DetectMatchedRequests checks if any proposed match requests have been booked on Playtomic
func (s *store) DetectMatchedRequests(padelMatches []*playtomic.PadelMatch) ([]string, error) {
	// Get all match requests in proposing status
	proposingRequests, err := s.GetMatchRequestsInProposingStatus()
	if err != nil {
		return nil, fmt.Errorf("failed to get proposing requests: %w", err)
	}

	if len(proposingRequests) == 0 {
		log.Debug("No match requests in proposing status to check")
		return nil, nil
	}

	var completedRequestIDs []string

	for _, request := range proposingRequests {
		// Skip if missing essential proposal data
		if request.ProposedDate == nil || request.ProposedStartTime == nil || 
		   request.ProposedEndTime == nil || request.BookingResponsibleID == nil ||
		   request.TeamAssignments == nil {
			log.Debug("Skipping request with incomplete proposal data", "requestID", request.ID)
			continue
		}

		// Check if any padel match matches this request
		for _, padelMatch := range padelMatches {
			if s.isMatchingRequest(&request, padelMatch) {
				log.Info("Found matching Playtomic match for request", 
					"requestID", request.ID, 
					"matchID", padelMatch.MatchID,
					"date", *request.ProposedDate)
				
				completedRequestIDs = append(completedRequestIDs, request.ID)
				break // Found a match, no need to check other padel matches for this request
			}
		}
	}

	return completedRequestIDs, nil
}

// isMatchingRequest checks if a Playtomic match corresponds to a proposed match request
func (s *store) isMatchingRequest(request *MatchRequest, padelMatch *playtomic.PadelMatch) bool {
	// Parse proposed date
	proposedDate, err := time.Parse("2006-01-02", *request.ProposedDate)
	if err != nil {
		log.Warn("Failed to parse proposed date", "date", *request.ProposedDate, "error", err)
		return false
	}

	// Check if match date matches (within same day)
	matchDate := time.Unix(padelMatch.Start, 0)
	if !isSameDate(proposedDate, matchDate) {
		return false
	}

	// Parse proposed time range
	proposedStart, err := time.Parse("15:04", *request.ProposedStartTime)
	if err != nil {
		log.Warn("Failed to parse proposed start time", "time", *request.ProposedStartTime, "error", err)
		return false
	}
	
	proposedEnd, err := time.Parse("15:04", *request.ProposedEndTime)
	if err != nil {
		log.Warn("Failed to parse proposed end time", "time", *request.ProposedEndTime, "error", err)
		return false
	}

	// Check if times overlap (allow 30min tolerance)
	matchStart := time.Unix(padelMatch.Start, 0)
	matchEnd := time.Unix(padelMatch.End, 0)
	
	proposedStartWithDate := time.Date(matchDate.Year(), matchDate.Month(), matchDate.Day(),
		proposedStart.Hour(), proposedStart.Minute(), 0, 0, matchDate.Location())
	proposedEndWithDate := time.Date(matchDate.Year(), matchDate.Month(), matchDate.Day(),
		proposedEnd.Hour(), proposedEnd.Minute(), 0, 0, matchDate.Location())

	tolerance := 30 * time.Minute
	if !timesOverlapWithTolerance(proposedStartWithDate, proposedEndWithDate, matchStart, matchEnd, tolerance) {
		return false
	}

	// Check if booking responsible player is the match owner
	bookingPlayerID := *request.BookingResponsibleID
	if padelMatch.OwnerID == bookingPlayerID {
		// Require all 4 players to match (same standard as isClubMatch in fetch logic)
		// This ensures we have the exact same match and maintains consistency with club match detection
		if s.verifyTeamComposition(request.TeamAssignments, padelMatch.Teams, 4) {
			return true
		}
	}

	return false
}

// isSameDate checks if two times are on the same calendar date
func isSameDate(date1, date2 time.Time) bool {
	y1, m1, d1 := date1.Date()
	y2, m2, d2 := date2.Date()
	return y1 == y2 && m1 == m2 && d1 == d2
}

// timesOverlapWithTolerance checks if two time ranges overlap within a tolerance
func timesOverlapWithTolerance(start1, end1, start2, end2 time.Time, tolerance time.Duration) bool {
	// Expand the ranges by the tolerance
	expandedStart1 := start1.Add(-tolerance)
	expandedEnd1 := end1.Add(tolerance)
	expandedStart2 := start2.Add(-tolerance) 
	expandedEnd2 := end2.Add(tolerance)
	
	// Check if ranges overlap
	return expandedStart1.Before(expandedEnd2) && expandedEnd1.After(expandedStart2)
}

// verifyTeamComposition checks if exactly the required number of players from the proposed teams are in the actual match
func (s *store) verifyTeamComposition(proposedTeams *TeamAssignments, actualTeams []playtomic.Team, requiredMatches int) bool {
	// Collect all proposed player IDs
	var proposedPlayerIDs []string
	for _, player := range proposedTeams.Team1 {
		proposedPlayerIDs = append(proposedPlayerIDs, player.ID)
	}
	for _, player := range proposedTeams.Team2 {
		proposedPlayerIDs = append(proposedPlayerIDs, player.ID)
	}

	// Collect all actual player IDs
	var actualPlayerIDs []string
	for _, team := range actualTeams {
		for _, player := range team.Players {
			actualPlayerIDs = append(actualPlayerIDs, player.UserID)
		}
	}

	// Count matches
	matches := 0
	for _, proposedID := range proposedPlayerIDs {
		for _, actualID := range actualPlayerIDs {
			if proposedID == actualID {
				matches++
				break
			}
		}
	}

	log.Debug("Team composition verification", 
		"proposedPlayers", len(proposedPlayerIDs), 
		"actualPlayers", len(actualPlayerIDs), 
		"matches", matches, 
		"required", requiredMatches)

	return matches >= requiredMatches
}
