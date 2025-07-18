package club

import (
	"database/sql"
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/log"
	"github.com/mauv0809/ideal-tribble/internal/playtomic"
	"github.com/vmihailenco/msgpack/v5"
)

// New creates a new ClubStore.
func New(db *sql.DB) ClubStore {
	return &store{
		db: db,
	}
}

// UpsertMatch inserts a new match or updates an existing one. It is "dumb" and
// does not change the processing status of an existing match.
func (s *store) UpsertMatch(match *playtomic.PadelMatch) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	tx, err := s.db.Begin()
	if err != nil {
		return err
	}

	teamsBlob, err := msgpack.Marshal(match.Teams)
	if err != nil {
		tx.Rollback()
		return err
	}
	resultsBlob, err := msgpack.Marshal(match.Results)
	if err != nil {
		tx.Rollback()
		return err
	}

	// This statement is the heart of the "dumb upsert".
	// ON CONFLICT, it updates all fields EXCEPT processing_status.
	stmt, err := tx.Prepare(`
		INSERT INTO matches (id, owner_id, owner_name, start_time, end_time, created_at, status, game_status, results_status, resource_name, access_code, price, tenant_id, tenant_name, match_type, teams_blob, results_blob, processing_status)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(id) DO UPDATE SET
			owner_id = excluded.owner_id,
			owner_name = excluded.owner_name,
			start_time = excluded.start_time,
			end_time = excluded.end_time,
			created_at = excluded.created_at,
			status = excluded.status,
			game_status = excluded.game_status,
			results_status = excluded.results_status,
			resource_name = excluded.resource_name,
			access_code = excluded.access_code,
			price = excluded.price,
			tenant_id = excluded.tenant_id,
			tenant_name = excluded.tenant_name,
			match_type = excluded.match_type,
			teams_blob = excluded.teams_blob,
			results_blob = excluded.results_blob;
	`)
	if err != nil {
		tx.Rollback()
		return err
	}
	defer stmt.Close()

	_, err = stmt.Exec(match.MatchID, match.OwnerID, match.OwnerName, match.Start, match.End, match.CreatedAt, match.Status, match.GameStatus, match.ResultsStatus, match.ResourceName, match.AccessCode, match.Price, match.Tenant.ID, match.Tenant.Name, match.MatchType, teamsBlob, resultsBlob, playtomic.StatusNew)
	if err != nil {
		tx.Rollback()
		return err
	}

	return tx.Commit()
}

// UpsertMatches inserts or updates multiple matches in a single transaction.
func (s *store) UpsertMatches(matches []*playtomic.PadelMatch) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	tx, err := s.db.Begin()
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	// Rollback is deferred to execute only if the transaction is not committed.
	defer tx.Rollback()

	stmt, err := tx.Prepare(`
		INSERT INTO matches (id, owner_id, owner_name, start_time, end_time, created_at, status, game_status, results_status, resource_name, access_code, price, tenant_id, tenant_name, match_type, teams_blob, results_blob, processing_status)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(id) DO UPDATE SET
			owner_id = excluded.owner_id,
			owner_name = excluded.owner_name,
			start_time = excluded.start_time,
			end_time = excluded.end_time,
			created_at = excluded.created_at,
			status = excluded.status,
			game_status = excluded.game_status,
			results_status = excluded.results_status,
			resource_name = excluded.resource_name,
			access_code = excluded.access_code,
			price = excluded.price,
			tenant_id = excluded.tenant_id,
			tenant_name = excluded.tenant_name,
			match_type = excluded.match_type,
			teams_blob = excluded.teams_blob,
			results_blob = excluded.results_blob;
	`)
	if err != nil {
		return fmt.Errorf("failed to prepare statement: %w", err)
	}
	defer stmt.Close()

	for _, match := range matches {
		teamsBlob, err := msgpack.Marshal(match.Teams)
		if err != nil {
			return fmt.Errorf("failed to marshal teams for match %s: %w", match.MatchID, err)
		}
		resultsBlob, err := msgpack.Marshal(match.Results)
		if err != nil {
			return fmt.Errorf("failed to marshal results for match %s: %w", match.MatchID, err)
		}

		_, err = stmt.Exec(match.MatchID, match.OwnerID, match.OwnerName, match.Start, match.End, match.CreatedAt, match.Status, match.GameStatus, match.ResultsStatus, match.ResourceName, match.AccessCode, match.Price, match.Tenant.ID, match.Tenant.Name, match.MatchType, teamsBlob, resultsBlob, playtomic.StatusNew)
		if err != nil {
			return fmt.Errorf("failed to execute statement for match %s: %w", match.MatchID, err)
		}
	}

	return tx.Commit()
}

