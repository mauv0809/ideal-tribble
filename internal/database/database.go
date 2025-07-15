package database

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/charmbracelet/log"
	"github.com/tursodatabase/go-libsql"
)

// InitDB initializes the database and ensures the schema is up to date.
func InitDB(dbName string, primaryUrl string, authToken string) (*sql.DB, func(), error) {
	// For local-only databases, dbName is the filename.
	// For embedded replicas, dbName is the local file, and primaryUrl is the remote.
	// We handle the local-only case separately for clarity.
	if primaryUrl == "" {
		log.Info("Initializing local-only SQLite database", "path", dbName)
		db, err := sql.Open("libsql", "file:"+dbName)
		if err != nil {
			return nil, nil, fmt.Errorf("failed to open local database: %w", err)
		}
		if err = createTables(db); err != nil {
			db.Close() // Close on error
			return nil, nil, fmt.Errorf("failed to create tables for local db: %w", err)
		}
		// No connector is returned for a simple local database.
		return db, nil, nil
	}
	//Remote only database
	// log.Info("Initializing Turso database", "url", primaryUrl)
	// db, err := sql.Open("libsql", primaryUrl+"?authToken="+authToken)
	// if err != nil {
	// 	fmt.Fprintf(os.Stderr, "failed to open db %s: %s", primaryUrl, err)
	// 	return nil, fmt.Errorf("failed to open db %s: %w", primaryUrl, err)
	// }
	// Embedded replica
	log.Info("Initializing embeded replica Turso database", "url", primaryUrl)
	dir, err := os.MkdirTemp("./tmp", "libsql-*")
	if err != nil {
		fmt.Println("Error creating temporary directory:", err)
		os.Exit(1)
	}

	dbPath := filepath.Join(dir, dbName)

	connector, err := libsql.NewEmbeddedReplicaConnector(dbPath, primaryUrl,
		libsql.WithAuthToken(authToken),
		libsql.WithSyncInterval(10*time.Second),
	)
	if err != nil {
		defer connector.Close()
		return nil, nil, fmt.Errorf("failed to create connector: %w", err)
	}

	db := sql.OpenDB(connector)
	if err = createTables(db); err != nil {
		db.Close() // Close on error
		if connector != nil {
			connector.Close()
		}
		return nil, nil, fmt.Errorf("failed to create tables for local db: %w", err)
	}
	teardown := func() {
		db.Close()
		if connector != nil {
			connector.Close()
		}
		os.RemoveAll(dir)

	}
	return db, teardown, nil

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
        teams_blob BLOB,
        results_blob BLOB,
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

	_, err = db.Exec(createPlayersTable)
	if err != nil {
		return err
	}

	_, err = db.Exec(createMatchesTable)
	if err != nil {
		return err
	}

	// Create indexes for efficient querying
	createMatchesProcessingIndex := `
	CREATE INDEX IF NOT EXISTS idx_matches_processing_game_results
	ON matches (processing_status, game_status, results_status);
	`
	_, err = db.Exec(createMatchesProcessingIndex)
	if err != nil {
		return err
	}

	createPlayersNameIndex := `
	CREATE INDEX IF NOT EXISTS idx_players_name ON players (name COLLATE NOCASE);
	`
	_, err = db.Exec(createPlayersNameIndex)
	if err != nil {
		return err
	}

	createPlayerStatsRankIndex := `
	CREATE INDEX IF NOT EXISTS idx_player_stats_rank ON player_stats (matches_won DESC, sets_won DESC, games_won DESC);
	`
	_, err = db.Exec(createPlayerStatsRankIndex)
	if err != nil {
		return err
	}

	createPlayersBallBringerRankIndex := `
	CREATE INDEX IF NOT EXISTS idx_players_ball_bringer_rank ON players (ball_bringer_count ASC, name ASC);
	`
	_, err = db.Exec(createPlayersBallBringerRankIndex)
	if err != nil {
		return err
	}

	createPlayersLevelIndex := `
	CREATE INDEX IF NOT EXISTS idx_players_level ON players (level DESC);
	`
	_, err = db.Exec(createPlayersLevelIndex)
	if err != nil {
		return err
	}

	_, err = db.Exec(createPlayerStatsTable)
	if err != nil {
		return err
	}
	log.Info("Database initialized successfully")
	return nil
}
