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
	"github.com/mauv0809/ideal-tribble/internal/matchmaking"
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
	loc, err := time.LoadLocation("Europe/Copenhagen")
	var timeStr string
	if err == nil {
		timeStr = time.Unix(match.Start, 0).In(loc).Format("Monday 02 Jan, 15:04")
	} else {
		timeStr = time.Unix(match.Start, 0).Format("Monday 02 Jan, 15:04")
	}
	detailsText := fmt.Sprintf("Court: %s\nTime: %s", match.ResourceName, timeStr)
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
	loc, err := time.LoadLocation("Europe/Copenhagen")
	var timeStr string
	if err == nil {
		timeStr = time.Unix(match.Start, 0).In(loc).Format("Monday 02 Jan, 15:04")
	} else {
		timeStr = time.Unix(match.Start, 0).Format("Monday 02 Jan, 15:04")
	}
	detailsText := fmt.Sprintf("%s at %s", match.ResourceName, timeStr)
	blocks = append(blocks, slack.NewSectionBlock(slack.NewTextBlockObject("plain_text", detailsText, false, false), nil, nil))

	if match.MatchType == playtomic.MatchTypeCompetitive {
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

// Matchmaking methods

// SendMatchAvailabilityRequest sends initial availability request message
func (s *Notifier) SendMatchAvailabilityRequest(request any, dryRun bool) (string, string, error) {
	matchRequest, ok := request.(*matchmaking.MatchRequest)
	if !ok {
		return "", "", fmt.Errorf("invalid request type for SendMatchAvailabilityRequest")
	}

	msg := s.formatMatchAvailabilityRequest(matchRequest)
	channelID, timestamp, err := s.sendMessage(msg, dryRun)
	if err != nil {
		return "", "", fmt.Errorf("failed to send match availability request: %w", err)
	}

	return channelID, timestamp, nil
}

// SendMatchProposal sends a reply with the proposed match in the thread
func (s *Notifier) SendMatchProposal(request any, proposal any, dryRun bool) error {
	matchRequest, ok := request.(*matchmaking.MatchRequest)
	if !ok {
		return fmt.Errorf("invalid request type for SendMatchProposal")
	}

	matchProposal, ok := proposal.(*matchmaking.MatchProposal)
	if !ok {
		return fmt.Errorf("invalid proposal type for SendMatchProposal")
	}

	msg := s.formatMatchProposal(matchRequest, matchProposal)
	_, _, err := s.sendMessageToThread(msg, matchRequest.ChannelID, matchRequest.ThreadTS, dryRun)
	return err
}

// SendMatchConfirmation sends a reply with confirmation in the thread
func (s *Notifier) SendMatchConfirmation(request any, dryRun bool) error {
	matchRequest, ok := request.(*matchmaking.MatchRequest)
	if !ok {
		return fmt.Errorf("invalid request type for SendMatchConfirmation")
	}

	msg := s.formatMatchConfirmation(matchRequest)
	_, _, err := s.sendMessageToThread(msg, matchRequest.ChannelID, matchRequest.ThreadTS, dryRun)
	return err
}

// FormatMatchRequestResponse formats a response for the /match command
func (s *Notifier) FormatMatchRequestResponse(request any) (any, error) {
	matchRequest, ok := request.(*matchmaking.MatchRequest)
	if !ok {
		return nil, fmt.Errorf("invalid request type for FormatMatchRequestResponse")
	}

	return s.formatMatchRequestResponse(matchRequest), nil
}

// SendDirectMessage sends a direct message to a user
func (s *Notifier) SendDirectMessage(userID string, text string) (string, string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Create a simple text message
	channelID, timestamp, err := s.api.PostMessageContext(
		ctx,
		userID, // In Slack, you can send DMs by using the user ID as the channel
		slack.MsgOptionText(text, false),
		slack.MsgOptionAsUser(true),
	)

	if err != nil {
		s.metrics.IncSlackNotifFailed()
		log.Error("Failed to send Slack DM", "error", err, "user", userID)
		return "", "", fmt.Errorf("failed to send DM: %w", err)
	}

	s.metrics.IncSlackNotifSent()
	log.Info("Successfully sent Slack DM", "user", userID, "timestamp", timestamp)
	return channelID, timestamp, nil
}

// sendMessageToThread sends a message to a thread
func (s *Notifier) sendMessageToThread(message slack.Message, channelID string, threadTS *string, dryRun bool) (string, string, error) {
	if dryRun {
		jsonMsg, _ := json.MarshalIndent(message, "", "  ")
		log.Info("[Dry Run] Would send Slack thread message", "channel", channelID, "thread_ts", threadTS, "message", string(jsonMsg))
		return "dry-run-ts", "dry-run-thread-ts", nil
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	options := []slack.MsgOption{
		slack.MsgOptionBlocks(message.Blocks.BlockSet...),
		slack.MsgOptionAsUser(true),
	}

	if threadTS != nil {
		options = append(options, slack.MsgOptionTS(*threadTS))
	}

	channelID, timestamp, err := s.api.PostMessageContext(ctx, channelID, options...)
	if err != nil {
		s.metrics.IncSlackNotifFailed()
		log.Error("Failed to send Slack thread message", "error", err, "channel", channelID, "thread_ts", threadTS)
		return "", "", fmt.Errorf("failed to post thread message: %w", err)
	}

	s.metrics.IncSlackNotifSent()
	log.Info("Successfully sent Slack thread message", "channel", channelID, "timestamp", timestamp, "thread_ts", threadTS)
	return channelID, timestamp, nil
}

// formatMatchAvailabilityRequest formats the initial availability request message
func (s *Notifier) formatMatchAvailabilityRequest(request *matchmaking.MatchRequest) slack.Message {
	// Header block
	headerBlock := slack.NewSectionBlock(
		slack.NewTextBlockObject("plain_text", fmt.Sprintf("🎾 Match Request by %s", request.RequesterName), true, false),
		nil, nil,
	)

	// Instruction block
	instructionText := "React with the day emojis for when you're available to play! We need at least 4 players for the same day.\n\n" +
		"📅 React with:\n" +
		"• 1️⃣ for Monday\n" +
		"• 2️⃣ for Tuesday\n" +
		"• 3️⃣ for Wednesday\n" +
		"• 4️⃣ for Thursday\n" +
		"• 5️⃣ for Friday\n" +
		"• 6️⃣ for Saturday\n" +
		"• 7️⃣ for Sunday\n\n" +
		"You can react with multiple emojis if you're available on multiple days!"

	instructionBlock := slack.NewSectionBlock(
		slack.NewTextBlockObject("plain_text", instructionText, true, false),
		nil, nil,
	)

	// Divider
	dividerBlock := slack.NewDividerBlock()

	// Footer block
	footerBlock := slack.NewContextBlock(
		"",
		slack.NewTextBlockObject("plain_text", fmt.Sprintf("Request ID: %s • Created: %s",
			request.ID,
			request.CreatedAt.Format("Jan 2, 2006 at 3:04 PM"),
		), true, false),
	)

	blocks := []slack.Block{
		headerBlock,
		instructionBlock,
		dividerBlock,
		footerBlock,
	}

	return slack.NewBlockMessage(blocks...)
}

// formatMatchProposal formats the match proposal message
func (s *Notifier) formatMatchProposal(request *matchmaking.MatchRequest, proposal *matchmaking.MatchProposal) slack.Message {
	// Header block
	headerBlock := slack.NewSectionBlock(
		slack.NewTextBlockObject("plain_text", "🎾 Match Proposal", true, false),
		nil, nil,
	)

	// Date and time
	dateText := fmt.Sprintf("📅 Date: %s\n⏰ Time: %s - %s",
		proposal.Date,
		proposal.StartTime,
		proposal.EndTime,
	)

	dateBlock := slack.NewSectionBlock(
		slack.NewTextBlockObject("plain_text", dateText, true, false),
		nil, nil,
	)

	// Teams
	team1Names := make([]string, len(proposal.TeamAssignments.Team1))
	for i, player := range proposal.TeamAssignments.Team1 {
		team1Names[i] = player.Name
	}

	team2Names := make([]string, len(proposal.TeamAssignments.Team2))
	for i, player := range proposal.TeamAssignments.Team2 {
		team2Names[i] = player.Name
	}

	teamsText := fmt.Sprintf("🏆 Team 1: %s\n🏆 Team 2: %s",
		strings.Join(team1Names, ", "),
		strings.Join(team2Names, ", "),
	)

	teamsBlock := slack.NewSectionBlock(
		slack.NewTextBlockObject("plain_text", teamsText, true, false),
		nil, nil,
	)

	// Booking responsibility
	bookingText := fmt.Sprintf("📋 Booking Responsibility: %s\nPlease book this match on Playtomic",
		proposal.BookingResponsibleName,
	)

	bookingBlock := slack.NewSectionBlock(
		slack.NewTextBlockObject("plain_text", bookingText, true, false),
		nil, nil,
	)

	// Footer
	footerBlock := slack.NewContextBlock(
		"",
		slack.NewTextBlockObject("plain_text", fmt.Sprintf("Request ID: %s • Proposed by: %s",
			request.ID,
			request.RequesterName,
		), true, false),
	)

	blocks := []slack.Block{
		headerBlock,
		dateBlock,
		teamsBlock,
		bookingBlock,
		slack.NewDividerBlock(),
		footerBlock,
	}

	return slack.NewBlockMessage(blocks...)
}

// formatMatchConfirmation formats the match confirmation message
func (s *Notifier) formatMatchConfirmation(request *matchmaking.MatchRequest) slack.Message {
	// Header block
	headerBlock := slack.NewSectionBlock(
		slack.NewTextBlockObject("plain_text", "✅ Match Confirmed!", true, false),
		nil, nil,
	)

	// Match details
	var detailsText string
	if request.ProposedDate != nil && request.ProposedStartTime != nil && request.ProposedEndTime != nil {
		detailsText = fmt.Sprintf("📅 Date: %s\n⏰ Time: %s - %s",
			*request.ProposedDate,
			*request.ProposedStartTime,
			*request.ProposedEndTime,
		)
	} else {
		detailsText = "Match details confirmed"
	}

	detailsBlock := slack.NewSectionBlock(
		slack.NewTextBlockObject("plain_text", detailsText, true, false),
		nil, nil,
	)

	// Teams (if available)
	blocks := []slack.Block{headerBlock, detailsBlock}

	if request.TeamAssignments != nil {
		team1Names := make([]string, len(request.TeamAssignments.Team1))
		for i, player := range request.TeamAssignments.Team1 {
			team1Names[i] = player.Name
		}

		team2Names := make([]string, len(request.TeamAssignments.Team2))
		for i, player := range request.TeamAssignments.Team2 {
			team2Names[i] = player.Name
		}

		teamsText := fmt.Sprintf("🏆 Team 1: %s\n🏆 Team 2: %s",
			strings.Join(team1Names, ", "),
			strings.Join(team2Names, ", "),
		)

		teamsBlock := slack.NewSectionBlock(
			slack.NewTextBlockObject("plain_text", teamsText, true, false),
			nil, nil,
		)
		blocks = append(blocks, teamsBlock)
	}

	// Booking reminder
	var bookingText string
	if request.BookingResponsibleName != nil {
		bookingText = fmt.Sprintf("📋 %s - Don't forget to book on Playtomic!", *request.BookingResponsibleName)
	} else {
		bookingText = "📋 Don't forget to book on Playtomic!"
	}

	bookingBlock := slack.NewSectionBlock(
		slack.NewTextBlockObject("plain_text", bookingText, true, false),
		nil, nil,
	)

	// Success message
	successBlock := slack.NewSectionBlock(
		slack.NewTextBlockObject("plain_text", "🎉 See you on the court!", true, false),
		nil, nil,
	)

	// Footer
	footerBlock := slack.NewContextBlock(
		"",
		slack.NewTextBlockObject("plain_text", fmt.Sprintf("Request ID: %s • Confirmed at: %s",
			request.ID,
			time.Now().Format("Jan 2, 2006 at 3:04 PM"),
		), true, false),
	)

	blocks = append(blocks,
		bookingBlock,
		successBlock,
		slack.NewDividerBlock(),
		footerBlock,
	)

	return slack.NewBlockMessage(blocks...)
}

// formatMatchRequestResponse formats a response for the /match command
func (s *Notifier) formatMatchRequestResponse(request *matchmaking.MatchRequest) slack.Message {
	var headerText, statusText string

	switch request.Status {
	case matchmaking.StatusCollectingAvailability:
		headerText = "✅ Match Request Created"
		statusText = "I'll post an availability message shortly!"
	case matchmaking.StatusProposingMatch:
		headerText = "🎾 Match Proposal Ready"
		statusText = "Check the channel for the proposed match details."
	case matchmaking.StatusConfirmed:
		headerText = "✅ Match Confirmed"
		statusText = "Your match has been confirmed!"
	case matchmaking.StatusCancelled:
		headerText = "❌ Match Request Cancelled"
		statusText = "The match request has been cancelled."
	default:
		headerText = "Match Request Status"
		statusText = fmt.Sprintf("Status: %s", request.Status)
	}

	// Header block
	headerBlock := slack.NewSectionBlock(
		slack.NewTextBlockObject("plain_text", headerText, true, false),
		nil, nil,
	)

	// Status block
	statusBlock := slack.NewSectionBlock(
		slack.NewTextBlockObject("plain_text", statusText, true, false),
		nil, nil,
	)

	// Details block
	detailsText := fmt.Sprintf("Request ID: %s\nCreated: %s",
		request.ID,
		request.CreatedAt.Format("Jan 2, 2006 at 3:04 PM"),
	)

	detailsBlock := slack.NewContextBlock(
		"",
		slack.NewTextBlockObject("plain_text", detailsText, true, false),
	)

	blocks := []slack.Block{
		headerBlock,
		statusBlock,
		detailsBlock,
	}

	return slack.NewBlockMessage(blocks...)
}
