package http

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/mauv0809/ideal-tribble/internal/club"
	"github.com/mauv0809/ideal-tribble/internal/config"
	"github.com/mauv0809/ideal-tribble/internal/database"
	"github.com/mauv0809/ideal-tribble/internal/metrics"
	"github.com/mauv0809/ideal-tribble/internal/playtomic"
	"github.com/mauv0809/ideal-tribble/internal/processor"
	internalslack "github.com/mauv0809/ideal-tribble/internal/slack"
	"github.com/slack-go/slack"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// mockPlaytomicClient is a mock implementation of the Playtomic client for testing.
type mockPlaytomicClient struct {
	GetMatchesFunc       func(params *playtomic.SearchMatchesParams) ([]playtomic.MatchSummary, error)
	GetSpecificMatchFunc func(matchID string) (playtomic.PadelMatch, error)
}

func (m *mockPlaytomicClient) GetMatches(params *playtomic.SearchMatchesParams) ([]playtomic.MatchSummary, error) {
	if m.GetMatchesFunc != nil {
		return m.GetMatchesFunc(params)
	}
	return []playtomic.MatchSummary{}, nil
}

func (m *mockPlaytomicClient) GetSpecificMatch(matchID string) (playtomic.PadelMatch, error) {
	if m.GetSpecificMatchFunc != nil {
		return m.GetSpecificMatchFunc(matchID)
	}
	return playtomic.PadelMatch{MatchID: matchID}, nil
}

// setupTestServer initializes a new server with a test database and mock clients.
func setupTestServer(t *testing.T, playtomicClient playtomic.PlaytomicClient, slackClient *internalslack.SlackClient) (*Server, func()) {
	t.Helper()

	// For handlers that use the store, we need a real db connection.
	db, err := database.InitDB(":memory:", "", "")
	require.NoError(t, err)
	clubStore := club.New(db)
	cfg := config.Config{} // Use a default config
	metricsStore := metrics.New(db)

	proc := processor.New(clubStore, slackClient, metricsStore)

	server := NewServer(clubStore, metricsStore, cfg, playtomicClient, slackClient, proc)

	teardown := func() {
		db.Close()
	}
	return server, teardown
}

func TestHealthCheckHandler(t *testing.T) {
	server, teardown := setupTestServer(t, &mockPlaytomicClient{}, internalslack.NewClient("", ""))
	defer teardown()

	req, err := http.NewRequest("GET", "/health", nil)
	require.NoError(t, err)

	rr := httptest.NewRecorder()
	handler := server.HealthCheckHandler()

	handler.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code, "handler returned wrong status code")
	assert.Equal(t, "OK!", rr.Body.String(), "handler returned unexpected body")
}

func TestListMembersHandler(t *testing.T) {
	server, teardown := setupTestServer(t, &mockPlaytomicClient{}, internalslack.NewClient("", ""))
	defer teardown()

	// Add some players to the store
	server.Store.AddPlayer("player1", "Player One", 1.0)
	server.Store.AddPlayer("player2", "Player Two", 1.0)

	req, err := http.NewRequest("GET", "/members", nil)
	require.NoError(t, err)

	rr := httptest.NewRecorder()
	handler := server.ListMembersHandler()
	handler.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)
	assert.Contains(t, rr.Body.String(), "Player One")
	assert.Contains(t, rr.Body.String(), "player2")
}

func TestPlayerStatsCommandHandler(t *testing.T) {
	server, teardown := setupTestServer(t, &mockPlaytomicClient{}, internalslack.NewClientWithAPI(nil, "C123"))
	defer teardown()

	// Setup: Add players and their stats for the match
	server.Store.AddPlayer("p1", "Morten Voss", 1.0)
	server.Store.AddPlayer("p2", "Player Two", 1.0)
	server.Store.AddPlayer("p3", "Player Three", 1.0)
	server.Store.AddPlayer("p4", "Player Four", 1.0)

	match := &playtomic.PadelMatch{
		MatchID: "match1",
		OwnerID: "p1",
		Teams: []playtomic.Team{
			{ID: "t1", TeamResult: "WON", Players: []playtomic.Player{{UserID: "p1", Name: "Morten Voss"}, {UserID: "p2", Name: "Player Two"}}},
			{ID: "t2", Players: []playtomic.Player{{UserID: "p3", Name: "Player Three"}, {UserID: "p4", Name: "Player Four"}}},
		},
		Results: []playtomic.SetResult{
			{Name: "Set-1", Scores: map[string]int{"t1": 6, "t2": 4}},
			{Name: "Set-2", Scores: map[string]int{"t1": 6, "t2": 4}},
		},
		ProcessingStatus: playtomic.StatusCompleted,
		ResultsStatus:    playtomic.ResultsStatusConfirmed,
		GameStatus:       playtomic.GameStatusUnknown,
	}
	server.Store.UpsertMatch(match)
	server.Store.UpdatePlayerStats(match)

	t.Run("handles found player", func(t *testing.T) {
		form := url.Values{}
		form.Set("text", "Morten")

		req, err := http.NewRequest("POST", "/command/player-stats", strings.NewReader(form.Encode()))
		require.NoError(t, err)
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

		rr := httptest.NewRecorder()
		handler := server.PlayerStatsCommandHandler()
		handler.ServeHTTP(rr, req)

		assert.Equal(t, http.StatusOK, rr.Code)
		assert.Contains(t, rr.Body.String(), "Stats for Morten Voss")
		assert.Contains(t, rr.Body.String(), "100.00%")
	})

	t.Run("handles not found player", func(t *testing.T) {
		form := url.Values{}
		form.Set("text", "Unknown")

		req, err := http.NewRequest("POST", "/command/player-stats", strings.NewReader(form.Encode()))
		require.NoError(t, err)
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

		rr := httptest.NewRecorder()
		handler := server.PlayerStatsCommandHandler()
		handler.ServeHTTP(rr, req)

		assert.Equal(t, http.StatusOK, rr.Code)
		assert.Contains(t, rr.Body.String(), "Sorry, I couldn't find a player matching")
	})

	t.Run("handles missing player name", func(t *testing.T) {
		req, err := http.NewRequest("POST", "/command/player-stats", nil)
		require.NoError(t, err)
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

		rr := httptest.NewRecorder()
		handler := server.PlayerStatsCommandHandler()
		handler.ServeHTTP(rr, req)

		assert.Equal(t, http.StatusBadRequest, rr.Code)
	})
}

