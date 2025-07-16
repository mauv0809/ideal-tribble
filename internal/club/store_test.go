package club_test

import (
	"database/sql"
	"testing"

	"github.com/mauv0809/ideal-tribble/internal/club"
	"github.com/mauv0809/ideal-tribble/internal/database"
	"github.com/mauv0809/ideal-tribble/internal/playtomic"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// setupTestDB creates a temporary in-memory SQLite database for testing.
func setupTestDB(t *testing.T) (club.ClubStore, *sql.DB, func()) {
	t.Helper()

	db, dbTeardown, err := database.InitDB(":memory:", "", "", "../../migrations")
	require.NoError(t, err)

	store := club.New(db)
	teardown := func() {
		dbTeardown()
		db.Close()
	}

	return store, db, teardown
}

func TestAddAndGetPlayers(t *testing.T) {
	store, _, teardown := setupTestDB(t)
	defer teardown()

	store.AddPlayer("player1", "Player One", 1.0)
	store.AddPlayer("player2", "Player Two", 2.0)

	assert.True(t, store.IsKnownPlayer("player1"))
	assert.False(t, store.IsKnownPlayer("player3"))

	allPlayers, err := store.GetAllPlayers()
	require.NoError(t, err)
	assert.Len(t, allPlayers, 2)
}

func TestUpsertMatch(t *testing.T) {
	store, db, teardown := setupTestDB(t)
	defer teardown()

	_, err := db.Exec(`INSERT INTO players (id, name) VALUES ('owner1', 'owner name')`)
	require.NoError(t, err)

	match := &playtomic.PadelMatch{MatchID: "match1", OwnerID: "owner1"}
	err = store.UpsertMatch(match)
	require.NoError(t, err)

	matches, err := store.GetMatchesForProcessing()
	require.NoError(t, err)
	require.Len(t, matches, 1)
	assert.Equal(t, "match1", matches[0].MatchID)
	assert.Equal(t, playtomic.StatusNew, matches[0].ProcessingStatus)

	match.ResourceName = "Court 1"
	err = store.UpsertMatch(match)
	require.NoError(t, err)

	matches, err = store.GetMatchesForProcessing()
	require.NoError(t, err)
	require.Len(t, matches, 1)
	assert.Equal(t, "Court 1", matches[0].ResourceName)
	assert.Equal(t, playtomic.StatusNew, matches[0].ProcessingStatus)
}

func TestGetPlayers(t *testing.T) {
	store, db, teardown := setupTestDB(t)
	defer teardown()

	_, err := db.Exec(`INSERT INTO players (id, name, level, ball_bringer_count) VALUES 
		('p1', 'Player One', 1.0, 1),
		('p2', 'Player Two', 2.0, 2),
		('p3', 'Player Three', 3.0, 3)`)
	require.NoError(t, err)

	t.Run("gets multiple players", func(t *testing.T) {
		players, err := store.GetPlayers([]string{"p1", "p3"})
		require.NoError(t, err)
		require.Len(t, players, 2)

		// Check if the correct players are returned, regardless of order
		playerMap := make(map[string]club.PlayerInfo)
		for _, p := range players {
			playerMap[p.ID] = p
		}

		assert.Contains(t, playerMap, "p1")
		assert.Contains(t, playerMap, "p3")
		assert.Equal(t, "Player One", playerMap["p1"].Name)
		assert.Equal(t, 1, playerMap["p1"].BallBringerCount)
		assert.Equal(t, "Player Three", playerMap["p3"].Name)
		assert.Equal(t, 3, playerMap["p3"].BallBringerCount)
	})

	t.Run("returns empty slice for no matches", func(t *testing.T) {
		players, err := store.GetPlayers([]string{"p4", "p5"})
		require.NoError(t, err)
		assert.Len(t, players, 0)
	})

	t.Run("returns empty slice for empty id slice", func(t *testing.T) {
		players, err := store.GetPlayers([]string{})
		require.NoError(t, err)
		assert.Len(t, players, 0)
	})
}

func TestSetBallBringer(t *testing.T) {
	store, db, teardown := setupTestDB(t)
	defer teardown()

	// Setup initial data
	_, err := db.Exec(`INSERT INTO players (id, name, ball_bringer_count) VALUES ('p1', 'Player One', 5)`)
	require.NoError(t, err)
	_, err = db.Exec(`INSERT INTO matches (id, owner_id, owner_name, start_time, end_time, created_at, status, game_status, results_status, resource_name, tenant_id, tenant_name, match_type) VALUES ('m1', 'p1', 'Player One', 0, 0, 0, 'status', 'game_status', 'results_status', 'resource', 'tenant', 'tenant_name', 'type')`)
	require.NoError(t, err)

	// Set the ball bringer
	err = store.SetBallBringer("m1", "p1", "Player One")
	require.NoError(t, err)

	// Verify match is updated
	var ballBringerID, ballBringerName string
	err = db.QueryRow("SELECT ball_bringer_id, ball_bringer_name FROM matches WHERE id = 'm1'").Scan(&ballBringerID, &ballBringerName)
	require.NoError(t, err)
	assert.Equal(t, "p1", ballBringerID)
	assert.Equal(t, "Player One", ballBringerName)

	// Verify player's count is incremented
	var count int
	err = db.QueryRow("SELECT ball_bringer_count FROM players WHERE id = 'p1'").Scan(&count)
	require.NoError(t, err)
	assert.Equal(t, 6, count)
}

func TestUpdateProcessingStatus(t *testing.T) {
	store, db, teardown := setupTestDB(t)
	defer teardown()

	_, err := db.Exec(`INSERT INTO players (id, name) VALUES ('owner1', 'owner name')`)
	require.NoError(t, err)

	match := &playtomic.PadelMatch{MatchID: "match1", OwnerID: "owner1"}
	err = store.UpsertMatch(match)
	require.NoError(t, err)

	err = store.UpdateProcessingStatus("match1", playtomic.StatusBookingNotified)
	require.NoError(t, err)

	matches, err := store.GetMatchesForProcessing()
	require.NoError(t, err)
	require.Len(t, matches, 1)
	assert.Equal(t, playtomic.StatusBookingNotified, matches[0].ProcessingStatus)
}

func TestGetPlayerStatsByName(t *testing.T) {
	store, db, teardown := setupTestDB(t)
	defer teardown()

	t.Run("finds player with stats", func(t *testing.T) {
		_, err := db.Exec(`INSERT INTO players (id, name) VALUES ('player1', 'Morten Voss')`)
		require.NoError(t, err)
		_, err = db.Exec(`
			INSERT INTO player_stats (player_id, matches_played, matches_won)
			VALUES ('player1', 10, 8)`)
		require.NoError(t, err)
		_, err = db.Exec(`UPDATE player_stats SET win_percentage = (CAST(matches_won AS REAL) / matches_played) * 100 WHERE player_id = 'player1'`)
		require.NoError(t, err)

		stats, err := store.GetPlayerStatsByName("morten")
		require.NoError(t, err)
		require.NotNil(t, stats)

		assert.Equal(t, "Morten Voss", stats.PlayerName)
		assert.Equal(t, 10, stats.MatchesPlayed)
		assert.Equal(t, 8, stats.MatchesWon)
		assert.InDelta(t, 80.0, stats.WinPercentage, 0.01)
	})

	t.Run("returns zero stats for player with no stats entry", func(t *testing.T) {
		_, err := db.Exec(`INSERT INTO players (id, name) VALUES ('player2', 'New Player')`)
		require.NoError(t, err)

		stats, err := store.GetPlayerStatsByName("New Player")
		require.NoError(t, err)
		require.NotNil(t, stats)
		assert.Equal(t, "New Player", stats.PlayerName)
		assert.Equal(t, 0, stats.MatchesPlayed)
	})

	t.Run("returns error when player not found", func(t *testing.T) {
		stats, err := store.GetPlayerStatsByName("nonexistent")
		assert.Error(t, err)
		assert.Nil(t, stats)
	})
}

func TestGetPlayersSortedByLevel(t *testing.T) {
	store, db, teardown := setupTestDB(t)
	defer teardown()

	_, err := db.Exec(`INSERT INTO players (id, name) VALUES ('owner1', 'owner name')`)
	require.NoError(t, err)

	players := []club.PlayerInfo{
		{ID: "p1", Name: "Player A", Level: 1.5},
		{ID: "p2", Name: "Player B", Level: 2.5},
		{ID: "p3", Name: "Player C", Level: 0.5},
	}

	for _, p := range players {
		store.AddPlayer(p.ID, p.Name, p.Level)
	}

	sortedPlayers, err := store.GetPlayersSortedByLevel()
	require.NoError(t, err)
	assert.Len(t, sortedPlayers, 3)

	assert.Equal(t, "Player B", sortedPlayers[0].Name)
	assert.Equal(t, "Player A", sortedPlayers[1].Name)
	assert.Equal(t, "Player C", sortedPlayers[2].Name)
}

func TestClear(t *testing.T) {
	store, _, teardown := setupTestDB(t)
	defer teardown()

	// Add some data
	store.AddPlayer("player1", "Player One", 1.0)
	match := &playtomic.PadelMatch{MatchID: "m1", OwnerID: "player1"}
	err := store.UpsertMatch(match)
	require.NoError(t, err)

	// Clear the database
	store.Clear()

	// Verify all data is cleared
	allPlayers, err := store.GetAllPlayers()
	require.NoError(t, err)
	assert.Len(t, allPlayers, 0)

	matches, err := store.GetMatchesForProcessing()
	require.NoError(t, err)
	assert.Len(t, matches, 0)
}

func TestUpdatePlayerStats(t *testing.T) {
	store, _, teardown := setupTestDB(t)
	defer teardown()

	t.Run("correctly updates stats for a single match", func(t *testing.T) {
		store.AddPlayer("p1", "Morten Voss", 1.0)
		store.AddPlayer("p2", "Player Two", 1.0)
		store.AddPlayer("p3", "Player Three", 1.0)
		store.AddPlayer("p4", "Player Four", 1.0)

		match := &playtomic.PadelMatch{
			MatchID: "match1",
			OwnerID: "p1",
			Teams: []playtomic.Team{
				{ID: "t1", TeamResult: "WON", Players: []playtomic.Player{{UserID: "p1", Name: "Morten Voss"}, {UserID: "p2", Name: "Player Two"}}},
				{ID: "t2", TeamResult: "LOST", Players: []playtomic.Player{{UserID: "p3", Name: "Player Three"}, {UserID: "p4", Name: "Player Four"}}},
			},
			Results: []playtomic.SetResult{
				{Name: "Set-1", Scores: map[string]int{"t1": 6, "t2": 4}},
				{Name: "Set-2", Scores: map[string]int{"t1": 7, "t2": 5}},
			},
		}

		store.UpdatePlayerStats(match)

		stats, err := store.GetPlayerStatsByName("Morten Voss")
		require.NoError(t, err)
		assert.Equal(t, 1, stats.MatchesPlayed)
		assert.Equal(t, 1, stats.MatchesWon)
		assert.Equal(t, 0, stats.MatchesLost)
		assert.Equal(t, 2, stats.SetsWon)
		assert.Equal(t, 0, stats.SetsLost)
		assert.Equal(t, 13, stats.GamesWon)
		assert.Equal(t, 9, stats.GamesLost)
		assert.InDelta(t, 100.0, stats.WinPercentage, 0.01)

		stats, err = store.GetPlayerStatsByName("Player Three")
		require.NoError(t, err)
		assert.Equal(t, 1, stats.MatchesPlayed)
		assert.Equal(t, 0, stats.MatchesWon)
		assert.Equal(t, 1, stats.MatchesLost)
		assert.Equal(t, 0, stats.SetsWon)
		assert.Equal(t, 2, stats.SetsLost)
		assert.Equal(t, 9, stats.GamesWon)
		assert.Equal(t, 13, stats.GamesLost)
		assert.InDelta(t, 0.0, stats.WinPercentage, 0.01)
	})
}

func TestUpdateNotificationTimestamp(t *testing.T) {
	store, db, teardown := setupTestDB(t)
	defer teardown()

	// Insert a player to satisfy foreign key constraint
	_, err := db.Exec(`INSERT INTO players (id, name) VALUES ('p1', 'Player One')`)
	require.NoError(t, err)

	// Insert a match with initial null timestamps
	match := &playtomic.PadelMatch{
		MatchID:          "test_match_id",
		OwnerID:          "p1",
		OwnerName:        "Player One",
		Start:            1678886400, // Example Unix timestamp
		End:              1678890000,
		CreatedAt:        1678880000,
		Status:           "STATUS_OPEN",
		GameStatus:       "GAME_STATUS_UNKNOWN",
		ResultsStatus:    "RESULTS_STATUS_WAITING",
		ResourceName:     "Court 1",
		Tenant:           playtomic.Tenant{ID: "tenant1", Name: "Tenant Name"},
		ProcessingStatus: "NEW",
		MatchType:        "PADEL_MATCH",
	}
	require.NoError(t, store.UpsertMatch(match))

	// Test updating booking_notified_ts
	err = store.UpdateNotificationTimestamp("test_match_id", "booking")
	require.NoError(t, err)

	var bookingTS sql.NullInt64
	err = db.QueryRow("SELECT booking_notified_ts FROM matches WHERE id = ?", "test_match_id").Scan(&bookingTS)
	require.NoError(t, err)
	assert.True(t, bookingTS.Valid)
	assert.NotZero(t, bookingTS.Int64) // Check that a timestamp was set

	// Test updating result_notified_ts
	err = store.UpdateNotificationTimestamp("test_match_id", "result")
	require.NoError(t, err)

	var resultTS sql.NullInt64
	err = db.QueryRow("SELECT result_notified_ts FROM matches WHERE id = ?", "test_match_id").Scan(&resultTS)
	require.NoError(t, err)
	assert.True(t, resultTS.Valid)
	assert.NotZero(t, resultTS.Int64) // Check that a timestamp was set

	// Test updating a non-existent match (should return no error, but no rows affected)
	err = store.UpdateNotificationTimestamp("non_existent_match", "booking")
	require.NoError(t, err)
}
