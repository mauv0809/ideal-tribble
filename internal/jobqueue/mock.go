package jobqueue

import (
	"encoding/json"
	"time"
)

// MockJobQueue is a mock implementation of JobQueue for testing
type MockJobQueue struct {
	EnqueueCalls        []EnqueueCall
	EnqueueDelayedCalls []EnqueueDelayedCall
	DequeueCalls        []DequeueCall
	CompleteCalls       []CompleteCall
	FailCalls           []FailCall
	CleanupCalls        []CleanupCall
	
	// Mock responses
	DequeueResponse *Job
	DequeueError    error
	EnqueueError    error
	CompleteError   error
	FailError       error
	CleanupError    error
}

type EnqueueCall struct {
	JobType string
	Payload interface{}
}

type EnqueueDelayedCall struct {
	JobType string
	Payload interface{}
	Delay   time.Duration
}

type DequeueCall struct{}

type CompleteCall struct {
	JobID string
}

type FailCall struct {
	JobID    string
	ErrorMsg string
}

type CleanupCall struct {
	OlderThan time.Duration
}

// NewMock creates a new mock job queue
func NewMock() *MockJobQueue {
	return &MockJobQueue{
		EnqueueCalls:        make([]EnqueueCall, 0),
		EnqueueDelayedCalls: make([]EnqueueDelayedCall, 0),
		DequeueCalls:        make([]DequeueCall, 0),
		CompleteCalls:       make([]CompleteCall, 0),
		FailCalls:           make([]FailCall, 0),
		CleanupCalls:        make([]CleanupCall, 0),
	}
}

func (m *MockJobQueue) Enqueue(jobType string, payload interface{}) error {
	m.EnqueueCalls = append(m.EnqueueCalls, EnqueueCall{
		JobType: jobType,
		Payload: payload,
	})
	return m.EnqueueError
}

func (m *MockJobQueue) EnqueueDelayed(jobType string, payload interface{}, delay time.Duration) error {
	m.EnqueueDelayedCalls = append(m.EnqueueDelayedCalls, EnqueueDelayedCall{
		JobType: jobType,
		Payload: payload,
		Delay:   delay,
	})
	return m.EnqueueError
}

func (m *MockJobQueue) Dequeue() (*Job, error) {
	m.DequeueCalls = append(m.DequeueCalls, DequeueCall{})
	return m.DequeueResponse, m.DequeueError
}

func (m *MockJobQueue) Complete(jobID string) error {
	m.CompleteCalls = append(m.CompleteCalls, CompleteCall{
		JobID: jobID,
	})
	return m.CompleteError
}

func (m *MockJobQueue) Fail(jobID string, errorMsg string) error {
	m.FailCalls = append(m.FailCalls, FailCall{
		JobID:    jobID,
		ErrorMsg: errorMsg,
	})
	return m.FailError
}

func (m *MockJobQueue) Cleanup(olderThan time.Duration) error {
	m.CleanupCalls = append(m.CleanupCalls, CleanupCall{
		OlderThan: olderThan,
	})
	return m.CleanupError
}

// Helper methods for testing

func (m *MockJobQueue) GetLastEnqueuedJob() (*EnqueueCall, bool) {
	if len(m.EnqueueCalls) == 0 {
		return nil, false
	}
	return &m.EnqueueCalls[len(m.EnqueueCalls)-1], true
}

func (m *MockJobQueue) GetEnqueuedJobByType(jobType string) (*EnqueueCall, bool) {
	for _, call := range m.EnqueueCalls {
		if call.JobType == jobType {
			return &call, true
		}
	}
	return nil, false
}

func (m *MockJobQueue) GetEnqueuedJobPayload(jobType string, target interface{}) bool {
	call, found := m.GetEnqueuedJobByType(jobType)
	if !found {
		return false
	}
	
	// Try to marshal/unmarshal to convert the payload
	data, err := json.Marshal(call.Payload)
	if err != nil {
		return false
	}
	
	err = json.Unmarshal(data, target)
	return err == nil
}

func (m *MockJobQueue) Reset() {
	m.EnqueueCalls = make([]EnqueueCall, 0)
	m.EnqueueDelayedCalls = make([]EnqueueDelayedCall, 0)
	m.DequeueCalls = make([]DequeueCall, 0)
	m.CompleteCalls = make([]CompleteCall, 0)
	m.FailCalls = make([]FailCall, 0)
	m.CleanupCalls = make([]CleanupCall, 0)
}