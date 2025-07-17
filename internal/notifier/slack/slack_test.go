package slack

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/mauv0809/ideal-tribble/internal/club"
	"github.com/mauv0809/ideal-tribble/internal/metrics"
	"github.com/mauv0809/ideal-tribble/internal/playtomic"
	slackapi "github.com/slack-go/slack"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// mockSlackAPI is a mock implementation of the parts of the slack.Client that we use.
type mockSlackAPI struct {
	postMessageContextFunc func(ctx context.Context, channelID string, options ...slackapi.MsgOption) (string, string, error)
}

func (m *mockSlackAPI) PostMessageContext(ctx context.Context, channelID string, options ...slackapi.MsgOption) (string, string, error) {
	if m.postMessageContextFunc != nil {
		return m.postMessageContextFunc(ctx, channelID, options...)
	}
	return "C12345", "123456789.12345", nil
}

func TestSendMessage_DryRun(t *testing.T) {
	metrics := metrics.NewMock()
	// Pass nil for the api, as it shouldn't be called in dry-run mode.
	notifier := NewNotifierWithAPI(nil, "C123", metrics)

	message := slackapi.NewBlockMessage()
	_, _, err := notifier.sendMessage(message, true)
	require.NoError(t, err)
}

func TestSendMessage_Success(t *testing.T) {
	postMessageCalled := false
	api := &mockSlackAPI{
		postMessageContextFunc: func(ctx context.Context, channelID string, options ...slackapi.MsgOption) (string, string, error) {
			postMessageCalled = true
			assert.Equal(t, "C123", channelID)
			return "C123", "ts123", nil
		},
	}

	metrics := metrics.NewMock()
	notifier := NewNotifierWithAPI(api, "C123", metrics)

	message := slackapi.NewBlockMessage(slackapi.NewSectionBlock(slackapi.NewTextBlockObject("plain_text", "hello", false, false), nil, nil))
	_, _, err := notifier.sendMessage(message, false)

	require.NoError(t, err)
	assert.True(t, postMessageCalled, "PostMessageContext should have been called")
	assert.Equal(t, 1, metrics.SlackNotifSent())
	assert.Equal(t, 0, metrics.SlackNotifFailed())
}

func TestSendMessage_Failure(t *testing.T) {
	postMessageCalled := false
	expectedErr := errors.New("slack API is down")
	api := &mockSlackAPI{
		postMessageContextFunc: func(ctx context.Context, channelID string, options ...slackapi.MsgOption) (string, string, error) {
			postMessageCalled = true
			return "", "", expectedErr
		},
	}

	metrics := metrics.NewMock()
	notifier := NewNotifierWithAPI(api, "C123", metrics)

	message := slackapi.NewBlockMessage()
	_, _, err := notifier.sendMessage(message, false)

	require.Error(t, err)
	assert.ErrorIs(t, err, expectedErr)
	assert.True(t, postMessageCalled, "PostMessageContext should have been called")
	assert.Equal(t, 0, metrics.SlackNotifSent())
	assert.Equal(t, 1, metrics.SlackNotifFailed())
}

// Test one of the public methods to ensure it calls the private sender.
func TestSendBookingNotification_CallsSender(t *testing.T) {
	postMessageCalled := false
	api := &mockSlackAPI{
		postMessageContextFunc: func(ctx context.Context, channelID string, options ...slackapi.MsgOption) (string, string, error) {
			postMessageCalled = true
			return "C123", "ts123", nil
		},
	}

	metrics := metrics.NewMock()
	notifier := NewNotifierWithAPI(api, "C123", metrics)

	match := &playtomic.PadelMatch{
		ResourceName: "Court 1",
		Start:        time.Now().Unix(),
	}

	err := notifier.SendBookingNotification(match, false)
	require.NoError(t, err)
	assert.True(t, postMessageCalled, "PostMessageContext should have been called via SendBookingNotification")
}
func TestFormatBookingNotification(t *testing.T) {
	match := &playtomic.PadelMatch{
		ResourceName: "Court 1",
		Start:        time.Date(2025, 7, 9, 20, 0, 0, 0, time.Local).Unix(),
		Teams: []playtomic.Team{
			{Players: []playtomic.Player{{Name: "Player A"}, {Name: "Player B"}}},
		},
		BallBringerName: "Player A",
	}
	client := &Notifier{channelID: "C123"}
	msg := client.formatBookingNotification(match)
	require.Len(t, msg.Blocks.BlockSet, 4, "Expected 4 blocks")

	// 1. Header Block
	header, ok := msg.Blocks.BlockSet[0].(*slackapi.HeaderBlock)
	require.True(t, ok, "First block should be a HeaderBlock")
	assert.Equal(t, "🎾 New match booked! 🎾", header.Text.Text)
	assert.True(t, *header.Text.Emoji)

	// 2. Details Section
	details, ok := msg.Blocks.BlockSet[1].(*slackapi.SectionBlock)
	require.True(t, ok, "Second block should be a SectionBlock")
	expectedDetails := "Court: Court 1\nTime: Wednesday 09 Jul, 20:00"
	assert.Equal(t, expectedDetails, details.Text.Text)

	// 3. Players Section
	players, ok := msg.Blocks.BlockSet[2].(*slackapi.SectionBlock)
	require.True(t, ok, "Third block should be a SectionBlock")
	expectedPlayers := "Players:\n• Player A\n• Player B"
	assert.Equal(t, expectedPlayers, players.Text.Text)

	// 4. Context Section
	contextBlock, ok := msg.Blocks.BlockSet[3].(*slackapi.ContextBlock)
	require.True(t, ok, "Fourth block should be a ContextBlock")
	require.Len(t, contextBlock.ContextElements.Elements, 1)

	ballBringerElement, ok := contextBlock.ContextElements.Elements[0].(*slackapi.TextBlockObject)
	require.True(t, ok)
	assert.Equal(t, "🎾 Player A is bringing balls!", ballBringerElement.Text)
}

