package metrics

import (
	"database/sql"
	"sync"

	"github.com/charmbracelet/log"
)

// store handles metric-related database operations.
type store struct {
	db *sql.DB
	mu sync.Mutex
}

// New creates a new metrics Store.
func New(db *sql.DB) MetricsStore {
	return &store{
		db: db,
	}
}

// Increment upserts a metric key and increments its value by one.
func (s *store) Increment(key string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	stmt, err := s.db.Prepare(`
		INSERT INTO metrics (key, value) VALUES (?, 1)
		ON CONFLICT(key) DO UPDATE SET value = value + 1;
	`)
	if err != nil {
		log.Error("Failed to prepare statement for metric increment", "error", err, "key", key)
		return
	}
	defer stmt.Close()

	_, err = stmt.Exec(key)
	if err != nil {
		log.Error("Failed to execute statement for metric increment", "error", err, "key", key)
	} else {
		log.Debug("Incremented metric", "key", key)
	}
}

// GetAll returns all metrics from the database.
func (s *store) GetAll() (map[string]int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	rows, err := s.db.Query("SELECT key, value FROM metrics")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	metrics := make(map[string]int)
	for rows.Next() {
		var key string
		var value int
		if err := rows.Scan(&key, &value); err != nil {
			return nil, err
		}
		metrics[key] = value
	}
	return metrics, nil
}
