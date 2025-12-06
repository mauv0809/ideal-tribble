package playtomic

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestCalculateTeamSetGameStats(t *testing.T) {
	t.Run("calculates stats for winning team", func(t *testing.T) {
		match := &PadelMatch{
			MatchID: "test-match",
			Teams: []Team{
				{ID: "team1", TeamResult: "WON"},
				{ID: "team2", TeamResult: "LOST"},
			},
			Results: []SetResult{
				{Name: "Set 1", Scores: map[string]int{"team1": 6, "team2": 4}},
				{Name: "Set 2", Scores: map[string]int{"team1": 6, "team2": 3}},
			},
		}

		stats := CalculateTeamSetGameStats(match, "team1")

		assert.Equal(t, 2, stats.SetsWon)
		assert.Equal(t, 0, stats.SetsLost)
		assert.Equal(t, 12, stats.GamesWon)
		assert.Equal(t, 7, stats.GamesLost)
	})

	t.Run("calculates stats for losing team", func(t *testing.T) {
		match := &PadelMatch{
			MatchID: "test-match",
			Teams: []Team{
				{ID: "team1", TeamResult: "WON"},
				{ID: "team2", TeamResult: "LOST"},
			},
			Results: []SetResult{
				{Name: "Set 1", Scores: map[string]int{"team1": 6, "team2": 4}},
				{Name: "Set 2", Scores: map[string]int{"team1": 6, "team2": 3}},
			},
		}

		stats := CalculateTeamSetGameStats(match, "team2")

		assert.Equal(t, 0, stats.SetsWon)
		assert.Equal(t, 2, stats.SetsLost)
		assert.Equal(t, 7, stats.GamesWon)
		assert.Equal(t, 12, stats.GamesLost)
	})

	t.Run("handles 3-set match with split sets", func(t *testing.T) {
		match := &PadelMatch{
			MatchID: "test-match",
			Teams: []Team{
				{ID: "team1", TeamResult: "WON"},
				{ID: "team2", TeamResult: "LOST"},
			},
			Results: []SetResult{
				{Name: "Set 1", Scores: map[string]int{"team1": 6, "team2": 4}},
				{Name: "Set 2", Scores: map[string]int{"team1": 3, "team2": 6}},
				{Name: "Set 3", Scores: map[string]int{"team1": 7, "team2": 5}},
			},
		}

		stats := CalculateTeamSetGameStats(match, "team1")

		assert.Equal(t, 2, stats.SetsWon)
		assert.Equal(t, 1, stats.SetsLost)
		assert.Equal(t, 16, stats.GamesWon)  // 6 + 3 + 7
		assert.Equal(t, 15, stats.GamesLost) // 4 + 6 + 5
	})

	t.Run("skips unplayed sets (0-0)", func(t *testing.T) {
		match := &PadelMatch{
			MatchID: "test-match",
			Teams: []Team{
				{ID: "team1", TeamResult: "WON"},
				{ID: "team2", TeamResult: "LOST"},
			},
			Results: []SetResult{
				{Name: "Set 1", Scores: map[string]int{"team1": 6, "team2": 4}},
				{Name: "Set 2", Scores: map[string]int{"team1": 6, "team2": 3}},
				{Name: "Set 3", Scores: map[string]int{"team1": 0, "team2": 0}}, // Unplayed
			},
		}

		stats := CalculateTeamSetGameStats(match, "team1")

		assert.Equal(t, 2, stats.SetsWon)
		assert.Equal(t, 0, stats.SetsLost)
		assert.Equal(t, 12, stats.GamesWon)
		assert.Equal(t, 7, stats.GamesLost)
	})

	t.Run("handles empty results", func(t *testing.T) {
		match := &PadelMatch{
			MatchID: "test-match",
			Teams: []Team{
				{ID: "team1"},
				{ID: "team2"},
			},
			Results: []SetResult{},
		}

		stats := CalculateTeamSetGameStats(match, "team1")

		assert.Equal(t, 0, stats.SetsWon)
		assert.Equal(t, 0, stats.SetsLost)
		assert.Equal(t, 0, stats.GamesWon)
		assert.Equal(t, 0, stats.GamesLost)
	})
}

func TestCalculateAllTeamsSetGameStats(t *testing.T) {
	t.Run("calculates stats for both teams", func(t *testing.T) {
		match := &PadelMatch{
			MatchID: "test-match",
			Teams: []Team{
				{ID: "team1", TeamResult: "WON"},
				{ID: "team2", TeamResult: "LOST"},
			},
			Results: []SetResult{
				{Name: "Set 1", Scores: map[string]int{"team1": 6, "team2": 4}},
				{Name: "Set 2", Scores: map[string]int{"team1": 3, "team2": 6}},
				{Name: "Set 3", Scores: map[string]int{"team1": 7, "team2": 5}},
			},
		}

		allStats := CalculateAllTeamsSetGameStats(match)

		// Team 1 stats
		assert.Equal(t, 2, allStats["team1"].SetsWon)
		assert.Equal(t, 1, allStats["team1"].SetsLost)
		assert.Equal(t, 16, allStats["team1"].GamesWon)
		assert.Equal(t, 15, allStats["team1"].GamesLost)

		// Team 2 stats (mirror of team 1)
		assert.Equal(t, 1, allStats["team2"].SetsWon)
		assert.Equal(t, 2, allStats["team2"].SetsLost)
		assert.Equal(t, 15, allStats["team2"].GamesWon)
		assert.Equal(t, 16, allStats["team2"].GamesLost)
	})

	t.Run("skips invalid sets", func(t *testing.T) {
		match := &PadelMatch{
			MatchID: "test-match",
			Results: []SetResult{
				{Name: "Set 1", Scores: map[string]int{"team1": 6, "team2": 4}},
				{Name: "Set 2", Scores: map[string]int{"team1": 6}}, // Invalid - only 1 team
			},
		}

		allStats := CalculateAllTeamsSetGameStats(match)

		// Only the valid set should be counted
		assert.Equal(t, 1, allStats["team1"].SetsWon)
		assert.Equal(t, 6, allStats["team1"].GamesWon)
	})
}
