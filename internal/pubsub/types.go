package pubsub

import "cloud.google.com/go/pubsub"

type client struct {
	client   *pubsub.Client
	teardown func()
}
