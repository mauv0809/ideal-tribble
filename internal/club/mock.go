package club

import (
	"sync"

	"github.com/mauv0809/ideal-tribble/internal/playtomic"
)

// MockStore is a mock implementation of the ClubStore interface for testing.
// It is safe for concurrent use.
type MockStore struct {
	mu sync.Mutex

	// Spies for method calls
	UpsertMatchFunc             func(match *playtomic.PadelMatch) error
	UpsertMatchesFunc           func(matches []*playtomic.PadelMatch) error
	UpdateProcessingStatusFunc  func(matchID string, status playtomic.ProcessingStatus) error
	GetMatchesForProcessingFunc func() ([]*playtomic.PadelMatch, error)
	GetPlayerStatsFunc          func() ([]PlayerStats, error)
	UpdatePlayerStatsFunc       func(match *playtomic.PadelMatch)
	AddPlayerFunc               func(playerID, name string, level float64)
	UpsertPlayersFunc           func(players []PlayerInfo) error
	IsKnownPlayerFunc           func(playerID string) bool
	ClearFunc                   func()
	ClearMatchFunc              func(matchID string)
	GetAllPlayersFunc           func() ([]PlayerInfo, error)
	GetPlayersSortedByLevelFunc func() ([]PlayerInfo, error)
	GetAllMatchesFunc           func() ([]*playtomic.PadelMatch, error)
	GetPlayerStatsByNameFunc    func(playerName string) (*PlayerStats, error)
	GetPlayersFunc              func(playerIDs []string) ([]PlayerInfo, error)
	SetBallBringerFunc          func(matchID, playerID, playerName string) error

	// Call records
	UpsertPlayersCalls          [][]PlayerInfo
	UpsertPlayerCalls           []PlayerInfo
	UpsertMatchCalls            []*playtomic.PadelMatch
	UpsertMatchesCalls          [][]*playtomic.PadelMatch
	UpdateProcessingStatusCalls []struct {
		MatchID string
		Status  playtomic.ProcessingStatus
	}
	GetPlayerStatsByNameCalls []string
	GetPlayersCalls           [][]string
}

// NewMockStore creates a new mock instance.
func NewMock() *MockStore {
	return &MockStore{}
}

// Reset clears all call records.
func (m *MockStore) Reset() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.UpsertMatchCalls = nil
	m.UpsertMatchesCalls = nil
	m.UpdateProcessingStatusCalls = nil
	m.GetPlayerStatsByNameCalls = nil
	m.GetPlayersCalls = nil
}

func (m *MockStore) UpsertMatch(match *playtomic.PadelMatch) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.UpsertMatchCalls = append(m.UpsertMatchCalls, match)
	if m.UpsertMatchFunc != nil {
		return m.UpsertMatchFunc(match)
	}
	return nil
}

func (m *MockStore) UpsertMatches(matches []*playtomic.PadelMatch) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.UpsertMatchesCalls = append(m.UpsertMatchesCalls, matches)
	if m.UpsertMatchesFunc != nil {
		return m.UpsertMatchesFunc(matches)
	}
	return nil
}

func (m *MockStore) UpdateProcessingStatus(matchID string, status playtomic.ProcessingStatus) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.UpdateProcessingStatusCalls = append(m.UpdateProcessingStatusCalls, struct {
		MatchID string
		Status  playtomic.ProcessingStatus
	}{matchID, status})
	if m.UpdateProcessingStatusFunc != nil {
		return m.UpdateProcessingStatusFunc(matchID, status)
	}
	return nil
}

func (m *MockStore) GetMatchesForProcessing() ([]*playtomic.PadelMatch, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.GetMatchesForProcessingFunc != nil {
		return m.GetMatchesForProcessingFunc()
	}
	return nil, nil
}

func (m *MockStore) GetPlayerStats() ([]PlayerStats, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.GetPlayerStatsFunc != nil {
		return m.GetPlayerStatsFunc()
	}
	return nil, nil
}

func (m *MockStore) UpdatePlayerStats(match *playtomic.PadelMatch) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.UpdatePlayerStatsFunc != nil {
		m.UpdatePlayerStatsFunc(match)
	}
}

func (m *MockStore) AddPlayer(playerID, name string, level float64) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.AddPlayerFunc != nil {
		m.AddPlayerFunc(playerID, name, level)
	}
}

func (m *MockStore) UpsertPlayers(players []PlayerInfo) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.UpsertPlayersFunc != nil {
		return m.UpsertPlayersFunc(players)
	}
	return nil
}

func (m *MockStore) IsKnownPlayer(playerID string) bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.IsKnownPlayerFunc != nil {
		return m.IsKnownPlayerFunc(playerID)
	}
	return false
}

func (m *MockStore) Clear() {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.ClearFunc != nil {
		m.ClearFunc()
	}
}

func (m *MockStore) ClearMatch(matchID string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.ClearMatchFunc != nil {
		m.ClearMatchFunc(matchID)
	}
}

func (m *MockStore) GetAllPlayers() ([]PlayerInfo, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.GetAllPlayersFunc != nil {
		return m.GetAllPlayersFunc()
	}
	return nil, nil
}

func (m *MockStore) GetPlayersSortedByLevel() ([]PlayerInfo, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.GetPlayersSortedByLevelFunc != nil {
		return m.GetPlayersSortedByLevelFunc()
	}
	return nil, nil
}

func (m *MockStore) GetAllMatches() ([]*playtomic.PadelMatch, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.GetAllMatchesFunc != nil {
		return m.GetAllMatchesFunc()
	}
	return nil, nil
}

func (m *MockStore) GetPlayerStatsByName(playerName string) (*PlayerStats, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.GetPlayerStatsByNameCalls = append(m.GetPlayerStatsByNameCalls, playerName)
	if m.GetPlayerStatsByNameFunc != nil {
		return m.GetPlayerStatsByNameFunc(playerName)
	}
	return nil, nil
}

func (m *MockStore) GetPlayers(playerIDs []string) ([]PlayerInfo, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.GetPlayersCalls = append(m.GetPlayersCalls, playerIDs)
	if m.GetPlayersFunc != nil {
		return m.GetPlayersFunc(playerIDs)
	}
	return nil, nil
}

func (m *MockStore) SetBallBringer(matchID, playerID, playerName string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.SetBallBringerFunc != nil {
		return m.SetBallBringerFunc(matchID, playerID, playerName)
	}
	return nil
}
