package matchmaking

import (
	"time"
)

// MatchRequestStatus represents the status of a match request
type MatchRequestStatus string

const (
	StatusCollectingAvailability MatchRequestStatus = "COLLECTING_AVAILABILITY"
	StatusProposingMatch         MatchRequestStatus = "PROPOSING_MATCH"
	StatusConfirmed              MatchRequestStatus = "CONFIRMED"
	StatusCancelled              MatchRequestStatus = "CANCELLED"
)

// MatchRequest represents a request to organize a match
type MatchRequest struct {
	ID                       string             `json:"id"`
	RequesterID              string             `json:"requester_id"`
	RequesterName            string             `json:"requester_name"`
	CreatedAt                time.Time          `json:"created_at"`
	UpdatedAt                time.Time          `json:"updated_at"`
	Status                   MatchRequestStatus `json:"status"`
	ChannelID                string             `json:"channel_id"`
	ThreadTS                 *string            `json:"thread_ts,omitempty"`
	AvailabilityMessageTS    *string            `json:"availability_message_ts,omitempty"`
	ProposedDate             *string            `json:"proposed_date,omitempty"`      // YYYY-MM-DD
	ProposedStartTime        *string            `json:"proposed_start_time,omitempty"` // HH:MM
	ProposedEndTime          *string            `json:"proposed_end_time,omitempty"`   // HH:MM
	BookingResponsibleID     *string            `json:"booking_responsible_id,omitempty"`
	BookingResponsibleName   *string            `json:"booking_responsible_name,omitempty"`
	TeamAssignments          *TeamAssignments   `json:"team_assignments,omitempty"`
}

// PlayerAvailability represents a player's availability for a match request
type PlayerAvailability struct {
	ID              int       `json:"id"`
	MatchRequestID  string    `json:"match_request_id"`
	PlayerID        string    `json:"player_id"`
	PlayerName      string    `json:"player_name"`
	AvailableDate   string    `json:"available_date"` // YYYY-MM-DD
	RespondedAt     time.Time `json:"responded_at"`
}

// TeamAssignments represents the team assignments for a confirmed match
type TeamAssignments struct {
	Team1 []Player `json:"team1"`
	Team2 []Player `json:"team2"`
}

// Player represents a player in team assignments
type Player struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

// AvailabilityResult represents the result of availability analysis
type AvailabilityResult struct {
	Date             string   `json:"date"`
	AvailablePlayers []Player `json:"available_players"`
	PlayerCount      int      `json:"player_count"`
}

// MatchProposal represents a proposed match
type MatchProposal struct {
	Date                   string           `json:"date"`
	StartTime              string           `json:"start_time"`
	EndTime                string           `json:"end_time"`
	AvailablePlayers       []Player         `json:"available_players"`
	TeamAssignments        TeamAssignments  `json:"team_assignments"`
	BookingResponsibleID   string           `json:"booking_responsible_id"`
	BookingResponsibleName string           `json:"booking_responsible_name"`
}