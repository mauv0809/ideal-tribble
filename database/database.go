package database

import (
	"database/sql"
	"fmt"

	_ "github.com/mattn/go-sqlite3"
)

// InitDB initializes the database and ensures the schema is up to date.
func InitDB(dataSourceName string) (*sql.DB, error) {
	db, err := sql.Open("sqlite3", dataSourceName)
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}
	if err = db.Ping(); err != nil {
		return nil, fmt.Errorf("failed to connect to database: %w", err)
	}

	if err = createTables(db); err != nil {
		return nil, fmt.Errorf("failed to create tables: %w", err)
	}

	return db, nil
}

func createTables(db *sql.DB) error {
	// Foreign key support is not enabled by default in SQLite
	_, err := db.Exec("PRAGMA foreign_keys = ON;")
	if err != nil {
		return err
	}

	createPlayersTable := `
    CREATE TABLE IF NOT EXISTS players (
        id TEXT PRIMARY KEY,
        name TEXT,
        is_initial INTEGER NOT NULL,
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
        booking_notified INTEGER NOT NULL,
        result_notified INTEGER NOT NULL,
        teams_json TEXT,
        results_json TEXT,
        ball_bringer_id TEXT,
        ball_bringer_name TEXT
    );`

	_, err = db.Exec(createPlayersTable)
	if err != nil {
		return err
	}

	_, err = db.Exec(createMatchesTable)
	return err
}
