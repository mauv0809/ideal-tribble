package http

import (
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/charmbracelet/log"
	"github.com/mauv0809/ideal-tribble/internal/club"
	"github.com/mauv0809/ideal-tribble/internal/config"
	"github.com/mauv0809/ideal-tribble/internal/database"
	"github.com/mauv0809/ideal-tribble/internal/http/handlers"
	"github.com/mauv0809/ideal-tribble/internal/matchmaking"
	"github.com/mauv0809/ideal-tribble/internal/metrics"
	"github.com/mauv0809/ideal-tribble/internal/notifier"
	"github.com/mauv0809/ideal-tribble/internal/playtomic"
	"github.com/mauv0809/ideal-tribble/internal/processor"
	"github.com/mauv0809/ideal-tribble/internal/pubsub"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/slack-go/slack"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"bytes"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
)

const testSlackSigningSecret = "test-signing-secret"

// setupTestServer initializes a new server with a test database and mock clients.
func setupTestServer(t *testing.T, playtomicClient playtomic.PlaytomicClient, notifier notifier.Notifier, slackSigningSecret string) (*Server, func()) {
	t.Helper()

	// For handlers that use the store, we need a real db connection for now.
	db, dbTeardown, err := database.InitDB(":memory:", "", "", "../../migrations")
	require.NoError(t, err)

	clubStore := club.New(db)
	cfg := config.Config{Slack: config.SlackConfig{SigningSecret: slackSigningSecret}} // Use a default config with the provided secret

	reg := prometheus.NewRegistry()
	metricsSvc := metrics.NewService(reg)
	metricsHandler := metrics.NewMetricsHandler(reg)
	pubsub := pubsub.NewMock("TEST")
	proc := processor.New(clubStore, notifier, metricsSvc, pubsub)
	matchMaking := matchmaking.NewStore(db)
	// A real mux is needed to prevent the router from being nil.
	server := NewServer(clubStore, metricsSvc, metricsHandler, cfg, playtomicClient, notifier, proc, matchMaking, pubsub)

	teardown := func() {
		if dbTeardown != nil {
			dbTeardown()
		}
		db.Close()
	}
	return server, teardown
}

// createSlackCommandRequest creates an http.Request suitable for testing Slack slash commands,
// including the necessary signature and timestamp headers for verification.
func createSlackCommandRequest(t *testing.T, targetURL string, form url.Values, signingSecret string) *http.Request {
	t.Helper()

	body := strings.NewReader(form.Encode())
	req, err := http.NewRequest("POST", targetURL, body)
	require.NoError(t, err)

	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	// Generate a timestamp within a reasonable range (e.g., +/- 5 minutes)
	timestamp := time.Now().Unix()
	req.Header.Set("X-Slack-Request-Timestamp", strconv.FormatInt(timestamp, 10))

	// Read the request body to generate the signature.
	// The body needs to be re-set as a new `io.ReadCloser` for the actual handler after this.
	bodyBytes, err := io.ReadAll(req.Body) // Read the original body
	require.NoError(t, err)

	// Reset the request body for the actual handler after reading for signature calculation.
	req.Body = io.NopCloser(bytes.NewBuffer(bodyBytes))

	baseString := fmt.Sprintf("v0:%d:%s", timestamp, string(bodyBytes))
	log.Info("Signing secret", "signingSecret", signingSecret, "baseString", baseString)
	h := hmac.New(sha256.New, []byte(signingSecret))
	h.Write([]byte(baseString))
	signature := hex.EncodeToString(h.Sum(nil))

	req.Header.Set("X-Slack-Signature", "v0="+signature)
	log.Info("Generated Slack signature", "signature", "v0="+signature)

	return req
}

func TestHealthCheckHandler(t *testing.T) {
	server, teardown := setupTestServer(t, playtomic.NewMockClient(), notifier.NewMock(), "")
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
	server, teardown := setupTestServer(t, playtomic.NewMockClient(), notifier.NewMock(), "")
	defer teardown()

	// Add some players to the store
	server.Store.AddPlayer("player1", "Player One", 1.0)
	server.Store.AddPlayer("player2", "Player Two", 1.0)

	req, err := http.NewRequest("GET", "/members", nil)
	require.NoError(t, err)

	rr := httptest.NewRecorder()
	handler := handlers.ListMembersHandler(server.Store)
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
	server, teardown := setupTestServer(t, playtomic.NewMockClient(), mockNotifier, testSlackSigningSecret)
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
		MatchTypeEnum: playtomic.MatchTypeEnumDoubles,
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

		req := createSlackCommandRequest(t, "/slack/command/player-stats", form, testSlackSigningSecret)

		rr := httptest.NewRecorder()
		server.Router.ServeHTTP(rr, req)

		assert.Equal(t, http.StatusOK, rr.Code)
	})

	t.Run("handles not found player", func(t *testing.T) {
		form := url.Values{}
		form.Set("text", "Unknown")

		req := createSlackCommandRequest(t, "/slack/command/player-stats", form, testSlackSigningSecret)

		rr := httptest.NewRecorder()
		server.Router.ServeHTTP(rr, req)

		assert.Equal(t, http.StatusOK, rr.Code)
	})

	t.Run("handles missing player name", func(t *testing.T) {
		req := createSlackCommandRequest(t, "/slack/command/player-stats", url.Values{}, testSlackSigningSecret)

		rr := httptest.NewRecorder()
		server.Router.ServeHTTP(rr, req)

		assert.Equal(t, http.StatusBadRequest, rr.Code)
	})

	t.Run("rejects request with invalid signature", func(t *testing.T) {
		form := url.Values{}
		form.Set("text", "Morten")

		req := createSlackCommandRequest(t, "/slack/command/player-stats", form, testSlackSigningSecret)

		// Tamper with the signature to make it invalid
		req.Header.Set("X-Slack-Signature", "v0=invalid-signature")

		rr := httptest.NewRecorder()
		server.Router.ServeHTTP(rr, req)

		assert.Equal(t, http.StatusUnauthorized, rr.Code)
	})

	t.Run("rejects request with missing signature", func(t *testing.T) {
		form := url.Values{}
		form.Set("text", "Morten")

		req := createSlackCommandRequest(t, "/slack/command/player-stats", form, testSlackSigningSecret)

		// Remove the signature header
		req.Header.Del("X-Slack-Signature")

		rr := httptest.NewRecorder()
		server.Router.ServeHTTP(rr, req)

		assert.Equal(t, http.StatusUnauthorized, rr.Code)
	})

	t.Run("rejects request with outdated timestamp", func(t *testing.T) {
		form := url.Values{}
		form.Set("text", "Morten")

		req := createSlackCommandRequest(t, "/slack/command/player-stats", form, testSlackSigningSecret)

		// Set an outdated timestamp (e.g., 6 minutes ago)
		req.Header.Set("X-Slack-Request-Timestamp", strconv.FormatInt(time.Now().Add(-6*time.Minute).Unix(), 10))

		rr := httptest.NewRecorder()
		server.Router.ServeHTTP(rr, req)

		assert.Equal(t, http.StatusUnauthorized, rr.Code)
	})
}

