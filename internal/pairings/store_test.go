package pairings

import (
	"database/sql"
	"testing"

	"github.com/mauv0809/ideal-tribble/internal/database"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func setupTestStore(t *testing.T) (PairingsStore, func()) {
	t.Helper()

	db, teardown, err := database.InitDB(":memory:", "", "", "../../migrations")
	require.NoError(t, err)

	store := New(db)
	return store, teardown
}

func TestAddTrackedPairing(t *testing.T) {
	store, teardown := setupTestStore(t)
	defer teardown()

	pairing, err := store.AddTrackedPairing("player1", "Player One", "player2", "Player Two")
	require.NoError(t, err)
	assert.NotNil(t, pairing)
	assert.Equal(t, "player1", pairing.Player1ID)
	assert.Equal(t, "Player One", pairing.Player1Name)
	assert.Equal(t, "player2", pairing.Player2ID)
	assert.Equal(t, "Player Two", pairing.Player2Name)
	assert.True(t, pairing.Active)
}

func TestAddTrackedPairing_NormalizesOrder(t *testing.T) {
	store, teardown := setupTestStore(t)
	defer teardown()

	// Add with player2 first, should be normalized to player1 first (alphabetically)
	pairing, err := store.AddTrackedPairing("player2", "Player Two", "player1", "Player One")
	require.NoError(t, err)

	// The smaller ID should be player1
	assert.Equal(t, "player1", pairing.Player1ID)
	assert.Equal(t, "Player One", pairing.Player1Name)
	assert.Equal(t, "player2", pairing.Player2ID)
	assert.Equal(t, "Player Two", pairing.Player2Name)
}

func TestAddTrackedPairing_Upsert(t *testing.T) {
	store, teardown := setupTestStore(t)
	defer teardown()

	// Add the pairing
	pairing1, err := store.AddTrackedPairing("player1", "Player One", "player2", "Player Two")
	require.NoError(t, err)

	// Adding same pairing with different names should update
	pairing2, err := store.AddTrackedPairing("player1", "Updated One", "player2", "Updated Two")
	require.NoError(t, err)

	// Should have same ID (upserted)
	assert.Equal(t, pairing1.ID, pairing2.ID)

	// Names should be updated
	assert.Equal(t, "Updated One", pairing2.Player1Name)
	assert.Equal(t, "Updated Two", pairing2.Player2Name)
}

func TestGetTrackedPairings(t *testing.T) {
	store, teardown := setupTestStore(t)
	defer teardown()

	// Add two pairings
	_, err := store.AddTrackedPairing("p1", "Player 1", "p2", "Player 2")
	require.NoError(t, err)
	_, err = store.AddTrackedPairing("p3", "Player 3", "p4", "Player 4")
	require.NoError(t, err)

	pairings, err := store.GetTrackedPairings()
	require.NoError(t, err)
	assert.Len(t, pairings, 2)
}

func TestGetActivePairings(t *testing.T) {
	store, teardown := setupTestStore(t)
	defer teardown()

	// Add two pairings
	p1, err := store.AddTrackedPairing("p1", "Player 1", "p2", "Player 2")
	require.NoError(t, err)
	_, err = store.AddTrackedPairing("p3", "Player 3", "p4", "Player 4")
	require.NoError(t, err)

	// Deactivate first pairing
	err = store.DeactivatePairing(p1.ID)
	require.NoError(t, err)

	// Should only return one active pairing
	pairings, err := store.GetActivePairings()
	require.NoError(t, err)
	assert.Len(t, pairings, 1)
	assert.Equal(t, "p3", pairings[0].Player1ID)
}

func TestGetPairingByPlayers(t *testing.T) {
	store, teardown := setupTestStore(t)
	defer teardown()

	_, err := store.AddTrackedPairing("player1", "Player One", "player2", "Player Two")
	require.NoError(t, err)

	// Should find with correct order
	pairing, err := store.GetPairingByPlayers("player1", "player2")
	require.NoError(t, err)
	assert.NotNil(t, pairing)

	// Should also find with reversed order
	pairing, err = store.GetPairingByPlayers("player2", "player1")
	require.NoError(t, err)
	assert.NotNil(t, pairing)

	// Should not find non-existent pairing
	pairing, err = store.GetPairingByPlayers("player1", "player3")
	require.NoError(t, err)
	assert.Nil(t, pairing)
}

func TestUpsertPairingMatch(t *testing.T) {
	store, teardown := setupTestStore(t)
	defer teardown()

	pairing, err := store.AddTrackedPairing("p1", "Player 1", "p2", "Player 2")
	require.NoError(t, err)

	match := &PairingMatch{
		PairingID:     pairing.ID,
		MatchID:       "match123",
		MatchDate:     1700000000,
		DayOfWeek:     3,
		HourOfDay:     14,
		Opponent1ID:   "opp1",
		Opponent1Name: "Opponent 1",
		Opponent2ID:   "opp2",
		Opponent2Name: "Opponent 2",
		PairingWon:    true,
		SetsWon:       2,
		SetsLost:      1,
		GamesWon:      15,
		GamesLost:     10,
		TenantID:      "venue1",
		TenantName:    "Test Venue",
	}

	err = store.UpsertPairingMatch(match)
	require.NoError(t, err)

	// Verify it was stored
	matches, err := store.GetPairingRecentMatches(pairing.ID, 10)
	require.NoError(t, err)
	assert.Len(t, matches, 1)
	assert.Equal(t, "match123", matches[0].MatchID)
	assert.True(t, matches[0].PairingWon)
}

func TestGetPairingOverallStats(t *testing.T) {
	store, teardown := setupTestStore(t)
	defer teardown()

	pairing, err := store.AddTrackedPairing("p1", "Player 1", "p2", "Player 2")
	require.NoError(t, err)

	// Add some matches
	matches := []*PairingMatch{
		{PairingID: pairing.ID, MatchID: "m1", MatchDate: 1700000000, PairingWon: true, SetsWon: 2, SetsLost: 0},
		{PairingID: pairing.ID, MatchID: "m2", MatchDate: 1700100000, PairingWon: true, SetsWon: 2, SetsLost: 1},
		{PairingID: pairing.ID, MatchID: "m3", MatchDate: 1700200000, PairingWon: false, SetsWon: 0, SetsLost: 2},
	}
	err = store.UpsertPairingMatches(matches)
	require.NoError(t, err)

	stats, err := store.GetPairingOverallStats(pairing.ID)
	require.NoError(t, err)
	assert.Equal(t, 3, stats.MatchesPlayed)
	assert.Equal(t, 2, stats.MatchesWon)
	assert.Equal(t, 1, stats.MatchesLost)
	assert.InDelta(t, 66.67, stats.WinPercentage, 0.1)
	assert.Equal(t, 4, stats.SetsWon)
	assert.Equal(t, 3, stats.SetsLost)
}

func TestGetPairingVsOpponentStats(t *testing.T) {
	store, teardown := setupTestStore(t)
	defer teardown()

	pairing, err := store.AddTrackedPairing("p1", "Player 1", "p2", "Player 2")
	require.NoError(t, err)

	// Add matches against two different opponent pairs
	matches := []*PairingMatch{
		{PairingID: pairing.ID, MatchID: "m1", MatchDate: 1700000000, Opponent1ID: "o1", Opponent2ID: "o2", PairingWon: true},
		{PairingID: pairing.ID, MatchID: "m2", MatchDate: 1700100000, Opponent1ID: "o1", Opponent2ID: "o2", PairingWon: true},
		{PairingID: pairing.ID, MatchID: "m3", MatchDate: 1700200000, Opponent1ID: "o3", Opponent2ID: "o4", PairingWon: false},
	}
	err = store.UpsertPairingMatches(matches)
	require.NoError(t, err)

	stats, err := store.GetPairingVsOpponentStats(pairing.ID)
	require.NoError(t, err)
	assert.Len(t, stats, 2)

	// First entry should be the most played opponent (o1, o2 with 2 matches)
	assert.Equal(t, 2, stats[0].MatchesPlayed)
	assert.Equal(t, 2, stats[0].MatchesWon)
	assert.Equal(t, 100.0, stats[0].WinPercentage)
}

func TestGetPairingTimeStats(t *testing.T) {
	store, teardown := setupTestStore(t)
	defer teardown()

	pairing, err := store.AddTrackedPairing("p1", "Player 1", "p2", "Player 2")
	require.NoError(t, err)

	matches := []*PairingMatch{
		{PairingID: pairing.ID, MatchID: "m1", MatchDate: 1700000000, DayOfWeek: 1, HourOfDay: 10, PairingWon: true},  // Monday morning
		{PairingID: pairing.ID, MatchID: "m2", MatchDate: 1700100000, DayOfWeek: 1, HourOfDay: 14, PairingWon: false}, // Monday afternoon
		{PairingID: pairing.ID, MatchID: "m3", MatchDate: 1700200000, DayOfWeek: 5, HourOfDay: 19, PairingWon: true},  // Friday evening
	}
	err = store.UpsertPairingMatches(matches)
	require.NoError(t, err)

	stats, err := store.GetPairingTimeStats(pairing.ID)
	require.NoError(t, err)

	// Check day of week stats
	assert.NotNil(t, stats.ByDayOfWeek[1]) // Monday
	assert.Equal(t, 2, stats.ByDayOfWeek[1].MatchesPlayed)
	assert.Equal(t, 1, stats.ByDayOfWeek[1].MatchesWon)

	assert.NotNil(t, stats.ByDayOfWeek[5]) // Friday
	assert.Equal(t, 1, stats.ByDayOfWeek[5].MatchesPlayed)

	// Check hour range stats
	assert.NotNil(t, stats.ByHourRange["morning"])
	assert.NotNil(t, stats.ByHourRange["afternoon"])
	assert.NotNil(t, stats.ByHourRange["evening"])
}

func TestGetPairingRecentForm(t *testing.T) {
	store, teardown := setupTestStore(t)
	defer teardown()

	pairing, err := store.AddTrackedPairing("p1", "Player 1", "p2", "Player 2")
	require.NoError(t, err)

	// Add 7 matches (5 wins, 2 losses)
	matches := []*PairingMatch{
		{PairingID: pairing.ID, MatchID: "m1", MatchDate: 1700700000, PairingWon: true},
		{PairingID: pairing.ID, MatchID: "m2", MatchDate: 1700600000, PairingWon: true},
		{PairingID: pairing.ID, MatchID: "m3", MatchDate: 1700500000, PairingWon: false},
		{PairingID: pairing.ID, MatchID: "m4", MatchDate: 1700400000, PairingWon: true},
		{PairingID: pairing.ID, MatchID: "m5", MatchDate: 1700300000, PairingWon: true},
		{PairingID: pairing.ID, MatchID: "m6", MatchDate: 1700200000, PairingWon: false},
		{PairingID: pairing.ID, MatchID: "m7", MatchDate: 1700100000, PairingWon: true},
	}
	err = store.UpsertPairingMatches(matches)
	require.NoError(t, err)

	form, err := store.GetPairingRecentForm(pairing.ID)
	require.NoError(t, err)

	// Last 5: W, W, L, W, W = 4 wins, 1 loss
	assert.Equal(t, 4, form.Last5Wins)
	assert.Equal(t, 1, form.Last5Losses)
	assert.InDelta(t, 80.0, form.Last5WinPct, 0.1)

	// Last 10 (we only have 7): 5 wins, 2 losses
	assert.Equal(t, 5, form.Last10Wins)
	assert.Equal(t, 2, form.Last10Losses)
}

func TestRemoveTrackedPairing(t *testing.T) {
	store, teardown := setupTestStore(t)
	defer teardown()

	pairing, err := store.AddTrackedPairing("p1", "Player 1", "p2", "Player 2")
	require.NoError(t, err)

	// Add a match
	err = store.UpsertPairingMatch(&PairingMatch{
		PairingID: pairing.ID,
		MatchID:   "m1",
		MatchDate: 1700000000,
	})
	require.NoError(t, err)

	// Remove the pairing (should cascade delete matches due to foreign key)
	err = store.RemoveTrackedPairing(pairing.ID)
	require.NoError(t, err)

	// Verify pairing is gone
	p, err := store.GetPairingByID(pairing.ID)
	require.NoError(t, err)
	assert.Nil(t, p)
}

// Helper to get db for direct queries
func getDB(s PairingsStore) *sql.DB {
	st := s.(*store)
	return st.db
}
