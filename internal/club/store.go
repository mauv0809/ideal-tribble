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
		_, err := stmt.Exec(playerID, stats["matches_played"], stats["matches_won"], stats["matches_lost"], stats["sets_won"], stats["sets_lost"], stats["games_won"], stats["games_lost"])
		if err != nil {
			log.Error("Failed to execute player_stats statement", "error", err, "playerID", playerID, "table", tableName)
		} else {
			log.Info("Updated player stats", "playerID", playerID, "matchType", matchType, "table", tableName)
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
		_, err := stmt.Exec(
			weekStartDate,
			playerID,
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
			log.Error("Failed to execute weekly_player_stats statement", "error", err, "playerID", playerID, "week", weekStartDate)
		} else {
			log.Info("Updated weekly player stats", "playerID", playerID, "week", weekStartDate, "matchType", matchTypeEnum)
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

	for _, set := range match.Results {
		// Since matches are always 2 teams, we can determine winner/loser directly.
		var teamIDs []string
		for teamID := range set.Scores {
			teamIDs = append(teamIDs, teamID)
		}

		if len(teamIDs) != 2 {
			log.Warn("Cannot determine set winner/loser for a set without exactly 2 teams", "matchID", match.MatchID, "scores", set.Scores)
			continue // Skip this set
		}

		var setWinnerID, setLoserID string
		var maxScore, minScore int

		if set.Scores[teamIDs[0]] > set.Scores[teamIDs[1]] {
			setWinnerID, setLoserID = teamIDs[0], teamIDs[1]
			maxScore, minScore = set.Scores[teamIDs[0]], set.Scores[teamIDs[1]]
		} else {
			setWinnerID, setLoserID = teamIDs[1], teamIDs[0]
			maxScore, minScore = set.Scores[teamIDs[1]], set.Scores[teamIDs[0]]
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
	rows, err := s.db.Query("SELECT id, name, ball_bringer_count_singles, ball_bringer_count_doubles, level FROM players ORDER BY name")
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
		if err := rows.Scan(&p.ID, &name, &p.BallBringerCountSingles, &p.BallBringerCountDoubles, &level); err != nil {
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

	query := "SELECT id, name, ball_bringer_count_singles, ball_bringer_count_doubles, level FROM players WHERE id IN (?" + strings.Repeat(",?", len(playerIDs)-1) + ")"
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
		if err := rows.Scan(&p.ID, &name, &p.BallBringerCountSingles, &p.BallBringerCountDoubles, &level); err != nil {
			log.Error("Failed to scan player row", "error", err)
			continue // Or handle error more gracefully
		}
		p.Name = name.String
		p.Level = level.Float64
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

	var countColumn string
	if matchTypeEnum.String == "SINGLES" {
		countColumn = "ball_bringer_count_singles"
	} else {
		countColumn = "ball_bringer_count_doubles"
	}

	// Find the player with the minimum count for the specific match type.
	// Ordering by name provides deterministic tie-breaking.
	query := fmt.Sprintf(`
		SELECT id, name
		FROM players
		WHERE id IN (?`+strings.Repeat(",?", len(playerIDs)-1)+`)
		ORDER BY %s ASC, name ASC
		LIMIT 1;
	`, countColumn)

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

	// Atomically increment the selected player's count for the correct match type
	updateQuery := fmt.Sprintf("UPDATE players SET %s = %s + 1 WHERE id = ?", countColumn, countColumn)
	_, err = tx.Exec(updateQuery, selectedPlayerID)
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

	rows, err := s.db.Query("SELECT id, name, ball_bringer_count_singles, ball_bringer_count_doubles, level FROM players ORDER BY level DESC")
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
		if err := rows.Scan(&p.ID, &name, &p.BallBringerCountSingles, &p.BallBringerCountDoubles, &level); err != nil {
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
		&player.BallBringerCountSingles,
		&player.BallBringerCountDoubles,
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
		SELECT id, name, level, ball_bringer_count_singles, ball_bringer_count_doubles, 
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
			&player.BallBringerCountSingles,
			&player.BallBringerCountDoubles,
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
		SELECT id, name, level, ball_bringer_count_singles, ball_bringer_count_doubles, 
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
			&player.BallBringerCountSingles,
			&player.BallBringerCountDoubles,
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
