package pubsub

import "cloud.google.com/go/pubsub"

type client struct {
	client   *pubsub.Client
	teardown func()
}

// EventType represents the type of event/message sent via pubsub.
type EventType string

const (
	EventAssignBallBoy     EventType = "assign-ball-boy"
	EventUpdatePlayerStats EventType = "update-player-stats"
	EventNotifyBooking     EventType = "notify-booking"
	EventNotifyResult      EventType = "notify-result"
)
