package club

import (
	"database/sql"
	"encoding/json"
	"sync"

	"github.com/charmbracelet/log"
	"github.com/mauv0809/ideal-tribble/playtomic"
)

// MatchNotificationState tracks the notification status of a match.
type MatchNotificationState struct {
	BookingNotified bool
	ResultNotified  bool
}

// PlayerInfo holds details about a player in the club.
type PlayerInfo struct {
	ID               string `json:"id"`
	Name             string `json:"name"`
	IsInitial        bool   `json:"is_initial"`
	BallBringerCount int    `json:"ball_bringer_count"`
}

// Store holds the state of all processed matches in a thread-safe manner.
type Store struct {
	db *sql.DB
	mu sync.RWMutex
}

// New creates a new Store.
func New(db *sql.DB) *Store {
	return &Store{
		db: db,
	}
}

// GetMatchState returns the match and its notification state.
func (s *Store) GetMatchState(matchID string) (*playtomic.PadelMatch, *MatchNotificationState, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var state MatchNotificationState
	var ballBringerID, ballBringerName, resultsStatus sql.NullString
	row := s.db.QueryRow("SELECT booking_notified, result_notified, ball_bringer_id, ball_bringer_name, results_status FROM matches WHERE id = ?", matchID)

	err := row.Scan(&state.BookingNotified, &state.ResultNotified, &ballBringerID, &ballBringerName, &resultsStatus)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil, false
		}
		log.Error("Failed to get match state", "error", err)
		return nil, nil, false
	}
	// We don't return the full match object, but we can return a partial one with the ball bringer info.
	match := &playtomic.PadelMatch{
		BallBringerID:   ballBringerID.String,
		BallBringerName: ballBringerName.String,
		ResultsStatus:   playtomic.ResultsStatus(resultsStatus.String),
	}

	return match, &state, true
}

// SetMatchState updates the store with the latest match details and notification state.
func (s *Store) SetMatchState(match *playtomic.PadelMatch, state *MatchNotificationState) {
	s.mu.Lock()
	defer s.mu.Unlock()

	tx, err := s.db.Begin()
	if err != nil {
		log.Error("Failed to begin transaction", "error", err)
		return
	}

	// Check if match exists to determine if we need to assign a ball bringer.
	var exists bool
	err = tx.QueryRow("SELECT EXISTS(SELECT 1 FROM matches WHERE id = ?)", match.MatchID).Scan(&exists)
	if err != nil && err != sql.ErrNoRows {
		log.Error("Failed to check for existing match", "error", err)
		tx.Rollback()
		return
	}

	if !exists {
		// New match, let's assign a ball bringer.
		var playerIDs []string
		for _, team := range match.Teams {
			for _, player := range team.Players {
				playerIDs = append(playerIDs, player.UserID)
			}
		}

		if len(playerIDs) > 0 {
			rows, err := tx.Query("SELECT id, name, ball_bringer_count FROM players WHERE id IN ("+buildQuestionMarks(len(playerIDs))+")", ToAnySlice(playerIDs)...)
			if err != nil {
				log.Error("Failed to query players for ball bringer assignment", "error", err)
				tx.Rollback()
				return
			}
			defer rows.Close()

			minCount := -1
			var bringerID, bringerName string

			for rows.Next() {
				var id string
				var name sql.NullString
				var count int
				if err := rows.Scan(&id, &name, &count); err != nil {
					log.Error("Failed to scan player for ball bringer assignment", "error", err)
					tx.Rollback()
					return
				}
				if minCount == -1 || count < minCount {
					minCount = count
					bringerID = id
					bringerName = name.String
				}
			}

			if bringerID != "" {
				match.BallBringerID = bringerID
				match.BallBringerName = bringerName
				_, err = tx.Exec("UPDATE players SET ball_bringer_count = ball_bringer_count + 1 WHERE id = ?", bringerID)
				if err != nil {
					log.Error("Failed to increment ball bringer count", "error", err)
					tx.Rollback()
					return
				}
				log.Debug("Assigned ball bringer.", "matchID", match.MatchID, "player", bringerName)
			}
		}
	}

	teamsJSON, err := json.Marshal(match.Teams)
	if err != nil {
		log.Error("Failed to marshal teams to JSON", "error", err)
		tx.Rollback()
		return
	}
	resultsJSON, err := json.Marshal(match.Results)
	if err != nil {
		log.Error("Failed to marshal results to JSON", "error", err)
		tx.Rollback()
		return
	}

	stmt, err := tx.Prepare(`
		INSERT INTO matches (id, owner_id, owner_name, start_time, end_time, created_at, status, game_status, resource_name, access_code, price, tenant_id, tenant_name, booking_notified, result_notified, teams_json, results_json, ball_bringer_id, ball_bringer_name, results_status)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(id) DO UPDATE SET
			status = excluded.status,
			game_status = excluded.game_status,
			results_status = excluded.results_status,
			result_notified = excluded.result_notified;
	`)
	if err != nil {
		log.Error("Failed to prepare statement", "error", err)
		tx.Rollback()
		return
	}
	defer stmt.Close()

	_, err = stmt.Exec(match.MatchID, match.OwnerID, match.OwnerName, match.Start, match.End, match.CreatedAt, match.Status, match.GameStatus, match.ResourceName, match.AccessCode, match.Price, match.Tenant.ID, match.Tenant.Name, state.BookingNotified, state.ResultNotified, teamsJSON, resultsJSON, match.BallBringerID, match.BallBringerName, match.ResultsStatus)
	if err != nil {
		log.Error("Failed to execute statement", "error", err)
		tx.Rollback()
		return
	}

	if err := tx.Commit(); err != nil {
		log.Error("Failed to commit transaction", "error", err)
	}
}

