package pairings

import (
	"time"

	"github.com/charmbracelet/log"
	"github.com/mauv0809/ideal-tribble/internal/playtomic"
)

// DetectPairingMatches checks matches against tracked pairings and returns
// PairingMatch records for any matches where a tracked pair plays together on the same team.
func DetectPairingMatches(matches []*playtomic.PadelMatch, trackedPairings []TrackedPairing) []*PairingMatch {
	log.Info("Starting pairing match detection",
		"totalMatches", len(matches),
		"trackedPairings", len(trackedPairings))

	if len(trackedPairings) == 0 || len(matches) == 0 {
		log.Info("No matches or pairings to process")
		return nil
	}

	// Build a map for quick lookup of pairings by player ID pairs
	pairingMap := make(map[string]*TrackedPairing)
	for i := range trackedPairings {
		p := &trackedPairings[i]
		// Key is normalized: smaller ID first
		key := normalizePairingKey(p.Player1ID, p.Player2ID)
		pairingMap[key] = p
		log.Debug("Tracking pairing", "key", key, "name", p.Player1Name+" & "+p.Player2Name)
	}

	var result []*PairingMatch

	// Counters for debugging
	var skippedNotDoubles, skippedNotPlayed, skippedNotConfirmed, skippedNoPairing int

	for _, match := range matches {
		// Skip matches that aren't doubles (we need 2v2 for pairings)
		if match.MatchTypeEnum != playtomic.MatchTypeEnumDoubles {
			skippedNotDoubles++
			continue
		}

		// Skip matches without results (we need winner/loser info)
		if match.GameStatus != playtomic.GameStatusPlayed {
			skippedNotPlayed++
			log.Debug("Skipping match - not played yet",
				"matchID", match.MatchID,
				"gameStatus", match.GameStatus,
				"matchDate", time.Unix(match.Start, 0).Format("2006-01-02 15:04"))
			continue
		}

		// Skip matches where results weren't confirmed (expired, pending, etc.)
		if match.ResultsStatus != playtomic.ResultsStatusConfirmed {
			skippedNotConfirmed++
			log.Info("Skipping match - results not confirmed",
				"matchID", match.MatchID,
				"resultsStatus", match.ResultsStatus,
				"matchDate", time.Unix(match.Start, 0).Format("2006-01-02 15:04"))
			continue
		}

		// Check each team for tracked pairings
		foundPairing := false
		for teamIdx, team := range match.Teams {
			if len(team.Players) != 2 {
				continue
			}

			// Check if this team is a tracked pairing
			key := normalizePairingKey(team.Players[0].UserID, team.Players[1].UserID)
			pairing, found := pairingMap[key]
			if !found {
				continue
			}

			foundPairing = true
			// Found a match with a tracked pairing!
			pairingMatch := buildPairingMatch(match, pairing, teamIdx)
			if pairingMatch != nil {
				result = append(result, pairingMatch)
				log.Info("Detected pairing match",
					"matchID", match.MatchID,
					"pairing", pairing.Player1Name+" & "+pairing.Player2Name,
					"won", pairingMatch.PairingWon,
					"matchDate", time.Unix(match.Start, 0).Format("2006-01-02 15:04"))
			}
		}
		if !foundPairing {
			skippedNoPairing++
		}
	}

	log.Info("Pairing match detection complete",
		"detected", len(result),
		"skippedNotDoubles", skippedNotDoubles,
		"skippedNotPlayed", skippedNotPlayed,
		"skippedNotConfirmed", skippedNotConfirmed,
		"skippedNoPairing", skippedNoPairing)

	return result
}

func normalizePairingKey(id1, id2 string) string {
	if id1 > id2 {
		id1, id2 = id2, id1
	}
	return id1 + ":" + id2
}

func buildPairingMatch(match *playtomic.PadelMatch, pairing *TrackedPairing, pairingTeamIdx int) *PairingMatch {
	if len(match.Teams) != 2 {
		return nil
	}

	// Determine opponent team
	opponentTeamIdx := 1 - pairingTeamIdx
	opponentTeam := match.Teams[opponentTeamIdx]
	pairingTeam := match.Teams[pairingTeamIdx]

	// Extract opponent info
	var opp1ID, opp1Name, opp2ID, opp2Name string
	if len(opponentTeam.Players) >= 1 {
		opp1ID = opponentTeam.Players[0].UserID
		opp1Name = opponentTeam.Players[0].Name
	}
	if len(opponentTeam.Players) >= 2 {
		opp2ID = opponentTeam.Players[1].UserID
		opp2Name = opponentTeam.Players[1].Name
	}

	// Normalize opponent order (smaller ID first)
	if opp1ID > opp2ID {
		opp1ID, opp2ID = opp2ID, opp1ID
		opp1Name, opp2Name = opp2Name, opp1Name
	}

	// Determine if pairing won
	pairingWon := pairingTeam.TeamResult == "WON"

	// Calculate sets and games for the pairing using shared utility
	stats := playtomic.CalculateTeamSetGameStats(match, pairingTeam.ID)

	// Extract time info
	matchTime := time.Unix(match.Start, 0)
	dayOfWeek := int(matchTime.Weekday())
	hourOfDay := matchTime.Hour()

	return &PairingMatch{
		PairingID:     pairing.ID,
		MatchID:       match.MatchID,
		MatchDate:     match.Start,
		DayOfWeek:     dayOfWeek,
		HourOfDay:     hourOfDay,
		Opponent1ID:   opp1ID,
		Opponent1Name: opp1Name,
		Opponent2ID:   opp2ID,
		Opponent2Name: opp2Name,
		PairingWon:    pairingWon,
		SetsWon:       stats.SetsWon,
		SetsLost:      stats.SetsLost,
		GamesWon:      stats.GamesWon,
		GamesLost:     stats.GamesLost,
		TenantID:      match.Tenant.ID,
		TenantName:    match.Tenant.Name,
	}
}
