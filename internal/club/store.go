package club

import (
	"database/sql"
	"encoding/json"
	"fmt"

	"github.com/charmbracelet/log"
	"github.com/mauv0809/ideal-tribble/internal/playtomic"
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

	teamsJSON, err := json.Marshal(match.Teams)
	if err != nil {
		tx.Rollback()
		return err
	}
	resultsJSON, err := json.Marshal(match.Results)
	if err != nil {
		tx.Rollback()
		return err
	}

	// This statement is the heart of the "dumb upsert".
	// ON CONFLICT, it updates all fields EXCEPT processing_status.
	stmt, err := tx.Prepare(`
		INSERT INTO matches (id, owner_id, owner_name, start_time, end_time, created_at, status, game_status, results_status, resource_name, access_code, price, tenant_id, tenant_name, match_type, teams_json, results_json, processing_status)
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
			teams_json = excluded.teams_json,
			results_json = excluded.results_json;
	`)
	if err != nil {
		tx.Rollback()
		return err
	}
	defer stmt.Close()

	_, err = stmt.Exec(match.MatchID, match.OwnerID, match.OwnerName, match.Start, match.End, match.CreatedAt, match.Status, match.GameStatus, match.ResultsStatus, match.ResourceName, match.AccessCode, match.Price, match.Tenant.ID, match.Tenant.Name, match.MatchType, teamsJSON, resultsJSON, playtomic.StatusNew)
	if err != nil {
		tx.Rollback()
		return err
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

// GetMatchesForProcessing retrieves all matches that are not yet in a completed state.
func (s *store) GetMatchesForProcessing() ([]*playtomic.PadelMatch, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	rows, err := s.db.Query(`
		SELECT id, owner_id, owner_name, start_time, end_time, created_at, status, game_status, results_status, resource_name, access_code, price, tenant_id, tenant_name, match_type, teams_json, results_json, ball_bringer_id, ball_bringer_name, processing_status
		FROM matches
		WHERE processing_status != ?
	`, playtomic.StatusCompleted)
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
	var teamsJSON, resultsJSON, ballBringerID, ballBringerName sql.NullString

	err := scanner.Scan(
		&match.MatchID, &match.OwnerID, &match.OwnerName, &match.Start, &match.End, &match.CreatedAt,
		&match.Status, &match.GameStatus, &match.ResultsStatus, &match.ResourceName, &match.AccessCode, &match.Price,
		&match.Tenant.ID, &match.Tenant.Name, &match.MatchType, &teamsJSON, &resultsJSON,
		&ballBringerID, &ballBringerName, &match.ProcessingStatus,
	)
	if err != nil {
		return nil, err
	}

	match.BallBringerID = ballBringerID.String
	match.BallBringerName = ballBringerName.String

	if teamsJSON.Valid && teamsJSON.String != "" {
		if err := json.Unmarshal([]byte(teamsJSON.String), &match.Teams); err != nil {
			log.Error("Failed to unmarshal teams_json", "error", err, "matchID", match.MatchID)
		}
	} else {
		match.Teams = []playtomic.Team{}
	}

	if resultsJSON.Valid && resultsJSON.String != "" {
		if err := json.Unmarshal([]byte(resultsJSON.String), &match.Results); err != nil {
			log.Error("Failed to unmarshal results_json", "error", err, "matchID", match.MatchID)
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

	_, err = tx.Exec("DELETE FROM metrics")
	if err != nil {
		log.Error("Failed to clear metrics table", "error", err)
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
		p.Level = float32(level.Float64)
		players = append(players, p)
	}
	return players, nil
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
		p.Level = float32(level.Float64)
		players = append(players, p)
	}
	return players, nil
}

// GetAllMatches retrieves all matches from the database.
func (s *store) GetAllMatches() ([]*playtomic.PadelMatch, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	rows, err := s.db.Query(`
		SELECT id, owner_id, owner_name, start_time, end_time, created_at, status, game_status, results_status, resource_name, access_code, price, tenant_id, tenant_name, match_type, teams_json, results_json, ball_bringer_id, ball_bringer_name, processing_status
		FROM matches ORDER BY start_time DESC
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
