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

	// Use the match type determined by the Playtomic client.
	matchTypeEnum := match.MatchTypeEnum

	// This statement is the heart of the "dumb upsert".
	// ON CONFLICT, it updates all fields EXCEPT processing_status.
	stmt, err := tx.Prepare(`
		INSERT INTO matches (id, owner_id, owner_name, start_time, end_time, created_at, status, game_status, results_status, resource_name, access_code, price, tenant_id, tenant_name, match_type, teams_blob, results_blob, processing_status, match_type_enum)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
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
			results_blob = excluded.results_blob,
			match_type_enum = excluded.match_type_enum;
	`)
	if err != nil {
		tx.Rollback()
		return err
	}
	defer stmt.Close()

	_, err = stmt.Exec(match.MatchID, match.OwnerID, match.OwnerName, match.Start, match.End, match.CreatedAt, match.Status, match.GameStatus, match.ResultsStatus, match.ResourceName, match.AccessCode, match.Price, match.Tenant.ID, match.Tenant.Name, match.MatchType, teamsBlob, resultsBlob, playtomic.StatusNew, matchTypeEnum)
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
		INSERT INTO matches (id, owner_id, owner_name, start_time, end_time, created_at, status, game_status, results_status, resource_name, access_code, price, tenant_id, tenant_name, match_type, teams_blob, results_blob, processing_status, match_type_enum)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
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
			results_blob = excluded.results_blob,
			match_type_enum = excluded.match_type_enum;
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

		// Use the match type determined by the Playtomic client.
		matchTypeEnum := match.MatchTypeEnum

		_, err = stmt.Exec(match.MatchID, match.OwnerID, match.OwnerName, match.Start, match.End, match.CreatedAt, match.Status, match.GameStatus, match.ResultsStatus, match.ResourceName, match.AccessCode, match.Price, match.Tenant.ID, match.Tenant.Name, match.MatchType, teamsBlob, resultsBlob, playtomic.StatusNew, matchTypeEnum)
		if err != nil {
			// Get team sizes for debugging
			var team1Size, team2Size int
			if len(match.Teams) > 0 {
				team1Size = len(match.Teams[0].Players)
			}
			if len(match.Teams) > 1 {
				team2Size = len(match.Teams[1].Players)
			}
			log.Error("Failed to upsert match",
				"matchID", match.MatchID,
				"matchTypeEnum", matchTypeEnum,
				"teamsCount", len(match.Teams),
				"team1Size", team1Size,
				"team2Size", team2Size,
				"ownerName", match.OwnerName,
				"error", err)
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

	var query string
	switch notificationType {
	case "booking":
		query = "UPDATE matches SET booking_notified_ts = ? WHERE id = ?"
	case "result":
		query = "UPDATE matches SET result_notified_ts = ? WHERE id = ?"
	default:
		return fmt.Errorf("invalid notification type: %s", notificationType)
	}

	_, err := s.db.Exec(query, time.Now().Unix(), matchID)
	if err != nil {
		return fmt.Errorf("failed to update %s timestamp for match %s: %w", notificationType, matchID, err)
	}

	// This check is a good practice to ensure the update actually happened.
	// It's commented out as it might be too verbose for this specific use case,
	// but it's a useful pattern.
	// if rowsAffected == 0 {
	// 	log.Warn("Update notification timestamp had no effect, match ID might not exist", "matchID", matchID, "type", notificationType)
	// }
	log.Debug("Successfully updated notification timestamp", "matchID", matchID, "type", notificationType)
	return nil
}