func TestLevelLeaderboardCommandHandler(t *testing.T) {
	mockNotifier := notifier.NewMock()
	mockNotifier.FormatLevelLeaderboardResponseFunc = func(players []club.PlayerInfo) (any, error) {
		return slack.Message{}, nil
	}
	server, teardown := setupTestServer(t, playtomic.NewMockClient(), mockNotifier, testSlackSigningSecret)
	defer teardown()

	// Setup: Add players with different levels
	server.Store.AddPlayer("p1", "Player A", 4.5)
	server.Store.AddPlayer("p2", "Player B", 3.5)
	server.Store.AddPlayer("p3", "Player C", 2.5)

	req := createSlackCommandRequest(t, "/slack/command/level-leaderboard", url.Values{}, testSlackSigningSecret)

	rr := httptest.NewRecorder()
	server.Router.ServeHTTP(rr, req)

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
			MatchTypeEnum: playtomic.MatchTypeEnumDoubles,
		}, nil
	}

	server, teardown := setupTestServer(t, mockClient, notifier.NewMock(), "")
	defer teardown()
	// Add a known player so that a match is fetched
	server.Store.AddPlayer("p1", "Player One", 1.0)
	server.Store.AddPlayer("p2", "Player Two", 1.0)
	server.Store.AddPlayer("p3", "Player Three", 1.0)
	server.Store.AddPlayer("p4", "Player Four", 1.0)

	req, err := http.NewRequest("GET", "/fetch", nil)
	require.NoError(t, err)

	rr := httptest.NewRecorder()
	handler := handlers.FetchMatchesHandler(server.Store, server.Metrics, server.Cfg, server.PlaytomicClient)
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
		server, teardown := setupTestServer(t, playtomic.NewMockClient(), notifier.NewMock(), "")
		defer teardown()

		server.Store.AddPlayer("p1", "Player One", 1.0)
		match := &playtomic.PadelMatch{
			MatchID:          "m1",
			OwnerID:          "p1",
			ProcessingStatus: playtomic.StatusNew,
			Start:            time.Now().Unix(),
			MatchTypeEnum:    playtomic.MatchTypeEnumDoubles,
			Teams: []playtomic.Team{
				{Players: []playtomic.Player{{UserID: "p1"}}},
				{Players: []playtomic.Player{{UserID: "p2"}}},
			},
		}
		require.NoError(t, server.Store.UpsertMatch(match))

		req, err := http.NewRequest("GET", "/process", nil)
		require.NoError(t, err)
		rr := httptest.NewRecorder()
		handler := handlers.ProcessMatchesHandler(server.Processor)
		handler.ServeHTTP(rr, req)

		assert.Equal(t, http.StatusOK, rr.Code)
		matches, err := server.Store.GetAllMatches()
		require.NoError(t, err)
		require.Len(t, matches, 1)
		assert.Equal(t, playtomic.StatusAssigningBallBringer, matches[0].ProcessingStatus)
	})

	t.Run("sends result notification for new but already played match", func(t *testing.T) {
		server, teardown := setupTestServer(t, playtomic.NewMockClient(), notifier.NewMock(), "")
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
			MatchTypeEnum: playtomic.MatchTypeEnumDoubles,
		}
		require.NoError(t, server.Store.UpsertMatch(match))

		req, err := http.NewRequest("GET", "/process", nil)
		require.NoError(t, err)
		rr := httptest.NewRecorder()
		handler := handlers.ProcessMatchesHandler(server.Processor)
		handler.ServeHTTP(rr, req)

		assert.Equal(t, http.StatusOK, rr.Code)
		matches, err := server.Store.GetAllMatches()
		require.NoError(t, err)
		require.Len(t, matches, 1)
		assert.Equal(t, playtomic.StatusUpdatingPlayerStats, matches[0].ProcessingStatus)
	})
}
