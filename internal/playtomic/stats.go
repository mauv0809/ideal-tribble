package playtomic

import "github.com/charmbracelet/log"

// TeamSetGameStats holds sets and games won/lost for a team in a match.
type TeamSetGameStats struct {
	SetsWon   int
	SetsLost  int
	GamesWon  int
	GamesLost int
}

// CalculateTeamSetGameStats calculates sets and games won/lost for a specific team in a match.
// Returns the stats for the specified teamID.
func CalculateTeamSetGameStats(match *PadelMatch, teamID string) TeamSetGameStats {
	var stats TeamSetGameStats

	for _, set := range match.Results {
		// Get all team IDs from scores
		var teamIDs []string
		for tid := range set.Scores {
			teamIDs = append(teamIDs, tid)
		}

		// Validate we have exactly 2 teams
		if len(teamIDs) != 2 {
			log.Warn("Cannot determine set winner/loser for a set without exactly 2 teams",
				"matchID", match.MatchID, "scores", set.Scores)
			continue
		}

		// Skip unplayed sets (both scores are 0)
		if set.Scores[teamIDs[0]] == 0 && set.Scores[teamIDs[1]] == 0 {
			continue
		}

		// Find this team's score and opponent's score
		teamScore := set.Scores[teamID]
		var opponentScore int
		for tid, score := range set.Scores {
			if tid != teamID {
				opponentScore = score
				break
			}
		}

		// Accumulate games
		stats.GamesWon += teamScore
		stats.GamesLost += opponentScore

		// Determine set winner
		if teamScore > opponentScore {
			stats.SetsWon++
		} else {
			stats.SetsLost++
		}
	}

	return stats
}

// CalculateAllTeamsSetGameStats calculates sets and games for all teams in a match.
// Returns a map of teamID -> stats.
func CalculateAllTeamsSetGameStats(match *PadelMatch) map[string]TeamSetGameStats {
	result := make(map[string]TeamSetGameStats)

	for _, set := range match.Results {
		// Get all team IDs from scores
		var teamIDs []string
		for tid := range set.Scores {
			teamIDs = append(teamIDs, tid)
		}

		// Validate we have exactly 2 teams
		if len(teamIDs) != 2 {
			log.Warn("Cannot determine set winner/loser for a set without exactly 2 teams",
				"matchID", match.MatchID, "scores", set.Scores)
			continue
		}

		// Skip unplayed sets (both scores are 0)
		if set.Scores[teamIDs[0]] == 0 && set.Scores[teamIDs[1]] == 0 {
			continue
		}

		// Determine winner and loser
		var winnerID, loserID string
		var winnerScore, loserScore int

		if set.Scores[teamIDs[0]] > set.Scores[teamIDs[1]] {
			winnerID, loserID = teamIDs[0], teamIDs[1]
			winnerScore, loserScore = set.Scores[teamIDs[0]], set.Scores[teamIDs[1]]
		} else {
			winnerID, loserID = teamIDs[1], teamIDs[0]
			winnerScore, loserScore = set.Scores[teamIDs[1]], set.Scores[teamIDs[0]]
		}

		// Update winner stats
		winnerStats := result[winnerID]
		winnerStats.SetsWon++
		winnerStats.GamesWon += winnerScore
		winnerStats.GamesLost += loserScore
		result[winnerID] = winnerStats

		// Update loser stats
		loserStats := result[loserID]
		loserStats.SetsLost++
		loserStats.GamesWon += loserScore
		loserStats.GamesLost += winnerScore
		result[loserID] = loserStats
	}

	return result
}
