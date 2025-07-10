package slack

import (
	"github.com/slack-go/slack"
)

// SlackClient is a wrapper around the official slack-go client.
type SlackClient struct {
	api       *slack.Client
	channelID string
}

// NotificationType defines the type of slack message to be sent.
type NotificationType int

const (
	BookingNotification NotificationType = iota
	ResultNotification
)
