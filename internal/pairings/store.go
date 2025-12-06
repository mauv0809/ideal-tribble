package pairings

import (
	"database/sql"
	"fmt"
	"sync"
	"time"

	"github.com/charmbracelet/log"
)

type store struct {
	db *sql.DB
	mu sync.RWMutex
}

// New creates a new PairingsStore.
func New(db *sql.DB) PairingsStore {
	return &store{db: db}
}

// Ensure store implements PairingsStore.
var _ PairingsStore = (*store)(nil)

// AddTrackedPairing adds a new pairing to track. Player IDs are normalized so that
// the smaller ID is always player1 to ensure uniqueness regardless of order.
func (s *store) AddTrackedPairing(player1ID, player1Name, player2ID, player2Name string) (*TrackedPairing, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Normalize order: smaller ID first
	if player1ID > player2ID {
		player1ID, player2ID = player2ID, player1ID
		player1Name, player2Name = player2Name, player1Name
	}

	now := time.Now().Unix()

	result, err := s.db.Exec(`
		INSERT INTO tracked_pairings (player1_id, player1_name, player2_id, player2_name, created_at, active)
		VALUES (?, ?, ?, ?, ?, 1)
		ON CONFLICT(player1_id, player2_id) DO UPDATE SET
			player1_name = excluded.player1_name,
			player2_name = excluded.player2_name,
			active = 1
	`, player1ID, player1Name, player2ID, player2Name, now)
	if err != nil {
		return nil, fmt.Errorf("failed to add tracked pairing: %w", err)
	}

	id, err := result.LastInsertId()
	if err != nil {
		// If LastInsertId fails (e.g., on conflict update), fetch by players
		pairing, err := s.getPairingByPlayersLocked(player1ID, player2ID)
		if err != nil {
			return nil, fmt.Errorf("failed to retrieve pairing after insert: %w", err)
		}
		return pairing, nil
	}

	return &TrackedPairing{
		ID:          id,
		Player1ID:   player1ID,
		Player1Name: player1Name,
		Player2ID:   player2ID,
		Player2Name: player2Name,
		CreatedAt:   now,
		Active:      true,
	}, nil
}

// RemoveTrackedPairing permanently deletes a tracked pairing and all its matches.
func (s *store) RemoveTrackedPairing(pairingID int64) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	_, err := s.db.Exec("DELETE FROM tracked_pairings WHERE id = ?", pairingID)
	if err != nil {
		return fmt.Errorf("failed to remove tracked pairing: %w", err)
	}
	return nil
}

// DeactivatePairing marks a pairing as inactive (stops tracking new matches).
func (s *store) DeactivatePairing(pairingID int64) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	_, err := s.db.Exec("UPDATE tracked_pairings SET active = 0 WHERE id = ?", pairingID)
	if err != nil {
		return fmt.Errorf("failed to deactivate pairing: %w", err)
	}
	return nil
}

// ActivatePairing marks a pairing as active.
func (s *store) ActivatePairing(pairingID int64) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	_, err := s.db.Exec("UPDATE tracked_pairings SET active = 1 WHERE id = ?", pairingID)
	if err != nil {
		return fmt.Errorf("failed to activate pairing: %w", err)
	}
	return nil
}

// GetTrackedPairings returns all tracked pairings.
func (s *store) GetTrackedPairings() ([]TrackedPairing, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	rows, err := s.db.Query(`
		SELECT id, player1_id, player1_name, player2_id, player2_name, created_at, active
		FROM tracked_pairings
		ORDER BY created_at DESC
	`)
	if err != nil {
		return nil, fmt.Errorf("failed to get tracked pairings: %w", err)
	}
	defer rows.Close()

	return s.scanPairings(rows)
}

// GetActivePairings returns only active tracked pairings.
func (s *store) GetActivePairings() ([]TrackedPairing, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	rows, err := s.db.Query(`
		SELECT id, player1_id, player1_name, player2_id, player2_name, created_at, active
		FROM tracked_pairings
		WHERE active = 1
		ORDER BY created_at DESC
	`)
	if err != nil {
		return nil, fmt.Errorf("failed to get active pairings: %w", err)
	}
	defer rows.Close()

	return s.scanPairings(rows)
}

// GetPairingByID retrieves a specific pairing by ID.
func (s *store) GetPairingByID(pairingID int64) (*TrackedPairing, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var p TrackedPairing
	var active int
	err := s.db.QueryRow(`
		SELECT id, player1_id, player1_name, player2_id, player2_name, created_at, active
		FROM tracked_pairings WHERE id = ?
	`, pairingID).Scan(&p.ID, &p.Player1ID, &p.Player1Name, &p.Player2ID, &p.Player2Name, &p.CreatedAt, &active)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to get pairing by ID: %w", err)
	}
	p.Active = active == 1
	return &p, nil
}

// GetPairingByPlayers retrieves a pairing by player IDs (order doesn't matter).
func (s *store) GetPairingByPlayers(player1ID, player2ID string) (*TrackedPairing, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	return s.getPairingByPlayersLocked(player1ID, player2ID)
}

