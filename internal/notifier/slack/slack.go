package slack

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/charmbracelet/log"
	"github.com/mauv0809/ideal-tribble/internal/club"
	"github.com/mauv0809/ideal-tribble/internal/metrics"
	"github.com/mauv0809/ideal-tribble/internal/notifier"
	"github.com/mauv0809/ideal-tribble/internal/playtomic"
	"github.com/slack-go/slack"
)

// slackClient is an interface that contains the methods from the slack.Client that we use.
// This allows for easy mocking in tests.
type slackClient interface {
	PostMessageContext(ctx context.Context, channelID string, options ...slack.MsgOption) (string, string, error)
}

var _ notifier.Notifier = &Notifier{}

// Notifier handles sending notifications to Slack.
type Notifier struct {
	api       slackClient
	channelID string
	metrics   metrics.Metrics
}

// NewNotifier creates a new Notifier.
func NewNotifier(token, channelID string, metrics metrics.Metrics) *Notifier {
	api := slack.New(token)
	return &Notifier{
		api:       api,
		channelID: channelID,
		metrics:   metrics,
	}
}

// NewNotifierWithAPI creates a new Notifier with a specific slack.Client instance.
// Useful for tests that need to intercept API calls.
func NewNotifierWithAPI(api slackClient, channelID string, metrics metrics.Metrics) *Notifier {
	return &Notifier{
		api:       api,
		channelID: channelID,
		metrics:   metrics,
	}
}

