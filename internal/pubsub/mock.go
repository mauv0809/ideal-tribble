package pubsub

import (
	"sync"
)

// MockPubSubClient is a mock implementation of PubSubClient for testing.
type MockPubSubClient struct {
	ProjectID        string
	SendMessageCalls []struct {
		Topic string // Changed to string to avoid type comparison issues
		Data  any
	}
	ProcessMessageFunc func(data []byte, returnValue any) error // Mock function for ProcessMessage
	mu                 sync.Mutex                               // Mutex to protect SendMessageCalls
}

// NewMock creates a new MockPubSubClient.
func NewMock(projectID string) *MockPubSubClient {
	return &MockPubSubClient{
		ProjectID: projectID,
	}
}

// SendMessage records the sent message for assertion and returns an error if ReceiveMessageFunc is set.
func (m *MockPubSubClient) SendMessage(topic EventType, data any) error {
	m.mu.Lock() // Protect SendMessageCalls from concurrent writes
	defer m.mu.Unlock()

	m.SendMessageCalls = append(m.SendMessageCalls, struct {
		Topic string
		Data  any
	}{Topic: string(topic), Data: data}) // Cast topic to string when storing
	return nil
}

// ProcessMessage is a mock implementation for processing messages.
func (m *MockPubSubClient) ProcessMessage(data []byte, returnValue any) error {
	if m.ProcessMessageFunc != nil {
		return m.ProcessMessageFunc(data, returnValue)
	}
	return nil // Default to no-op for ProcessMessage
}
