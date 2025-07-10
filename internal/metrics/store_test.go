package metrics

import (
	"os"
	"testing"

	"github.com/mauv0809/ideal-tribble/internal/database"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// setupTestDB creates a temporary in-memory SQLite database for testing.
func setupTestDB(t *testing.T) (MetricsStore, func()) {
	t.Helper()

	tmpfile, err := os.CreateTemp("", "testdb_metrics_*.db")
	require.NoError(t, err)

	db, err := database.InitDB(tmpfile.Name(), "", "")
	require.NoError(t, err)

	store := New(db)

	teardown := func() {
		db.Close()
		os.Remove(tmpfile.Name())
	}

	return store, teardown
}

func TestIncrementAndGetAll(t *testing.T) {
	store, teardown := setupTestDB(t)
	defer teardown()

	// 1. Initially, there should be no metrics
	metrics, err := store.GetAll()
	require.NoError(t, err)
	assert.Empty(t, metrics)

	// 2. Increment a new key
	store.Increment("checks_run")
	metrics, err = store.GetAll()
	require.NoError(t, err)
	assert.Equal(t, map[string]int{"checks_run": 1}, metrics)

	// 3. Increment the same key again
	store.Increment("checks_run")
	metrics, err = store.GetAll()
	require.NoError(t, err)
	assert.Equal(t, map[string]int{"checks_run": 2}, metrics)

	// 4. Increment a different key
	store.Increment("slack_notifications_sent")
	metrics, err = store.GetAll()
	require.NoError(t, err)
	assert.Equal(t, map[string]int{
		"checks_run":               2,
		"slack_notifications_sent": 1,
	}, metrics)
}
