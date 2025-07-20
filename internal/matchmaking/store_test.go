package matchmaking_test

import (
	"database/sql"
	"testing"

	"github.com/mauv0809/ideal-tribble/internal/club"
	"github.com/mauv0809/ideal-tribble/internal/database"
	"github.com/mauv0809/ideal-tribble/internal/matchmaking"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// setupTestDB creates a temporary in-memory SQLite database for testing.
func setupTestDB(t *testing.T) (matchmaking.MatchmakingService, *sql.DB, func()) {
	t.Helper()

	db, dbTeardown, err := database.InitDB(":memory:", "", "", "../../migrations")
	require.NoError(t, err)

	clubStore := club.New(db)
	store := matchmaking.NewStore(db, clubStore)

	teardown := func() {
		dbTeardown()
		db.Close()
	}

	return store, db, teardown
}

func TestCanProposeMatch(t *testing.T) {
	store, db, teardown := setupTestDB(t)
	defer teardown()

	// Create test users in the players table first (required by foreign key)
	clubStore := club.New(db)
	clubStore.AddPlayer("user1", "Test User", 1.0)
	clubStore.AddPlayer("player1", "Player 1", 1.0)
	clubStore.AddPlayer("player2", "Player 2", 1.0)
	clubStore.AddPlayer("player3", "Player 3", 1.0)
	clubStore.AddPlayer("player4", "Player 4", 1.0)
	clubStore.AddPlayer("player5", "Player 5", 1.0)
	clubStore.AddPlayer("player6", "Player 6", 1.0)

	// Create a test match request
	request, err := store.CreateMatchRequest("user1", "Test User", "channel1")
	require.NoError(t, err)

	tests := []struct {
		name           string
		availabilities []struct {
			playerID   string
			playerName string
			dates      []string
		}
		expectedCanPropose bool
		expectedPlayerCount int
		expectedDate       string
	}{
		{
			name: "enough players available",
			availabilities: []struct {
				playerID   string
				playerName string
				dates      []string
			}{
				{"player1", "Player 1", []string{"2024-01-15"}},
				{"player2", "Player 2", []string{"2024-01-15"}},
				{"player3", "Player 3", []string{"2024-01-15"}},
				{"player4", "Player 4", []string{"2024-01-15"}},
			},
			expectedCanPropose: true,
			expectedPlayerCount: 4,
			expectedDate: "2024-01-15",
		},
		{
			name: "not enough players",
			availabilities: []struct {
				playerID   string
				playerName string
				dates      []string
			}{
				{"player1", "Player 1", []string{"2024-01-16"}},
				{"player2", "Player 2", []string{"2024-01-16"}},
				{"player3", "Player 3", []string{"2024-01-16"}},
			},
			expectedCanPropose: false,
			expectedPlayerCount: 3,
			expectedDate: "2024-01-16",
		},
		{
			name: "multiple dates, best one selected",
			availabilities: []struct {
				playerID   string
				playerName string
				dates      []string
			}{
				{"player1", "Player 1", []string{"2024-01-17", "2024-01-18"}},
				{"player2", "Player 2", []string{"2024-01-17", "2024-01-18"}},
				{"player3", "Player 3", []string{"2024-01-17"}},
				{"player4", "Player 4", []string{"2024-01-18"}},
				{"player5", "Player 5", []string{"2024-01-18"}},
				{"player6", "Player 6", []string{"2024-01-18"}},
			},
			expectedCanPropose: true,
			expectedPlayerCount: 5,
			expectedDate: "2024-01-18",
		},
		{
			name: "no availability",
			availabilities: []struct {
				playerID   string
				playerName string
				dates      []string
			}{},
			expectedCanPropose: false,
			expectedPlayerCount: 0,
			expectedDate: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Clear any existing availability
			_, err := db.Exec("DELETE FROM match_request_availability WHERE match_request_id = ?", request.ID)
			require.NoError(t, err)

			// Add test availability data
			for _, avail := range tt.availabilities {
				for _, date := range avail.dates {
					err := store.AddPlayerAvailability(request.ID, avail.playerID, avail.playerName, date)
					require.NoError(t, err)
				}
			}

			// Test CanProposeMatch
			canPropose, result, err := store.CanProposeMatch(request.ID)
			require.NoError(t, err)

			assert.Equal(t, tt.expectedCanPropose, canPropose)

			if tt.expectedPlayerCount > 0 {
				require.NotNil(t, result)
				assert.Equal(t, tt.expectedPlayerCount, result.PlayerCount)
				assert.Equal(t, tt.expectedDate, result.Date)
			} else {
				assert.Nil(t, result)
			}
		})
	}
}

