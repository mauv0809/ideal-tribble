package processor

import (
	"sync"
	"testing"

	"github.com/mauv0809/ideal-tribble/internal/club"
	"github.com/mauv0809/ideal-tribble/internal/metrics"
	"github.com/mauv0809/ideal-tribble/internal/playtomic"
	"github.com/mauv0809/ideal-tribble/internal/slack"
	"github.com/stretchr/testify/assert"
)

// mockStore is a mock implementation of the Store interface for testing.
type mockStore struct {
	mu                      sync.Mutex
	matches                 []*playtomic.PadelMatch
	updateStatusCalls       []playtomic.ProcessingStatus
	updatePlayerStatsCalled bool
}

func (m *mockStore) GetMatchesForProcessing() ([]*playtomic.PadelMatch, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.matches, nil
}

func (m *mockStore) UpdateProcessingStatus(matchID string, status playtomic.ProcessingStatus) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, m := range m.matches {
		if m.MatchID == matchID {
			m.ProcessingStatus = status
		}
	}
	m.updateStatusCalls = append(m.updateStatusCalls, status)
	return nil
}

func (m *mockStore) UpdatePlayerStats(match *playtomic.PadelMatch) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.updatePlayerStatsCalled = true
}

func (m *mockStore) AddPlayer(playerID, name string, level float64) {}
func (m *mockStore) IsKnownPlayer(playerID string) bool {
	return true
}
func (m *mockStore) Clear()                                    {}
func (m *mockStore) ClearMatch(matchID string)                 {}
func (m *mockStore) GetAllPlayers() ([]club.PlayerInfo, error) { return nil, nil }
func (m *mockStore) GetAllMatches() ([]*playtomic.PadelMatch, error) {
	return m.matches, nil
}
func (m *mockStore) GetPlayerStats() ([]club.PlayerStats, error) { return nil, nil }

// mockNotifier is a mock implementation of the Notifier interface for testing.
type mockNotifier struct {
	mu                   sync.Mutex
	bookingNotified      bool
	resultNotified       bool
	lastNotificationType slack.NotificationType
}

func (m *mockNotifier) SendNotification(match *playtomic.PadelMatch, notificationType slack.NotificationType, metrics metrics.MetricsStore, dryRun bool) (string, string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if notificationType == slack.BookingNotification {
		m.bookingNotified = true
	}
	if notificationType == slack.ResultNotification {
		m.resultNotified = true
	}
	m.lastNotificationType = notificationType
	return "", "", nil
}

// mockMetrics is a mock implementation of the metrics store.
type mockMetrics struct {
	mu     sync.Mutex
	counts map[string]int
}

func newMockMetrics() *mockMetrics {
	return &mockMetrics{counts: make(map[string]int)}
}

func (m *mockMetrics) Increment(key string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.counts[key]++
}

func (m *mockMetrics) GetAll() (map[string]int, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.counts, nil
}

func TestProcessor_ProcessMatch(t *testing.T) {
	t.Run("new match sends booking notification and stops", func(t *testing.T) {
		store := &mockStore{
			matches: []*playtomic.PadelMatch{
				{MatchID: "m1", ProcessingStatus: playtomic.StatusNew},
			},
		}
		notifier := &mockNotifier{}
		metrics := newMockMetrics()
		p := New(store, notifier, metrics)

		p.ProcessMatches(false)

		assert.True(t, notifier.bookingNotified)
		assert.False(t, notifier.resultNotified)
		assert.Equal(t, playtomic.StatusBookingNotified, store.matches[0].ProcessingStatus)
		assert.Equal(t, 1, metrics.counts["matches_processed"])
		assert.Equal(t, 1, len(store.updateStatusCalls), "Should have only updated status once")
	})

	t.Run("new and played match transitions to completion in one run", func(t *testing.T) {
		store := &mockStore{
			matches: []*playtomic.PadelMatch{
				{
					MatchID:          "m1",
					ProcessingStatus: playtomic.StatusNew,
					GameStatus:       playtomic.GameStatusPlayed,
					ResultsStatus:    playtomic.ResultsStatusConfirmed,
				},
			},
		}
		notifier := &mockNotifier{}
		metrics := newMockMetrics()
		p := New(store, notifier, metrics)

		p.ProcessMatches(false)

		assert.False(t, notifier.bookingNotified, "Booking notification should be skipped")
		assert.True(t, notifier.resultNotified, "Result notification should be sent")
		assert.True(t, store.updatePlayerStatsCalled, "Player stats should be updated")
		assert.Equal(t, playtomic.StatusCompleted, store.matches[0].ProcessingStatus, "Match should be marked as completed")

		expectedStatusUpdates := []playtomic.ProcessingStatus{
			playtomic.StatusResultAvailable,
			playtomic.StatusResultNotified,
			playtomic.StatusStatsUpdated,
			playtomic.StatusCompleted,
		}
		assert.Equal(t, expectedStatusUpdates, store.updateStatusCalls)
	})

	t.Run("new and played match with unconfirmed results sends no notification", func(t *testing.T) {
		store := &mockStore{
			matches: []*playtomic.PadelMatch{
				{
					MatchID:          "m1",
					ProcessingStatus: playtomic.StatusNew,
					GameStatus:       playtomic.GameStatusPlayed,
					ResultsStatus:    playtomic.ResultsStatusValidating, // Results are not confirmed
				},
			},
		}
		notifier := &mockNotifier{}
		metrics := newMockMetrics()
		p := New(store, notifier, metrics)

		p.ProcessMatches(false)

		assert.False(t, notifier.bookingNotified, "Booking notification should not be sent")
		assert.False(t, notifier.resultNotified, "Result notification should not be sent")
		assert.Equal(t, playtomic.StatusBookingNotified, store.matches[0].ProcessingStatus, "State should advance to booking notified")
		assert.Equal(t, 0, metrics.counts["matches_processed"], "No metric should be incremented")
	})
}
