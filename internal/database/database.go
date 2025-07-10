package database

import (
	"database/sql"
	"fmt"
	"os"

	"github.com/charmbracelet/log"
	_ "github.com/mattn/go-sqlite3"
	_ "github.com/tursodatabase/libsql-client-go/libsql"
)

// InitDB initializes the database and ensures the schema is up to date.
func InitDB(dbPath string, primaryUrl string, authToken string) (*sql.DB, error) {
	// For local-only databases, dbPath is the filename.
	// For embedded replicas, dbPath is the local file, and primaryUrl is the remote.
	// We handle the local-only case separately for clarity.
	if primaryUrl == "" {
		log.Info("Initializing local-only SQLite database", "path", dbPath)
		db, err := sql.Open("libsql", "file:"+dbPath)
		if err != nil {
			return nil, fmt.Errorf("failed to open local database: %w", err)
		}
		if err = createTables(db); err != nil {
			db.Close() // Close on error
			return nil, fmt.Errorf("failed to create tables for local db: %w", err)
		}
		// No connector is returned for a simple local database.
		return db, nil
	}
	log.Info("Initializing Turso database", "url", primaryUrl)
	db, err := sql.Open("libsql", primaryUrl+"?authToken="+authToken)
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to open db %s: %s", primaryUrl, err)
		return nil, fmt.Errorf("failed to open db %s: %w", primaryUrl, err)
	}
	if err = createTables(db); err != nil {
		db.Close() // Close on error
		return nil, fmt.Errorf("failed to create tables for local db: %w", err)
	}
	return db, nil

}

func createTables(db *sql.DB) error {
	// Foreign key support is not enabled by default in SQLite
	_, err := db.Exec("PRAGMA foreign_keys = ON;")
	if err != nil {
		log.Error("Error enabling foreign keys:", "error", err)
		return err
	}

	createPlayersTable := `
    CREATE TABLE IF NOT EXISTS players (
        id TEXT PRIMARY KEY,
        name TEXT,
        level DOUBLE NOT NULL DEFAULT 0,
        ball_bringer_count INTEGER NOT NULL DEFAULT 0
    );`

	createMatchesTable := `
    CREATE TABLE IF NOT EXISTS matches (
        id TEXT PRIMARY KEY,
        owner_id TEXT NOT NULL,
        owner_name TEXT NOT NULL,
        start_time INTEGER NOT NULL,
        end_time INTEGER NOT NULL,
        created_at INTEGER NOT NULL,
        status TEXT NOT NULL,
        game_status TEXT NOT NULL,
        results_status TEXT NOT NULL,
        resource_name TEXT NOT NULL,
        access_code TEXT,
        price TEXT,
        tenant_id TEXT NOT NULL,
        tenant_name TEXT NOT NULL,
        processing_status TEXT NOT NULL DEFAULT 'NEW',
		match_type TEXT NOT NULL,
        teams_json TEXT,
        results_json TEXT,
        ball_bringer_id TEXT,
        ball_bringer_name TEXT,
        FOREIGN KEY (owner_id) REFERENCES players(id),
        FOREIGN KEY (ball_bringer_id) REFERENCES players(id) ON DELETE SET NULL
    );`

	createPlayerStatsTable := `
	CREATE TABLE IF NOT EXISTS player_stats (
		player_id TEXT PRIMARY KEY,
		matches_played INTEGER NOT NULL DEFAULT 0,
		matches_won INTEGER NOT NULL DEFAULT 0,
		matches_lost INTEGER NOT NULL DEFAULT 0,
		sets_won INTEGER NOT NULL DEFAULT 0,
		sets_lost INTEGER NOT NULL DEFAULT 0,
		games_won INTEGER NOT NULL DEFAULT 0,
		games_lost INTEGER NOT NULL DEFAULT 0,
		FOREIGN KEY (player_id) REFERENCES players(id) ON DELETE CASCADE
	);`

	createMetricsTable := `
	CREATE TABLE IF NOT EXISTS metrics (
		key TEXT PRIMARY KEY,
		value INTEGER NOT NULL DEFAULT 0
	);`

	_, err = db.Exec(createPlayersTable)
	if err != nil {
		return err
	}

	_, err = db.Exec(createMatchesTable)
	if err != nil {
		return err
	}
	_, err = db.Exec(createPlayerStatsTable)
	if err != nil {
		return err
	}
	_, err = db.Exec(createMetricsTable)
	if err != nil {
		return err
	}
	log.Info("Database initialized successfully")
	return nil
}
