package handlers

import (
	"net/http"

	"github.com/mauv0809/ideal-tribble/internal/playtomic"
)

// ContextKey is a custom type to avoid key collisions in context.
type ContextKey string

const (
	DryRunKey ContextKey = "dryRun"
)

// isDryRunFromContext is a helper to safely retrieve the dry_run flag from the request context.
func IsDryRunFromContext(r *http.Request) bool {
	dryRun, ok := r.Context().Value(DryRunKey).(bool)
	return ok && dryRun
}

// isClubMatch determines if a match qualifies as a club match by checking against a pre-fetched set of known player IDs.
func isClubMatch(match playtomic.PadelMatch, knownPlayerIDs map[string]struct{}) bool {
	knownPlayers := 0
	totalPlayers := 0
	for _, team := range match.Teams {
		totalPlayers += len(team.Players)
		for _, player := range team.Players {
			if _, ok := knownPlayerIDs[player.UserID]; ok {
				knownPlayers++
			}
		}
	}

	// A doubles match is a club match if it has exactly 4 players and all are known.
	if totalPlayers == 4 {
		return knownPlayers == 4
	}

	// A singles match is a club match if it has exactly 2 players and both are known.
	if totalPlayers == 2 {
		return knownPlayers == 2
	}

	// Any other configuration (e.g., 1v2, 3 players) is not considered a standard club match.
	return false
}
