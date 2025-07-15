package pubsub

import (
	"sync"
)

// MockPubSubClient is a mock implementation of PubSubClient for testing.
// It is safe for concurrent use.
type MockPubSubClient struct {
	mu sync.Mutex

	// Spies for method calls
	SendMessageFunc    func(topic EventType, data any) error
	ProcessMessageFunc func(data []byte, returnValue any) error

	// Call records
	SendMessageCalls    []SendMessageCall
	ProcessMessageCalls []ProcessMessageCall
}

// SendMessageCall holds the arguments for a call to SendMessage.
type SendMessageCall struct {
	Topic string
	Data  any
}

// ProcessMessageCall holds the arguments for a call to ProcessMessage.
type ProcessMessageCall struct {
	Data        []byte
	ReturnValue any
}

// NewMock creates a new mock PubSubClient. The projectID is ignored.
func NewMock(projectID string) *MockPubSubClient {
	return &MockPubSubClient{}
}

// Reset clears all call records.
func (m *MockPubSubClient) Reset() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.SendMessageCalls = nil
	m.ProcessMessageCalls = nil
}

// SendMessage records the call and executes the mock function if provided.
func (m *MockPubSubClient) SendMessage(topic EventType, data any) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.SendMessageCalls = append(m.SendMessageCalls, SendMessageCall{Topic: string(topic), Data: data})
	if m.SendMessageFunc != nil {
		return m.SendMessageFunc(topic, data)
	}
	return nil
}

// ProcessMessage records the call and executes the mock function if provided.
func (m *MockPubSubClient) ProcessMessage(data []byte, returnValue any) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.ProcessMessageCalls = append(m.ProcessMessageCalls, ProcessMessageCall{Data: data, ReturnValue: returnValue})
	if m.ProcessMessageFunc != nil {
		return m.ProcessMessageFunc(data, returnValue)
	}
	return nil
}