// UpdateProcessingStatus transitions a match to a new state.
func (s *store) UpdateProcessingStatus(matchID string, status playtomic.ProcessingStatus) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	_, err := s.db.Exec("UPDATE matches SET processing_status = ? WHERE id = ?", status, matchID)
	return err
}

// UpdateNotificationTimestamp updates the timestamp for a specific notification type for a match.
func (s *store) UpdateNotificationTimestamp(matchID string, notificationType string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	var columnName string
	switch notificationType {
	case "booking":
		columnName = "booking_notified_ts"
	case "result":
		columnName = "result_notified_ts"
	default:
		return fmt.Errorf("invalid notification type: %s", notificationType)
	}

	query := fmt.Sprintf("UPDATE matches SET %s = ? WHERE id = ?", columnName)
	_, err := s.db.Exec(query, time.Now().Unix(), matchID)
	if err != nil {
		return fmt.Errorf("failed to update %s timestamp for match %s: %w", notificationType, matchID, err)
	}
	log.Debug("Successfully updated notification timestamp", "matchID", matchID, "type", notificationType)
	return nil
}

// GetMatchesForProcessing retrieves all matches that are not yet in a completed state.
func (s *store) GetMatchesForProcessing() ([]*playtomic.PadelMatch, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	rows, err := s.db.Query(`
		SELECT id, owner_id, owner_name, start_time, end_time, created_at, status, game_status, results_status, resource_name, access_code, price, tenant_id, tenant_name, match_type, teams_blob, results_blob, ball_bringer_id, ball_bringer_name, processing_status, booking_notified_ts, result_notified_ts
		FROM matches
		WHERE processing_status != ?
		AND game_status != ?
		AND (game_status != ? OR results_status != ?)
	`, playtomic.StatusCompleted, playtomic.GameStatusCanceled, playtomic.GameStatusPlayed, playtomic.ResultsStatusWaitingFor)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var matches []*playtomic.PadelMatch
	for rows.Next() {
		match, err := s.scanMatch(rows)
		if err != nil {
			log.Error("Failed to scan match row", "error", err)
			continue
		}
		matches = append(matches, match)
	}
	return matches, nil
}

// scanMatch is a helper function to scan a single match row.
func (s *store) scanMatch(scanner interface{ Scan(...any) error }) (*playtomic.PadelMatch, error) {
	var match playtomic.PadelMatch
	var teamsBlob, resultsBlob []byte
	var ballBringerID, ballBringerName sql.NullString
	var bookingNotifiedTs, resultNotifiedTs sql.NullInt64 // New nullable timestamp fields

	err := scanner.Scan(
		&match.MatchID, &match.OwnerID, &match.OwnerName, &match.Start, &match.End, &match.CreatedAt,
		&match.Status, &match.GameStatus, &match.ResultsStatus, &match.ResourceName, &match.AccessCode, &match.Price,
		&match.Tenant.ID, &match.Tenant.Name, &match.MatchType, &teamsBlob, &resultsBlob,
		&ballBringerID, &ballBringerName, &match.ProcessingStatus,
		&bookingNotifiedTs, &resultNotifiedTs, // Include new fields here
	)
	if err != nil {
		return nil, err
	}

	match.BallBringerID = ballBringerID.String
	match.BallBringerName = ballBringerName.String

	// Assign nullable timestamps to match struct
	if bookingNotifiedTs.Valid {
		match.BookingNotifiedTs = &bookingNotifiedTs.Int64
	}
	if resultNotifiedTs.Valid {
		match.ResultNotifiedTs = &resultNotifiedTs.Int64
	}

	if len(teamsBlob) > 0 {
		if err := msgpack.Unmarshal(teamsBlob, &match.Teams); err != nil {
			log.Error("Failed to unmarshal teams_blob", "error", err, "matchID", match.MatchID)
		}
	} else {
		match.Teams = []playtomic.Team{}
	}

	if len(resultsBlob) > 0 {
		if err := msgpack.Unmarshal(resultsBlob, &match.Results); err != nil {
			log.Error("Failed to unmarshal results_blob", "error", err, "matchID", match.MatchID)
		}
	} else {
		match.Results = []playtomic.SetResult{}
	}

	return &match, nil
}