func TestLevelLeaderboardCommandHandler(t *testing.T) {
	server, teardown := setupTestServer(t, &mockPlaytomicClient{}, internalslack.NewClientWithAPI(nil, "C123"))
	defer teardown()

	// Setup: Add players with different levels
	server.Store.AddPlayer("p1", "Player A", 4.5)
	server.Store.AddPlayer("p2", "Player B", 3.5)
	server.Store.AddPlayer("p3", "Player C", 2.5)

	req, err := http.NewRequest("POST", "/command/level-leaderboard", nil)
	require.NoError(t, err)

	rr := httptest.NewRecorder()
	handler := server.LevelLeaderboardCommandHandler()
	handler.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)

	// Unmarshal the response to inspect the blocks directly
	var msg slack.Message
	err = json.Unmarshal(rr.Body.Bytes(), &msg)
	require.NoError(t, err)

	require.Len(t, msg.Blocks.BlockSet, 4, "Expected a header and three player blocks")

	// Check that the header is present
	header, ok := msg.Blocks.BlockSet[0].(*slack.HeaderBlock)
	require.True(t, ok)
	assert.Equal(t, "🏆 Player Leaderboard (by Level) 🏆", header.Text.Text)

	// Check the order of players
	player1, ok := msg.Blocks.BlockSet[1].(*slack.SectionBlock)
	require.True(t, ok)
	assert.Contains(t, player1.Text.Text, "1. 🥇 Player A")

	player2, ok := msg.Blocks.BlockSet[2].(*slack.SectionBlock)
	require.True(t, ok)
	assert.Contains(t, player2.Text.Text, "2. 🥈 Player B")

	player3, ok := msg.Blocks.BlockSet[3].(*slack.SectionBlock)
	require.True(t, ok)
	assert.Contains(t, player3.Text.Text, "3. 🥉 Player C")
}

func TestFetchMatchesHandler(t *testing.T) {
	mockClient := &mockPlaytomicClient{}
	ownerID := "p1"
	// Mock the GetMatches endpoint
	mockClient.GetMatchesFunc = func(params *playtomic.SearchMatchesParams) ([]playtomic.MatchSummary, error) {
		return []playtomic.MatchSummary{
			{MatchID: "m1", OwnerID: &ownerID},
			{MatchID: "m2", OwnerID: nil}, // No owner, should be skipped
		}, nil
	}
	// Mock the GetSpecificMatch endpoint
	mockClient.GetSpecificMatchFunc = func(matchID string) (playtomic.PadelMatch, error) {
		return playtomic.PadelMatch{
			MatchID: matchID,
			OwnerID: ownerID,
			Teams: []playtomic.Team{
				{Players: []playtomic.Player{{UserID: "p1"}, {UserID: "p2"}}},
				{Players: []playtomic.Player{{UserID: "p3"}, {UserID: "p4"}}},
			},
		}, nil
	}

	server, teardown := setupTestServer(t, mockClient, internalslack.NewClient("", ""))
	defer teardown()
	// Add a known player so that a match is fetched
	server.Store.AddPlayer("p1", "Player One", 1.0)
	server.Store.AddPlayer("p2", "Player Two", 1.0)
	server.Store.AddPlayer("p3", "Player Three", 1.0)

	req, err := http.NewRequest("GET", "/fetch", nil)
	require.NoError(t, err)

	rr := httptest.NewRecorder()
	handler := server.FetchMatchesHandler()
	handler.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)

	// Verify that the correct match was upserted
	matches, err := server.Store.GetAllMatches()
	require.NoError(t, err)
	require.Len(t, matches, 1)
	assert.Equal(t, "m1", matches[0].MatchID)
	assert.Equal(t, playtomic.StatusNew, matches[0].ProcessingStatus)

	// Verify that all players from the match were added to the store
	assert.True(t, server.Store.IsKnownPlayer("p1"))
	assert.True(t, server.Store.IsKnownPlayer("p2"))
	assert.True(t, server.Store.IsKnownPlayer("p3"))
	assert.True(t, server.Store.IsKnownPlayer("p4"))
}

