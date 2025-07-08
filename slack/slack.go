package slack

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/charmbracelet/log"
	"github.com/mauv0809/ideal-tribble/playtomic"
)

var (
	slackBotToken  = os.Getenv("SLACK_BOT_TOKEN")
	slackChannelID = os.Getenv("SLACK_CHANNEL_ID")
)

// SendMessage sends a message to the configured Slack channel.
func SendMessage(message string) error {
	payload := map[string]string{
		"channel": slackChannelID,
		"text":    message,
	}

	jsonPayload, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("failed to marshal Slack payload: %w", err)
	}

	req, err := http.NewRequest("POST", "https://slack.com/api/chat.postMessage", bytes.NewBuffer(jsonPayload))
	if err != nil {
		return fmt.Errorf("failed to create Slack request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+slackBotToken)

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to send Slack message: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("received non-OK HTTP status from Slack: %s", resp.Status)
	}

	return nil
}

// SendBookingNotification sends a Slack message for a new match booking.
func SendBookingNotification(match *playtomic.PadelMatch, debug bool) {
	var builder strings.Builder
	builder.WriteString("🎾 *New match booked!* 🎾\n\n")
	builder.WriteString(fmt.Sprintf("*%s* at *%s*\n", match.ResourceName, time.Unix(match.Start, 0).Format("Monday 02 Jan, 15:04")))

	for _, team := range match.Teams {
		for _, player := range team.Players {
			builder.WriteString(fmt.Sprintf("• %s\n", player.Name))
		}
	}

	if match.BallBringerName != "" {
		builder.WriteString(fmt.Sprintf("\n🎾 %s is bringing balls!\n", match.BallBringerName))
	}

	builder.WriteString(fmt.Sprintf("\nPrice: *%s*\n", match.Price))

	if debug {
		log.Info("Debug mode: Not sending Slack message for new booking.")
		return
	}

	if err := SendMessage(builder.String()); err != nil {
		log.Error("Failed to send Slack message", "error", err)
	}
}

// SendResultNotification sends a Slack message for a finished match.
func SendResultNotification(match *playtomic.PadelMatch, debug bool) {
	var builder strings.Builder
	builder.WriteString("🎾 *Match finished!* 🎾\n\n")
	builder.WriteString(fmt.Sprintf("*%s* at *%s*\n", match.ResourceName, time.Unix(match.Start, 0).Format("Monday 02 Jan, 15:04")))

	teamNames := make(map[string]string)
	for _, team := range match.Teams {
		var playerNames []string
		for _, player := range team.Players {
			playerNames = append(playerNames, player.Name)
		}
		teamNames[team.ID] = strings.Join(playerNames, " & ")
	}

	builder.WriteString("\n*Teams:*\n")
	for _, team := range match.Teams {
		builder.WriteString(fmt.Sprintf("• *Team %s*: %s\n", team.ID, teamNames[team.ID]))
	}

	if match.BallBringerName != "" {
		builder.WriteString(fmt.Sprintf("\n🎱 %s brought the balls!\n", match.BallBringerName))
	}

	if len(match.Results) > 0 {
		builder.WriteString("\n*Result:*\n")
		for _, result := range match.Results {
			builder.WriteString(fmt.Sprintf("▪️ *%s*: ", result.Name))
			var scores []string
			for teamID, score := range result.Scores {
				scores = append(scores, fmt.Sprintf("%s: %d", teamNames[teamID], score))
			}
			builder.WriteString(strings.Join(scores, ", ") + "\n")
		}
	}
	if debug {
		log.Info("Debug mode: Not sending Slack message for result.")
		return
	}

	if err := SendMessage(builder.String()); err != nil {
		log.Error("Failed to send Slack message", "error", err)
	}
}
