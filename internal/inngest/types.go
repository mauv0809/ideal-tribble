package inngest

import (
	"github.com/inngest/inngestgo"
)

type client struct {
	inngestClient inngestgo.Client
}

// MatchData is the payload for our events.
// It should contain all necessary information to process the match.
type MatchData struct {
	MatchID string `json:"matchId"`
	// Add other relevant fields from your playtomic.Booking struct
}
