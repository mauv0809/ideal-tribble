package club_test

import (
	"database/sql"
	"testing"

	_ "github.com/mattn/go-sqlite3"
	"github.com/mauv0809/ideal-tribble/internal/club"
	"github.com/mauv0809/ideal-tribble/internal/playtomic"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const testSchema = `
CREATE TABLE IF NOT EXISTS players (
	id TEXT PRIMARY KEY,
	name TEXT,
	level DOUBLE NOT NULL DEFAULT 0,
	ball_bringer_count INTEGER NOT NULL DEFAULT 0,
	created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);
CREATE TABLE IF NOT EXISTS matches (
	id TEXT PRIMARY KEY,
	owner_id TEXT NOT NULL,
	owner_name TEXT NOT NULL,
	start_time INTEGER NOT NULL,
	end_time INTEGER NOT NULL,
	created_at INTEGER NOT NULL,
	status TEXT NOT NULL,
	game_status TEXT NOT NULL,
	results_status TEXT NOT NULL,
	resource_name TEXT NOT NULL,
	access_code TEXT,
	price TEXT,
	tenant_id TEXT NOT NULL,
	tenant_name TEXT NOT NULL,
	processing_status TEXT NOT NULL DEFAULT 'NEW',
	match_type TEXT NOT NULL,
	teams_json TEXT,
	results_json TEXT,
	ball_bringer_id TEXT,
	ball_bringer_name TEXT,
	FOREIGN KEY (owner_id) REFERENCES players(id) ON DELETE SET NULL,
	FOREIGN KEY (ball_bringer_id) REFERENCES players(id) ON DELETE SET NULL
);
CREATE TABLE IF NOT EXISTS player_stats (
	player_id TEXT PRIMARY KEY,
	matches_played INTEGER NOT NULL DEFAULT 0,
	matches_won INTEGER NOT NULL DEFAULT 0,
	matches_lost INTEGER NOT NULL DEFAULT 0,
	sets_won INTEGER NOT NULL DEFAULT 0,
	sets_lost INTEGER NOT NULL DEFAULT 0,
	games_won INTEGER NOT NULL DEFAULT 0,
	games_lost INTEGER NOT NULL DEFAULT 0,
	win_percentage REAL NOT NULL DEFAULT 0,
	FOREIGN KEY (player_id) REFERENCES players(id) ON DELETE CASCADE
);
CREATE TABLE IF NOT EXISTS metrics (
	key TEXT PRIMARY KEY,
	value INTEGER NOT NULL DEFAULT 0
);
`

// setupTestDB creates a temporary in-memory SQLite database for testing.
func setupTestDB(t *testing.T) (club.ClubStore, *sql.DB, func()) {
	t.Helper()

	db, err := sql.Open("sqlite3", ":memory:")
	require.NoError(t, err)

	_, err = db.Exec(testSchema)
	require.NoError(t, err)

	store := club.New(db)

	teardown := func() {
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

	_, err := db.Exec(`INSERT INTO players (id, name, level) VALUES
		('player1', 'Player B', 3.5),
		('player2', 'Player C', 2.5),
		('player3', 'Player A', 4.5)
	`)
	require.NoError(t, err)

	players, err := store.GetPlayersSortedByLevel()
	require.NoError(t, err)
	require.Len(t, players, 3)

	assert.Equal(t, "Player A", players[0].Name)
	assert.Equal(t, float32(4.5), players[0].Level)
	assert.Equal(t, "Player B", players[1].Name)
	assert.Equal(t, "Player C", players[2].Name)
}

func TestClear(t *testing.T) {
	store, db, teardown := setupTestDB(t)
	defer teardown()

	_, err := db.Exec(`INSERT INTO players (id, name) VALUES ('p1', 'p1 name')`)
	require.NoError(t, err)
	err = store.UpsertMatch(&playtomic.PadelMatch{MatchID: "m1", OwnerID: "p1"})
	require.NoError(t, err)

	store.Clear()

	var count int
	err = db.QueryRow("SELECT COUNT(*) FROM matches").Scan(&count)
	require.NoError(t, err)
	assert.Zero(t, count)

	err = db.QueryRow("SELECT COUNT(*) FROM players").Scan(&count)
	require.NoError(t, err)
	assert.Zero(t, count)
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