func (s *store) getPairingByPlayersLocked(player1ID, player2ID string) (*TrackedPairing, error) {
	// Normalize order
	if player1ID > player2ID {
		player1ID, player2ID = player2ID, player1ID
	}

	var p TrackedPairing
	var active int
	err := s.db.QueryRow(`
		SELECT id, player1_id, player1_name, player2_id, player2_name, created_at, active
		FROM tracked_pairings WHERE player1_id = ? AND player2_id = ?
	`, player1ID, player2ID).Scan(&p.ID, &p.Player1ID, &p.Player1Name, &p.Player2ID, &p.Player2Name, &p.CreatedAt, &active)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to get pairing by players: %w", err)
	}
	p.Active = active == 1
	return &p, nil
}

func (s *store) scanPairings(rows *sql.Rows) ([]TrackedPairing, error) {
	var pairings []TrackedPairing
	for rows.Next() {
		var p TrackedPairing
		var active int
		if err := rows.Scan(&p.ID, &p.Player1ID, &p.Player1Name, &p.Player2ID, &p.Player2Name, &p.CreatedAt, &active); err != nil {
			log.Error("Failed to scan pairing row", "error", err)
			continue
		}
		p.Active = active == 1
		pairings = append(pairings, p)
	}
	return pairings, nil
}

// UpsertPairingMatch inserts or updates a match for a tracked pairing.
func (s *store) UpsertPairingMatch(match *PairingMatch) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	pairingWon := 0
	if match.PairingWon {
		pairingWon = 1
	}

	_, err := s.db.Exec(`
		INSERT INTO pairing_matches (pairing_id, match_id, match_date, day_of_week, hour_of_day,
			opponent1_id, opponent1_name, opponent2_id, opponent2_name, pairing_won,
			sets_won, sets_lost, games_won, games_lost, tenant_id, tenant_name)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(pairing_id, match_id) DO UPDATE SET
			match_date = excluded.match_date,
			day_of_week = excluded.day_of_week,
			hour_of_day = excluded.hour_of_day,
			opponent1_id = excluded.opponent1_id,
			opponent1_name = excluded.opponent1_name,
			opponent2_id = excluded.opponent2_id,
			opponent2_name = excluded.opponent2_name,
			pairing_won = excluded.pairing_won,
			sets_won = excluded.sets_won,
			sets_lost = excluded.sets_lost,
			games_won = excluded.games_won,
			games_lost = excluded.games_lost,
			tenant_id = excluded.tenant_id,
			tenant_name = excluded.tenant_name
	`, match.PairingID, match.MatchID, match.MatchDate, match.DayOfWeek, match.HourOfDay,
		match.Opponent1ID, match.Opponent1Name, match.Opponent2ID, match.Opponent2Name, pairingWon,
		match.SetsWon, match.SetsLost, match.GamesWon, match.GamesLost, match.TenantID, match.TenantName)
	if err != nil {
		return fmt.Errorf("failed to upsert pairing match: %w", err)
	}
	return nil
}

// UpsertPairingMatches inserts or updates multiple matches in a transaction.
func (s *store) UpsertPairingMatches(matches []*PairingMatch) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	tx, err := s.db.Begin()
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback()

	stmt, err := tx.Prepare(`
		INSERT INTO pairing_matches (pairing_id, match_id, match_date, day_of_week, hour_of_day,
			opponent1_id, opponent1_name, opponent2_id, opponent2_name, pairing_won,
			sets_won, sets_lost, games_won, games_lost, tenant_id, tenant_name)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(pairing_id, match_id) DO UPDATE SET
			match_date = excluded.match_date,
			day_of_week = excluded.day_of_week,
			hour_of_day = excluded.hour_of_day,
			opponent1_id = excluded.opponent1_id,
			opponent1_name = excluded.opponent1_name,
			opponent2_id = excluded.opponent2_id,
			opponent2_name = excluded.opponent2_name,
			pairing_won = excluded.pairing_won,
			sets_won = excluded.sets_won,
			sets_lost = excluded.sets_lost,
			games_won = excluded.games_won,
			games_lost = excluded.games_lost,
			tenant_id = excluded.tenant_id,
			tenant_name = excluded.tenant_name
	`)
	if err != nil {
		return fmt.Errorf("failed to prepare statement: %w", err)
	}
	defer stmt.Close()

	for _, match := range matches {
		pairingWon := 0
		if match.PairingWon {
			pairingWon = 1
		}
		_, err = stmt.Exec(match.PairingID, match.MatchID, match.MatchDate, match.DayOfWeek, match.HourOfDay,
			match.Opponent1ID, match.Opponent1Name, match.Opponent2ID, match.Opponent2Name, pairingWon,
			match.SetsWon, match.SetsLost, match.GamesWon, match.GamesLost, match.TenantID, match.TenantName)
		if err != nil {
			return fmt.Errorf("failed to upsert match %s: %w", match.MatchID, err)
		}
	}

	return tx.Commit()
}

