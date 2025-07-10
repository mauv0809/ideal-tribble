package playtomic

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/rafa-garcia/go-playtomic-api/client"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGetSpecificMatch(t *testing.T) {
	// Sample JSON response from the Playtomic API
	mockJSONResponse := `{
		"owner_id": "user-123",
		"start_date": "2025-07-09T18:00:00",
		"end_date": "2025-07-09T19:30:00",
		"created_at": "2025-07-08T10:00:00",
		"status": "CONFIRMED",
		"game_status": "PLAYED",
		"results_status": "CONFIRMED",
		"resource_name": "Court 1",
		"price": "20 EUR",
		"tenant": { "tenant_id": "tenant-abc", "tenant_name": "Padel Club" },
		"teams": [{
			"team_id": "1",
			"players": [
				{ "user_id": "user-123", "name": "Player A" },
				{ "user_id": "user-456", "name": "Player B" }
			]
		}],
		"results": [{
			"name": "Set 1",
			"scores": [
				{ "team_id": "1", "score": 6 }
			]
		}],
		"merchant_access_code": { "code": "12345" }
	}`

	// Create a mock HTTP server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Verify the request path
		assert.Equal(t, "/v1/matches/match-abc", r.URL.Path)
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprintln(w, mockJSONResponse)
	}))
	defer server.Close()

	// Create our APIClient and point it to the mock server
	client := APIClient{
		httpClient: server.Client(),
		apiClient:  client.NewClient(), // Dummy client, not used in this specific test
		BaseURL:    server.URL,
	}

	// Call the method under test
	match, err := client.GetSpecificMatch("match-abc")

	// Assertions
	require.NoError(t, err)
	assert.Equal(t, "match-abc", match.MatchID)
	assert.Equal(t, "user-123", match.OwnerID)
	assert.Equal(t, "Court 1", match.ResourceName)
	assert.Equal(t, "12345", match.AccessCode)
	assert.Equal(t, GameStatusPlayed, match.GameStatus)
	assert.NotEqual(t, 0, match.Start, "Start time should be parsed")
	assert.Len(t, match.Teams, 1)
	assert.Len(t, match.Teams[0].Players, 2)
	assert.Equal(t, "Player A", match.Teams[0].Players[0].Name)
}