// GetMatchesForProcessing retrieves all matches that are not yet in a completed state.
func (s *store) GetMatchesForProcessing() ([]*playtomic.PadelMatch, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	rows, err := s.db.Query(`
		SELECT id, owner_id, owner_name, start_time, end_time, created_at, status, game_status, results_status, resource_name, access_code, price, tenant_id, tenant_name, match_type, teams_blob, results_blob, ball_bringer_id, ball_bringer_name, processing_status, booking_notified_ts, result_notified_ts, match_type_enum
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
		&bookingNotifiedTs, &resultNotifiedTs, &match.MatchTypeEnum, // Include new fields here
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
	// Use the match type determined by the Playtomic client.
	matchType := match.MatchTypeEnum
	if matchType == "" {
		log.Debug("Skipping stats update for match with undetermined type", "matchID", match.MatchID)
		return
	}

	tx, err := s.db.Begin()
	if err != nil {
		log.Error("Failed to begin transaction for stats update", "error", err, "matchID", match.MatchID)
		return
	}
	defer tx.Rollback() // Rollback on error

	playerStats := aggregateMatchStats(match)

	// Choose the correct table based on match type
	var tableName string
	switch matchType {
	case "SINGLES":
		tableName = "player_stats_singles"
	case "DOUBLES":
		tableName = "player_stats_doubles"
	default:
		log.Error("Unknown match type, skipping stats update", "matchType", matchType, "matchID", match.MatchID)
		return
	}

	stmt, err := tx.Prepare(fmt.Sprintf(`
		INSERT INTO %s (player_id, matches_played, matches_won, matches_lost, sets_won, sets_lost, games_won, games_lost)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(player_id) DO UPDATE SET
			matches_played = matches_played + excluded.matches_played,
			matches_won = matches_won + excluded.matches_won,
			matches_lost = matches_lost + excluded.matches_lost,
			sets_won = sets_won + excluded.sets_won,
			sets_lost = sets_lost + excluded.sets_lost,
			games_won = games_won + excluded.games_won,
			games_lost = games_lost + excluded.games_lost;
	`, tableName))
	if err != nil {
		log.Error("Failed to prepare player_stats statement", "error", err, "table", tableName)
		return
	}
	defer stmt.Close()

	for playerID, stats := range playerStats {
		// Resolve alias to canonical ID (for manual players linked to Playtomic players)
		resolvedID := s.resolvePlayerIDLocked(playerID)
		_, err := stmt.Exec(resolvedID, stats["matches_played"], stats["matches_won"], stats["matches_lost"], stats["sets_won"], stats["sets_lost"], stats["games_won"], stats["games_lost"])
		if err != nil {
			log.Error("Failed to execute player_stats statement", "error", err, "playerID", resolvedID, "table", tableName)
		} else {
			if resolvedID != playerID {
				log.Info("Updated player stats (resolved alias)", "originalID", playerID, "resolvedID", resolvedID, "matchType", matchType, "table", tableName)
			} else {
				log.Info("Updated player stats", "playerID", resolvedID, "matchType", matchType, "table", tableName)
			}
		}
	}

	if err := tx.Commit(); err != nil {
		log.Error("Failed to commit player_stats transaction", "error", err)
	}
}

// UpdateWeeklyStats updates the weekly performance snapshot for each player in a match.
func (s *store) UpdateWeeklyStats(match *playtomic.PadelMatch) {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Use the match type determined by the Playtomic client.
	matchTypeEnum := match.MatchTypeEnum
	if matchTypeEnum == "" {
		log.Debug("Skipping weekly stats update for match with undetermined type", "matchID", match.MatchID)
		return
	}

	// Calculate the start of the week for this match
	if match.End == 0 {
		log.Warn("Skipping weekly stats update for match with zero end time", "matchID", match.MatchID)
		return
	}
	weekStartDate := getWeekStartDate(match.End)

	tx, err := s.db.Begin()
	if err != nil {
		log.Error("Failed to begin transaction for weekly stats update", "error", err, "matchID", match.MatchID)
		return
	}
	defer tx.Rollback() // Rollback on error

	playerStats := aggregateMatchStats(match)

	// Prepare the upsert statement for weekly_player_stats
	stmt, err := tx.Prepare(`
		INSERT INTO weekly_player_stats (week_start_date, player_id, match_type_enum, matches_played, matches_won, matches_lost, sets_won, sets_lost, games_won, games_lost)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(week_start_date, player_id, match_type_enum) DO UPDATE SET
			matches_played = matches_played + excluded.matches_played,
			matches_won = matches_won + excluded.matches_won,
			matches_lost = matches_lost + excluded.matches_lost,
			sets_won = sets_won + excluded.sets_won,
			sets_lost = sets_lost + excluded.sets_lost,
			games_won = games_won + excluded.games_won,
			games_lost = games_lost + excluded.games_lost;
	`)
	if err != nil {
		log.Error("Failed to prepare weekly_player_stats statement", "error", err, "matchID", match.MatchID)
		return
	}
	defer stmt.Close()

	for playerID, stats := range playerStats {
		// Resolve alias to canonical ID (for manual players linked to Playtomic players)
		resolvedID := s.resolvePlayerIDLocked(playerID)
		_, err := stmt.Exec(
			weekStartDate,
			resolvedID,
			matchTypeEnum,
			stats["matches_played"],
			stats["matches_won"],
			stats["matches_lost"],
			stats["sets_won"],
			stats["sets_lost"],
			stats["games_won"],
			stats["games_lost"],
		)
		if err != nil {
			log.Error("Failed to execute weekly_player_stats statement", "error", err, "playerID", resolvedID, "week", weekStartDate)
		} else {
			log.Info("Updated weekly player stats", "playerID", resolvedID, "week", weekStartDate, "matchType", matchTypeEnum)
		}
	}

	if err := tx.Commit(); err != nil {
		log.Error("Failed to commit weekly_player_stats transaction", "error", err)
	}
}

// aggregateMatchStats calculates per-player stats for a single match.
func aggregateMatchStats(match *playtomic.PadelMatch) map[string]map[string]int {
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

	// Use the shared utility to calculate set/game stats per team
	teamStats := playtomic.CalculateAllTeamsSetGameStats(match)

	// Map team stats to individual players
	for _, team := range match.Teams {
		stats := teamStats[team.ID]
		for _, player := range team.Players {
			playerStats[player.UserID]["sets_won"] += stats.SetsWon
			playerStats[player.UserID]["sets_lost"] += stats.SetsLost
			playerStats[player.UserID]["games_won"] += stats.GamesWon
			playerStats[player.UserID]["games_lost"] += stats.GamesLost
		}
	}

	return playerStats
}

// getWeekStartDate returns the Unix timestamp for the start of the week (Sunday 00:00:00)
// for a given timestamp.
func getWeekStartDate(timestamp int64) int64 {
	t := time.Unix(timestamp, 0).UTC() // Use UTC for consistency
	weekday := t.Weekday()             // Sunday = 0, Monday = 1, ...

	// Truncate to the beginning of the day
	startOfDay := t.Truncate(24 * time.Hour)

	// Subtract days to get to the previous Sunday
	startOfWeek := startOfDay.AddDate(0, 0, -int(weekday))

	return startOfWeek.Unix()
}

// GetPlayerStatsByName retrieves the statistics for a single player by their name.
// It performs a case-insensitive, fuzzy search (e.g., "morten" will match "Morten Voss").
// The matchType can be "SINGLES", "DOUBLES", or "ALL" for combined stats.
func (s *store) GetPlayerStatsByName(playerName string, matchType playtomic.MatchTypeEnum) (*PlayerStats, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var query string
	var fromClause string

	switch matchType {
	case playtomic.MatchTypeEnumSingles:
		fromClause = "player_stats_singles"
		query = `SELECT p.id, p.name, COALESCE(s.matches_played, 0), COALESCE(s.matches_won, 0), COALESCE(s.matches_lost, 0), COALESCE(s.sets_won, 0), COALESCE(s.sets_lost, 0), COALESCE(s.games_won, 0), COALESCE(s.games_lost, 0)
                 FROM players p LEFT JOIN %s s ON p.id = s.player_id WHERE p.name LIKE ? COLLATE NOCASE LIMIT 1`
		query = fmt.Sprintf(query, fromClause)
	case playtomic.MatchTypeEnumDoubles:
		fromClause = "player_stats_doubles"
		query = `SELECT p.id, p.name, COALESCE(s.matches_played, 0), COALESCE(s.matches_won, 0), COALESCE(s.matches_lost, 0), COALESCE(s.sets_won, 0), COALESCE(s.sets_lost, 0), COALESCE(s.games_won, 0), COALESCE(s.games_lost, 0)
                 FROM players p LEFT JOIN %s s ON p.id = s.player_id WHERE p.name LIKE ? COLLATE NOCASE LIMIT 1`
		query = fmt.Sprintf(query, fromClause)
	default: // MatchTypeEnumAll or empty
		query = `SELECT p.id, p.name, COALESCE(SUM(s.matches_played), 0), COALESCE(SUM(s.matches_won), 0), COALESCE(SUM(s.matches_lost), 0), COALESCE(SUM(s.sets_won), 0), COALESCE(SUM(s.sets_lost), 0), COALESCE(SUM(s.games_won), 0), COALESCE(SUM(s.games_lost), 0)
                 FROM players p LEFT JOIN (
                     SELECT player_id, matches_played, matches_won, matches_lost, sets_won, sets_lost, games_won, games_lost FROM player_stats_singles
                     UNION ALL
                     SELECT player_id, matches_played, matches_won, matches_lost, sets_won, sets_lost, games_won, games_lost FROM player_stats_doubles
                 ) s ON p.id = s.player_id
                 WHERE p.name LIKE ? COLLATE NOCASE GROUP BY p.id, p.name LIMIT 1`
	}

	var stat PlayerStats
	// Use a fuzzy search pattern.
	pattern := "%" + playerName + "%"

	row := s.db.QueryRow(query, pattern)
	err := row.Scan( // The GROUP BY p.id, p.name makes this safe without an aggregate on name
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

func (s *store) GetPlayerStats(matchType playtomic.MatchTypeEnum) ([]PlayerStats, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var query string
	switch matchType {
	case playtomic.MatchTypeEnumSingles:
		query = `SELECT p.id, p.name, s.matches_played, s.matches_won, s.matches_lost, s.sets_won, s.sets_lost, s.games_won, s.games_lost
                 FROM players p JOIN player_stats_singles s ON p.id = s.player_id
                 WHERE s.matches_played > 0 ORDER BY s.matches_won DESC, s.sets_won DESC, s.games_won DESC`
	case playtomic.MatchTypeEnumDoubles:
		query = `SELECT p.id, p.name, s.matches_played, s.matches_won, s.matches_lost, s.sets_won, s.sets_lost, s.games_won, s.games_lost
                 FROM players p JOIN player_stats_doubles s ON p.id = s.player_id
                 WHERE s.matches_played > 0 ORDER BY s.matches_won DESC, s.sets_won DESC, s.games_won DESC`
	default: // MatchTypeEnumAll or empty
		query = `SELECT p.id, p.name,
                   COALESCE(SUM(s.matches_played), 0) as total_matches_played,
                   COALESCE(SUM(s.matches_won), 0) as total_matches_won,
                   COALESCE(SUM(s.matches_lost), 0) as total_matches_lost,
                   COALESCE(SUM(s.sets_won), 0) as total_sets_won,
                   COALESCE(SUM(s.sets_lost), 0) as total_sets_lost,
                   COALESCE(SUM(s.games_won), 0) as total_games_won,
                   COALESCE(SUM(s.games_lost), 0) as total_games_lost
                 FROM players p LEFT JOIN (
                     SELECT player_id, matches_played, matches_won, matches_lost, sets_won, sets_lost, games_won, games_lost FROM player_stats_singles
                     UNION ALL
                     SELECT player_id, matches_played, matches_won, matches_lost, sets_won, sets_lost, games_won, games_lost FROM player_stats_doubles
                 ) s ON p.id = s.player_id
                 WHERE COALESCE(s.matches_played, 0) > 0
                 GROUP BY p.id, p.name
                 ORDER BY total_matches_won DESC, total_sets_won DESC, total_games_won DESC`
	}

	rows, err := s.db.Query(query)
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

	// Use INSERT...ON CONFLICT to perform an atomic "upsert".
	// This is safer than the previous "check-then-act" pattern and prevents race conditions.
	stmt, err := s.db.Prepare(`
		INSERT INTO players (id, name, level)
		VALUES (?, ?, ?)
		ON CONFLICT(id) DO UPDATE SET
			name = excluded.name,
			level = excluded.level;
	`)
	if err != nil {
		log.Error("Failed to prepare upsert player statement", "error", err)
		return
	}
	defer stmt.Close()
	if _, err := stmt.Exec(playerID, name, level); err != nil {
		log.Error("Failed to upsert player", "error", err, "playerID", playerID)
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
	rows, err := s.db.Query("SELECT id, name, ball_bringer_count_singles, ball_bringer_count_doubles, last_ball_boy_date_singles, last_ball_boy_date_doubles, booking_count_singles, booking_count_doubles, level FROM players ORDER BY name")
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
		var lastBallBoyDateSingles, lastBallBoyDateDoubles sql.NullInt64
		if err := rows.Scan(&p.ID, &name, &p.BallBringerCountSingles, &p.BallBringerCountDoubles, &lastBallBoyDateSingles, &lastBallBoyDateDoubles, &p.BookingCountSingles, &p.BookingCountDoubles, &level); err != nil {
			log.Error("Failed to scan player row", "error", err)
			continue
		}
		p.Name = name.String // handle NULL name from db
		p.Level = level.Float64
		if lastBallBoyDateSingles.Valid {
			p.LastBallBoyDateSingles = &lastBallBoyDateSingles.Int64
		}
		if lastBallBoyDateDoubles.Valid {
			p.LastBallBoyDateDoubles = &lastBallBoyDateDoubles.Int64
		}
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

	query := "SELECT id, name, ball_bringer_count_singles, ball_bringer_count_doubles, last_ball_boy_date_singles, last_ball_boy_date_doubles, booking_count_singles, booking_count_doubles, level FROM players WHERE id IN (?" + strings.Repeat(",?", len(playerIDs)-1) + ")"
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
		var lastBallBoyDateSingles, lastBallBoyDateDoubles sql.NullInt64
		if err := rows.Scan(&p.ID, &name, &p.BallBringerCountSingles, &p.BallBringerCountDoubles, &lastBallBoyDateSingles, &lastBallBoyDateDoubles, &p.BookingCountSingles, &p.BookingCountDoubles, &level); err != nil {
			log.Error("Failed to scan player row", "error", err)
			continue // Or handle error more gracefully
		}
		p.Name = name.String
		p.Level = level.Float64
		if lastBallBoyDateSingles.Valid {
			p.LastBallBoyDateSingles = &lastBallBoyDateSingles.Int64
		}
		if lastBallBoyDateDoubles.Valid {
			p.LastBallBoyDateDoubles = &lastBallBoyDateDoubles.Int64
		}
		players = append(players, p)
	}
	return players, nil
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

	// Get match type and check if a ball bringer is already assigned
	var existingBallBringerID, existingBallBringerName, matchTypeEnum sql.NullString
	err = tx.QueryRow("SELECT ball_bringer_id, ball_bringer_name, match_type_enum FROM matches WHERE id = ?", matchID).Scan(&existingBallBringerID, &existingBallBringerName, &matchTypeEnum)
	if err != nil && err != sql.ErrNoRows {
		return "", "", fmt.Errorf("failed to query match details for %s: %w", matchID, err)
	}

	if existingBallBringerID.Valid && existingBallBringerName.Valid {
		log.Info("Ball bringer already assigned for match. Returning existing assignment.", "matchID", matchID, "playerID", existingBallBringerID.String, "playerName", existingBallBringerName.String)
		return existingBallBringerID.String, existingBallBringerName.String, nil
	}

	if !matchTypeEnum.Valid || (matchTypeEnum.String != "SINGLES" && matchTypeEnum.String != "DOUBLES") {
		return "", "", fmt.Errorf("cannot assign ball bringer for match %s with invalid type: %s", matchID, matchTypeEnum.String)
	}

	var countColumn, dateColumn string
	if matchTypeEnum.String == "SINGLES" {
		countColumn = "ball_bringer_count_singles"
		dateColumn = "last_ball_boy_date_singles"
	} else {
		countColumn = "ball_bringer_count_doubles"
		dateColumn = "last_ball_boy_date_doubles"
	}

	// Find the player using time-based fairness:
	// 1. Primary: Who hasn't been ball boy for the longest time (NULL sorts first = new players)
	// 2. Tiebreaker 1: Lowest total count (handles ties and provides secondary fairness)
	// 3. Tiebreaker 2: Alphabetical by name (deterministic)
	query := fmt.Sprintf(`
		SELECT id, name
		FROM players
		WHERE id IN (?`+strings.Repeat(",?", len(playerIDs)-1)+`)
		ORDER BY %s ASC, %s ASC, name ASC
		LIMIT 1;
	`, dateColumn, countColumn)

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

	// Atomically increment the selected player's count and update the last ball boy date
	// Using unixepoch('now') for SQLite to get current Unix timestamp
	updateQuery := fmt.Sprintf("UPDATE players SET %s = %s + 1, %s = unixepoch('now') WHERE id = ?", countColumn, countColumn, dateColumn)
	_, err = tx.Exec(updateQuery, selectedPlayerID)
	if err != nil {
		return "", "", fmt.Errorf("failed to increment ball bringer count and update date for player %s: %w", selectedPlayerID, err)
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

// AssignBookingResponsibleAtomically finds the player with the minimum booking count among the given player IDs,
// increments their booking count, and returns their ID and name.
func (s *store) AssignBookingResponsibleAtomically(playerIDs []string) (string, string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if len(playerIDs) == 0 {
		return "", "", fmt.Errorf("no player IDs provided")
	}

	tx, err := s.db.Begin()
	if err != nil {
		return "", "", fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback()

	// For now, we'll use doubles booking count since most matches are doubles
	// In the future, we could add a match type parameter
	placeholders := strings.Repeat("?,", len(playerIDs))
	placeholders = placeholders[:len(placeholders)-1] // Remove trailing comma

	query := fmt.Sprintf(`
		SELECT id, name, booking_count_doubles 
		FROM players 
		WHERE id IN (%s) 
		ORDER BY booking_count_doubles ASC, id ASC
		LIMIT 1
	`, placeholders)

	args := make([]interface{}, len(playerIDs))
	for i, id := range playerIDs {
		args[i] = id
	}

	var bookingResponsibleID, bookingResponsibleName string
	var currentCount int
	err = tx.QueryRow(query, args...).Scan(&bookingResponsibleID, &bookingResponsibleName, &currentCount)
	if err != nil {
		return "", "", fmt.Errorf("failed to find booking responsible player: %w", err)
	}

	// Increment the booking count
	updateQuery := `UPDATE players SET booking_count_doubles = booking_count_doubles + 1 WHERE id = ?`
	_, err = tx.Exec(updateQuery, bookingResponsibleID)
	if err != nil {
		return "", "", fmt.Errorf("failed to increment booking count: %w", err)
	}

	if err = tx.Commit(); err != nil {
		return "", "", fmt.Errorf("failed to commit booking assignment transaction: %w", err)
	}

	log.Info("Assigned booking responsibility atomically", 
		"playerID", bookingResponsibleID, 
		"playerName", bookingResponsibleName, 
		"previousCount", currentCount,
		"newCount", currentCount+1)

	return bookingResponsibleID, bookingResponsibleName, nil
}

// GetPlayersSortedByLevel retrieves all players from the database, sorted by their level.
func (s *store) GetPlayersSortedByLevel() ([]PlayerInfo, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	rows, err := s.db.Query("SELECT id, name, ball_bringer_count_singles, ball_bringer_count_doubles, last_ball_boy_date_singles, last_ball_boy_date_doubles, booking_count_singles, booking_count_doubles, level FROM players ORDER BY level DESC")
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
		var lastBallBoyDateSingles, lastBallBoyDateDoubles sql.NullInt64
		if err := rows.Scan(&p.ID, &name, &p.BallBringerCountSingles, &p.BallBringerCountDoubles, &lastBallBoyDateSingles, &lastBallBoyDateDoubles, &p.BookingCountSingles, &p.BookingCountDoubles, &level); err != nil {
			log.Error("Failed to scan player row", "error", err)
			continue
		}
		p.Name = name.String
		p.Level = level.Float64
		if lastBallBoyDateSingles.Valid {
			p.LastBallBoyDateSingles = &lastBallBoyDateSingles.Int64
		}
		if lastBallBoyDateDoubles.Valid {
			p.LastBallBoyDateDoubles = &lastBallBoyDateDoubles.Int64
		}
		players = append(players, p)
	}
	return players, nil
}

// GetAllMatches retrieves all matches from the database.
func (s *store) GetAllMatches() ([]*playtomic.PadelMatch, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	rows, err := s.db.Query(`
		SELECT id, owner_id, owner_name, start_time, end_time, created_at, status, game_status, results_status, resource_name, access_code, price, tenant_id, tenant_name, match_type, teams_blob, results_blob, ball_bringer_id, ball_bringer_name, processing_status, booking_notified_ts, result_notified_ts, match_type_enum
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
		SELECT id, name, level, ball_bringer_count_singles, ball_bringer_count_doubles,
			   last_ball_boy_date_singles, last_ball_boy_date_doubles,
			   booking_count_singles, booking_count_doubles,
			   slack_user_id, slack_username, slack_display_name,
			   mapping_status, mapping_confidence, mapping_updated_at
		FROM players
		WHERE slack_user_id = ?
	`

	row := s.db.QueryRow(query, slackUserID)

	var player PlayerInfo
	var lastBallBoyDateSingles, lastBallBoyDateDoubles sql.NullInt64
	err := row.Scan(
		&player.ID,
		&player.Name,
		&player.Level,
		&player.BallBringerCountSingles,
		&player.BallBringerCountDoubles,
		&lastBallBoyDateSingles,
		&lastBallBoyDateDoubles,
		&player.BookingCountSingles,
		&player.BookingCountDoubles,
		&player.SlackUserID,
		&player.SlackUsername,
		&player.SlackDisplayName,
		&player.MappingStatus,
		&player.MappingConfidence,
		&player.MappingUpdatedAt,
	)

	if lastBallBoyDateSingles.Valid {
		player.LastBallBoyDateSingles = &lastBallBoyDateSingles.Int64
	}
	if lastBallBoyDateDoubles.Valid {
		player.LastBallBoyDateDoubles = &lastBallBoyDateDoubles.Int64
	}

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
		SELECT id, name, level, ball_bringer_count_singles, ball_bringer_count_doubles,
			   last_ball_boy_date_singles, last_ball_boy_date_doubles,
			   booking_count_singles, booking_count_doubles,
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
		var lastBallBoyDateSingles, lastBallBoyDateDoubles sql.NullInt64
		err := rows.Scan(
			&player.ID,
			&player.Name,
			&player.Level,
			&player.BallBringerCountSingles,
			&player.BallBringerCountDoubles,
			&lastBallBoyDateSingles,
			&lastBallBoyDateDoubles,
			&player.BookingCountSingles,
			&player.BookingCountDoubles,
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
		if lastBallBoyDateSingles.Valid {
			player.LastBallBoyDateSingles = &lastBallBoyDateSingles.Int64
		}
		if lastBallBoyDateDoubles.Valid {
			player.LastBallBoyDateDoubles = &lastBallBoyDateDoubles.Int64
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
		SELECT id, name, level, ball_bringer_count_singles, ball_bringer_count_doubles,
			   last_ball_boy_date_singles, last_ball_boy_date_doubles,
			   booking_count_singles, booking_count_doubles,
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
		var lastBallBoyDateSingles, lastBallBoyDateDoubles sql.NullInt64
		err := rows.Scan(
			&player.ID,
			&player.Name,
			&player.Level,
			&player.BallBringerCountSingles,
			&player.BallBringerCountDoubles,
			&lastBallBoyDateSingles,
			&lastBallBoyDateDoubles,
			&player.BookingCountSingles,
			&player.BookingCountDoubles,
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
		if lastBallBoyDateSingles.Valid {
			player.LastBallBoyDateSingles = &lastBallBoyDateSingles.Int64
		}
		if lastBallBoyDateDoubles.Valid {
			player.LastBallBoyDateDoubles = &lastBallBoyDateDoubles.Int64
		}
		players = append(players, player)
	}

	return players, nil
}

// Player alias methods for manual player mapping

// CreatePlayerAlias creates a new player alias record for a manual player
func (s *store) CreatePlayerAlias(manualID, manualName string) (*PlayerAlias, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	now := time.Now().Unix()
	result, err := s.db.Exec(`
		INSERT INTO player_aliases (manual_player_id, manual_player_name, created_at, updated_at)
		VALUES (?, ?, ?, ?)
	`, manualID, manualName, now, now)
	if err != nil {
		return nil, fmt.Errorf("failed to create player alias: %w", err)
	}

	id, err := result.LastInsertId()
	if err != nil {
		return nil, fmt.Errorf("failed to get last insert id: %w", err)
	}

	return &PlayerAlias{
		ID:               id,
		ManualPlayerID:   manualID,
		ManualPlayerName: manualName,
		Confirmed:        false,
		CreatedAt:        now,
		UpdatedAt:        now,
	}, nil
}

// GetPlayerAlias retrieves a player alias by manual player ID
func (s *store) GetPlayerAlias(manualID string) (*PlayerAlias, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var alias PlayerAlias
	var playtomicID, playtomicName sql.NullString
	var confidence sql.NullFloat64
	var confirmed int

	err := s.db.QueryRow(`
		SELECT id, manual_player_id, manual_player_name, playtomic_player_id, playtomic_player_name, confirmed, confidence, created_at, updated_at
		FROM player_aliases
		WHERE manual_player_id = ?
	`, manualID).Scan(
		&alias.ID,
		&alias.ManualPlayerID,
		&alias.ManualPlayerName,
		&playtomicID,
		&playtomicName,
		&confirmed,
		&confidence,
		&alias.CreatedAt,
		&alias.UpdatedAt,
	)

	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to get player alias: %w", err)
	}

	alias.Confirmed = confirmed == 1
	if playtomicID.Valid {
		alias.PlaytomicPlayerID = &playtomicID.String
	}
	if playtomicName.Valid {
		alias.PlaytomicPlayerName = &playtomicName.String
	}
	if confidence.Valid {
		alias.Confidence = &confidence.Float64
	}

	return &alias, nil
}

// GetAllPlayerAliases retrieves all player aliases
func (s *store) GetAllPlayerAliases() ([]PlayerAlias, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	rows, err := s.db.Query(`
		SELECT id, manual_player_id, manual_player_name, playtomic_player_id, playtomic_player_name, confirmed, confidence, created_at, updated_at
		FROM player_aliases
		ORDER BY manual_player_name COLLATE NOCASE
	`)
	if err != nil {
		return nil, fmt.Errorf("failed to get all player aliases: %w", err)
	}
	defer rows.Close()

	var aliases []PlayerAlias
	for rows.Next() {
		var alias PlayerAlias
		var playtomicID, playtomicName sql.NullString
		var confidence sql.NullFloat64
		var confirmed int

		err := rows.Scan(
			&alias.ID,
			&alias.ManualPlayerID,
			&alias.ManualPlayerName,
			&playtomicID,
			&playtomicName,
			&confirmed,
			&confidence,
			&alias.CreatedAt,
			&alias.UpdatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan player alias: %w", err)
		}

		alias.Confirmed = confirmed == 1
		if playtomicID.Valid {
			alias.PlaytomicPlayerID = &playtomicID.String
		}
		if playtomicName.Valid {
			alias.PlaytomicPlayerName = &playtomicName.String
		}
		if confidence.Valid {
			alias.Confidence = &confidence.Float64
		}

		aliases = append(aliases, alias)
	}

	return aliases, nil
}

// GetUnlinkedAliases retrieves all player aliases that are not linked to a Playtomic player
func (s *store) GetUnlinkedAliases() ([]PlayerAlias, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	rows, err := s.db.Query(`
		SELECT id, manual_player_id, manual_player_name, playtomic_player_id, playtomic_player_name, confirmed, confidence, created_at, updated_at
		FROM player_aliases
		WHERE playtomic_player_id IS NULL OR confirmed = 0
		ORDER BY manual_player_name COLLATE NOCASE
	`)
	if err != nil {
		return nil, fmt.Errorf("failed to get unlinked aliases: %w", err)
	}
	defer rows.Close()

	var aliases []PlayerAlias
	for rows.Next() {
		var alias PlayerAlias
		var playtomicID, playtomicName sql.NullString
		var confidence sql.NullFloat64
		var confirmed int

		err := rows.Scan(
			&alias.ID,
			&alias.ManualPlayerID,
			&alias.ManualPlayerName,
			&playtomicID,
			&playtomicName,
			&confirmed,
			&confidence,
			&alias.CreatedAt,
			&alias.UpdatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan unlinked alias: %w", err)
		}

		alias.Confirmed = confirmed == 1
		if playtomicID.Valid {
			alias.PlaytomicPlayerID = &playtomicID.String
		}
		if playtomicName.Valid {
			alias.PlaytomicPlayerName = &playtomicName.String
		}
		if confidence.Valid {
			alias.Confidence = &confidence.Float64
		}

		aliases = append(aliases, alias)
	}

	return aliases, nil
}

// LinkPlayerAlias links a manual player to a Playtomic player
func (s *store) LinkPlayerAlias(manualID, playtomicID, playtomicName string, confirmed bool, confidence float64) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	confirmedInt := 0
	if confirmed {
		confirmedInt = 1
	}

	now := time.Now().Unix()
	result, err := s.db.Exec(`
		UPDATE player_aliases
		SET playtomic_player_id = ?, playtomic_player_name = ?, confirmed = ?, confidence = ?, updated_at = ?
		WHERE manual_player_id = ?
	`, playtomicID, playtomicName, confirmedInt, confidence, now, manualID)
	if err != nil {
		return fmt.Errorf("failed to link player alias: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}
	if rowsAffected == 0 {
		return fmt.Errorf("no alias found with manual_player_id: %s", manualID)
	}

	log.Info("Linked player alias", "manualID", manualID, "playtomicID", playtomicID, "confirmed", confirmed)
	return nil
}

// UnlinkPlayerAlias removes the link between a manual player and a Playtomic player
func (s *store) UnlinkPlayerAlias(manualID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	now := time.Now().Unix()
	result, err := s.db.Exec(`
		UPDATE player_aliases
		SET playtomic_player_id = NULL, playtomic_player_name = NULL, confirmed = 0, confidence = NULL, updated_at = ?
		WHERE manual_player_id = ?
	`, now, manualID)
	if err != nil {
		return fmt.Errorf("failed to unlink player alias: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}
	if rowsAffected == 0 {
		return fmt.Errorf("no alias found with manual_player_id: %s", manualID)
	}

	log.Info("Unlinked player alias", "manualID", manualID)
	return nil
}

// ResolvePlayerID returns the canonical player ID for stats aggregation.
// If the playerID is a manual player with a confirmed alias, return the linked Playtomic ID.
// Otherwise, return the original playerID.
func (s *store) ResolvePlayerID(playerID string) string {
	if !strings.HasPrefix(playerID, "manual_") {
		return playerID // Not a manual player, no resolution needed
	}

	alias, err := s.GetPlayerAlias(playerID)
	if err != nil || alias == nil || !alias.Confirmed || alias.PlaytomicPlayerID == nil {
		return playerID // Keep manual ID
	}
	return *alias.PlaytomicPlayerID // Return linked Playtomic ID
}

// resolvePlayerIDLocked resolves a player ID without acquiring locks (caller must hold lock)
func (s *store) resolvePlayerIDLocked(playerID string) string {
	if !strings.HasPrefix(playerID, "manual_") {
		return playerID // Not a manual player, no resolution needed
	}

	var playtomicID sql.NullString
	var confirmed int
	err := s.db.QueryRow(`
		SELECT playtomic_player_id, confirmed
		FROM player_aliases
		WHERE manual_player_id = ?
	`, playerID).Scan(&playtomicID, &confirmed)

	if err != nil || !playtomicID.Valid || confirmed != 1 {
		return playerID // Keep manual ID
	}
	return playtomicID.String // Return linked Playtomic ID
}

// SuggestPlayersForName finds players with names similar to the input and returns suggestions
func (s *store) SuggestPlayersForName(name string, limit int) ([]PlayerSuggestion, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if name == "" || limit <= 0 {
		return nil, nil
	}

	// Get all players (Playtomic players)
	allPlayers, err := s.getAllPlayersLocked()
	if err != nil {
		return nil, fmt.Errorf("failed to get all players: %w", err)
	}

	// Get all manual players (from aliases)
	aliases, err := s.getAllPlayerAliasesLocked()
	if err != nil {
		return nil, fmt.Errorf("failed to get all aliases: %w", err)
	}

	// Calculate similarity for each player
	var suggestions []PlayerSuggestion
	normalizedInput := normalizeName(name)

	// Check Playtomic players
	for _, player := range allPlayers {
		normalizedPlayer := normalizeName(player.Name)
		confidence := stringSimilarity(normalizedInput, normalizedPlayer)
		if confidence > 0.3 { // Minimum threshold
			suggestions = append(suggestions, PlayerSuggestion{
				Player:     player,
				Confidence: confidence,
				Reasons:    getMatchReasons(normalizedInput, normalizedPlayer),
			})
		}
	}

	// Check manual players (aliases)
	for _, alias := range aliases {
		normalizedAlias := normalizeName(alias.ManualPlayerName)
		confidence := stringSimilarity(normalizedInput, normalizedAlias)
		if confidence > 0.3 { // Minimum threshold
			// Create a PlayerInfo from the alias
			player := PlayerInfo{
				ID:   alias.ManualPlayerID,
				Name: alias.ManualPlayerName,
			}
			suggestions = append(suggestions, PlayerSuggestion{
				Player:     player,
				Confidence: confidence,
				Reasons:    getMatchReasons(normalizedInput, normalizedAlias),
			})
		}
	}

	// Sort by confidence descending
	for i := 0; i < len(suggestions)-1; i++ {
		for j := i + 1; j < len(suggestions); j++ {
			if suggestions[j].Confidence > suggestions[i].Confidence {
				suggestions[i], suggestions[j] = suggestions[j], suggestions[i]
			}
		}
	}

	// Return top N
	if len(suggestions) > limit {
		suggestions = suggestions[:limit]
	}

	return suggestions, nil
}

// getAllPlayersLocked retrieves all players without acquiring lock (caller must hold lock)
func (s *store) getAllPlayersLocked() ([]PlayerInfo, error) {
	rows, err := s.db.Query("SELECT id, name, level FROM players ORDER BY name")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var players []PlayerInfo
	for rows.Next() {
		var p PlayerInfo
		var name sql.NullString
		var level sql.NullFloat64
		if err := rows.Scan(&p.ID, &name, &level); err != nil {
			continue
		}
		p.Name = name.String
		p.Level = level.Float64
		players = append(players, p)
	}
	return players, nil
}

// getAllPlayerAliasesLocked retrieves all aliases without acquiring lock (caller must hold lock)
func (s *store) getAllPlayerAliasesLocked() ([]PlayerAlias, error) {
	rows, err := s.db.Query(`
		SELECT id, manual_player_id, manual_player_name, playtomic_player_id, playtomic_player_name, confirmed, confidence, created_at, updated_at
		FROM player_aliases
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var aliases []PlayerAlias
	for rows.Next() {
		var alias PlayerAlias
		var playtomicID, playtomicName sql.NullString
		var confidence sql.NullFloat64
		var confirmed int

		if err := rows.Scan(
			&alias.ID,
			&alias.ManualPlayerID,
			&alias.ManualPlayerName,
			&playtomicID,
			&playtomicName,
			&confirmed,
			&confidence,
			&alias.CreatedAt,
			&alias.UpdatedAt,
		); err != nil {
			continue
		}

		alias.Confirmed = confirmed == 1
		if playtomicID.Valid {
			alias.PlaytomicPlayerID = &playtomicID.String
		}
		if playtomicName.Valid {
			alias.PlaytomicPlayerName = &playtomicName.String
		}
		if confidence.Valid {
			alias.Confidence = &confidence.Float64
		}

		aliases = append(aliases, alias)
	}

	return aliases, nil
}

// normalizeName normalizes a name for comparison (lowercase, remove special chars)
func normalizeName(name string) string {
	name = strings.TrimSpace(name)
	name = strings.ToLower(name)
	var result strings.Builder
	for _, r := range name {
		if (r >= 'a' && r <= 'z') || r == ' ' {
			result.WriteRune(r)
		}
	}
	return strings.Join(strings.Fields(result.String()), " ")
}

// stringSimilarity calculates similarity between two strings using Levenshtein distance
func stringSimilarity(s1, s2 string) float64 {
	if s1 == s2 {
		return 1.0
	}
	if s1 == "" || s2 == "" {
		return 0.0
	}

	distance := levenshteinDistance(s1, s2)
	maxLen := len(s1)
	if len(s2) > maxLen {
		maxLen = len(s2)
	}
	if maxLen == 0 {
		return 1.0
	}

	return 1.0 - float64(distance)/float64(maxLen)
}

// levenshteinDistance calculates the Levenshtein distance between two strings
func levenshteinDistance(s1, s2 string) int {
	if s1 == s2 {
		return 0
	}
	if len(s1) == 0 {
		return len(s2)
	}
	if len(s2) == 0 {
		return len(s1)
	}

	matrix := make([][]int, len(s1)+1)
	for i := range matrix {
		matrix[i] = make([]int, len(s2)+1)
	}

	for i := 0; i <= len(s1); i++ {
		matrix[i][0] = i
	}
	for j := 0; j <= len(s2); j++ {
		matrix[0][j] = j
	}

	for i := 1; i <= len(s1); i++ {
		for j := 1; j <= len(s2); j++ {
			cost := 0
			if s1[i-1] != s2[j-1] {
				cost = 1
			}
			deletion := matrix[i-1][j] + 1
			insertion := matrix[i][j-1] + 1
			substitution := matrix[i-1][j-1] + cost
			matrix[i][j] = minInt(deletion, minInt(insertion, substitution))
		}
	}

	return matrix[len(s1)][len(s2)]
}

func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// getMatchReasons provides human-readable reasons for the match
func getMatchReasons(input, target string) []string {
	var reasons []string
	if input == target {
		reasons = append(reasons, "Exact match")
	} else if strings.HasPrefix(target, input) || strings.HasPrefix(input, target) {
		reasons = append(reasons, "Name starts with")
	} else if strings.Contains(target, input) || strings.Contains(input, target) {
		reasons = append(reasons, "Name contains")
	} else {
		reasons = append(reasons, "Similar name")
	}
	return reasons
}

// CreateManualMatch creates a new manually entered match
func (s *store) CreateManualMatch(input *ManualMatchInput, createdBy string) (*playtomic.PadelMatch, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Generate a unique match ID with manual_ prefix
	matchID := fmt.Sprintf("manual_%d", time.Now().UnixNano())

	// Process players and create aliases for new ones
	team1Players := make([]playtomic.Player, len(input.Team1Players))
	team2Players := make([]playtomic.Player, len(input.Team2Players))

	for i, p := range input.Team1Players {
		playerID, err := s.getOrCreateManualPlayerLocked(p.ID, p.Name)
		if err != nil {
			return nil, fmt.Errorf("failed to process team1 player %d: %w", i, err)
		}
		team1Players[i] = playtomic.Player{
			UserID: playerID,
			Name:   p.Name,
		}
	}

	for i, p := range input.Team2Players {
		playerID, err := s.getOrCreateManualPlayerLocked(p.ID, p.Name)
		if err != nil {
			return nil, fmt.Errorf("failed to process team2 player %d: %w", i, err)
		}
		team2Players[i] = playtomic.Player{
			UserID: playerID,
			Name:   p.Name,
		}
	}

	// Determine winner based on sets won
	team1SetsWon := 0
	team2SetsWon := 0
	for _, set := range input.Sets {
		if set.Team1Games > set.Team2Games {
			team1SetsWon++
		} else if set.Team2Games > set.Team1Games {
			team2SetsWon++
		}
	}

	team1Result := "LOST"
	team2Result := "LOST"
	if team1SetsWon > team2SetsWon {
		team1Result = "WON"
	} else if team2SetsWon > team1SetsWon {
		team2Result = "WON"
	}

	// Build teams
	team1ID := fmt.Sprintf("team1_%s", matchID)
	team2ID := fmt.Sprintf("team2_%s", matchID)

	teams := []playtomic.Team{
		{
			ID:         team1ID,
			Players:    team1Players,
			TeamResult: team1Result,
		},
		{
			ID:         team2ID,
			Players:    team2Players,
			TeamResult: team2Result,
		},
	}

	// Build results
	results := make([]playtomic.SetResult, len(input.Sets))
	for i, set := range input.Sets {
		results[i] = playtomic.SetResult{
			Name: fmt.Sprintf("Set %d", i+1),
			Scores: map[string]int{
				team1ID: set.Team1Games,
				team2ID: set.Team2Games,
			},
		}
	}

	// Determine results status
	resultsStatus := playtomic.ResultsStatusConfirmed
	if len(input.Sets) == 0 {
		resultsStatus = playtomic.ResultsStatusWaitingFor
	}

	// Build the match
	match := &playtomic.PadelMatch{
		MatchID:       matchID,
		OwnerID:       team1Players[0].UserID,
		OwnerName:     team1Players[0].Name,
		Start:         input.MatchDate.Unix(),
		End:           input.MatchDate.Unix() + 3600, // Assume 1 hour match
		CreatedAt:     time.Now().Unix(),
		Status:        "PLAYED",
		GameStatus:    playtomic.GameStatusPlayed,
		ResultsStatus: resultsStatus,
		ResourceName:  input.VenueName,
		Tenant: playtomic.Tenant{
			ID:   "manual",
			Name: input.VenueName,
		},
		MatchType:        playtomic.MatchType(input.CompetitionMode),
		MatchTypeEnum:    input.MatchTypeEnum,
		Teams:            teams,
		Results:          results,
		ProcessingStatus: playtomic.StatusNew,
	}

	// Serialize teams and results
	teamsBlob, err := msgpack.Marshal(teams)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal teams: %w", err)
	}
	resultsBlob, err := msgpack.Marshal(results)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal results: %w", err)
	}

	// Set owner to first player of team 1 (required for FK constraint)
	ownerID := team1Players[0].UserID
	ownerName := team1Players[0].Name

	// Insert the match
	_, err = s.db.Exec(`
		INSERT INTO matches (id, owner_id, owner_name, start_time, end_time, created_at, status, game_status, results_status, resource_name, access_code, price, tenant_id, tenant_name, match_type, teams_blob, results_blob, processing_status, match_type_enum, source)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`, matchID, ownerID, ownerName, match.Start, match.End, match.CreatedAt, match.Status, match.GameStatus, match.ResultsStatus, match.ResourceName, "", "", match.Tenant.ID, match.Tenant.Name, match.MatchType, teamsBlob, resultsBlob, playtomic.StatusNew, match.MatchTypeEnum, "manual")
	if err != nil {
		return nil, fmt.Errorf("failed to insert manual match: %w", err)
	}

	log.Info("Created manual match", "matchID", matchID, "venue", input.VenueName, "type", input.MatchTypeEnum)
	return match, nil
}