// GetPairingOverallStats returns overall statistics for a pairing.
func (s *store) GetPairingOverallStats(pairingID int64) (*PairingStats, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	// Get pairing info
	var stats PairingStats
	err := s.db.QueryRow(`
		SELECT tp.id, tp.player1_name, tp.player2_name,
			COUNT(pm.id) as matches_played,
			SUM(pm.pairing_won) as matches_won,
			COUNT(pm.id) - SUM(pm.pairing_won) as matches_lost,
			COALESCE(SUM(pm.sets_won), 0) as sets_won,
			COALESCE(SUM(pm.sets_lost), 0) as sets_lost,
			COALESCE(SUM(pm.games_won), 0) as games_won,
			COALESCE(SUM(pm.games_lost), 0) as games_lost
		FROM tracked_pairings tp
		LEFT JOIN pairing_matches pm ON tp.id = pm.pairing_id
		WHERE tp.id = ?
		GROUP BY tp.id
	`, pairingID).Scan(&stats.PairingID, &stats.Player1Name, &stats.Player2Name,
		&stats.MatchesPlayed, &stats.MatchesWon, &stats.MatchesLost,
		&stats.SetsWon, &stats.SetsLost, &stats.GamesWon, &stats.GamesLost)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to get pairing stats: %w", err)
	}

	if stats.MatchesPlayed > 0 {
		stats.WinPercentage = float64(stats.MatchesWon) / float64(stats.MatchesPlayed) * 100
	}

	// Calculate streaks
	streaks, err := s.calculateStreaks(pairingID)
	if err != nil {
		log.Error("Failed to calculate streaks", "error", err)
	} else {
		stats.CurrentStreak = streaks.current
		stats.LongestWin = streaks.longestWin
		stats.LongestLoss = streaks.longestLoss
	}

	return &stats, nil
}

type streakInfo struct {
	current     int
	longestWin  int
	longestLoss int
}

