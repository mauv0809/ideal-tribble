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
	UpsertMatchFunc                 func(match *playtomic.PadelMatch) error
	UpsertMatchesFunc               func(matches []*playtomic.PadelMatch) error
	UpdateProcessingStatusFunc      func(matchID string, status playtomic.ProcessingStatus) error
	GetMatchesForProcessingFunc     func() ([]*playtomic.PadelMatch, error)
	GetPlayerStatsFunc              func(matchType playtomic.MatchTypeEnum) ([]PlayerStats, error)
	UpdatePlayerStatsFunc           func(match *playtomic.PadelMatch)
	UpdateWeeklyStatsFunc           func(match *playtomic.PadelMatch)
	AddPlayerFunc                   func(playerID, name string, level float64)
	UpsertPlayersFunc               func(players []PlayerInfo) error
	IsKnownPlayerFunc               func(playerID string) bool
	ClearFunc                       func()
	ClearMatchFunc                  func(matchID string)
	GetAllPlayersFunc               func() ([]PlayerInfo, error)
	GetPlayersSortedByLevelFunc     func() ([]PlayerInfo, error)
	GetAllMatchesFunc               func() ([]*playtomic.PadelMatch, error)
	GetPlayerStatsByNameFunc        func(playerName string, matchType playtomic.MatchTypeEnum) (*PlayerStats, error)
	GetPlayersFunc                  func(playerIDs []string) ([]PlayerInfo, error)
	AssignBallBringerAtomicallyFunc    func(matchID string, playerIDs []string) (string, string, error)
	AssignBookingResponsibleAtomicallyFunc func(playerIDs []string) (string, string, error)
	UpdateNotificationTimestampFunc        func(matchID string, notificationType string) error

	// Call records
	UpsertPlayersCalls          [][]PlayerInfo
	UpsertPlayerCalls           []PlayerInfo
	UpsertMatchCalls            []*playtomic.PadelMatch
	UpsertMatchesCalls          [][]*playtomic.PadelMatch
	UpdateProcessingStatusCalls []struct {
		MatchID string
		Status  playtomic.ProcessingStatus
	}
	GetPlayerStatsByNameCalls []struct {
		PlayerName    string
		MatchTypeEnum playtomic.MatchTypeEnum
	}
	GetPlayersCalls                  [][]string
	AssignBallBringerAtomicallyCalls []struct {
		MatchID   string
		PlayerIDs []string
	}
	AssignBookingResponsibleAtomicallyCalls [][]string
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
	m.AssignBookingResponsibleAtomicallyCalls = nil
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

func (m *MockStore) GetPlayerStats(matchType playtomic.MatchTypeEnum) ([]PlayerStats, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.GetPlayerStatsFunc != nil {
		return m.GetPlayerStatsFunc(matchType)
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
func (m *MockStore) UpdateWeeklyStats(match *playtomic.PadelMatch) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.UpdateWeeklyStatsFunc != nil {
		m.UpdateWeeklyStatsFunc(match)
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

func (m *MockStore) GetPlayerStatsByName(playerName string, matchType playtomic.MatchTypeEnum) (*PlayerStats, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.GetPlayerStatsByNameCalls = append(m.GetPlayerStatsByNameCalls, struct {
		PlayerName    string
		MatchTypeEnum playtomic.MatchTypeEnum
	}{playerName, matchType})
	if m.GetPlayerStatsByNameFunc != nil {
		return m.GetPlayerStatsByNameFunc(playerName, matchType)
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

func (m *MockStore) AssignBallBringerAtomically(matchID string, playerIDs []string) (string, string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.AssignBallBringerAtomicallyCalls = append(m.AssignBallBringerAtomicallyCalls, struct {
		MatchID   string
		PlayerIDs []string
	}{
		MatchID:   matchID,
		PlayerIDs: playerIDs,
	})
	if m.AssignBallBringerAtomicallyFunc != nil {
		return m.AssignBallBringerAtomicallyFunc(matchID, playerIDs)
	}
	return "", "", nil
}

func (m *MockStore) AssignBookingResponsibleAtomically(playerIDs []string) (string, string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.AssignBookingResponsibleAtomicallyCalls = append(m.AssignBookingResponsibleAtomicallyCalls, playerIDs)
	if m.AssignBookingResponsibleAtomicallyFunc != nil {
		return m.AssignBookingResponsibleAtomicallyFunc(playerIDs)
	}
	return "", "", nil
}

func (m *MockStore) UpdateNotificationTimestamp(matchID string, notificationType string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.UpdateNotificationTimestampFunc != nil {
		return m.UpdateNotificationTimestampFunc(matchID, notificationType)
	}
	return nil
}