func TestFormatResultNotification(t *testing.T) {
	match := &playtomic.PadelMatch{
		ResourceName: "Court 1",
		Start:        time.Date(2025, 7, 9, 20, 0, 0, 0, time.Local).Unix(),
		MatchType:    playtomic.MatchTypeCompetition,
		Teams: []playtomic.Team{
			{ID: "t1", TeamResult: "WON", Players: []playtomic.Player{{Name: "Player A"}, {Name: "Player B"}}},
			{ID: "t2", Players: []playtomic.Player{{Name: "Player C"}, {Name: "Player D"}}},
		},
		BallBringerName: "Player C",
		Results: []playtomic.SetResult{
			{Name: "Set 1", Scores: map[string]int{"t1": 6, "t2": 2}},
			{Name: "Set 2", Scores: map[string]int{"t1": 7, "t2": 5}},
		},
	}
	client := &Notifier{channelID: "C123"}
	msg := client.formatResultNotification(match)

	require.Len(t, msg.Blocks.BlockSet, 4, "Expected 4 blocks")

	// Check header and details
	header, ok := msg.Blocks.BlockSet[0].(*slackapi.HeaderBlock)
	require.True(t, ok)
	assert.Equal(t, "🎾 Match finished! 🎾", header.Text.Text)

	details, ok := msg.Blocks.BlockSet[1].(*slackapi.SectionBlock)
	require.True(t, ok)
	assert.Equal(t, "Court 1 at Wednesday 09 Jul, 20:00", details.Text.Text)

	// Check results section
	resultsSection, ok := msg.Blocks.BlockSet[2].(*slackapi.SectionBlock)
	require.True(t, ok)
	assert.Equal(t, "Result: Player A & Player B won! 🏆", resultsSection.Text.Text)
	require.Len(t, resultsSection.Fields, 2)

	// With sorting, the order is now deterministic
	expectedSet1 := "Set 1\n• Player A & Player B: 6\n• Player C & Player D: 2"
	expectedSet2 := "Set 2\n• Player A & Player B: 7\n• Player C & Player D: 5"
	assert.Equal(t, expectedSet1, resultsSection.Fields[0].Text)
	assert.Equal(t, expectedSet2, resultsSection.Fields[1].Text)

	// Check context block
	contextBlock, ok := msg.Blocks.BlockSet[3].(*slackapi.ContextBlock)
	require.True(t, ok)
	require.Len(t, contextBlock.ContextElements.Elements, 1)

	ballBringerElement, ok := contextBlock.ContextElements.Elements[0].(*slackapi.TextBlockObject)
	require.True(t, ok)
	assert.Equal(t, "🎾 Player C brought the balls!", ballBringerElement.Text)
}