func (s *store) calculateStreaks(pairingID int64) (*streakInfo, error) {
	rows, err := s.db.Query(`
		SELECT pairing_won FROM pairing_matches
		WHERE pairing_id = ?
		ORDER BY match_date DESC
	`, pairingID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var results []bool
	for rows.Next() {
		var won int
		if err := rows.Scan(&won); err != nil {
			continue
		}
		results = append(results, won == 1)
	}

	if len(results) == 0 {
		return &streakInfo{}, nil
	}

	// Current streak (from most recent)
	currentStreak := 0
	currentWon := results[0]
	for _, won := range results {
		if won == currentWon {
			if currentWon {
				currentStreak++
			} else {
				currentStreak--
			}
		} else {
			break
		}
	}

	// Longest streaks
	longestWin, longestLoss := 0, 0
	winStreak, lossStreak := 0, 0
	for _, won := range results {
		if won {
			winStreak++
			lossStreak = 0
			if winStreak > longestWin {
				longestWin = winStreak
			}
		} else {
			lossStreak++
			winStreak = 0
			if lossStreak > longestLoss {
				longestLoss = lossStreak
			}
		}
	}

	return &streakInfo{
		current:     currentStreak,
		longestWin:  longestWin,
		longestLoss: longestLoss,
	}, nil
}

// GetPairingVsOpponentStats returns stats against each opponent pair.
func (s *store) GetPairingVsOpponentStats(pairingID int64) ([]OpponentStats, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	rows, err := s.db.Query(`
		SELECT
			opponent1_id, opponent1_name, opponent2_id, opponent2_name,
			COUNT(*) as matches_played,
			SUM(pairing_won) as matches_won,
			COUNT(*) - SUM(pairing_won) as matches_lost,
			COALESCE(SUM(sets_won), 0) as sets_won,
			COALESCE(SUM(sets_lost), 0) as sets_lost,
			COALESCE(SUM(games_won), 0) as games_won,
			COALESCE(SUM(games_lost), 0) as games_lost
		FROM pairing_matches
		WHERE pairing_id = ?
		GROUP BY opponent1_id, opponent2_id
		ORDER BY matches_played DESC
	`, pairingID)
	if err != nil {
		return nil, fmt.Errorf("failed to get opponent stats: %w", err)
	}
	defer rows.Close()

	var stats []OpponentStats
	for rows.Next() {
		var s OpponentStats
		if err := rows.Scan(&s.Opponent1ID, &s.Opponent1Name, &s.Opponent2ID, &s.Opponent2Name,
			&s.MatchesPlayed, &s.MatchesWon, &s.MatchesLost,
			&s.SetsWon, &s.SetsLost, &s.GamesWon, &s.GamesLost); err != nil {
			log.Error("Failed to scan opponent stats row", "error", err)
			continue
		}
		if s.MatchesPlayed > 0 {
			s.WinPercentage = float64(s.MatchesWon) / float64(s.MatchesPlayed) * 100
		}
		stats = append(stats, s)
	}
	return stats, nil
}

// GetPairingVsSpecificOpponent returns stats against a specific opponent pair.
func (s *store) GetPairingVsSpecificOpponent(pairingID int64, opponent1ID, opponent2ID string) (*OpponentStats, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var stats OpponentStats
	err := s.db.QueryRow(`
		SELECT
			opponent1_id, opponent1_name, opponent2_id, opponent2_name,
			COUNT(*) as matches_played,
			SUM(pairing_won) as matches_won,
			COUNT(*) - SUM(pairing_won) as matches_lost,
			COALESCE(SUM(sets_won), 0) as sets_won,
			COALESCE(SUM(sets_lost), 0) as sets_lost,
			COALESCE(SUM(games_won), 0) as games_won,
			COALESCE(SUM(games_lost), 0) as games_lost
		FROM pairing_matches
		WHERE pairing_id = ?
		  AND ((opponent1_id = ? AND opponent2_id = ?) OR (opponent1_id = ? AND opponent2_id = ?))
		GROUP BY opponent1_id, opponent2_id
	`, pairingID, opponent1ID, opponent2ID, opponent2ID, opponent1ID).Scan(
		&stats.Opponent1ID, &stats.Opponent1Name, &stats.Opponent2ID, &stats.Opponent2Name,
		&stats.MatchesPlayed, &stats.MatchesWon, &stats.MatchesLost,
		&stats.SetsWon, &stats.SetsLost, &stats.GamesWon, &stats.GamesLost)
	if err != nil {
		return nil, fmt.Errorf("failed to get specific opponent stats: %w", err)
	}
	if stats.MatchesPlayed > 0 {
		stats.WinPercentage = float64(stats.MatchesWon) / float64(stats.MatchesPlayed) * 100
	}
	return &stats, nil
}

// GetPairingTimeStats returns performance by time of day and day of week.
func (s *store) GetPairingTimeStats(pairingID int64) (*TimeStats, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	stats := &TimeStats{
		ByDayOfWeek: make(map[int]*DayStats),
		ByHourRange: make(map[string]*DayStats),
	}

	// Stats by day of week
	rows, err := s.db.Query(`
		SELECT day_of_week, COUNT(*), SUM(pairing_won)
		FROM pairing_matches
		WHERE pairing_id = ?
		GROUP BY day_of_week
	`, pairingID)
	if err != nil {
		return nil, fmt.Errorf("failed to get day of week stats: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var day, played, won int
		if err := rows.Scan(&day, &played, &won); err != nil {
			continue
		}
		ds := &DayStats{
			MatchesPlayed: played,
			MatchesWon:    won,
			MatchesLost:   played - won,
		}
		if played > 0 {
			ds.WinPercentage = float64(won) / float64(played) * 100
		}
		stats.ByDayOfWeek[day] = ds
	}

	// Stats by hour range
	rows2, err := s.db.Query(`
		SELECT
			CASE
				WHEN hour_of_day < 12 THEN 'morning'
				WHEN hour_of_day < 17 THEN 'afternoon'
				ELSE 'evening'
			END as time_range,
			COUNT(*), SUM(pairing_won)
		FROM pairing_matches
		WHERE pairing_id = ?
		GROUP BY time_range
	`, pairingID)
	if err != nil {
		return nil, fmt.Errorf("failed to get hour range stats: %w", err)
	}
	defer rows2.Close()

	for rows2.Next() {
		var timeRange string
		var played, won int
		if err := rows2.Scan(&timeRange, &played, &won); err != nil {
			continue
		}
		ds := &DayStats{
			MatchesPlayed: played,
			MatchesWon:    won,
			MatchesLost:   played - won,
		}
		if played > 0 {
			ds.WinPercentage = float64(won) / float64(played) * 100
		}
		stats.ByHourRange[timeRange] = ds
	}

	return stats, nil
}

// GetPairingVenueStats returns performance by venue.
func (s *store) GetPairingVenueStats(pairingID int64) ([]VenueStats, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	rows, err := s.db.Query(`
		SELECT tenant_id, tenant_name, COUNT(*), SUM(pairing_won)
		FROM pairing_matches
		WHERE pairing_id = ? AND tenant_id IS NOT NULL
		GROUP BY tenant_id
		ORDER BY COUNT(*) DESC
	`, pairingID)
	if err != nil {
		return nil, fmt.Errorf("failed to get venue stats: %w", err)
	}
	defer rows.Close()

	var stats []VenueStats
	for rows.Next() {
		var s VenueStats
		var played, won int
		if err := rows.Scan(&s.TenantID, &s.TenantName, &played, &won); err != nil {
			continue
		}
		s.MatchesPlayed = played
		s.MatchesWon = won
		s.MatchesLost = played - won
		if played > 0 {
			s.WinPercentage = float64(won) / float64(played) * 100
		}
		stats = append(stats, s)
	}
	return stats, nil
}

// GetPairingRecentForm returns recent form stats.
func (s *store) GetPairingRecentForm(pairingID int64) (*RecentForm, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	rows, err := s.db.Query(`
		SELECT pairing_won FROM pairing_matches
		WHERE pairing_id = ?
		ORDER BY match_date DESC
		LIMIT 10
	`, pairingID)
	if err != nil {
		return nil, fmt.Errorf("failed to get recent form: %w", err)
	}
	defer rows.Close()

	var results []bool
	for rows.Next() {
		var won int
		if err := rows.Scan(&won); err != nil {
			continue
		}
		results = append(results, won == 1)
	}

	form := &RecentForm{}
	for i, won := range results {
		if i < 5 {
			if won {
				form.Last5Wins++
			} else {
				form.Last5Losses++
			}
		}
		if won {
			form.Last10Wins++
		} else {
			form.Last10Losses++
		}
	}

	if form.Last5Wins+form.Last5Losses > 0 {
		form.Last5WinPct = float64(form.Last5Wins) / float64(form.Last5Wins+form.Last5Losses) * 100
	}
	if form.Last10Wins+form.Last10Losses > 0 {
		form.Last10WinPct = float64(form.Last10Wins) / float64(form.Last10Wins+form.Last10Losses) * 100
	}

	return form, nil
}

// GetPairingRecentMatches returns recent matches for a pairing.
func (s *store) GetPairingRecentMatches(pairingID int64, limit int) ([]PairingMatch, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if limit <= 0 {
		limit = 10
	}

	rows, err := s.db.Query(`
		SELECT id, pairing_id, match_id, match_date, day_of_week, hour_of_day,
			opponent1_id, opponent1_name, opponent2_id, opponent2_name, pairing_won,
			sets_won, sets_lost, games_won, games_lost, tenant_id, tenant_name
		FROM pairing_matches
		WHERE pairing_id = ?
		ORDER BY match_date DESC
		LIMIT ?
	`, pairingID, limit)
	if err != nil {
		return nil, fmt.Errorf("failed to get recent matches: %w", err)
	}
	defer rows.Close()

	return s.scanMatches(rows)
}

// GetPairingMatchesPaginated returns matches for a pairing with pagination.
func (s *store) GetPairingMatchesPaginated(pairingID int64, limit, offset int) ([]PairingMatch, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if limit <= 0 {
		limit = 10
	}

	rows, err := s.db.Query(`
		SELECT id, pairing_id, match_id, match_date, day_of_week, hour_of_day,
			opponent1_id, opponent1_name, opponent2_id, opponent2_name, pairing_won,
			sets_won, sets_lost, games_won, games_lost, tenant_id, tenant_name
		FROM pairing_matches
		WHERE pairing_id = ?
		ORDER BY match_date DESC
		LIMIT ? OFFSET ?
	`, pairingID, limit, offset)
	if err != nil {
		return nil, fmt.Errorf("failed to get paginated matches: %w", err)
	}
	defer rows.Close()

	return s.scanMatches(rows)
}

// GetPairingMatchesFiltered returns matches for a pairing with filter and pagination.
func (s *store) GetPairingMatchesFiltered(pairingID int64, filter string, limit, offset int) ([]PairingMatch, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if limit <= 0 {
		limit = 10
	}

	var query string
	var args []interface{}

	switch filter {
	case "wins":
		query = `
			SELECT id, pairing_id, match_id, match_date, day_of_week, hour_of_day,
				opponent1_id, opponent1_name, opponent2_id, opponent2_name, pairing_won,
				sets_won, sets_lost, games_won, games_lost, tenant_id, tenant_name
			FROM pairing_matches
			WHERE pairing_id = ? AND pairing_won = 1
			ORDER BY match_date DESC
			LIMIT ? OFFSET ?`
		args = []interface{}{pairingID, limit, offset}
	case "losses":
		query = `
			SELECT id, pairing_id, match_id, match_date, day_of_week, hour_of_day,
				opponent1_id, opponent1_name, opponent2_id, opponent2_name, pairing_won,
				sets_won, sets_lost, games_won, games_lost, tenant_id, tenant_name
			FROM pairing_matches
			WHERE pairing_id = ? AND pairing_won = 0
			ORDER BY match_date DESC
			LIMIT ? OFFSET ?`
		args = []interface{}{pairingID, limit, offset}
	default:
		query = `
			SELECT id, pairing_id, match_id, match_date, day_of_week, hour_of_day,
				opponent1_id, opponent1_name, opponent2_id, opponent2_name, pairing_won,
				sets_won, sets_lost, games_won, games_lost, tenant_id, tenant_name
			FROM pairing_matches
			WHERE pairing_id = ?
			ORDER BY match_date DESC
			LIMIT ? OFFSET ?`
		args = []interface{}{pairingID, limit, offset}
	}

	rows, err := s.db.Query(query, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to get filtered matches: %w", err)
	}
	defer rows.Close()

	return s.scanMatches(rows)
}

// GetPairingMatchCount returns the total number of matches for a pairing.
func (s *store) GetPairingMatchCount(pairingID int64) (int, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var count int
	err := s.db.QueryRow(`SELECT COUNT(*) FROM pairing_matches WHERE pairing_id = ?`, pairingID).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("failed to get match count: %w", err)
	}
	return count, nil
}

// GetPairingMatchCountFiltered returns the number of matches for a pairing with filter.
func (s *store) GetPairingMatchCountFiltered(pairingID int64, filter string) (int, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var count int
	var err error

	switch filter {
	case "wins":
		err = s.db.QueryRow(`SELECT COUNT(*) FROM pairing_matches WHERE pairing_id = ? AND pairing_won = 1`, pairingID).Scan(&count)
	case "losses":
		err = s.db.QueryRow(`SELECT COUNT(*) FROM pairing_matches WHERE pairing_id = ? AND pairing_won = 0`, pairingID).Scan(&count)
	default:
		err = s.db.QueryRow(`SELECT COUNT(*) FROM pairing_matches WHERE pairing_id = ?`, pairingID).Scan(&count)
	}

	if err != nil {
		return 0, fmt.Errorf("failed to get filtered match count: %w", err)
	}
	return count, nil
}

// GetPairingMatchByID returns a specific match by pairing and match ID.
func (s *store) GetPairingMatchByID(pairingID int64, matchID string) (*PairingMatch, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	row := s.db.QueryRow(`
		SELECT id, pairing_id, match_id, match_date, day_of_week, hour_of_day,
			opponent1_id, opponent1_name, opponent2_id, opponent2_name, pairing_won,
			sets_won, sets_lost, games_won, games_lost, tenant_id, tenant_name
		FROM pairing_matches
		WHERE pairing_id = ? AND match_id = ?
	`, pairingID, matchID)

	var m PairingMatch
	var won int
	var opp1ID, opp1Name, opp2ID, opp2Name, tenantID, tenantName sql.NullString
	if err := row.Scan(&m.ID, &m.PairingID, &m.MatchID, &m.MatchDate, &m.DayOfWeek, &m.HourOfDay,
		&opp1ID, &opp1Name, &opp2ID, &opp2Name, &won,
		&m.SetsWon, &m.SetsLost, &m.GamesWon, &m.GamesLost, &tenantID, &tenantName); err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("match not found")
		}
		return nil, fmt.Errorf("failed to get match: %w", err)
	}

	m.PairingWon = won == 1
	m.Opponent1ID = opp1ID.String
	m.Opponent1Name = opp1Name.String
	m.Opponent2ID = opp2ID.String
	m.Opponent2Name = opp2Name.String
	m.TenantID = tenantID.String
	m.TenantName = tenantName.String

	return &m, nil
}