// UpdatePlayerStats acquires a lock and calls the unexported method.
func (s *store) UpdatePlayerStats(match *playtomic.PadelMatch) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.updatePlayerStatsLocked(match)
}

func (s *store) updatePlayerStatsLocked(match *playtomic.PadelMatch) {
	tx, err := s.db.Begin()
	if err != nil {
		log.Error("Failed to begin transaction for stats update", "error", err, "matchID", match.MatchID)
		return
	}

	// Using a map to aggregate stats per player before updating the DB.
	playerStats := make(map[string]map[string]int)

	var winningTeamID string
	for _, team := range match.Teams {
		if team.TeamResult == "WON" {
			winningTeamID = team.ID
			break
		}
	}

	for _, team := range match.Teams {
		isWinningTeam := team.ID == winningTeamID
		for _, player := range team.Players {
			if _, ok := playerStats[player.UserID]; !ok {
				playerStats[player.UserID] = make(map[string]int)
			}
			playerStats[player.UserID]["matches_played"]++
			if isWinningTeam {
				playerStats[player.UserID]["matches_won"]++
			} else {
				playerStats[player.UserID]["matches_lost"]++
			}
		}
	}

	for _, set := range match.Results {
		var setWinnerID, setLoserID string
		var maxScore, minScore int = -1, -1

		// Determine the winner and loser of the set
		for teamID, score := range set.Scores {
			if maxScore == -1 || score > maxScore {
				maxScore = score
				setWinnerID = teamID
			}
			if minScore == -1 || score < minScore {
				minScore = score
			}
		}
		// Find the losing team ID
		for teamID, score := range set.Scores {
			if score < maxScore {
				setLoserID = teamID
				minScore = score
				break
			}
		}

		// Update stats for the winning team's players
		for _, team := range match.Teams {
			switch team.ID {
			case setWinnerID:
				for _, player := range team.Players {
					playerStats[player.UserID]["sets_won"]++
					playerStats[player.UserID]["games_won"] += maxScore
					playerStats[player.UserID]["games_lost"] += minScore
				}
			case setLoserID:
				for _, player := range team.Players {
					playerStats[player.UserID]["sets_lost"]++
					playerStats[player.UserID]["games_won"] += minScore
					playerStats[player.UserID]["games_lost"] += maxScore
				}
			}
		}
	}

	for playerID, stats := range playerStats {
		stmt, err := tx.Prepare(`
			INSERT INTO player_stats (player_id, matches_played, matches_won, matches_lost, sets_won, sets_lost, games_won, games_lost)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?)
			ON CONFLICT(player_id) DO UPDATE SET
				matches_played = matches_played + excluded.matches_played,
				matches_won = matches_won + excluded.matches_won,
				matches_lost = matches_lost + excluded.matches_lost,
				sets_won = sets_won + excluded.sets_won,
				sets_lost = sets_lost + excluded.sets_lost,
				games_won = games_won + excluded.games_won,
				games_lost = games_lost + excluded.games_lost;
		`)
		if err != nil {
			log.Error("Failed to prepare player_stats statement", "error", err, "playerID", playerID)
			continue
		}
		defer stmt.Close()

		_, err = stmt.Exec(playerID, stats["matches_played"], stats["matches_won"], stats["matches_lost"], stats["sets_won"], stats["sets_lost"], stats["games_won"], stats["games_lost"])
		if err != nil {
			log.Error("Failed to execute player_stats statement", "error", err, "playerID", playerID)
		} else {
			log.Info("Updated player stats", "playerID", playerID)
		}
	}

	if err := tx.Commit(); err != nil {
		log.Error("Failed to commit player_stats transaction", "error", err)
	}
}

