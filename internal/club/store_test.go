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

	store.AddPlayer("owner1", "owner name", 1.0)

	match := &playtomic.PadelMatch{
		MatchID: "match1",
		OwnerID: "owner1",
		Teams: []playtomic.Team{
			{Players: []playtomic.Player{{UserID: "p1"}, {UserID: "p2"}}},
			{Players: []playtomic.Player{{UserID: "p3"}, {UserID: "p4"}}},
		},
		MatchTypeEnum: playtomic.MatchTypeEnumDoubles,
	}
	err := store.UpsertMatch(match)
	require.NoError(t, err)

	matches, err := store.GetMatchesForProcessing()
	require.NoError(t, err)
	require.Len(t, matches, 1)
	assert.Equal(t, "match1", matches[0].MatchID)

	var matchTypeEnum sql.NullString
	err = db.QueryRow("SELECT match_type_enum FROM matches WHERE id = 'match1'").Scan(&matchTypeEnum)
	require.NoError(t, err)
	assert.Equal(t, "DOUBLES", matchTypeEnum.String)
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

	t.Run("gets multiple players", func(t *testing.T) {
		_, err := db.Exec(`INSERT INTO players (id, name, ball_bringer_count_singles, ball_bringer_count_doubles) VALUES
		('p1', 'Player One', 1, 5),
		('p3', 'Player Three', 3, 7)`)
		require.NoError(t, err)
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
		assert.Equal(t, 1, playerMap["p1"].BallBringerCountSingles)
		assert.Equal(t, 5, playerMap["p1"].BallBringerCountDoubles)
		assert.Equal(t, "Player Three", playerMap["p3"].Name)
		assert.Equal(t, 3, playerMap["p3"].BallBringerCountSingles)
		assert.Equal(t, 7, playerMap["p3"].BallBringerCountDoubles)
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

func TestUpdateProcessingStatus(t *testing.T) {
	store, _, teardown := setupTestDB(t)
	defer teardown()

	store.AddPlayer("owner1", "owner name", 1.0)

	match := &playtomic.PadelMatch{MatchID: "match1", OwnerID: "owner1", MatchTypeEnum: playtomic.MatchTypeEnumDoubles}
	err := store.UpsertMatch(match)
	require.NoError(t, err)

	err = store.UpdateProcessingStatus("match1", playtomic.StatusBookingNotified)
	require.NoError(t, err)

	matches, err := store.GetMatchesForProcessing()
	require.NoError(t, err)
	require.Len(t, matches, 1)
	assert.Equal(t, playtomic.StatusBookingNotified, matches[0].ProcessingStatus)
}

func TestGetPlayerStatsByName(t *testing.T) {
	store, _, teardown := setupTestDB(t)
	defer teardown()

	t.Run("finds player with stats", func(t *testing.T) {
		store.AddPlayer("player1", "Morten Voss", 1.0)
		match := &playtomic.PadelMatch{
			MatchID: "match1",
			OwnerID: "p1",
			Teams: []playtomic.Team{
				{ID: "t1", TeamResult: "WON", Players: []playtomic.Player{{UserID: "player1", Name: "Morten Voss"}, {UserID: "p2", Name: "Player Two"}}},
				{ID: "t2", TeamResult: "LOST", Players: []playtomic.Player{{UserID: "p3", Name: "Player Three"}, {UserID: "p4", Name: "Player Four"}}},
			},
			MatchTypeEnum: playtomic.MatchTypeEnumDoubles,
			Results: []playtomic.SetResult{
				{Name: "Set-1", Scores: map[string]int{"t1": 6, "t2": 4}},
				{Name: "Set-2", Scores: map[string]int{"t1": 7, "t2": 5}},
			},
		}
		store.UpsertMatch(match)
		store.UpdatePlayerStats(match)

		stats, err := store.GetPlayerStatsByName("morten", playtomic.MatchTypeEnumAll)
		require.NoError(t, err)
		require.NotNil(t, stats)

		assert.Equal(t, "Morten Voss", stats.PlayerName)
		assert.Equal(t, 1, stats.MatchesPlayed)
		assert.Equal(t, 1, stats.MatchesWon)
		assert.InDelta(t, 100.0, stats.WinPercentage, 0.01)
	})

	t.Run("returns zero stats for player with no stats entry", func(t *testing.T) {
		store.AddPlayer("player2", "New Player", 1.0)

		stats, err := store.GetPlayerStatsByName("New Player", playtomic.MatchTypeEnumAll)
		require.NoError(t, err)
		require.NotNil(t, stats)
		assert.Equal(t, "New Player", stats.PlayerName)
		assert.Equal(t, 0, stats.MatchesPlayed)
	})

	t.Run("returns error when player not found", func(t *testing.T) {
		stats, err := store.GetPlayerStatsByName("nonexistent", playtomic.MatchTypeEnumAll)
		assert.Error(t, err)
		assert.Nil(t, stats)
	})
}

func TestGetPlayersSortedByLevel(t *testing.T) {
	store, _, teardown := setupTestDB(t)
	defer teardown()

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
	match := &playtomic.PadelMatch{MatchID: "m1", OwnerID: "player1", MatchTypeEnum: playtomic.MatchTypeEnumDoubles}
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
			MatchTypeEnum: playtomic.MatchTypeEnumDoubles,
			Results: []playtomic.SetResult{
				{Name: "Set-1", Scores: map[string]int{"t1": 6, "t2": 4}},
				{Name: "Set-2", Scores: map[string]int{"t1": 7, "t2": 5}},
			},
		}

		store.UpdatePlayerStats(match)

		stats, err := store.GetPlayerStatsByName("Morten Voss", playtomic.MatchTypeEnumDoubles)
		require.NoError(t, err)
		assert.Equal(t, 1, stats.MatchesPlayed)
		assert.Equal(t, 1, stats.MatchesWon)
		assert.Equal(t, 0, stats.MatchesLost)
		assert.Equal(t, 2, stats.SetsWon)
		assert.Equal(t, 0, stats.SetsLost)
		assert.Equal(t, 13, stats.GamesWon)
		assert.Equal(t, 9, stats.GamesLost)
		assert.InDelta(t, 100.0, stats.WinPercentage, 0.01)

		stats, err = store.GetPlayerStatsByName("Player Three", playtomic.MatchTypeEnumDoubles)
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
		MatchTypeEnum:    playtomic.MatchTypeEnumDoubles,
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

func TestAssignBallBringerAtomically(t *testing.T) {
	store, db, teardown := setupTestDB(t)
	defer teardown()

	// Setup players with different initial counts
	_, err := db.Exec(`INSERT INTO players (id, name, ball_bringer_count_singles, ball_bringer_count_doubles) VALUES
		('p1', 'Player A', 2, 5),
		('p2', 'Player B', 1, 6),
		('p3', 'Player C', 3, 5),
		('p4', 'Player D', 4, 7)`)
	require.NoError(t, err)

	t.Run("assigns correctly for a DOUBLES match", func(t *testing.T) {
		// P1 and P3 have the lowest doubles count (5). P1 should be chosen due to alphabetical tie-break.
		match := &playtomic.PadelMatch{
			MatchID:          "test_match_id_doubles",
			OwnerID:          "p1",
			OwnerName:        "Player A",
			Start:            1678886400, // Example Unix timestamp
			End:              1678890000,
			CreatedAt:        1678880000,
			Status:           "STATUS_OPEN",
			GameStatus:       "GAME_STATUS_UNKNOWN",
			ResultsStatus:    "RESULTS_STATUS_WAITING",
			ResourceName:     "Court 1",
			Tenant:           playtomic.Tenant{ID: "tenant1", Name: "Tenant Name"},
			ProcessingStatus: "NEW",
			MatchTypeEnum:    playtomic.MatchTypeEnumDoubles,
			MatchType:        playtomic.MatchTypeCompetitive,
			Teams: []playtomic.Team{
				{ID: "t1", Players: []playtomic.Player{{UserID: "p1", Name: "Player A"}, {UserID: "p2", Name: "Player B"}}},
				{ID: "t2", Players: []playtomic.Player{{UserID: "p3", Name: "Player C"}, {UserID: "p4", Name: "Player D"}}},
			},
		}
		require.NoError(t, store.UpsertMatch(match))

		id, name, err := store.AssignBallBringerAtomically(match.MatchID, []string{"p1", "p2", "p3", "p4"})
		require.NoError(t, err)
		assert.Equal(t, "p1", id)
		assert.Equal(t, "Player A", name)

		var count int
		err = db.QueryRow("SELECT ball_bringer_count_doubles FROM players WHERE id = 'p1'").Scan(&count)
		require.NoError(t, err)
		assert.Equal(t, 6, count, "Doubles count for p1 should be incremented")
	})

	t.Run("assigns correctly for a SINGLES match", func(t *testing.T) {
		// P2 has the lowest singles count (1).
		match := &playtomic.PadelMatch{
			MatchID:          "test_match_id_singles",
			OwnerID:          "p1",
			OwnerName:        "Player A",
			Start:            1678886400, // Example Unix timestamp
			End:              1678890000,
			CreatedAt:        1678880000,
			Status:           "STATUS_OPEN",
			GameStatus:       "GAME_STATUS_UNKNOWN",
			ResultsStatus:    "RESULTS_STATUS_WAITING",
			ResourceName:     "Court 1",
			Tenant:           playtomic.Tenant{ID: "tenant1", Name: "Tenant Name"},
			ProcessingStatus: "NEW",
			MatchTypeEnum:    playtomic.MatchTypeEnumSingles,
			MatchType:        playtomic.MatchTypeCompetitive,
			Teams: []playtomic.Team{
				{ID: "t1", Players: []playtomic.Player{{UserID: "p1", Name: "Player A"}}},
				{ID: "t2", Players: []playtomic.Player{{UserID: "p2", Name: "Player B"}}},
			},
		}
		require.NoError(t, store.UpsertMatch(match))

		id, name, err := store.AssignBallBringerAtomically(match.MatchID, []string{"p1", "p2"})
		require.NoError(t, err)
		assert.Equal(t, "p2", id)
		assert.Equal(t, "Player B", name)

		var count int
		err = db.QueryRow("SELECT ball_bringer_count_singles FROM players WHERE id = 'p2'").Scan(&count)
		require.NoError(t, err)
		assert.Equal(t, 2, count, "Singles count for p2 should be incremented")
	})
}
