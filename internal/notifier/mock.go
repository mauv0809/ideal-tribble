package notifier

import (
	"sync"

	"github.com/mauv0809/ideal-tribble/internal/club"
	"github.com/mauv0809/ideal-tribble/internal/playtomic"
)

// Mock is a mock implementation of the Notifier interface for testing.
// It is safe for concurrent use.
type Mock struct {
	mu sync.Mutex

	// Call records
	SendBookingNotificationCalls []struct{ Match *playtomic.PadelMatch }
	SendResultNotificationCalls  []struct{ Match *playtomic.PadelMatch }
	SendLeaderboardCalls         [][]club.PlayerStats
	SendLevelLeaderboardCalls    [][]club.PlayerInfo
	SendPlayerStatsCalls         []struct {
		Stats *club.PlayerStats
		Query string
	}
	SendPlayerNotFoundCalls []string

	// Spies for format functions
	FormatLeaderboardResponseFunc      func(stats []club.PlayerStats) (any, error)
	FormatLevelLeaderboardResponseFunc func(players []club.PlayerInfo) (any, error)
	FormatPlayerStatsResponseFunc      func(stats *club.PlayerStats, query string) (any, error)
	FormatPlayerNotFoundResponseFunc   func(query string) (any, error)
	FormatMatchRequestResponseFunc     func(request any) (any, error)

	// Spies for matchmaking functions
	SendMatchAvailabilityRequestFunc func(request any, dryRun bool) (string, string, error)
	SendMatchProposalFunc            func(request any, proposal any, dryRun bool) error
	SendMatchConfirmationFunc        func(request any, dryRun bool) error

	// For direct messages
	SendDirectMessageFunc func(userID string, text string) (string, string, error)

	// Call records for format functions
	LastLeaderboardResponse      any
	LastLevelLeaderboardResponse any
	LastPlayerStatsResponse      any
	LastPlayerNotFoundResponse   any
}

// NewMock creates a new mock instance.
func NewMock() *Mock {
	return &Mock{}
}

// Reset clears all call records.
func (m *Mock) Reset() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.SendBookingNotificationCalls = nil
	m.SendResultNotificationCalls = nil
	m.SendLeaderboardCalls = nil
	m.SendLevelLeaderboardCalls = nil
	m.SendPlayerStatsCalls = nil
	m.SendPlayerNotFoundCalls = nil
	m.LastLeaderboardResponse = nil
	m.LastLevelLeaderboardResponse = nil
	m.LastPlayerStatsResponse = nil
	m.LastPlayerNotFoundResponse = nil
}

func (m *Mock) SendBookingNotification(match *playtomic.PadelMatch, dryRun bool) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.SendBookingNotificationCalls = append(m.SendBookingNotificationCalls, struct{ Match *playtomic.PadelMatch }{match})
	return nil
}

func (m *Mock) SendResultNotification(match *playtomic.PadelMatch, dryRun bool) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.SendResultNotificationCalls = append(m.SendResultNotificationCalls, struct{ Match *playtomic.PadelMatch }{match})
	return nil
}

func (m *Mock) SendLeaderboard(stats []club.PlayerStats, dryRun bool) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.SendLeaderboardCalls = append(m.SendLeaderboardCalls, stats)
	return nil
}

func (m *Mock) SendLevelLeaderboard(players []club.PlayerInfo, dryRun bool) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.SendLevelLeaderboardCalls = append(m.SendLevelLeaderboardCalls, players)
	return nil
}

func (m *Mock) SendPlayerStats(stats *club.PlayerStats, query string, dryRun bool) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.SendPlayerStatsCalls = append(m.SendPlayerStatsCalls, struct {
		Stats *club.PlayerStats
		Query string
	}{stats, query})
	return nil
}

func (m *Mock) SendPlayerNotFound(query string, dryRun bool) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.SendPlayerNotFoundCalls = append(m.SendPlayerNotFoundCalls, query)
	return nil
}

func (m *Mock) FormatLeaderboardResponse(stats []club.PlayerStats) (any, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.FormatLeaderboardResponseFunc != nil {
		resp, err := m.FormatLeaderboardResponseFunc(stats)
		m.LastLeaderboardResponse = resp
		return resp, err
	}
	return "formatted_leaderboard", nil
}

func (m *Mock) FormatLevelLeaderboardResponse(players []club.PlayerInfo) (any, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.FormatLevelLeaderboardResponseFunc != nil {
		resp, err := m.FormatLevelLeaderboardResponseFunc(players)
		m.LastLevelLeaderboardResponse = resp
		return resp, err
	}
	return "formatted_level_leaderboard", nil
}

func (m *Mock) FormatPlayerStatsResponse(stats *club.PlayerStats, query string) (any, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.FormatPlayerStatsResponseFunc != nil {
		resp, err := m.FormatPlayerStatsResponseFunc(stats, query)
		m.LastPlayerStatsResponse = resp
		return resp, err
	}
	return "formatted_player_stats", nil
}

func (m *Mock) FormatPlayerNotFoundResponse(query string) (any, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.FormatPlayerNotFoundResponseFunc != nil {
		resp, err := m.FormatPlayerNotFoundResponseFunc(query)
		m.LastPlayerNotFoundResponse = resp
		return resp, err
	}
	return "formatted_player_not_found", nil
}
func (m *Mock) FormatMatchRequestResponse(request any) (any, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.FormatMatchRequestResponseFunc != nil {
		return m.FormatMatchRequestResponseFunc(request)
	}
	return "formatted_match_request", nil
}
func (m *Mock) SendDirectMessage(userID string, text string) (string, string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.SendDirectMessageFunc != nil {
		return m.SendDirectMessageFunc(userID, text)
	}
	return "", "", nil
}
func (m *Mock) SendMatchAvailabilityRequest(request any, dryRun bool) (string, string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.SendMatchAvailabilityRequestFunc != nil {
		return m.SendMatchAvailabilityRequestFunc(request, dryRun)
	}
	return "", "", nil
}
func (m *Mock) SendMatchConfirmation(request any, dryRun bool) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.SendMatchConfirmationFunc != nil {
		return m.SendMatchConfirmationFunc(request, dryRun)
	}
	return nil
}
func (m *Mock) SendMatchProposal(request any, proposal any, dryRun bool) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.SendMatchProposalFunc != nil {
		return m.SendMatchProposalFunc(request, proposal, dryRun)
	}
	return nil
}