func TestProcessMatchesHandler(t *testing.T) {
	t.Run("sends booking notification for new match", func(t *testing.T) {
		bookingNotificationSent := false
		resultNotificationSent := false
		slackServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// A simple way to distinguish notifications is to check the message content
			bodyBytes, _ := io.ReadAll(r.Body)
			bodyString := string(bodyBytes)
			if strings.Contains(bodyString, "New+match+booked") {
				bookingNotificationSent = true
			}
			if strings.Contains(bodyString, "Match+finished") {
				resultNotificationSent = true
			}
			w.Header().Set("Content-Type", "application/json")
			w.Write([]byte(`{"ok": true, "ts": "12345.67890"}`))
		}))
		defer slackServer.Close()

		slackAPI := slack.New("test-token", slack.OptionAPIURL(slackServer.URL+"/"))
		slackClient := internalslack.NewClientWithAPI(slackAPI, "C123")
		server, teardown := setupTestServer(t, &mockPlaytomicClient{}, slackClient)
		defer teardown()

		server.Store.AddPlayer("p1", "Player One", 1.0)
		match := &playtomic.PadelMatch{
			MatchID:          "m1",
			OwnerID:          "p1",
			ProcessingStatus: playtomic.StatusNew,
			Start:            time.Now().Unix(),
		}
		require.NoError(t, server.Store.UpsertMatch(match))

		req, err := http.NewRequest("GET", "/process", nil)
		require.NoError(t, err)
		rr := httptest.NewRecorder()
		handler := server.ProcessMatchesHandler()
		handler.ServeHTTP(rr, req)

		assert.Equal(t, http.StatusOK, rr.Code)
		assert.True(t, bookingNotificationSent, "Expected booking notification to be sent")
		assert.False(t, resultNotificationSent, "Expected result notification NOT to be sent")
		matches, err := server.Store.GetAllMatches()
		require.NoError(t, err)
		require.Len(t, matches, 1)
		assert.Equal(t, playtomic.StatusBookingNotified, matches[0].ProcessingStatus)
	})

	t.Run("sends result notification for new but already played match", func(t *testing.T) {
		bookingNotificationSent := false
		resultNotificationSent := false
		slackServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			bodyBytes, _ := io.ReadAll(r.Body)
			bodyString := string(bodyBytes)
			if strings.Contains(bodyString, "New+match+booked") {
				bookingNotificationSent = true
			}
			if strings.Contains(bodyString, "Match+finished") {
				resultNotificationSent = true
			}
			w.Header().Set("Content-Type", "application/json")
			w.Write([]byte(`{"ok": true, "ts": "12345.67890"}`))
		}))
		defer slackServer.Close()

		slackAPI := slack.New("test-token", slack.OptionAPIURL(slackServer.URL+"/"))
		slackClient := internalslack.NewClientWithAPI(slackAPI, "C123")
		server, teardown := setupTestServer(t, &mockPlaytomicClient{}, slackClient)
		defer teardown()

		// Add players to the store first to satisfy foreign key constraints.
		server.Store.AddPlayer("p1", "Player One", 1.0)
		server.Store.AddPlayer("p2", "Player Two", 1.0)
		server.Store.AddPlayer("p3", "Player Three", 1.0)
		server.Store.AddPlayer("p4", "Player Four", 1.0)
		match := &playtomic.PadelMatch{
			MatchID:          "m1",
			OwnerID:          "p1",
			ProcessingStatus: playtomic.StatusNew,
			GameStatus:       playtomic.GameStatusPlayed,
			ResultsStatus:    playtomic.ResultsStatusConfirmed,
			Start:            time.Now().Unix(),
			Teams: []playtomic.Team{
				{Players: []playtomic.Player{{UserID: "p1"}, {UserID: "p2"}}},
				{Players: []playtomic.Player{{UserID: "p3"}, {UserID: "p4"}}},
			},
		}
		require.NoError(t, server.Store.UpsertMatch(match))

		req, err := http.NewRequest("GET", "/process", nil)
		require.NoError(t, err)
		rr := httptest.NewRecorder()
		handler := server.ProcessMatchesHandler()
		handler.ServeHTTP(rr, req)

		assert.Equal(t, http.StatusOK, rr.Code)
		assert.False(t, bookingNotificationSent, "Expected booking notification NOT to be sent")
		assert.True(t, resultNotificationSent, "Expected result notification to be sent")

		matches, err := server.Store.GetAllMatches()
		require.NoError(t, err)
		require.Len(t, matches, 1)
		assert.Equal(t, playtomic.StatusCompleted, matches[0].ProcessingStatus)
	})
}