// getOrCreateManualPlayerLocked gets or creates a manual player (caller must hold lock)
func (s *store) getOrCreateManualPlayerLocked(playerID, playerName string) (string, error) {
	if playerID != "" {
		// Check if it's an existing player
		var exists bool
		err := s.db.QueryRow("SELECT EXISTS(SELECT 1 FROM players WHERE id = ?)", playerID).Scan(&exists)
		if err != nil {
			return "", fmt.Errorf("failed to check player existence: %w", err)
		}
		if exists {
			return playerID, nil
		}

		// Check if it's an existing alias
		var aliasExists bool
		err = s.db.QueryRow("SELECT EXISTS(SELECT 1 FROM player_aliases WHERE manual_player_id = ?)", playerID).Scan(&aliasExists)
		if err != nil {
			return "", fmt.Errorf("failed to check alias existence: %w", err)
		}
		if aliasExists {
			return playerID, nil
		}
	}

	// Create new manual player
	manualID := fmt.Sprintf("manual_%d", time.Now().UnixNano())
	now := time.Now().Unix()

	// Insert into players table first (required for foreign key constraints)
	_, err := s.db.Exec(`
		INSERT INTO players (id, name, level) VALUES (?, ?, 0)
	`, manualID, playerName)
	if err != nil {
		return "", fmt.Errorf("failed to create player record: %w", err)
	}

	// Then insert into player_aliases table
	_, err = s.db.Exec(`
		INSERT INTO player_aliases (manual_player_id, manual_player_name, created_at, updated_at)
		VALUES (?, ?, ?, ?)
	`, manualID, playerName, now, now)
	if err != nil {
		return "", fmt.Errorf("failed to create player alias: %w", err)
	}

	log.Info("Created new manual player", "manualID", manualID, "name", playerName)
	return manualID, nil
}

