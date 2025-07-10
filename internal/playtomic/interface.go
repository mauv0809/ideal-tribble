package playtomic

// PlaytomicClient defines the interface for interacting with the Playtomic API.
// This allows for mock implementations to be used in tests.
type PlaytomicClient interface {
	GetMatches(params *SearchMatchesParams) ([]MatchSummary, error)
	GetSpecificMatch(matchID string) (PadelMatch, error)
}
