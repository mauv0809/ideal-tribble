package pairings

// PairingsStore defines the interface for pairing analytics storage and retrieval.
type PairingsStore interface {
	// Pairing management
	AddTrackedPairing(player1ID, player1Name, player2ID, player2Name string) (*TrackedPairing, error)
	RemoveTrackedPairing(pairingID int64) error
	DeactivatePairing(pairingID int64) error
	ActivatePairing(pairingID int64) error
	GetTrackedPairings() ([]TrackedPairing, error)
	GetActivePairings() ([]TrackedPairing, error)
	GetPairingByID(pairingID int64) (*TrackedPairing, error)
	GetPairingByPlayers(player1ID, player2ID string) (*TrackedPairing, error)

	// Match storage
	UpsertPairingMatch(match *PairingMatch) error
	UpsertPairingMatches(matches []*PairingMatch) error

	// Analytics queries
	GetPairingOverallStats(pairingID int64) (*PairingStats, error)
	GetPairingVsOpponentStats(pairingID int64) ([]OpponentStats, error)
	GetPairingVsSpecificOpponent(pairingID int64, opponent1ID, opponent2ID string) (*OpponentStats, error)
	GetPairingTimeStats(pairingID int64) (*TimeStats, error)
	GetPairingVenueStats(pairingID int64) ([]VenueStats, error)
	GetPairingRecentForm(pairingID int64) (*RecentForm, error)
	GetPairingRecentMatches(pairingID int64, limit int) ([]PairingMatch, error)
	GetPairingMatchesPaginated(pairingID int64, limit, offset int) ([]PairingMatch, error)
	GetPairingMatchesFiltered(pairingID int64, filter string, limit, offset int) ([]PairingMatch, error)
	GetPairingMatchCount(pairingID int64) (int, error)
	GetPairingMatchCountFiltered(pairingID int64, filter string) (int, error)
	GetPairingMatchByID(pairingID int64, matchID string) (*PairingMatch, error)
	GetHeadToHead(pairingID int64, opponent1ID, opponent2ID string) ([]PairingMatch, error)

	// Global stats (across all pairings)
	GetTotalMatchCount() (int, error)
	GetRecentMatchCount(days int) (int, error)
	GetRecentMatchesAllPairings(limit int) ([]PairingMatch, error)

	// Individual player analytics
	GetIndividualOpponentStats(pairingID int64) ([]IndividualPlayerStats, error)
	GetIndividualOpponentDetail(pairingID int64, playerID string) (*IndividualPlayerStats, []PlayerPartnerStats, error)
	GetMatchesVsIndividualOpponent(pairingID int64, playerID string) ([]PairingMatch, error)

	// Player lookup
	GetAllKnownPlayers() ([]KnownPlayer, error)
}
