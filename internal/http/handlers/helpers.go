package handlers

import (
	"net/http"

	"github.com/mauv0809/ideal-tribble/internal/club"
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

// isClubMatch determines if a match qualifies as a club match
func isClubMatch(match playtomic.PadelMatch, store club.ClubStore) bool {
	knownPlayers := 0
	totalPlayers := 0
	for _, team := range match.Teams {
		totalPlayers += len(team.Players)
		for _, player := range team.Players {
			if store.IsKnownPlayer(player.UserID) {
				knownPlayers++
			}
		}
	}

	if totalPlayers >= 4 && knownPlayers >= 4 {
		return true
	}
	if totalPlayers > 0 && totalPlayers < 4 && knownPlayers == totalPlayers {
		return true
	}
	return false
}
