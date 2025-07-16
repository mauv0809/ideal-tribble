package database

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestInitDB_CreatesTables(t *testing.T) {

	db, teardown, err := InitDB(":memory:", "", "", "../../migrations")
	require.NoError(t, err, "InitDB should not return an error")
	if teardown != nil {
		defer teardown()
	} else {
		defer db.Close()
	}

	// Check if the 'players' table was created
	var playersTableName string
	err = db.QueryRow("SELECT name FROM sqlite_master WHERE type='table' AND name='players'").Scan(&playersTableName)
	require.NoError(t, err, "Querying for players table should not produce an error")
	assert.Equal(t, "players", playersTableName, "The 'players' table should be created")

	// Check if the 'matches' table was created
	var matchesTableName string
	err = db.QueryRow("SELECT name FROM sqlite_master WHERE type='table' AND name='matches'").Scan(&matchesTableName)
	require.NoError(t, err, "Querying for matches table should not produce an error")
	assert.Equal(t, "matches", matchesTableName, "The 'matches' table should be created")

	// Check if the 'players_stats' table was created
	var playersStatsTableName string
	err = db.QueryRow("SELECT name FROM sqlite_master WHERE type='table' AND name='player_stats'").Scan(&playersStatsTableName)
	require.NoError(t, err, "Querying for players_stats table should not produce an error")
	assert.Equal(t, "player_stats", playersStatsTableName, "The 'players_stats' table should be created")
}