func TestAnalyzeAvailability(t *testing.T) {
	store, db, teardown := setupTestDB(t)
	defer teardown()

	// Create test users in the players table first
	clubStore := club.New(db)
	clubStore.AddPlayer("user1", "Test User", 1.0)
	clubStore.AddPlayer("player1", "Player 1", 1.0)
	clubStore.AddPlayer("player2", "Player 2", 1.0)
	clubStore.AddPlayer("player3", "Player 3", 1.0)
	clubStore.AddPlayer("player4", "Player 4", 1.0)
	clubStore.AddPlayer("player5", "Player 5", 1.0)

	// Create a test match request
	request, err := store.CreateMatchRequest("user1", "Test User", "channel1")
	require.NoError(t, err)

	// Add some availability data
	availabilities := []struct {
		playerID   string
		playerName string
		dates      []string
	}{
		{"player1", "Player 1", []string{"2024-01-15", "2024-01-16"}},
		{"player2", "Player 2", []string{"2024-01-15"}},
		{"player3", "Player 3", []string{"2024-01-16"}},
		{"player4", "Player 4", []string{"2024-01-16"}},
		{"player5", "Player 5", []string{"2024-01-16"}},
	}

	for _, avail := range availabilities {
		for _, date := range avail.dates {
			err := store.AddPlayerAvailability(request.ID, avail.playerID, avail.playerName, date)
			require.NoError(t, err)
		}
	}

	// Test AnalyzeAvailability
	results, err := store.AnalyzeAvailability(request.ID)
	require.NoError(t, err)

	// Should have 2 dates
	assert.Len(t, results, 2)

	// Results should be sorted by player count (descending)
	assert.Equal(t, "2024-01-16", results[0].Date)
	assert.Equal(t, 4, results[0].PlayerCount)
	assert.Equal(t, "2024-01-15", results[1].Date)
	assert.Equal(t, 2, results[1].PlayerCount)
}

func TestAddRemovePlayerAvailability(t *testing.T) {
	store, db, teardown := setupTestDB(t)
	defer teardown()

	// Create test users in the players table first
	clubStore := club.New(db)
	clubStore.AddPlayer("user1", "Test User", 1.0)
	clubStore.AddPlayer("player1", "Player 1", 1.0)

	// Create a test match request
	request, err := store.CreateMatchRequest("user1", "Test User", "channel1")
	require.NoError(t, err)

	// Test adding availability
	err = store.AddPlayerAvailability(request.ID, "player1", "Player 1", "2024-01-15")
	require.NoError(t, err)

	// Verify it was added
	availabilities, err := store.GetPlayerAvailability(request.ID)
	require.NoError(t, err)
	assert.Len(t, availabilities, 1)
	assert.Equal(t, "player1", availabilities[0].PlayerID)
	assert.Equal(t, "2024-01-15", availabilities[0].AvailableDate)

	// Test adding duplicate (should not create duplicate)
	err = store.AddPlayerAvailability(request.ID, "player1", "Player 1", "2024-01-15")
	require.NoError(t, err)

	availabilities, err = store.GetPlayerAvailability(request.ID)
	require.NoError(t, err)
	assert.Len(t, availabilities, 1) // Still only 1

	// Test removing availability
	err = store.RemovePlayerAvailability(request.ID, "player1", "2024-01-15")
	require.NoError(t, err)

	availabilities, err = store.GetPlayerAvailability(request.ID)
	require.NoError(t, err)
	assert.Len(t, availabilities, 0)
}