// GetHeadToHead returns all matches against a specific opponent pair.
func (s *store) GetHeadToHead(pairingID int64, opponent1ID, opponent2ID string) ([]PairingMatch, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	// Normalize opponent order
	if opponent1ID > opponent2ID {
		opponent1ID, opponent2ID = opponent2ID, opponent1ID
	}

	rows, err := s.db.Query(`
		SELECT id, pairing_id, match_id, match_date, day_of_week, hour_of_day,
			opponent1_id, opponent1_name, opponent2_id, opponent2_name, pairing_won,
			sets_won, sets_lost, games_won, games_lost, tenant_id, tenant_name
		FROM pairing_matches
		WHERE pairing_id = ? AND opponent1_id = ? AND opponent2_id = ?
		ORDER BY match_date DESC
	`, pairingID, opponent1ID, opponent2ID)
	if err != nil {
		return nil, fmt.Errorf("failed to get head to head: %w", err)
	}
	defer rows.Close()

	return s.scanMatches(rows)
}

// GetTotalMatchCount returns the total count of all pairing matches.
func (s *store) GetTotalMatchCount() (int, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var count int
	err := s.db.QueryRow(`SELECT COUNT(*) FROM pairing_matches`).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("failed to get total match count: %w", err)
	}
	return count, nil
}

