package database

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestInitDB_CreatesTables(t *testing.T) {
	// Create a temporary file for the SQLite database
	tmpfile, err := os.CreateTemp("", "testdb_*.sqlite")
	require.NoError(t, err)
	defer os.Remove(tmpfile.Name()) // Clean up the file

	db, err := InitDB(tmpfile.Name(), "", "")
	require.NoError(t, err, "InitDB should not return an error")
	defer db.Close()

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
}
