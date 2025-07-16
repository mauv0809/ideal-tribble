package pubsub

import "cloud.google.com/go/pubsub"

type client struct {
	client   *pubsub.Client
	teardown func()
}

// EventType represents the type of event/message sent via pubsub.
type EventType string

const (
	EventAssignBallBoy     EventType = "assign_ball_boy"
	EventUpdatePlayerStats EventType = "update_player_stats"
	EventNotifyBooking     EventType = "notify_booking"
	EventNotifyResult      EventType = "notify_result"
)