// GetRecentMatchCount returns the count of pairing matches in the last N days.
func (s *store) GetRecentMatchCount(days int) (int, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	cutoff := time.Now().AddDate(0, 0, -days).Unix()
	var count int
	err := s.db.QueryRow(`SELECT COUNT(*) FROM pairing_matches WHERE match_date >= ?`, cutoff).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("failed to get recent match count: %w", err)
	}
	return count, nil
}

func (s *store) GetRecentMatchesAllPairings(limit int) ([]PairingMatch, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	rows, err := s.db.Query(`
		SELECT id, pairing_id, match_id, match_date, day_of_week, hour_of_day,
		       opponent1_id, opponent1_name, opponent2_id, opponent2_name,
		       pairing_won, sets_won, sets_lost, games_won, games_lost,
		       tenant_id, tenant_name
		FROM pairing_matches
		ORDER BY match_date DESC
		LIMIT ?
	`, limit)
	if err != nil {
		return nil, fmt.Errorf("failed to get recent matches: %w", err)
	}
	defer rows.Close()

	return s.scanMatches(rows)
}

func (s *store) scanMatches(rows *sql.Rows) ([]PairingMatch, error) {
	var matches []PairingMatch
	for rows.Next() {
		var m PairingMatch
		var won int
		var opp1ID, opp1Name, opp2ID, opp2Name, tenantID, tenantName sql.NullString
		if err := rows.Scan(&m.ID, &m.PairingID, &m.MatchID, &m.MatchDate, &m.DayOfWeek, &m.HourOfDay,
			&opp1ID, &opp1Name, &opp2ID, &opp2Name, &won,
			&m.SetsWon, &m.SetsLost, &m.GamesWon, &m.GamesLost, &tenantID, &tenantName); err != nil {
			log.Error("Failed to scan match row", "error", err)
			continue
		}
		m.PairingWon = won == 1
		m.Opponent1ID = opp1ID.String
		m.Opponent1Name = opp1Name.String
		m.Opponent2ID = opp2ID.String
		m.Opponent2Name = opp2Name.String
		m.TenantID = tenantID.String
		m.TenantName = tenantName.String
		matches = append(matches, m)
	}
	return matches, nil
}

// GetIndividualOpponentStats returns aggregated stats against each individual opponent player.
func (s *store) GetIndividualOpponentStats(pairingID int64) ([]IndividualPlayerStats, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	// We need to aggregate stats for each individual player who appeared as either opponent1 or opponent2
	rows, err := s.db.Query(`
		WITH individual_opponents AS (
			SELECT opponent1_id as player_id, opponent1_name as player_name,
				pairing_won, sets_won, sets_lost, games_won, games_lost
			FROM pairing_matches WHERE pairing_id = ?
			UNION ALL
			SELECT opponent2_id as player_id, opponent2_name as player_name,
				pairing_won, sets_won, sets_lost, games_won, games_lost
			FROM pairing_matches WHERE pairing_id = ?
		)
		SELECT
			player_id, MAX(player_name) as player_name,
			COUNT(*) as matches_played,
			SUM(pairing_won) as matches_won,
			COUNT(*) - SUM(pairing_won) as matches_lost,
			COALESCE(SUM(sets_won), 0) as sets_won,
			COALESCE(SUM(sets_lost), 0) as sets_lost,
			COALESCE(SUM(games_won), 0) as games_won,
			COALESCE(SUM(games_lost), 0) as games_lost
		FROM individual_opponents
		WHERE player_id IS NOT NULL AND player_id != ''
		GROUP BY player_id
		ORDER BY matches_played DESC
	`, pairingID, pairingID)
	if err != nil {
		return nil, fmt.Errorf("failed to get individual opponent stats: %w", err)
	}
	defer rows.Close()

	var stats []IndividualPlayerStats
	for rows.Next() {
		var ps IndividualPlayerStats
		if err := rows.Scan(&ps.PlayerID, &ps.PlayerName, &ps.MatchesPlayed,
			&ps.MatchesWon, &ps.MatchesLost, &ps.SetsWon, &ps.SetsLost,
			&ps.GamesWon, &ps.GamesLost); err != nil {
			log.Error("Failed to scan individual player stats", "error", err)
			continue
		}
		if ps.MatchesPlayed > 0 {
			ps.WinPercentage = float64(ps.MatchesWon) / float64(ps.MatchesPlayed) * 100
		}
		stats = append(stats, ps)
	}
	return stats, nil
}

