package pairings

import (
	"testing"
	"time"

	"github.com/mauv0809/ideal-tribble/internal/playtomic"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDetectPairingMatches(t *testing.T) {
	t.Run("returns nil for empty inputs", func(t *testing.T) {
		result := DetectPairingMatches(nil, nil)
		assert.Nil(t, result)

		result = DetectPairingMatches([]*playtomic.PadelMatch{}, []TrackedPairing{})
		assert.Nil(t, result)
	})

	t.Run("detects pairing match when team plays together", func(t *testing.T) {
		now := time.Now().Unix()

		pairings := []TrackedPairing{
			{
				ID:          1,
				Player1ID:   "player1",
				Player1Name: "Alice",
				Player2ID:   "player2",
				Player2Name: "Bob",
				Active:      true,
			},
		}

		matches := []*playtomic.PadelMatch{
			{
				MatchID:       "match1",
				MatchTypeEnum: playtomic.MatchTypeEnumDoubles,
				GameStatus:    playtomic.GameStatusPlayed,
				ResultsStatus: playtomic.ResultsStatusConfirmed,
				Start:         now,
				Tenant:        playtomic.Tenant{ID: "t1", Name: "Test Court"},
				Teams: []playtomic.Team{
					{
						ID:         "team1",
						TeamResult: "WON",
						Players: []playtomic.Player{
							{UserID: "player1", Name: "Alice"},
							{UserID: "player2", Name: "Bob"},
						},
					},
					{
						ID:         "team2",
						TeamResult: "LOST",
						Players: []playtomic.Player{
							{UserID: "player3", Name: "Charlie"},
							{UserID: "player4", Name: "Diana"},
						},
					},
				},
				Results: []playtomic.SetResult{
					{Name: "Set 1", Scores: map[string]int{"team1": 6, "team2": 4}},
					{Name: "Set 2", Scores: map[string]int{"team1": 6, "team2": 3}},
				},
			},
		}

		result := DetectPairingMatches(matches, pairings)
		require.Len(t, result, 1)

		assert.Equal(t, int64(1), result[0].PairingID)
		assert.Equal(t, "match1", result[0].MatchID)
		assert.True(t, result[0].PairingWon)
		assert.Equal(t, "Charlie", result[0].Opponent1Name)
		assert.Equal(t, "Diana", result[0].Opponent2Name)
	})

	t.Run("skips singles matches", func(t *testing.T) {
		pairings := []TrackedPairing{
			{ID: 1, Player1ID: "p1", Player1Name: "A", Player2ID: "p2", Player2Name: "B"},
		}

		matches := []*playtomic.PadelMatch{
			{
				MatchID:       "singles_match",
				MatchTypeEnum: playtomic.MatchTypeEnumSingles, // Singles
				GameStatus:    playtomic.GameStatusPlayed,
				ResultsStatus: playtomic.ResultsStatusConfirmed,
				Teams: []playtomic.Team{
					{ID: "t1", Players: []playtomic.Player{{UserID: "p1", Name: "A"}}},
					{ID: "t2", Players: []playtomic.Player{{UserID: "p3", Name: "C"}}},
				},
			},
		}

		result := DetectPairingMatches(matches, pairings)
		assert.Nil(t, result)
	})

	t.Run("skips unplayed matches", func(t *testing.T) {
		pairings := []TrackedPairing{
			{ID: 1, Player1ID: "p1", Player1Name: "A", Player2ID: "p2", Player2Name: "B"},
		}

		matches := []*playtomic.PadelMatch{
			{
				MatchID:       "unplayed_match",
				MatchTypeEnum: playtomic.MatchTypeEnumDoubles,
				GameStatus:    "SCHEDULED", // Not played yet
				ResultsStatus: playtomic.ResultsStatusWaitingFor,
				Teams: []playtomic.Team{
					{ID: "t1", Players: []playtomic.Player{{UserID: "p1"}, {UserID: "p2"}}},
					{ID: "t2", Players: []playtomic.Player{{UserID: "p3"}, {UserID: "p4"}}},
				},
			},
		}

		result := DetectPairingMatches(matches, pairings)
		assert.Nil(t, result)
	})

	t.Run("skips matches with unconfirmed results", func(t *testing.T) {
		pairings := []TrackedPairing{
			{ID: 1, Player1ID: "p1", Player1Name: "A", Player2ID: "p2", Player2Name: "B"},
		}

		matches := []*playtomic.PadelMatch{
			{
				MatchID:       "pending_match",
				MatchTypeEnum: playtomic.MatchTypeEnumDoubles,
				GameStatus:    playtomic.GameStatusPlayed,
				ResultsStatus: "PENDING", // Not confirmed
				Teams: []playtomic.Team{
					{ID: "t1", Players: []playtomic.Player{{UserID: "p1"}, {UserID: "p2"}}},
					{ID: "t2", Players: []playtomic.Player{{UserID: "p3"}, {UserID: "p4"}}},
				},
			},
		}

		result := DetectPairingMatches(matches, pairings)
		assert.Nil(t, result)
	})

	t.Run("detects loss correctly", func(t *testing.T) {
		now := time.Now().Unix()

		pairings := []TrackedPairing{
			{ID: 1, Player1ID: "p1", Player1Name: "A", Player2ID: "p2", Player2Name: "B"},
		}

		matches := []*playtomic.PadelMatch{
			{
				MatchID:       "loss_match",
				MatchTypeEnum: playtomic.MatchTypeEnumDoubles,
				GameStatus:    playtomic.GameStatusPlayed,
				ResultsStatus: playtomic.ResultsStatusConfirmed,
				Start:         now,
				Tenant:        playtomic.Tenant{ID: "t1", Name: "Court"},
				Teams: []playtomic.Team{
					{
						ID:         "t1",
						TeamResult: "LOST", // The pairing lost
						Players:    []playtomic.Player{{UserID: "p1", Name: "A"}, {UserID: "p2", Name: "B"}},
					},
					{
						ID:         "t2",
						TeamResult: "WON",
						Players:    []playtomic.Player{{UserID: "p3", Name: "C"}, {UserID: "p4", Name: "D"}},
					},
				},
				Results: []playtomic.SetResult{
					{Name: "Set 1", Scores: map[string]int{"t1": 3, "t2": 6}},
					{Name: "Set 2", Scores: map[string]int{"t1": 4, "t2": 6}},
				},
			},
		}

		result := DetectPairingMatches(matches, pairings)
		require.Len(t, result, 1)
		assert.False(t, result[0].PairingWon)
	})
}

func TestDetectPairingMatchesWithResolver(t *testing.T) {
	t.Run("uses resolver to map manual IDs to playtomic IDs", func(t *testing.T) {
		now := time.Now().Unix()

		// Pairing is tracked with Playtomic IDs
		pairings := []TrackedPairing{
			{
				ID:          1,
				Player1ID:   "playtomic_alice",
				Player1Name: "Alice",
				Player2ID:   "playtomic_bob",
				Player2Name: "Bob",
				Active:      true,
			},
		}

		// Match has manual player IDs
		matches := []*playtomic.PadelMatch{
			{
				MatchID:       "manual_match",
				MatchTypeEnum: playtomic.MatchTypeEnumDoubles,
				GameStatus:    playtomic.GameStatusPlayed,
				ResultsStatus: playtomic.ResultsStatusConfirmed,
				Start:         now,
				Tenant:        playtomic.Tenant{ID: "t1", Name: "Manual Court"},
				Teams: []playtomic.Team{
					{
						ID:         "team1",
						TeamResult: "WON",
						Players: []playtomic.Player{
							{UserID: "manual_123", Name: "Alice"},  // Manual ID for Alice
							{UserID: "manual_456", Name: "Bob"},   // Manual ID for Bob
						},
					},
					{
						ID:         "team2",
						TeamResult: "LOST",
						Players: []playtomic.Player{
							{UserID: "player3", Name: "Charlie"},
							{UserID: "player4", Name: "Diana"},
						},
					},
				},
				Results: []playtomic.SetResult{
					{Name: "Set 1", Scores: map[string]int{"team1": 6, "team2": 4}},
				},
			},
		}

		// Resolver that maps manual IDs to playtomic IDs
		resolver := func(playerID string) string {
			switch playerID {
			case "manual_123":
				return "playtomic_alice"
			case "manual_456":
				return "playtomic_bob"
			default:
				return playerID
			}
		}

		// Without resolver - should NOT detect (IDs don't match)
		resultWithoutResolver := DetectPairingMatchesWithResolver(matches, pairings, DefaultResolver)
		assert.Nil(t, resultWithoutResolver, "Without resolver, manual IDs should not match pairing")

		// With resolver - should detect (IDs are resolved)
		resultWithResolver := DetectPairingMatchesWithResolver(matches, pairings, resolver)
		require.Len(t, resultWithResolver, 1, "With resolver, manual IDs should be resolved to match pairing")

		assert.Equal(t, int64(1), resultWithResolver[0].PairingID)
		assert.Equal(t, "manual_match", resultWithResolver[0].MatchID)
		assert.True(t, resultWithResolver[0].PairingWon)
	})

	t.Run("handles mixed manual and playtomic players", func(t *testing.T) {
		now := time.Now().Unix()

		pairings := []TrackedPairing{
			{
				ID:          1,
				Player1ID:   "playtomic_alice",
				Player1Name: "Alice",
				Player2ID:   "playtomic_bob",
				Player2Name: "Bob",
			},
		}

		// One player is playtomic, one is manual
		matches := []*playtomic.PadelMatch{
			{
				MatchID:       "mixed_match",
				MatchTypeEnum: playtomic.MatchTypeEnumDoubles,
				GameStatus:    playtomic.GameStatusPlayed,
				ResultsStatus: playtomic.ResultsStatusConfirmed,
				Start:         now,
				Tenant:        playtomic.Tenant{ID: "t1", Name: "Mixed Court"},
				Teams: []playtomic.Team{
					{
						ID:         "team1",
						TeamResult: "WON",
						Players: []playtomic.Player{
							{UserID: "playtomic_alice", Name: "Alice"}, // Playtomic ID
							{UserID: "manual_bob", Name: "Bob"},       // Manual ID
						},
					},
					{
						ID:         "team2",
						TeamResult: "LOST",
						Players: []playtomic.Player{
							{UserID: "player3", Name: "Charlie"},
							{UserID: "player4", Name: "Diana"},
						},
					},
				},
				Results: []playtomic.SetResult{
					{Name: "Set 1", Scores: map[string]int{"team1": 6, "team2": 4}},
				},
			},
		}

		resolver := func(playerID string) string {
			if playerID == "manual_bob" {
				return "playtomic_bob"
			}
			return playerID
		}

		result := DetectPairingMatchesWithResolver(matches, pairings, resolver)
		require.Len(t, result, 1)
		assert.Equal(t, "mixed_match", result[0].MatchID)
	})

	t.Run("resolver returns original ID when no mapping exists", func(t *testing.T) {
		now := time.Now().Unix()

		pairings := []TrackedPairing{
			{ID: 1, Player1ID: "p1", Player1Name: "A", Player2ID: "p2", Player2Name: "B"},
		}

		matches := []*playtomic.PadelMatch{
			{
				MatchID:       "no_alias_match",
				MatchTypeEnum: playtomic.MatchTypeEnumDoubles,
				GameStatus:    playtomic.GameStatusPlayed,
				ResultsStatus: playtomic.ResultsStatusConfirmed,
				Start:         now,
				Tenant:        playtomic.Tenant{ID: "t1", Name: "Court"},
				Teams: []playtomic.Team{
					{
						ID:         "team1",
						TeamResult: "WON",
						Players:    []playtomic.Player{{UserID: "p1", Name: "A"}, {UserID: "p2", Name: "B"}},
					},
					{
						ID:         "team2",
						TeamResult: "LOST",
						Players:    []playtomic.Player{{UserID: "p3", Name: "C"}, {UserID: "p4", Name: "D"}},
					},
				},
				Results: []playtomic.SetResult{
					{Name: "Set 1", Scores: map[string]int{"team1": 6, "team2": 4}},
				},
			},
		}

		// Resolver that doesn't change anything (like DefaultResolver)
		noopResolver := func(id string) string { return id }

		result := DetectPairingMatchesWithResolver(matches, pairings, noopResolver)
		require.Len(t, result, 1)
		assert.Equal(t, "no_alias_match", result[0].MatchID)
	})
}

func TestNormalizePairingKey(t *testing.T) {
	// Test that order doesn't matter
	key1 := normalizePairingKey("alice", "bob")
	key2 := normalizePairingKey("bob", "alice")
	assert.Equal(t, key1, key2, "Key should be the same regardless of order")

	// Test the format
	assert.Equal(t, "alice:bob", key1)
}
