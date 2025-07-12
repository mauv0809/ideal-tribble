package pubsub

type PubSubClient interface {
	SendMessage(topic string, data any) error
	ProcessMessage(data []byte, returnValue any) error
}