// GetIndividualOpponentDetail returns stats against a specific player and breakdown by their partners.
func (s *store) GetIndividualOpponentDetail(pairingID int64, playerID string) (*IndividualPlayerStats, []PlayerPartnerStats, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	// Get overall stats against this player
	var stats IndividualPlayerStats
	err := s.db.QueryRow(`
		WITH individual_matches AS (
			SELECT opponent1_id as player_id, opponent1_name as player_name,
				pairing_won, sets_won, sets_lost, games_won, games_lost
			FROM pairing_matches WHERE pairing_id = ? AND opponent1_id = ?
			UNION ALL
			SELECT opponent2_id as player_id, opponent2_name as player_name,
				pairing_won, sets_won, sets_lost, games_won, games_lost
			FROM pairing_matches WHERE pairing_id = ? AND opponent2_id = ?
		)
		SELECT
			player_id, MAX(player_name) as player_name,
			COUNT(*) as matches_played,
			SUM(pairing_won) as matches_won,
			COUNT(*) - SUM(pairing_won) as matches_lost,
			COALESCE(SUM(sets_won), 0) as sets_won,
			COALESCE(SUM(sets_lost), 0) as sets_lost,
			COALESCE(SUM(games_won), 0) as games_won,
			COALESCE(SUM(games_lost), 0) as games_lost
		FROM individual_matches
		WHERE player_id IS NOT NULL
		GROUP BY player_id
	`, pairingID, playerID, pairingID, playerID).Scan(
		&stats.PlayerID, &stats.PlayerName, &stats.MatchesPlayed,
		&stats.MatchesWon, &stats.MatchesLost, &stats.SetsWon, &stats.SetsLost,
		&stats.GamesWon, &stats.GamesLost)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil, nil
		}
		return nil, nil, fmt.Errorf("failed to get individual opponent detail: %w", err)
	}
	if stats.MatchesPlayed > 0 {
		stats.WinPercentage = float64(stats.MatchesWon) / float64(stats.MatchesPlayed) * 100
	}

	// Get breakdown by partner
	rows, err := s.db.Query(`
		WITH partner_matches AS (
			SELECT opponent2_id as partner_id, opponent2_name as partner_name, pairing_won
			FROM pairing_matches WHERE pairing_id = ? AND opponent1_id = ?
			UNION ALL
			SELECT opponent1_id as partner_id, opponent1_name as partner_name, pairing_won
			FROM pairing_matches WHERE pairing_id = ? AND opponent2_id = ?
		)
		SELECT
			partner_id, MAX(partner_name) as partner_name,
			COUNT(*) as matches_played,
			SUM(pairing_won) as matches_won,
			COUNT(*) - SUM(pairing_won) as matches_lost
		FROM partner_matches
		WHERE partner_id IS NOT NULL AND partner_id != ''
		GROUP BY partner_id
		ORDER BY matches_played DESC
	`, pairingID, playerID, pairingID, playerID)
	if err != nil {
		return &stats, nil, fmt.Errorf("failed to get partner breakdown: %w", err)
	}
	defer rows.Close()

	var partners []PlayerPartnerStats
	for rows.Next() {
		var ps PlayerPartnerStats
		if err := rows.Scan(&ps.PartnerID, &ps.PartnerName, &ps.MatchesPlayed,
			&ps.MatchesWon, &ps.MatchesLost); err != nil {
			log.Error("Failed to scan partner stats", "error", err)
			continue
		}
		if ps.MatchesPlayed > 0 {
			ps.WinPercentage = float64(ps.MatchesWon) / float64(ps.MatchesPlayed) * 100
		}
		partners = append(partners, ps)
	}

	return &stats, partners, nil
}

// GetMatchesVsIndividualOpponent returns all matches against a specific individual opponent.
func (s *store) GetMatchesVsIndividualOpponent(pairingID int64, playerID string) ([]PairingMatch, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	rows, err := s.db.Query(`
		SELECT id, pairing_id, match_id, match_date, day_of_week, hour_of_day,
			opponent1_id, opponent1_name, opponent2_id, opponent2_name, pairing_won,
			sets_won, sets_lost, games_won, games_lost, tenant_id, tenant_name
		FROM pairing_matches
		WHERE pairing_id = ? AND (opponent1_id = ? OR opponent2_id = ?)
		ORDER BY match_date DESC
	`, pairingID, playerID, playerID)
	if err != nil {
		return nil, fmt.Errorf("failed to get matches vs individual opponent: %w", err)
	}
	defer rows.Close()

	return s.scanMatches(rows)
}