// GetPlayerStatsByName retrieves the statistics for a single player by their name.
// It performs a case-insensitive, fuzzy search (e.g., "morten" will match "Morten Voss").
func (s *store) GetPlayerStatsByName(playerName string) (*PlayerStats, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	query := `
		SELECT
			p.id,
			p.name,
			COALESCE(ps.matches_played, 0),
			COALESCE(ps.matches_won, 0),
			COALESCE(ps.matches_lost, 0),
			COALESCE(ps.sets_won, 0),
			COALESCE(ps.sets_lost, 0),
			COALESCE(ps.games_won, 0),
			COALESCE(ps.games_lost, 0)
		FROM players p
		LEFT JOIN player_stats ps ON p.id = ps.player_id
		WHERE p.name LIKE ? COLLATE NOCASE
		LIMIT 1
	`

	var stat PlayerStats
	// Use a fuzzy search pattern.
	pattern := "%" + playerName + "%"

	row := s.db.QueryRow(query, pattern)
	err := row.Scan(
		&stat.PlayerID,
		&stat.PlayerName,
		&stat.MatchesPlayed,
		&stat.MatchesWon,
		&stat.MatchesLost,
		&stat.SetsWon,
		&stat.SetsLost,
		&stat.GamesWon,
		&stat.GamesLost,
	)

	if err != nil {
		if err == sql.ErrNoRows {
			log.Info("No stats found for player matching pattern", "pattern", pattern)
			return nil, fmt.Errorf("player matching '%s' not found", playerName)
		}
		log.Error("Failed to query player stats by name", "error", err, "pattern", pattern)
		return nil, fmt.Errorf("database error: %w", err)
	}

	if stat.MatchesPlayed > 0 {
		stat.WinPercentage = (float64(stat.MatchesWon) / float64(stat.MatchesPlayed)) * 100
	}

	log.Debug("Found player stats by name", "player", stat.PlayerName)
	return &stat, nil
}