func (s *Notifier) sendMessage(message slack.Message, dryRun bool) (string, string, error) {
	if dryRun {
		jsonMsg, _ := json.MarshalIndent(message, "", "  ")
		log.Info("[Dry Run] Would send Slack message", "channel", s.channelID, "message", string(jsonMsg))
		return "dry-run-ts", "dry-run-thread-ts", nil
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	channelID, timestamp, err := s.api.PostMessageContext(
		ctx,
		s.channelID,
		slack.MsgOptionBlocks(message.Blocks.BlockSet...),
		slack.MsgOptionAsUser(true),
	)

	if err != nil {
		s.metrics.IncSlackNotifFailed()
		log.Error("Failed to send Slack message", "error", err, "channel", channelID)
		return "", "", fmt.Errorf("failed to post message: %w", err)
	}

	s.metrics.IncSlackNotifSent()
	log.Info("Successfully sent Slack message", "channel", channelID, "timestamp", timestamp)
	return channelID, timestamp, nil
}

// Implement the Notifier interface
func (s *Notifier) SendBookingNotification(match *playtomic.PadelMatch, dryRun bool) error {
	msg := s.formatBookingNotification(match)
	_, _, err := s.sendMessage(msg, dryRun)
	return err
}

func (s *Notifier) SendResultNotification(match *playtomic.PadelMatch, dryRun bool) error {
	msg := s.formatResultNotification(match)
	_, _, err := s.sendMessage(msg, dryRun)
	return err
}

func (s *Notifier) SendLeaderboard(stats []club.PlayerStats, dryRun bool) error {
	msg := s.formatLeaderboard(stats)
	_, _, err := s.sendMessage(msg, dryRun)
	return err
}

func (s *Notifier) SendLevelLeaderboard(players []club.PlayerInfo, dryRun bool) error {
	msg := s.formatLevelLeaderboard(players)
	_, _, err := s.sendMessage(msg, dryRun)
	return err
}

func (s *Notifier) SendPlayerStats(stats *club.PlayerStats, query string, dryRun bool) error {
	msg := s.formatPlayerStats(stats, query)
	_, _, err := s.sendMessage(msg, dryRun)
	return err
}

func (s *Notifier) SendPlayerNotFound(query string, dryRun bool) error {
	msg := s.formatPlayerNotFound(query)
	_, _, err := s.sendMessage(msg, dryRun)
	return err
}

// FormatLeaderboardResponse formats a leaderboard message for a slash command response.
func (s *Notifier) FormatLeaderboardResponse(stats []club.PlayerStats) (any, error) {
	return s.formatLeaderboard(stats), nil
}

// FormatLevelLeaderboardResponse formats a level leaderboard message for a slash command response.
func (s *Notifier) FormatLevelLeaderboardResponse(players []club.PlayerInfo) (any, error) {
	return s.formatLevelLeaderboard(players), nil
}

// FormatPlayerStatsResponse formats a player stats message for a slash command response.
func (s *Notifier) FormatPlayerStatsResponse(stats *club.PlayerStats, query string) (any, error) {
	return s.formatPlayerStats(stats, query), nil
}

// FormatPlayerNotFoundResponse formats a player not found message for a slash command response.
func (s *Notifier) FormatPlayerNotFoundResponse(query string) (any, error) {
	return s.formatPlayerNotFound(query), nil
}

// formatBookingNotification creates the Slack message for a new match booking using Block Kit.
func (s *Notifier) formatBookingNotification(match *playtomic.PadelMatch) slack.Message {

	blocks := make([]slack.Block, 0)

	// Header - The Header block itself provides bolding. No asterisks needed.
	headerText := slack.NewTextBlockObject("plain_text", "🎾 New match booked! 🎾", true, false)
	blocks = append(blocks, slack.NewHeaderBlock(headerText))

	// Details - Use newlines for clear separation.
	detailsText := fmt.Sprintf("Court: %s\nTime: %s", match.ResourceName, time.Unix(match.Start, 0).Format("Monday 02 Jan, 15:04"))
	blocks = append(blocks, slack.NewSectionBlock(slack.NewTextBlockObject("plain_text", detailsText, true, false), nil, nil))

	// Players
	var playerNames []string
	for _, team := range match.Teams {
		for _, player := range team.Players {
			if player.Name != "" {
				playerNames = append(playerNames, fmt.Sprintf("• %s", player.Name))
			}
		}
	}
	if len(playerNames) > 0 {
		playersText := "Players:\n" + strings.Join(playerNames, "\n")
		blocks = append(blocks, slack.NewSectionBlock(slack.NewTextBlockObject("plain_text", playersText, true, false), nil, nil))
	}

	// Context - For simpler, single-line info.
	var contextElements []slack.MixedElement
	if match.BallBringerName != "" {
		contextElements = append(contextElements, slack.NewTextBlockObject("plain_text", fmt.Sprintf("🎾 %s is bringing balls!", match.BallBringerName), true, false))
	}
	if len(contextElements) > 0 {
		blocks = append(blocks, slack.NewContextBlock("", contextElements...))
	}

	return slack.NewBlockMessage(blocks...)
}

// formatResultNotification creates the Slack message for a finished match using Block Kit.
func (s *Notifier) formatResultNotification(match *playtomic.PadelMatch) slack.Message {
	blocks := make([]slack.Block, 0)

	// Header
	headerText := slack.NewTextBlockObject("plain_text", "🎾 Match finished! 🎾", true, false)
	blocks = append(blocks, slack.NewHeaderBlock(headerText))

	// Details
	detailsText := fmt.Sprintf("%s at %s", match.ResourceName, time.Unix(match.Start, 0).Format("Monday 02 Jan, 15:04"))
	blocks = append(blocks, slack.NewSectionBlock(slack.NewTextBlockObject("plain_text", detailsText, false, false), nil, nil))

	if match.MatchType == playtomic.MatchTypeCompetition {
		// Results
		if len(match.Results) > 0 {
			teamNames := make(map[string]string)
			var winningTeamName string
			for _, team := range match.Teams {
				var playerNames []string
				for _, player := range team.Players {
					playerNames = append(playerNames, player.Name)
				}
				currentTeamName := strings.Join(playerNames, " & ")
				teamNames[team.ID] = currentTeamName

				if team.TeamResult == "WON" {
					winningTeamName = currentTeamName
				}
			}

			var resultsFields []*slack.TextBlockObject
			for _, result := range match.Results {
				var scoresText []string
				for teamID, score := range result.Scores {
					if teamName, ok := teamNames[teamID]; ok {
						scoresText = append(scoresText, fmt.Sprintf("• %s: %d", teamName, score))
					}
				}
				if len(scoresText) > 0 {
					sort.Strings(scoresText) // Sort to ensure deterministic order
					setResultText := fmt.Sprintf("%s\n%s", result.Name, strings.Join(scoresText, "\n"))
					resultsFields = append(resultsFields, slack.NewTextBlockObject("plain_text", setResultText, true, false))
				}
			}

			resultHeaderText := "Result:"
			if winningTeamName != "" {
				resultHeaderText = fmt.Sprintf("Result: %s won! 🏆", winningTeamName)
			}

			if len(resultsFields) > 0 {
				blocks = append(blocks, slack.NewSectionBlock(slack.NewTextBlockObject("plain_text", resultHeaderText, true, false), resultsFields, nil))
			} else {
				blocks = append(blocks, slack.NewSectionBlock(slack.NewTextBlockObject("plain_text", "Result: No scores reported.", true, false), nil, nil))
			}
		} else {
			blocks = append(blocks, slack.NewSectionBlock(slack.NewTextBlockObject("plain_text", "Result: No scores reported.", true, false), nil, nil))
		}
	} else {
		// Players
		var playerNames []string
		for _, team := range match.Teams {
			for _, player := range team.Players {
				if player.Name != "" {
					playerNames = append(playerNames, fmt.Sprintf("• %s", player.Name))
				}
			}
		}
		if len(playerNames) > 0 {
			playersText := "Players:\n" + strings.Join(playerNames, "\n")
			blocks = append(blocks, slack.NewSectionBlock(slack.NewTextBlockObject("plain_text", playersText, true, false), nil, nil))
		}

	}

	// Context (Ball Bringer)
	if match.BallBringerName != "" {
		ballBringerText := fmt.Sprintf("🎾 %s brought the balls!", match.BallBringerName)
		blocks = append(blocks, slack.NewContextBlock("", slack.NewTextBlockObject("plain_text", ballBringerText, true, false)))
	}

	return slack.NewBlockMessage(blocks...)
}

// formatLeaderboard creates a Slack message to display the player leaderboard.
func (s *Notifier) formatLeaderboard(stats []club.PlayerStats) slack.Message {
	blocks := make([]slack.Block, 0)

	// Header
	headerText := slack.NewTextBlockObject("plain_text", "🏆 Player Leaderboard 🏆", true, false)
	blocks = append(blocks, slack.NewHeaderBlock(headerText))

	if len(stats) == 0 {
		blocks = append(blocks, slack.NewSectionBlock(slack.NewTextBlockObject("plain_text", "No stats available yet. Go play some matches!", true, false), nil, nil))
		return slack.NewBlockMessage(blocks...)
	}

	// Player Ranks
	for i, stat := range stats {
		rank := i + 1
		var medal string
		switch rank {
		case 1:
			medal = "🥇"
		case 2:
			medal = "🥈"
		case 3:
			medal = "🥉"
		}

		playerText := fmt.Sprintf("%d. %s %s\n> Match Win %%: %.2f%% (%d/%d) | Sets Won: %d | Games Won: %d",
			rank,
			medal,
			stat.PlayerName,
			stat.WinPercentage,
			stat.MatchesWon,
			stat.MatchesPlayed,
			stat.SetsWon,
			stat.GamesWon,
		)
		blocks = append(blocks, slack.NewSectionBlock(slack.NewTextBlockObject("plain_text", playerText, true, false), nil, nil))
	}

	return slack.NewBlockMessage(blocks...)
}

// formatLevelLeaderboard creates a Slack message to display the player leaderboard by level.
func (s *Notifier) formatLevelLeaderboard(players []club.PlayerInfo) slack.Message {
	blocks := make([]slack.Block, 0)

	// Header
	headerText := slack.NewTextBlockObject("plain_text", "🏆 Player Leaderboard (by Level) 🏆", true, false)
	blocks = append(blocks, slack.NewHeaderBlock(headerText))

	if len(players) == 0 {
		blocks = append(blocks, slack.NewSectionBlock(slack.NewTextBlockObject("plain_text", "No players found.", true, false), nil, nil))
		return slack.NewBlockMessage(blocks...)
	}

	// Player Ranks
	for i, player := range players {
		rank := i + 1
		var medal string
		switch rank {
		case 1:
			medal = "🥇"
		case 2:
			medal = "🥈"
		case 3:
			medal = "🥉"
		}

		playerText := fmt.Sprintf("%d. %s %s\n> *Level*: %.2f",
			rank,
			medal,
			player.Name,
			player.Level,
		)
		blocks = append(blocks, slack.NewSectionBlock(slack.NewTextBlockObject("mrkdwn", playerText, false, false), nil, nil))
	}

	return slack.NewBlockMessage(blocks...)
}

// formatPlayerStats creates a Slack message to display a single player's stats.
func (s *Notifier) formatPlayerStats(stat *club.PlayerStats, query string) slack.Message {
	blocks := make([]slack.Block, 0)

	// Header
	headerText := fmt.Sprintf("🏆 Stats for %s 🏆", stat.PlayerName)
	blocks = append(blocks, slack.NewHeaderBlock(slack.NewTextBlockObject("plain_text", headerText, true, false)))

	// Player Ranks
	playerText := fmt.Sprintf("> *Match Win %%*: %.2f%% (%d/%d)\n> *Sets Won*: %d\n> *Games Won*: %d",
		stat.WinPercentage,
		stat.MatchesWon,
		stat.MatchesPlayed,
		stat.SetsWon,
		stat.GamesWon,
	)
	blocks = append(blocks, slack.NewSectionBlock(slack.NewTextBlockObject("mrkdwn", playerText, false, false), nil, nil))

	return slack.NewBlockMessage(blocks...)
}

// formatPlayerNotFound creates a Slack message for when a player's stats are not found.
func (s *Notifier) formatPlayerNotFound(query string) slack.Message {
	text := fmt.Sprintf("Sorry, I couldn't find a player matching *%s*. Try a different name.", query)
	return slack.NewBlockMessage(
		slack.NewSectionBlock(slack.NewTextBlockObject("mrkdwn", text, false, false), nil, nil),
	)
}
