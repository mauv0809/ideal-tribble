package pubsub

type PubSubClient interface {
	SendMessage(topic EventType, data any) error
	ProcessMessage(data []byte, returnValue any) error
}