// GetAllKnownPlayers returns all players we know about (from pairings and opponents).
func (s *store) GetAllKnownPlayers() ([]KnownPlayer, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	// Get players from tracked pairings and from match opponents
	rows, err := s.db.Query(`
		SELECT DISTINCT player_id, player_name FROM (
			-- Players from tracked pairings
			SELECT player1_id as player_id, player1_name as player_name FROM tracked_pairings
			UNION
			SELECT player2_id as player_id, player2_name as player_name FROM tracked_pairings
			UNION
			-- Opponents from matches
			SELECT opponent1_id as player_id, opponent1_name as player_name FROM pairing_matches
			WHERE opponent1_id IS NOT NULL AND opponent1_id != ''
			UNION
			SELECT opponent2_id as player_id, opponent2_name as player_name FROM pairing_matches
			WHERE opponent2_id IS NOT NULL AND opponent2_id != ''
		)
		ORDER BY player_name
	`)
	if err != nil {
		return nil, fmt.Errorf("failed to get known players: %w", err)
	}
	defer rows.Close()

	var players []KnownPlayer
	for rows.Next() {
		var p KnownPlayer
		if err := rows.Scan(&p.PlayerID, &p.PlayerName); err != nil {
			log.Error("Failed to scan known player", "error", err)
			continue
		}
		players = append(players, p)
	}
	return players, nil
}

// GetLastFetchTimestamp returns the last time matches were fetched (Unix timestamp).
func (s *store) GetLastFetchTimestamp() (int64, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var value string
	err := s.db.QueryRow(`SELECT value FROM app_settings WHERE key = 'last_fetch_timestamp'`).Scan(&value)
	if err != nil {
		return 0, fmt.Errorf("failed to get last fetch timestamp: %w", err)
	}

	var timestamp int64
	fmt.Sscanf(value, "%d", &timestamp)
	return timestamp, nil
}

// SetLastFetchTimestamp updates the last fetch timestamp.
func (s *store) SetLastFetchTimestamp(timestamp int64) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	_, err := s.db.Exec(`
		INSERT INTO app_settings (key, value, updated_at)
		VALUES ('last_fetch_timestamp', ?, ?)
		ON CONFLICT(key) DO UPDATE SET value = excluded.value, updated_at = excluded.updated_at
	`, fmt.Sprintf("%d", timestamp), timestamp)
	if err != nil {
		return fmt.Errorf("failed to set last fetch timestamp: %w", err)
	}
	return nil
}

// GetMostActivePairing returns the pairing with the most matches in the last N days.
func (s *store) GetMostActivePairing(days int) (*PairingHighlight, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	cutoff := time.Now().AddDate(0, 0, -days).Unix()

	var pairingID int64
	var player1Name, player2Name string
	var matchesPlayed, matchesWon int

	err := s.db.QueryRow(`
		SELECT tp.id, tp.player1_name, tp.player2_name,
			COUNT(*) as matches_played,
			SUM(pm.pairing_won) as matches_won
		FROM pairing_matches pm
		JOIN tracked_pairings tp ON pm.pairing_id = tp.id
		WHERE pm.match_date >= ?
		GROUP BY pm.pairing_id
		ORDER BY matches_played DESC
		LIMIT 1
	`, cutoff).Scan(&pairingID, &player1Name, &player2Name, &matchesPlayed, &matchesWon)

	if err != nil {
		if err.Error() == "sql: no rows in result set" {
			return nil, nil // No matches in time period
		}
		return nil, fmt.Errorf("failed to get most active pairing: %w", err)
	}

	winPct := 0.0
	if matchesPlayed > 0 {
		winPct = float64(matchesWon) / float64(matchesPlayed) * 100
	}

	return &PairingHighlight{
		PairingID:     pairingID,
		PairingName:   player1Name + " & " + player2Name,
		MatchesPlayed: matchesPlayed,
		MatchesWon:    matchesWon,
		WinPercentage: winPct,
	}, nil
}

// GetBestPerformingPairing returns the pairing with the highest win rate in the last N days,
// with a minimum number of matches required to qualify.
func (s *store) GetBestPerformingPairing(days int, minMatches int) (*PairingHighlight, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	cutoff := time.Now().AddDate(0, 0, -days).Unix()

	var pairingID int64
	var player1Name, player2Name string
	var matchesPlayed, matchesWon int

	err := s.db.QueryRow(`
		SELECT tp.id, tp.player1_name, tp.player2_name,
			COUNT(*) as matches_played,
			SUM(pm.pairing_won) as matches_won
		FROM pairing_matches pm
		JOIN tracked_pairings tp ON pm.pairing_id = tp.id
		WHERE pm.match_date >= ?
		GROUP BY pm.pairing_id
		HAVING matches_played >= ?
		ORDER BY (CAST(SUM(pm.pairing_won) AS REAL) / COUNT(*)) DESC, matches_played DESC
		LIMIT 1
	`, cutoff, minMatches).Scan(&pairingID, &player1Name, &player2Name, &matchesPlayed, &matchesWon)

	if err != nil {
		if err.Error() == "sql: no rows in result set" {
			return nil, nil // No pairing qualifies
		}
		return nil, fmt.Errorf("failed to get best performing pairing: %w", err)
	}

	winPct := 0.0
	if matchesPlayed > 0 {
		winPct = float64(matchesWon) / float64(matchesPlayed) * 100
	}

	return &PairingHighlight{
		PairingID:     pairingID,
		PairingName:   player1Name + " & " + player2Name,
		MatchesPlayed: matchesPlayed,
		MatchesWon:    matchesWon,
		WinPercentage: winPct,
	}, nil
}
