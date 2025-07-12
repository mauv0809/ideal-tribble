package pubsub

import (
	"context"

	"cloud.google.com/go/pubsub"
	"github.com/charmbracelet/log"
	"github.com/vmihailenco/msgpack/v5"
)

func New(projectID string) PubSubClient {
	ctx := context.Background()
	pubSubC, err := pubsub.NewClient(ctx, projectID)
	if err != nil {
		log.Fatalf("Failed to create client: %v", err)
	}
	teardown := func() {
		pubSubC.Close()
	}

	return &client{
		client:   pubSubC,
		teardown: teardown,
	}

}
func (c *client) SendMessage(topic string, data any) error {
	ctx := context.Background()
	msgpackData, err := msgpack.Marshal(data)
	if err != nil {
		log.Error("MessagePack marshal error", "error", err)
		return err
	}
	message := &pubsub.Message{
		Data: msgpackData,
	}
	result := c.client.Topic(topic).Publish(ctx, message)
	serverID, err := result.Get(ctx)
	if err != nil {
		log.Error("Failed to publish message", "error", err, "topic", topic)
		return err
	}
	log.Info("SendMessage", "serverID", serverID)
	return nil
}

func (c *client) ProcessMessage(data []byte, returnValue any) error {
	// Unmarshal the MessagePack data into the provided pointer struct
	err := msgpack.Unmarshal(data, returnValue)
	if err != nil {
		log.Error("MessagePack unmarshal error", "error", err)
		return err
	}
	return nil
}
