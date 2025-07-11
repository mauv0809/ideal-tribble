package playtomic

import "sync"

// MockClient is a mock implementation of the PlaytomicClient interface for testing.
// It is safe for concurrent use.
type MockClient struct {
	mu sync.Mutex

	// Spies for method calls
	GetMatchesFunc       func(params *SearchMatchesParams) ([]MatchSummary, error)
	GetSpecificMatchFunc func(matchID string) (PadelMatch, error)

	// Call records
	GetMatchesCalls       []*SearchMatchesParams
	GetSpecificMatchCalls []string
}

// NewMockClient creates a new mock instance.
func NewMockClient() *MockClient {
	return &MockClient{}
}

// Reset clears all call records.
func (m *MockClient) Reset() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.GetMatchesCalls = nil
	m.GetSpecificMatchCalls = nil
}

func (m *MockClient) GetMatches(params *SearchMatchesParams) ([]MatchSummary, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.GetMatchesCalls = append(m.GetMatchesCalls, params)
	if m.GetMatchesFunc != nil {
		return m.GetMatchesFunc(params)
	}
	return []MatchSummary{}, nil
}

func (m *MockClient) GetSpecificMatch(matchID string) (PadelMatch, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.GetSpecificMatchCalls = append(m.GetSpecificMatchCalls, matchID)
	if m.GetSpecificMatchFunc != nil {
		return m.GetSpecificMatchFunc(matchID)
	}
	return PadelMatch{}, nil
}
