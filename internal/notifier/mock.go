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
