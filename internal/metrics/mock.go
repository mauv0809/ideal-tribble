package metrics

import "sync"

// Mock is a mock implementation of the Metrics interface for testing.
// It is safe for concurrent use.
type Mock struct {
	mu                  sync.Mutex
	fetcherRuns         int
	matchesProcessed    int
	processingDurations []float64
	slackNotifSent      int
	slackNotifFailed    int
	startupTime         float64
}

// NewMock creates a new mock instance.
func NewMock() *Mock {
	return &Mock{
		processingDurations: make([]float64, 0),
	}
}

func (m *Mock) IncFetcherRuns() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.fetcherRuns++
}

func (m *Mock) IncMatchesProcessed() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.matchesProcessed++
}

func (m *Mock) ObserveProcessingDuration(duration float64) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.processingDurations = append(m.processingDurations, duration)
}

func (m *Mock) IncSlackNotifSent() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.slackNotifSent++
}

func (m *Mock) IncSlackNotifFailed() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.slackNotifFailed++
}

func (m *Mock) SetStartupTime(duration float64) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.startupTime = duration
}

// FetcherRuns returns the number of times IncFetcherRuns was called.
func (m *Mock) FetcherRuns() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.fetcherRuns
}

// MatchesProcessed returns the number of times IncMatchesProcessed was called.
func (m *Mock) MatchesProcessed() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.matchesProcessed
}

// SlackNotifSent returns the number of times IncSlackNotifSent was called.
func (m *Mock) SlackNotifSent() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.slackNotifSent
}

// SlackNotifFailed returns the number of times IncSlackNotifFailed was called.
func (m *Mock) SlackNotifFailed() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.slackNotifFailed
}
