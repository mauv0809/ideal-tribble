package http

import (
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
	"github.com/mauv0809/ideal-tribble/internal/notifier"
	"github.com/mauv0809/ideal-tribble/internal/playtomic"
	"github.com/mauv0809/ideal-tribble/internal/processor"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/slack-go/slack"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// setupTestServer initializes a new server with a test database and mock clients.
func setupTestServer(t *testing.T, playtomicClient playtomic.PlaytomicClient, notifier notifier.Notifier) (*Server, func()) {
	t.Helper()

	// For handlers that use the store, we need a real db connection for now.
	db, dbTeardown, err := database.InitDB(":memory:", "", "")
	require.NoError(t, err)

	clubStore := club.New(db)
	cfg := config.Config{} // Use a default config

	reg := prometheus.NewRegistry()
	metricsSvc := metrics.NewService(reg)
	metricsHandler := metrics.NewMetricsHandler(reg)
	proc := processor.New(clubStore, notifier, metricsSvc)

	// A real mux is needed to prevent the router from being nil.
	server := NewServer(clubStore, metricsSvc, metricsHandler, cfg, playtomicClient, notifier, proc)

	teardown := func() {
		if dbTeardown != nil {
			dbTeardown()
		}
		db.Close()
	}
	return server, teardown
}

func TestHealthCheckHandler(t *testing.T) {
	server, teardown := setupTestServer(t, playtomic.NewMockClient(), notifier.NewMock())
	defer teardown()

	req, err := http.NewRequest("GET", "/health", nil)
	require.NoError(t, err)

	rr := httptest.NewRecorder()
	// Use the server's router to serve the request, which is more robust.
	server.Router.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code, "handler returned wrong status code")
	assert.Equal(t, "OK!", rr.Body.String(), "handler returned unexpected body")
}

func TestListMembersHandler(t *testing.T) {
	server, teardown := setupTestServer(t, playtomic.NewMockClient(), notifier.NewMock())
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
	mockNotifier := notifier.NewMock()
	mockNotifier.FormatPlayerStatsResponseFunc = func(stats *club.PlayerStats, query string) (any, error) {
		return slack.Message{}, nil
	}
	mockNotifier.FormatPlayerNotFoundResponseFunc = func(query string) (any, error) {
		return slack.Message{}, nil
	}
	server, teardown := setupTestServer(t, playtomic.NewMockClient(), mockNotifier)
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
	mockNotifier := notifier.NewMock()
	mockNotifier.FormatLevelLeaderboardResponseFunc = func(players []club.PlayerInfo) (any, error) {
		return slack.Message{}, nil
	}
	server, teardown := setupTestServer(t, playtomic.NewMockClient(), mockNotifier)
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
}

func TestFetchMatchesHandler(t *testing.T) {
	mockClient := playtomic.NewMockClient()
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

	server, teardown := setupTestServer(t, mockClient, notifier.NewMock())
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
}

func TestProcessMatchesHandler(t *testing.T) {
	t.Run("sends booking notification for new match", func(t *testing.T) {
		server, teardown := setupTestServer(t, playtomic.NewMockClient(), notifier.NewMock())
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
		matches, err := server.Store.GetAllMatches()
		require.NoError(t, err)
		require.Len(t, matches, 1)
		assert.Equal(t, playtomic.StatusBookingNotified, matches[0].ProcessingStatus)
	})

	t.Run("sends result notification for new but already played match", func(t *testing.T) {
		server, teardown := setupTestServer(t, playtomic.NewMockClient(), notifier.NewMock())
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
		matches, err := server.Store.GetAllMatches()
		require.NoError(t, err)
		require.Len(t, matches, 1)
		assert.Equal(t, playtomic.StatusCompleted, matches[0].ProcessingStatus)
	})
}