func (s *store) GetPlayerStats() ([]PlayerStats, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	rows, err := s.db.Query(`
		SELECT
			ps.player_id,
			p.name,
			ps.matches_played,
			ps.matches_won,
			ps.matches_lost,
			ps.sets_won,
			ps.sets_lost,
			ps.games_won,
			ps.games_lost
		FROM player_stats ps
		JOIN players p ON ps.player_id = p.id
		ORDER BY ps.matches_won DESC, ps.sets_won DESC, ps.games_won DESC;
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var stats []PlayerStats
	for rows.Next() {
		var stat PlayerStats
		err := rows.Scan(
			&stat.PlayerID,
			&stat.PlayerName,
			&stat.MatchesPlayed,
			&stat.MatchesWon,
			&stat.MatchesLost,
			&stat.SetsWon,
			&stat.SetsLost,
			&stat.GamesWon,
			&stat.GamesLost,
		)
		if err != nil {
			return nil, err
		}
		if stat.MatchesPlayed > 0 {
			stat.WinPercentage = (float64(stat.MatchesWon) / float64(stat.MatchesPlayed)) * 100
		}
		stats = append(stats, stat)
	}
	return stats, nil
}

func (s *store) AddPlayer(playerID, name string, level float64) {
	s.mu.Lock()
	defer s.mu.Unlock()

	var exists bool
	err := s.db.QueryRow("SELECT EXISTS(SELECT 1 FROM players WHERE id = ?)", playerID).Scan(&exists)
	if err != nil {
		log.Error("Failed to check if player exists", "error", err, "playerID", playerID)
		return
	}

	if !exists {
		_, err := s.db.Exec("INSERT INTO players (id, name, level) VALUES (?, ?, ?)", playerID, name, level)
		if err != nil {
			log.Error("Failed to add player", "error", err, "playerID", playerID)
		} else {
			log.Info("Discovered and added new player to the store", "playerID", playerID, "name", name, "player_level", level)
		}
	} else {
		_, err := s.db.Exec("UPDATE players SET name = ?, level = ? WHERE id = ?", name, level, playerID)
		if err != nil {
			log.Error("Failed to update player", "error", err, "playerID", playerID)
		} else {
			log.Info("Updated existing player in the store", "playerID", playerID, "name", name, "player_level", level)
		}
	}
}

// UpsertPlayers inserts or updates multiple players in a single transaction.
func (s *store) UpsertPlayers(players []PlayerInfo) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	tx, err := s.db.Begin()
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback()

	stmt, err := tx.Prepare(`
		INSERT INTO players (id, name, level)
		VALUES (?, ?, ?)
		ON CONFLICT(id) DO UPDATE SET
			name = excluded.name,
			level = excluded.level;
	`)
	if err != nil {
		return fmt.Errorf("failed to prepare statement for players: %w", err)
	}
	defer stmt.Close()

	for _, player := range players {
		if player.ID == "" {
			log.Warn("Skipping player with empty ID")
			continue
		}
		_, err := stmt.Exec(player.ID, player.Name, player.Level)
		if err != nil {
			return fmt.Errorf("failed to execute statement for player %s: %w", player.ID, err)
		}
	}

	return tx.Commit()
}

func (s *store) IsKnownPlayer(playerID string) bool {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var exists bool
	err := s.db.QueryRow("SELECT EXISTS(SELECT 1 FROM players WHERE id = ?)", playerID).Scan(&exists)
	if err != nil {
		log.Error("Failed to check if player exists", "error", err, "playerID", playerID)
		return false
	}
	return exists
}

func (s *store) Clear() {
	s.mu.Lock()
	defer s.mu.Unlock()

	tx, err := s.db.Begin()
	if err != nil {
		log.Error("Failed to begin transaction for clearing store", "error", err)
		return
	}

	_, err = tx.Exec("DELETE FROM matches")
	if err != nil {
		log.Error("Failed to clear matches table", "error", err)
		tx.Rollback()
		return
	}

	_, err = tx.Exec("DELETE FROM players")
	if err != nil {
		log.Error("Failed to clear players table", "error", err)
		tx.Rollback()
		return
	}

	if err := tx.Commit(); err != nil {
		log.Error("Failed to commit transaction for clearing store", "error", err)
	}
}

func (s *store) ClearMatch(matchID string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	_, err := s.db.Exec("DELETE FROM matches WHERE id = ?", matchID)
	if err != nil {
		log.Error("Failed to clear match", "error", err, "matchID", matchID)
	}
}

func (s *store) GetAllPlayers() ([]PlayerInfo, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	rows, err := s.db.Query("SELECT id, name, ball_bringer_count, level FROM players ORDER BY name")
	if err != nil {
		log.Error("Failed to query all players", "error", err)
		return nil, err
	}
	defer rows.Close()

	var players []PlayerInfo
	for rows.Next() {
		var p PlayerInfo
		var name sql.NullString
		var level sql.NullFloat64
		if err := rows.Scan(&p.ID, &name, &p.BallBringerCount, &level); err != nil {
			log.Error("Failed to scan player row", "error", err)
			continue
		}
		p.Name = name.String // handle NULL name from db
		p.Level = level.Float64
		players = append(players, p)
	}
	return players, nil
}

// GetPlayers retrieves information for a specific list of players.
func (s *store) GetPlayers(playerIDs []string) ([]PlayerInfo, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if len(playerIDs) == 0 {
		return []PlayerInfo{}, nil
	}

	query := "SELECT id, name, ball_bringer_count, level FROM players WHERE id IN (?" + strings.Repeat(",?", len(playerIDs)-1) + ")"
	args := make([]interface{}, len(playerIDs))
	for i, id := range playerIDs {
		args[i] = id
	}

	rows, err := s.db.Query(query, args...)
	if err != nil {
		log.Error("Failed to query players by IDs", "error", err)
		return nil, err
	}
	defer rows.Close()

	var players []PlayerInfo
	for rows.Next() {
		var p PlayerInfo
		var name sql.NullString
		var level sql.NullFloat64
		if err := rows.Scan(&p.ID, &name, &p.BallBringerCount, &level); err != nil {
			log.Error("Failed to scan player row", "error", err)
			continue // Or handle error more gracefully
		}
		p.Name = name.String
		p.Level = level.Float64
		players = append(players, p)
	}
	return players, nil
}

// SetBallBringer assigns a player as the ball bringer for a match and increments their count.
// This function is now deprecated and replaced by AssignBallBringerAtomically to prevent race conditions.
func (s *store) SetBallBringer(matchID, playerID, playerName string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	tx, err := s.db.Begin()
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}

	// Update the match with the ball bringer's details
	_, err = tx.Exec("UPDATE matches SET ball_bringer_id = ?, ball_bringer_name = ? WHERE id = ?", playerID, playerName, matchID)
	if err != nil {
		tx.Rollback()
		return fmt.Errorf("failed to update match with ball bringer: %w", err)
	}

	// Increment the player's ball bringer count
	_, err = tx.Exec("UPDATE players SET ball_bringer_count = ball_bringer_count + 1 WHERE id = ?", playerID)
	if err != nil {
		tx.Rollback()
		return fmt.Errorf("failed to increment ball bringer count: %w", err)
	}

	return tx.Commit()
}

// AssignBallBringerAtomically finds the player with the minimum ball_bringer_count among the given player IDs,
// assigns them as the ball bringer for the match, and atomically increments their count.
func (s *store) AssignBallBringerAtomically(matchID string, playerIDs []string) (string, string, error) {
	s.mu.Lock() // Ensure only one ball bringer assignment process runs at a time
	defer s.mu.Unlock()

	if len(playerIDs) == 0 {
		return "", "", fmt.Errorf("no player IDs provided for ball bringer assignment")
	}

	tx, err := s.db.Begin()
	if err != nil {
		return "", "", fmt.Errorf("failed to begin transaction for atomic ball bringer assignment: %w", err)
	}
	defer tx.Rollback() // Rollback on error by default

	// Check if a ball bringer is already assigned to this match
	var existingBallBringerID, existingBallBringerName sql.NullString
	err = tx.QueryRow("SELECT ball_bringer_id, ball_bringer_name FROM matches WHERE id = ?", matchID).Scan(&existingBallBringerID, &existingBallBringerName)
	if err != nil && err != sql.ErrNoRows {
		return "", "", fmt.Errorf("failed to query existing ball bringer for match %s: %w", matchID, err)
	}

	if existingBallBringerID.Valid && existingBallBringerName.Valid {
		log.Info("Ball bringer already assigned for match. Returning existing assignment.", "matchID", matchID, "playerID", existingBallBringerID.String, "playerName", existingBallBringerName.String)
		return existingBallBringerID.String, existingBallBringerName.String, nil
	}

	// Find the player with the minimum ball_bringer_count among the provided playerIDs
	// Using SQL to find the minimum and then update ensures atomicity for selection and increment.
	query := `
		SELECT id, name
		FROM players
		WHERE id IN (
			?` + strings.Repeat(",?", len(playerIDs)-1) + `
		)
		ORDER BY ball_bringer_count ASC, name ASC -- Order by name for deterministic tie-breaking
		LIMIT 1;
	`
	args := ToAnySlice(playerIDs) // Helper to convert []string to []any

	var selectedPlayerID string
	var selectedPlayerName string
	err = tx.QueryRow(query, args...).Scan(&selectedPlayerID, &selectedPlayerName)
	if err != nil {
		if err == sql.ErrNoRows {
			return "", "", fmt.Errorf("no eligible players found for ball bringer assignment among IDs: %v", playerIDs)
		}
		return "", "", fmt.Errorf("failed to select next ball bringer: %w", err)
	}

	// Atomically increment the selected player's ball bringer count
	_, err = tx.Exec("UPDATE players SET ball_bringer_count = ball_bringer_count + 1 WHERE id = ?", selectedPlayerID)
	if err != nil {
		return "", "", fmt.Errorf("failed to increment ball bringer count for player %s: %w", selectedPlayerID, err)
	}

	// Update the match with the ball bringer's details
	_, err = tx.Exec("UPDATE matches SET ball_bringer_id = ?, ball_bringer_name = ? WHERE id = ?", selectedPlayerID, selectedPlayerName, matchID)
	if err != nil {
		return "", "", fmt.Errorf("failed to update match %s with ball bringer %s: %w", matchID, selectedPlayerID, err)
	}

	if err := tx.Commit(); err != nil {
		return "", "", fmt.Errorf("failed to commit atomic ball bringer assignment transaction: %w", err)
	}

	log.Info("Atomically assigned ball bringer", "matchID", matchID, "playerID", selectedPlayerID, "playerName", selectedPlayerName)
	return selectedPlayerID, selectedPlayerName, nil
}

// GetPlayersSortedByLevel retrieves all players from the database, sorted by their level.
func (s *store) GetPlayersSortedByLevel() ([]PlayerInfo, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	rows, err := s.db.Query("SELECT id, name, ball_bringer_count, level FROM players ORDER BY level DESC")
	if err != nil {
		log.Error("Failed to query all players sorted by level", "error", err)
		return nil, err
	}
	defer rows.Close()

	var players []PlayerInfo
	for rows.Next() {
		var p PlayerInfo
		var name sql.NullString
		var level sql.NullFloat64
		if err := rows.Scan(&p.ID, &name, &p.BallBringerCount, &level); err != nil {
			log.Error("Failed to scan player row", "error", err)
			continue
		}
		p.Name = name.String
		p.Level = level.Float64
		players = append(players, p)
	}
	return players, nil
}

// GetAllMatches retrieves all matches from the database.
func (s *store) GetAllMatches() ([]*playtomic.PadelMatch, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	rows, err := s.db.Query(`
		SELECT id, owner_id, owner_name, start_time, end_time, created_at, status, game_status, results_status, resource_name, access_code, price, tenant_id, tenant_name, match_type, teams_blob, results_blob, ball_bringer_id, ball_bringer_name, processing_status, booking_notified_ts, result_notified_ts
		FROM matches
	`)
	if err != nil {
		log.Error("Failed to query all matches", "error", err)
		return nil, err
	}
	defer rows.Close()

	var matches []*playtomic.PadelMatch
	for rows.Next() {
		match, err := s.scanMatch(rows)
		if err != nil {
			log.Error("Failed to scan match row", "error", err)
			continue
		}
		matches = append(matches, match)
	}
	return matches, nil
}

func ToAnySlice[T any](s []T) []any {
	a := make([]any, len(s))
	for i, v := range s {
		a[i] = v
	}
	return a
}

// Slack mapping methods implementation

// GetPlayerBySlackUserID retrieves a player by their Slack user ID
func (s *store) GetPlayerBySlackUserID(slackUserID string) (*PlayerInfo, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	query := `
		SELECT id, name, level, ball_bringer_count, 
			   slack_user_id, slack_username, slack_display_name, 
			   mapping_status, mapping_confidence, mapping_updated_at
		FROM players 
		WHERE slack_user_id = ?
	`
	
	row := s.db.QueryRow(query, slackUserID)
	
	var player PlayerInfo
	err := row.Scan(
		&player.ID,
		&player.Name,
		&player.Level,
		&player.BallBringerCount,
		&player.SlackUserID,
		&player.SlackUsername,
		&player.SlackDisplayName,
		&player.MappingStatus,
		&player.MappingConfidence,
		&player.MappingUpdatedAt,
	)
	
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil // No player found with this Slack user ID
		}
		return nil, fmt.Errorf("failed to get player by slack user ID: %w", err)
	}
	
	return &player, nil
}

// GetUnmappedPlayers retrieves all players that don't have a Slack mapping
func (s *store) GetUnmappedPlayers() ([]PlayerInfo, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	query := `
		SELECT id, name, level, ball_bringer_count, 
			   slack_user_id, slack_username, slack_display_name, 
			   mapping_status, mapping_confidence, mapping_updated_at
		FROM players 
		WHERE slack_user_id IS NULL OR slack_user_id = ''
		ORDER BY name COLLATE NOCASE
	`
	
	rows, err := s.db.Query(query)
	if err != nil {
		return nil, fmt.Errorf("failed to get unmapped players: %w", err)
	}
	defer rows.Close()
	
	var players []PlayerInfo
	for rows.Next() {
		var player PlayerInfo
		err := rows.Scan(
			&player.ID,
			&player.Name,
			&player.Level,
			&player.BallBringerCount,
			&player.SlackUserID,
			&player.SlackUsername,
			&player.SlackDisplayName,
			&player.MappingStatus,
			&player.MappingConfidence,
			&player.MappingUpdatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan unmapped player: %w", err)
		}
		players = append(players, player)
	}
	
	return players, nil
}

// UpdatePlayerSlackMapping updates a player's Slack mapping information
func (s *store) UpdatePlayerSlackMapping(playerID, slackUserID, slackUsername, slackDisplayName, status string, confidence float64) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	query := `
		UPDATE players 
		SET slack_user_id = ?, 
			slack_username = ?, 
			slack_display_name = ?, 
			mapping_status = ?, 
			mapping_confidence = ?, 
			mapping_updated_at = ?
		WHERE id = ?
	`
	
	now := time.Now().Unix()
	_, err := s.db.Exec(query, slackUserID, slackUsername, slackDisplayName, status, confidence, now, playerID)
	if err != nil {
		return fmt.Errorf("failed to update player Slack mapping: %w", err)
	}
	
	log.Info("Updated player Slack mapping", "player_id", playerID, "slack_user_id", slackUserID, "status", status, "confidence", confidence)
	return nil
}

// FindPlayersByNameSimilarity finds players with names similar to the search term
func (s *store) FindPlayersByNameSimilarity(searchName string) ([]PlayerInfo, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	// Simple similarity search using SQL LIKE - could be enhanced with more sophisticated algorithms
	searchPattern := "%" + strings.ToLower(searchName) + "%"
	
	query := `
		SELECT id, name, level, ball_bringer_count, 
			   slack_user_id, slack_username, slack_display_name, 
			   mapping_status, mapping_confidence, mapping_updated_at
		FROM players 
		WHERE LOWER(name) LIKE ? 
		   OR LOWER(name) LIKE ? 
		   OR LOWER(name) LIKE ?
		ORDER BY 
			CASE 
				WHEN LOWER(name) = LOWER(?) THEN 1  -- Exact match
				WHEN LOWER(name) LIKE LOWER(?) THEN 2  -- Starts with
				ELSE 3  -- Contains
			END,
			name COLLATE NOCASE
	`
	
	// Pattern variations for better matching
	startsWith := strings.ToLower(searchName) + "%"
	endsWith := "%" + strings.ToLower(searchName)
	
	rows, err := s.db.Query(query, searchPattern, startsWith, endsWith, searchName, searchName+"%")
	if err != nil {
		return nil, fmt.Errorf("failed to find players by name similarity: %w", err)
	}
	defer rows.Close()
	
	var players []PlayerInfo
	for rows.Next() {
		var player PlayerInfo
		err := rows.Scan(
			&player.ID,
			&player.Name,
			&player.Level,
			&player.BallBringerCount,
			&player.SlackUserID,
			&player.SlackUsername,
			&player.SlackDisplayName,
			&player.MappingStatus,
			&player.MappingConfidence,
			&player.MappingUpdatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan similar player: %w", err)
		}
		players = append(players, player)
	}
	
	return players, nil
}