func TestFormatLeaderboard(t *testing.T) {
	t.Run("displays leaderboard with stats", func(t *testing.T) {
		stats := []club.PlayerStats{
			{PlayerName: "Player A", MatchesPlayed: 10, MatchesWon: 8, WinPercentage: 80.0, SetsWon: 16, GamesWon: 96},
			{PlayerName: "Player B", MatchesPlayed: 10, MatchesWon: 6, WinPercentage: 60.0, SetsWon: 12, GamesWon: 80},
			{PlayerName: "Player C", MatchesPlayed: 10, MatchesWon: 4, WinPercentage: 40.0, SetsWon: 8, GamesWon: 64},
		}

		client := &Notifier{channelID: "C123"}
		msg := client.formatLeaderboard(stats)

		require.Len(t, msg.Blocks.BlockSet, 4, "Expected 4 blocks (header + 3 players)")

		// Check header
		header, ok := msg.Blocks.BlockSet[0].(*slackapi.HeaderBlock)
		require.True(t, ok)
		assert.Equal(t, "🏆 Player Leaderboard 🏆", header.Text.Text)

		// Check first player
		player1, ok := msg.Blocks.BlockSet[1].(*slackapi.SectionBlock)
		require.True(t, ok)
		assert.Contains(t, player1.Text.Text, "1. 🥇 Player A")
		assert.Contains(t, player1.Text.Text, "> Match Win %: 80.00% (8/10)")

		// Check second player
		player2, ok := msg.Blocks.BlockSet[2].(*slackapi.SectionBlock)
		require.True(t, ok)
		assert.Contains(t, player2.Text.Text, "2. 🥈 Player B")

		// Check third player
		player3, ok := msg.Blocks.BlockSet[3].(*slackapi.SectionBlock)
		require.True(t, ok)
		assert.Contains(t, player3.Text.Text, "3. 🥉 Player C")
	})

	t.Run("displays message when no stats are available", func(t *testing.T) {
		stats := []club.PlayerStats{}

		client := &Notifier{channelID: "C123"}
		msg := client.formatLeaderboard(stats)

		require.Len(t, msg.Blocks.BlockSet, 2, "Expected 2 blocks (header + message)")

		// Check message
		message, ok := msg.Blocks.BlockSet[1].(*slackapi.SectionBlock)
		require.True(t, ok)
		assert.Equal(t, "No stats available yet. Go play some matches!", message.Text.Text)
	})
}

func TestFormatPlayerStats(t *testing.T) {
	client := &Notifier{channelID: "C123"}

	t.Run("formats stats for a found player", func(t *testing.T) {
		stat := &club.PlayerStats{
			PlayerName:    "Morten Voss",
			MatchesPlayed: 10,
			MatchesWon:    8,
			WinPercentage: 80.0,
			SetsWon:       16,
			GamesWon:      96,
		}

		msg := client.formatPlayerStats(stat, "Morten")
		require.Len(t, msg.Blocks.BlockSet, 2)

		header, ok := msg.Blocks.BlockSet[0].(*slackapi.HeaderBlock)
		require.True(t, ok)
		assert.Equal(t, "🏆 Stats for Morten Voss 🏆", header.Text.Text)

		section, ok := msg.Blocks.BlockSet[1].(*slackapi.SectionBlock)
		require.True(t, ok)
		assert.Contains(t, section.Text.Text, "> *Match Win %*: 80.00% (8/10)")
		assert.Contains(t, section.Text.Text, "> *Sets Won*: 16")
		assert.Contains(t, section.Text.Text, "> *Games Won*: 96")
	})

	t.Run("formats message for a player not found", func(t *testing.T) {
		msg := client.formatPlayerNotFound("Unknown Player")
		require.Len(t, msg.Blocks.BlockSet, 1)

		section, ok := msg.Blocks.BlockSet[0].(*slackapi.SectionBlock)
		require.True(t, ok)
		assert.Equal(t, "Sorry, I couldn't find a player matching *Unknown Player*. Try a different name.", section.Text.Text)
	})
}

func TestFormatLevelLeaderboard(t *testing.T) {
	client := &Notifier{channelID: "C123"}

	t.Run("formats leaderboard with players", func(t *testing.T) {
		players := []club.PlayerInfo{
			{Name: "Player A", Level: 4.5},
			{Name: "Player B", Level: 3.5},
			{Name: "Player C", Level: 3.5},
			{Name: "Player D", Level: 2.0},
		}

		msg := client.formatLevelLeaderboard(players)
		require.Len(t, msg.Blocks.BlockSet, 5) // Header + 4 players

		header, ok := msg.Blocks.BlockSet[0].(*slackapi.HeaderBlock)
		require.True(t, ok)
		assert.Equal(t, "🏆 Player Leaderboard (by Level) 🏆", header.Text.Text)

		// Check first player
		player1, ok := msg.Blocks.BlockSet[1].(*slackapi.SectionBlock)
		require.True(t, ok)
		assert.Contains(t, player1.Text.Text, "1. 🥇 Player A")
		assert.Contains(t, player1.Text.Text, "> *Level*: 4.50")

		// Check second player
		player2, ok := msg.Blocks.BlockSet[2].(*slackapi.SectionBlock)
		require.True(t, ok)
		assert.Contains(t, player2.Text.Text, "2. 🥈 Player B")
		assert.Contains(t, player2.Text.Text, "> *Level*: 3.50")
	})

	t.Run("formats message for no players", func(t *testing.T) {
		msg := client.formatLevelLeaderboard([]club.PlayerInfo{})
		require.Len(t, msg.Blocks.BlockSet, 2) // Header + message

		message, ok := msg.Blocks.BlockSet[1].(*slackapi.SectionBlock)
		require.True(t, ok)
		assert.Equal(t, "No players found.", message.Text.Text)
	})
}