// Clear resets the store to its initial empty state.
func (s *Store) Clear() {
	s.mu.Lock()
	defer s.mu.Unlock()
	_, err := s.db.Exec("DELETE FROM matches; DELETE FROM players;")
	if err != nil {
		log.Error("Failed to clear store", "error", err)
	}
}

// ClearMatch removes a specific match from the store.
func (s *Store) ClearMatch(matchID string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	_, err := s.db.Exec("DELETE FROM matches WHERE id = ?", matchID)
	if err != nil {
		log.Error("Failed to clear match", "error", err)
	}
}

// AddInitialPlayers adds the seed list of player IDs to the store.
func (s *Store) AddInitialPlayers(playerIDs []string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	tx, err := s.db.Begin()
	if err != nil {
		log.Error("Failed to begin transaction", "error", err)
		return
	}

	stmt, err := tx.Prepare("INSERT OR IGNORE INTO players (id, is_initial) VALUES (?, ?)")
	if err != nil {
		log.Error("Failed to prepare statement", "error", err)
		tx.Rollback()
		return
	}
	defer stmt.Close()

	for _, id := range playerIDs {
		_, err := stmt.Exec(id, 1)
		if err != nil {
			log.Error("Failed to insert initial player", "error", err)
			tx.Rollback()
			return
		}
	}
	if err := tx.Commit(); err != nil {
		log.Error("Failed to commit transaction", "error", err)
	}
}

// AddPlayer adds a single player ID to the store if not already present.
func (s *Store) AddPlayer(playerID, name string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	var exists bool
	err := s.db.QueryRow("SELECT EXISTS(SELECT 1 FROM players WHERE id = ?)", playerID).Scan(&exists)
	if err != nil {
		log.Error("Failed to check if player exists", "error", err)
		return
	}

	if !exists {
		_, err := s.db.Exec("INSERT INTO players (id, name, is_initial) VALUES (?, ?, ?)", playerID, name, 0)
		if err != nil {
			log.Error("Failed to add player", "error", err)
		} else {
			log.Info("Discovered and added new player to the store.", "playerID", playerID)
		}
	}
}

// IsKnownPlayer checks if a player ID is in the store.
func (s *Store) IsKnownPlayer(playerID string) bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	var exists bool
	err := s.db.QueryRow("SELECT EXISTS(SELECT 1 FROM players WHERE id = ?)", playerID).Scan(&exists)
	if err != nil {
		log.Error("Failed to check if player is known", "error", err)
		return false
	}
	return exists
}

// IsInitialPlayer checks if a player ID is one of the initial players.
func (s *Store) IsInitialPlayer(playerID string) bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	var isInitial bool
	err := s.db.QueryRow("SELECT is_initial FROM players WHERE id = ?", playerID).Scan(&isInitial)
	if err != nil {
		if err != sql.ErrNoRows {
			log.Error("Failed to check if player is initial", "error", err)
		}
		return false
	}
	return isInitial
}

// GetAllPlayers returns a list of all players in the store.
func (s *Store) GetAllPlayers() ([]PlayerInfo, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	rows, err := s.db.Query("SELECT id, name, is_initial, ball_bringer_count FROM players")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var players []PlayerInfo
	for rows.Next() {
		var p PlayerInfo
		var name sql.NullString
		if err := rows.Scan(&p.ID, &name, &p.IsInitial, &p.BallBringerCount); err != nil {
			return nil, err
		}
		if name.Valid {
			p.Name = name.String
		}
		players = append(players, p)
	}
	return players, nil
}

// GetAllMatches returns a list of all matches in the store.
func (s *Store) GetAllMatches() ([]playtomic.PadelMatch, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	rows, err := s.db.Query(`
		SELECT id, owner_id, owner_name, start_time, end_time, created_at, status, game_status, resource_name, access_code, price, tenant_id, tenant_name, teams_json, results_json, ball_bringer_id, ball_bringer_name, results_status 
		FROM matches
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var matches []playtomic.PadelMatch
	for rows.Next() {
		var m playtomic.PadelMatch
		var teamsJSON, resultsJSON, ballBringerID, ballBringerName, resultsStatus sql.NullString
		err := rows.Scan(
			&m.MatchID, &m.OwnerID, &m.OwnerName, &m.Start, &m.End, &m.CreatedAt,
			&m.Status, &m.GameStatus, &m.ResourceName, &m.AccessCode, &m.Price,
			&m.Tenant.ID, &m.Tenant.Name, &teamsJSON, &resultsJSON,
			&ballBringerID, &ballBringerName, &resultsStatus,
		)

		if err != nil {
			return nil, err
		}
		if teamsJSON.Valid {
			if err := json.Unmarshal([]byte(teamsJSON.String), &m.Teams); err != nil {
				log.Error("Failed to unmarshal teams", "error", err)
			}
		}
		if resultsJSON.Valid {
			if err := json.Unmarshal([]byte(resultsJSON.String), &m.Results); err != nil {
				log.Error("Failed to unmarshal results", "error", err)
			}
		}
		m.BallBringerID = ballBringerID.String
		m.BallBringerName = ballBringerName.String
		m.ResultsStatus = playtomic.ResultsStatus(resultsStatus.String)
		matches = append(matches, m)
	}
	return matches, nil
}

// buildQuestionMarks is a helper to generate placeholders for IN queries.
func buildQuestionMarks(n int) string {
	if n <= 0 {
		return ""
	}
	marks := "?"
	for i := 1; i < n; i++ {
		marks += ",?"
	}
	return marks
}

// ToAnySlice converts a slice of a specific type to a slice of any.
func ToAnySlice[T any](s []T) []any {
	result := make([]any, len(s))
	for i, v := range s {
		result[i] = v
	}
	return result
}