// GetManualMatches retrieves all manually entered matches
func (s *store) GetManualMatches() ([]*playtomic.PadelMatch, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	rows, err := s.db.Query(`
		SELECT id, owner_id, owner_name, start_time, end_time, created_at, status, game_status, results_status, resource_name, access_code, price, tenant_id, tenant_name, match_type, teams_blob, results_blob, ball_bringer_id, ball_bringer_name, processing_status, booking_notified_ts, result_notified_ts, match_type_enum
		FROM matches
		WHERE source = 'manual'
		ORDER BY start_time DESC
	`)
	if err != nil {
		return nil, fmt.Errorf("failed to query manual matches: %w", err)
	}
	defer rows.Close()

	var matches []*playtomic.PadelMatch
	for rows.Next() {
		match, err := s.scanMatch(rows)
		if err != nil {
			log.Error("Failed to scan manual match row", "error", err)
			continue
		}
		matches = append(matches, match)
	}

	return matches, nil
}

// GetDistinctVenues returns distinct venue names that match the query
func (s *store) GetDistinctVenues(query string, limit int) ([]string, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if limit <= 0 {
		limit = 10
	}

	// Query distinct tenant names (venues) that match the search query
	// Using LIKE for case-insensitive prefix matching
	rows, err := s.db.Query(`
		SELECT DISTINCT tenant_name
		FROM matches
		WHERE tenant_name != ''
		  AND tenant_name LIKE ?
		ORDER BY tenant_name
		LIMIT ?
	`, query+"%", limit)
	if err != nil {
		return nil, fmt.Errorf("failed to query venues: %w", err)
	}
	defer rows.Close()

	var venues []string
	for rows.Next() {
		var venue string
		if err := rows.Scan(&venue); err != nil {
			continue
		}
		venues = append(venues, venue)
	}

	return venues, nil
}

// Ping checks database connectivity by executing a simple query.
func (s *store) Ping() error {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var result int
	err := s.db.QueryRow("SELECT 1").Scan(&result)
	if err != nil {
		return fmt.Errorf("database ping failed: %w", err)
	}
	return nil
}