func TestIsActiveMatchRequestMessage(t *testing.T) {
	store, db, teardown := setupTestDB(t)
	defer teardown()

	// Create a test user in the players table first
	clubStore := club.New(db)
	clubStore.AddPlayer("user1", "Test User", 1.0)

	// Create a test match request
	request, err := store.CreateMatchRequest("user1", "Test User", "channel1")
	require.NoError(t, err)

	// Update with message timestamp
	err = store.UpdateMatchRequestMessageTimestamps(request.ID, "thread123", "msg123")
	require.NoError(t, err)

	// Test with correct message timestamp
	requestID, isActive, err := store.IsActiveMatchRequestMessage("msg123")
	require.NoError(t, err)
	assert.True(t, isActive)
	assert.Equal(t, request.ID, requestID)

	// Test with incorrect message timestamp
	requestID, isActive, err = store.IsActiveMatchRequestMessage("msg999")
	require.NoError(t, err)
	assert.False(t, isActive)
	assert.Empty(t, requestID)

	// Change status to confirmed (no longer collecting availability)
	err = store.UpdateMatchRequestStatus(request.ID, matchmaking.StatusConfirmed)
	require.NoError(t, err)

	// Should no longer be active
	requestID, isActive, err = store.IsActiveMatchRequestMessage("msg123")
	require.NoError(t, err)
	assert.False(t, isActive)
	assert.Empty(t, requestID)
}

func TestCreateAndGetMatchRequest(t *testing.T) {
	store, db, teardown := setupTestDB(t)
	defer teardown()

	// Create a test user in the players table first
	clubStore := club.New(db)
	clubStore.AddPlayer("user123", "Test User", 1.0)

	// Create a match request
	request, err := store.CreateMatchRequest("user123", "Test User", "channel456")
	require.NoError(t, err)

	assert.NotEmpty(t, request.ID)
	assert.Equal(t, "user123", request.RequesterID)
	assert.Equal(t, "Test User", request.RequesterName)
	assert.Equal(t, "channel456", request.ChannelID)
	assert.Equal(t, matchmaking.StatusCollectingAvailability, request.Status)
	assert.NotZero(t, request.CreatedAt)
	assert.NotZero(t, request.UpdatedAt)

	// Retrieve the match request
	retrieved, err := store.GetMatchRequest(request.ID)
	require.NoError(t, err)

	assert.Equal(t, request.ID, retrieved.ID)
	assert.Equal(t, request.RequesterID, retrieved.RequesterID)
	assert.Equal(t, request.RequesterName, retrieved.RequesterName)
	assert.Equal(t, request.ChannelID, retrieved.ChannelID)
	assert.Equal(t, request.Status, retrieved.Status)
}

func TestGetActiveMatchRequests(t *testing.T) {
	store, db, teardown := setupTestDB(t)
	defer teardown()

	// Create test users in the players table first
	clubStore := club.New(db)
	clubStore.AddPlayer("user1", "User 1", 1.0)
	clubStore.AddPlayer("user2", "User 2", 1.0)
	clubStore.AddPlayer("user3", "User 3", 1.0)

	// Create some match requests
	request1, err := store.CreateMatchRequest("user1", "User 1", "channel1")
	require.NoError(t, err)

	request2, err := store.CreateMatchRequest("user2", "User 2", "channel1")
	require.NoError(t, err)

	request3, err := store.CreateMatchRequest("user3", "User 3", "channel1")
	require.NoError(t, err)

	// Confirm one request (should make it inactive)
	err = store.UpdateMatchRequestStatus(request3.ID, matchmaking.StatusConfirmed)
	require.NoError(t, err)

	// Get active requests
	activeRequests, err := store.GetActiveMatchRequests()
	require.NoError(t, err)

	// Should have 2 active requests (request1 and request2)
	assert.Len(t, activeRequests, 2)

	// Check that the confirmed request is not in the list
	activeIDs := make([]string, len(activeRequests))
	for i, req := range activeRequests {
		activeIDs[i] = req.ID
	}

	assert.Contains(t, activeIDs, request1.ID)
	assert.Contains(t, activeIDs, request2.ID)
	assert.NotContains(t, activeIDs, request3.ID)
}