package metrics

// MetricsStore defines the interface for metric storage and retrieval.
// This allows for easy mocking in tests and decouples consumers
// from the concrete implementation.
type MetricsStore interface {
	Increment(key string)
	GetAll() (map[string]int, error)
}
