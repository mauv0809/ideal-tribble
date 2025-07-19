package processor

import (
	"errors"
	"testing"
	"time"

	"github.com/mauv0809/ideal-tribble/internal/club"
	"github.com/mauv0809/ideal-tribble/internal/metrics"
	"github.com/mauv0809/ideal-tribble/internal/notifier"
	"github.com/mauv0809/ideal-tribble/internal/playtomic"
	pubsubPkg "github.com/mauv0809/ideal-tribble/internal/pubsub"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestProcessor_ProcessMatches(t *testing.T) {
	t.Run("new upcoming match sends assign ball boy event and changes status", func(t *testing.T) {
		// Setup
		store := club.NewMock()
		notif := notifier.NewMock()
		metr := metrics.NewMock()
		psClient := pubsubPkg.NewMock("TEST")
		p := New(store, notif, metr, psClient)

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
		// Mock AssignBallBringerAtomically which is now used by the processor
		store.AssignBallBringerAtomicallyFunc = func(matchID string, playerIDs []string) (string, string, error) {
			// Simulate finding and assigning player p1
			if assert.Equal(t, "m1", matchID) && assert.Contains(t, playerIDs, "p1") {
				return "p1", "Player 1", nil
			}
			return "", "", errors.New("mock error: could not assign ball bringer")
		}

		// Execute
		p.ProcessMatches(false)

		// Assert that the ball boy assignment event was sent
		require.Len(t, psClient.SendMessageCalls, 1, "An AssignBallBoy pubsub message should be sent")
		assert.Equal(t, string(pubsubPkg.EventAssignBallBoy), string(psClient.SendMessageCalls[0].Topic))
		sentMatch, ok := psClient.SendMessageCalls[0].Data.(*playtomic.PadelMatch)
		require.True(t, ok, "Data sent to pubsub should be a PadelMatch")
		assert.Equal(t, "m1", sentMatch.MatchID)

		// Assert that the match status transitioned to StatusAssigningBallBringer
		require.Len(t, store.UpdateProcessingStatusCalls, 1, "Status should be updated once to StatusAssigningBallBringer")
		assert.Equal(t, playtomic.StatusAssigningBallBringer, store.UpdateProcessingStatusCalls[0].Status)

		// Ensure no other notifications or status updates happened in this step
		require.Len(t, notif.SendBookingNotificationCalls, 0, "No booking notification should be sent synchronously")
		require.Len(t, notif.SendResultNotificationCalls, 0, "No result notification should be sent synchronously")
	})

	t.Run("new and played match with confirmed results transitions to result notified", func(t *testing.T) {
		// Setup
		store := club.NewMock()
		notif := notifier.NewMock()
		metr := metrics.NewMock()
		psClient := pubsubPkg.NewMock("TEST")
		p := New(store, notif, metr, psClient)

		match := &playtomic.PadelMatch{
			MatchID:          "m1",
			ProcessingStatus: playtomic.StatusNew,
			GameStatus:       playtomic.GameStatusPlayed,
			ResultsStatus:    playtomic.ResultsStatusConfirmed,
			End:              time.Now().Unix(), // Set end time to now to trigger result notification
			MatchTypeEnum:    playtomic.MatchTypeEnumDoubles,
		}
		store.GetMatchesForProcessingFunc = func() ([]*playtomic.PadelMatch, error) {
			// Ensure the returned match has the End time set for the test
			match.End = time.Now().Unix()
			return []*playtomic.PadelMatch{match}, nil
		}

		// Execute
		p.ProcessMatches(false)

		// Assert
		require.Len(t, notif.SendBookingNotificationCalls, 0, "Booking notification should be skipped")
		// After the first ProcessMatches call, only one PubSub message (EventNotifyResult) should be sent,
		// and the match should transition up to StatusResultNotified.
		require.Len(t, psClient.SendMessageCalls, 1, "Only one pubsub message should be sent: EventNotifyResult")
		assert.Equal(t, string(pubsubPkg.EventNotifyResult), string(psClient.SendMessageCalls[0].Topic))
		sentMatch1, ok1 := psClient.SendMessageCalls[0].Data.(*playtomic.PadelMatch)
		require.True(t, ok1, "Data sent to pubsub should be a PadelMatch")
		assert.Equal(t, "m1", sentMatch1.MatchID)

		require.Len(t, store.UpdateProcessingStatusCalls, 1, "Status should be updated once: New -> ResultAvailable")
		assert.Equal(t, playtomic.StatusResultAvailable, store.UpdateProcessingStatusCalls[0].Status)

		// Simulate the next processing cycle, triggered by the PubSub event for EventNotifyResult.
		// The match should now be in StatusResultNotified (as updated by the PubSub handler).
		match.ProcessingStatus = playtomic.StatusResultNotified // Manually update status for next mock call
		psClient.SendMessageCalls = nil                         // Clear previous PubSub calls
		store.UpdateProcessingStatusCalls = nil                 // Clear previous status updates

		p.ProcessMatches(false)

		// Assert the next step: EventUpdatePlayerStats is sent, and status becomes StatusStatsUpdated
		require.Len(t, psClient.SendMessageCalls, 1, "One pubsub message should be sent in the second cycle: EventUpdatePlayerStats")
		assert.Equal(t, string(pubsubPkg.EventUpdatePlayerStats), string(psClient.SendMessageCalls[0].Topic))
		sentMatch2, ok2 := psClient.SendMessageCalls[0].Data.(*playtomic.PadelMatch)
		require.True(t, ok2, "Data sent to pubsub should be a PadelMatch")
		assert.Equal(t, "m1", sentMatch2.MatchID)

		// The processor should update the status to 'UPDATING_PLAYER_STATS' to prevent re-processing.
		require.Len(t, store.UpdateProcessingStatusCalls, 1, "Status should be updated to 'UPDATING_PLAYER_STATS'")
		assert.Equal(t, playtomic.StatusUpdatingPlayerStats, store.UpdateProcessingStatusCalls[0].Status)
	})

	t.Run("match with booking notified transitions to result notified after being played", func(t *testing.T) {
		// Setup
		store := club.NewMock()
		notif := notifier.NewMock()
		metr := metrics.NewMock()
		psClient := pubsubPkg.NewMock("TEST")
		p := New(store, notif, metr, psClient)

		match := &playtomic.PadelMatch{
			MatchID:          "m1",
			ProcessingStatus: playtomic.StatusBookingNotified,
			GameStatus:       playtomic.GameStatusPlayed,
			ResultsStatus:    playtomic.ResultsStatusConfirmed,
			End:              time.Now().Unix(), // Set end time to now to trigger result notification
			MatchTypeEnum:    playtomic.MatchTypeEnumDoubles,
		}
		store.GetMatchesForProcessingFunc = func() ([]*playtomic.PadelMatch, error) {
			// Ensure the returned match has the End time set for the test
			match.End = time.Now().Unix()
			return []*playtomic.PadelMatch{match}, nil
		}

		// Execute
		p.ProcessMatches(false)

		// Assert
		require.Len(t, notif.SendBookingNotificationCalls, 0, "Booking notification should not be sent again")
		// After the first ProcessMatches call, only one PubSub message (EventNotifyResult) should be sent,
		// and the match should transition up to StatusResultNotified.
		require.Len(t, psClient.SendMessageCalls, 1, "Only one pubsub message should be sent: EventNotifyResult")
		assert.Equal(t, string(pubsubPkg.EventNotifyResult), string(psClient.SendMessageCalls[0].Topic))
		sentMatch1, ok1 := psClient.SendMessageCalls[0].Data.(*playtomic.PadelMatch)
		require.True(t, ok1, "Data sent to pubsub should be a PadelMatch")
		assert.Equal(t, "m1", sentMatch1.MatchID)

		require.Len(t, store.UpdateProcessingStatusCalls, 1, "Status should be updated once: BookingNotified -> ResultAvailable")
		assert.Equal(t, playtomic.StatusResultAvailable, store.UpdateProcessingStatusCalls[0].Status)

		// Simulate the next processing cycle, triggered by the PubSub event for EventNotifyResult.
		// The match should now be in StatusResultNotified.
		match.ProcessingStatus = playtomic.StatusResultNotified // Manually update status for next mock call
		psClient.SendMessageCalls = nil                         // Clear previous PubSub calls
		store.UpdateProcessingStatusCalls = nil                 // Clear previous status updates

		p.ProcessMatches(false)

		// Assert the next step: EventUpdatePlayerStats is sent, and status becomes StatusStatsUpdated
		require.Len(t, psClient.SendMessageCalls, 1, "One pubsub message should be sent in the second cycle: EventUpdatePlayerStats")
		assert.Equal(t, string(pubsubPkg.EventUpdatePlayerStats), string(psClient.SendMessageCalls[0].Topic))
		sentMatch2, ok2 := psClient.SendMessageCalls[0].Data.(*playtomic.PadelMatch)
		require.True(t, ok2, "Data sent to pubsub should be a PadelMatch")
		assert.Equal(t, "m1", sentMatch2.MatchID)

		// The processor should update the status to 'UPDATING_PLAYER_STATS' to prevent re-processing.
		require.Len(t, store.UpdateProcessingStatusCalls, 1, "Status should be updated to 'UPDATING_PLAYER_STATS'")
		assert.Equal(t, playtomic.StatusUpdatingPlayerStats, store.UpdateProcessingStatusCalls[0].Status)
	})

	t.Run("new and played match with unconfirmed results sends no notifications", func(t *testing.T) {
		// Setup
		store := club.NewMock()
		notif := notifier.NewMock()
		metr := metrics.NewMock()
		psClient := pubsubPkg.NewMock("TEST")
		p := New(store, notif, metr, psClient)

		match := &playtomic.PadelMatch{
			MatchID:          "m1",
			ProcessingStatus: playtomic.StatusNew,
			GameStatus:       playtomic.GameStatusPlayed,
			ResultsStatus:    playtomic.ResultsStatusValidating, // Not confirmed
			MatchTypeEnum:    playtomic.MatchTypeEnumDoubles,
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
