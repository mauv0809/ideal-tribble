package processor

import (
	"testing"

	"github.com/mauv0809/ideal-tribble/internal/club"
	"github.com/mauv0809/ideal-tribble/internal/metrics"
	"github.com/mauv0809/ideal-tribble/internal/notifier"
	"github.com/mauv0809/ideal-tribble/internal/playtomic"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestProcessor_ProcessMatches(t *testing.T) {
	t.Run("new upcoming match sends booking notification and assigns ball bringer", func(t *testing.T) {
		// Setup
		store := club.NewMock()
		notif := notifier.NewMock()
		metr := metrics.NewMock()
		p := New(store, notif, metr)

		match := &playtomic.PadelMatch{
			MatchID:          "m1",
			ProcessingStatus: playtomic.StatusNew,
			Teams: []playtomic.Team{
				{Players: []playtomic.Player{{UserID: "p1", Name: "Player 1"}, {UserID: "p2", Name: "Player 2"}}},
			},
		}
		store.GetMatchesForProcessingFunc = func() ([]*playtomic.PadelMatch, error) {
			return []*playtomic.PadelMatch{match}, nil
		}
		store.GetPlayersFunc = func(ids []string) ([]club.PlayerInfo, error) {
			return []club.PlayerInfo{
				{ID: "p1", Name: "Player 1", BallBringerCount: 1},
				{ID: "p2", Name: "Player 2", BallBringerCount: 2},
			}, nil
		}
		store.SetBallBringerFunc = func(matchID, playerID, playerName string) error {
			match.BallBringerID = playerID
			match.BallBringerName = playerName
			return nil
		}

		// Execute
		p.ProcessMatches(false)

		// Assert
		assert.Equal(t, "p1", match.BallBringerID, "Ball bringer should be assigned to player with lowest count")
		require.Len(t, notif.SendBookingNotificationCalls, 1, "A booking notification should be sent")
		assert.Equal(t, "m1", notif.SendBookingNotificationCalls[0].Match.MatchID)
		require.Len(t, notif.SendResultNotificationCalls, 0, "No result notification should be sent")
		require.Len(t, store.UpdateProcessingStatusCalls, 1, "Status should be updated once")
		assert.Equal(t, playtomic.StatusBookingNotified, store.UpdateProcessingStatusCalls[0].Status)
	})

	t.Run("new and played match with confirmed results transitions to completion", func(t *testing.T) {
		// Setup
		store := club.NewMock()
		notif := notifier.NewMock()
		metr := metrics.NewMock()
		p := New(store, notif, metr)

		match := &playtomic.PadelMatch{
			MatchID:          "m1",
			ProcessingStatus: playtomic.StatusNew,
			GameStatus:       playtomic.GameStatusPlayed,
			ResultsStatus:    playtomic.ResultsStatusConfirmed,
		}
		store.GetMatchesForProcessingFunc = func() ([]*playtomic.PadelMatch, error) {
			return []*playtomic.PadelMatch{match}, nil
		}
		var statsUpdated bool
		store.UpdatePlayerStatsFunc = func(m *playtomic.PadelMatch) {
			statsUpdated = true
		}

		// Execute
		p.ProcessMatches(false)

		// Assert
		require.Len(t, notif.SendBookingNotificationCalls, 0, "Booking notification should be skipped")
		assert.True(t, statsUpdated, "Player stats should be updated")
		require.Len(t, store.UpdateProcessingStatusCalls, 4, "Status should be updated four times")
		assert.Equal(t, playtomic.StatusResultAvailable, store.UpdateProcessingStatusCalls[0].Status)
		assert.Equal(t, playtomic.StatusResultNotified, store.UpdateProcessingStatusCalls[1].Status)
		assert.Equal(t, playtomic.StatusStatsUpdated, store.UpdateProcessingStatusCalls[2].Status)
		assert.Equal(t, playtomic.StatusCompleted, store.UpdateProcessingStatusCalls[3].Status)
	})

	t.Run("match with booking notified transitions to completion after being played", func(t *testing.T) {
		// Setup
		store := club.NewMock()
		notif := notifier.NewMock()
		metr := metrics.NewMock()
		p := New(store, notif, metr)

		match := &playtomic.PadelMatch{
			MatchID:          "m1",
			ProcessingStatus: playtomic.StatusBookingNotified,
			GameStatus:       playtomic.GameStatusPlayed,
			ResultsStatus:    playtomic.ResultsStatusConfirmed,
		}
		store.GetMatchesForProcessingFunc = func() ([]*playtomic.PadelMatch, error) {
			return []*playtomic.PadelMatch{match}, nil
		}
		var statsUpdated bool
		store.UpdatePlayerStatsFunc = func(m *playtomic.PadelMatch) {
			statsUpdated = true
		}

		// Execute
		p.ProcessMatches(false)

		// Assert
		require.Len(t, notif.SendBookingNotificationCalls, 0, "Booking notification should not be sent again")
		assert.True(t, statsUpdated, "Player stats should be updated")
		require.Len(t, store.UpdateProcessingStatusCalls, 4, "Status should be updated four times")
		assert.Equal(t, playtomic.StatusResultAvailable, store.UpdateProcessingStatusCalls[0].Status)
		assert.Equal(t, playtomic.StatusResultNotified, store.UpdateProcessingStatusCalls[1].Status)
		assert.Equal(t, playtomic.StatusStatsUpdated, store.UpdateProcessingStatusCalls[2].Status)
		assert.Equal(t, playtomic.StatusCompleted, store.UpdateProcessingStatusCalls[3].Status)
	})

	t.Run("new and played match with unconfirmed results sends no notifications", func(t *testing.T) {
		// Setup
		store := club.NewMock()
		notif := notifier.NewMock()
		metr := metrics.NewMock()
		p := New(store, notif, metr)

		match := &playtomic.PadelMatch{
			MatchID:          "m1",
			ProcessingStatus: playtomic.StatusNew,
			GameStatus:       playtomic.GameStatusPlayed,
			ResultsStatus:    playtomic.ResultsStatusValidating, // Not confirmed
		}
		store.GetMatchesForProcessingFunc = func() ([]*playtomic.PadelMatch, error) {
			return []*playtomic.PadelMatch{match}, nil
		}

		// Execute
		p.ProcessMatches(false)

		// Assert
		require.Len(t, notif.SendBookingNotificationCalls, 0, "No booking notification should be sent")
		require.Len(t, notif.SendResultNotificationCalls, 0, "No result notification should be sent")
		require.Len(t, store.UpdateProcessingStatusCalls, 1, "Status should be updated once")
		assert.Equal(t, playtomic.StatusBookingNotified, store.UpdateProcessingStatusCalls[0].Status)
	})
}
