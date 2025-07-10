package slack

import (
	"errors"

	"github.com/charmbracelet/log"
	"github.com/mauv0809/ideal-tribble/internal/metrics"
	"github.com/mauv0809/ideal-tribble/internal/playtomic"
	"github.com/slack-go/slack"
)

// NewClient creates a new Slack client wrapper.
func NewClient(token, channelID string) *SlackClient {
	api := slack.New(token)
	return &SlackClient{
		api:       api,
		channelID: channelID,
	}
}

// NewClientWithAPI creates a new Slack client with a custom API client. Used for testing.
func NewClientWithAPI(api *slack.Client, channelID string) *SlackClient {
	return &SlackClient{
		api:       api,
		channelID: channelID,
	}
}

// SendNotification formats and sends a message to slack based on the notification type.
func (c *SlackClient) SendNotification(match *playtomic.PadelMatch, notificationType NotificationType, metrics metrics.MetricsStore, dryRun bool) (string, string, error) {
	if c.api == nil || c.channelID == "" {
		log.Warn("Slack client or channel ID is not configured. Skipping notification.")
		return "", "", errors.New("slack client or channel ID is not configured")
	}

	var msg slack.Message
	switch notificationType {
	case BookingNotification:
		msg = c.FormatBookingNotification(match)
	case ResultNotification:
		msg = c.FormatResultNotification(match)
	default:
		log.Error("Unknown notification type provided", "type", notificationType)
		return "", "", errors.New("unknown notification type provided")
	}
	if dryRun {
		log.Info("Dry run mode: Slack notification not sent.", "matchID", match.MatchID, "notificationType", notificationType, "msg", msg)
		return "", "", nil
	}

	responeChannel, message_ts, err := c.api.PostMessage(c.channelID, slack.MsgOptionBlocks(msg.Blocks.BlockSet...))
	if err != nil {
		log.Error("Failed to send Slack message", "error", err, "matchID", match.MatchID)
		return "", "", err
	}
	if metrics != nil {
		metrics.Increment("slack_notifications_sent")
	}
	return responeChannel, message_ts, nil
}
func (c *SlackClient) SendMessage(message slack.Message, metrics metrics.MetricsStore, dryRun bool) (string, string, error) {
	if c.api == nil || c.channelID == "" {
		log.Warn("Slack client or channel ID is not configured. Skipping notification.")
		return "", "", errors.New("slack client or channel ID is not configured")
	}

	if dryRun {
		log.Info("Dry run mode: Slack notification not sent.", "msg", message)
		return "", "", nil
	}

	responeChannel, message_ts, err := c.api.PostMessage(c.channelID, slack.MsgOptionBlocks(message.Blocks.BlockSet...))
	if err != nil {
		log.Error("Failed to send Slack message", "error", err)
		return "", "", err
	}
	if metrics != nil {
		metrics.Increment("slack_notifications_sent")
	}
	return responeChannel, message_ts, nil
}
