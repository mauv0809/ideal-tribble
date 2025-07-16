package database

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/charmbracelet/log"
	"github.com/pressly/goose/v3" // NEW: Import goose
	"github.com/tursodatabase/go-libsql"
)

// InitDB initializes the database and ensures the schema is up to date.
func InitDB(dbName string, primaryUrl string, authToken string, migrationsDir string) (*sql.DB, func(), error) {
	// For local-only databases, dbName is the filename.
	// For embedded replicas, dbName is the local file, and primaryUrl is the remote.
	// We handle the local-only case separately for clarity.
	if primaryUrl == "" {
		log.Info("Initializing local-only SQLite database", "path", dbName)
		db, err := sql.Open("libsql", "file:"+dbName)
		if err != nil {
			return nil, nil, fmt.Errorf("failed to open local database: %w", err)
		}
		if err = createTables(db, migrationsDir); err != nil {
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
	if err = createTables(db, migrationsDir); err != nil {
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

func createTables(db *sql.DB, migrationsDir string) error {
	// Foreign key support is not enabled by default in SQLite
	_, err := db.Exec("PRAGMA foreign_keys = ON;")
	if err != nil {
		log.Error("Error enabling foreign keys:", "error", err)
		return err
	}

	// Use goose for migrations
	log.Info("Running database migrations with Goose...")

	// Set the database dialect for Goose
	err = goose.SetDialect("sqlite3")
	if err != nil {
		return fmt.Errorf("failed to set goose dialect: %w", err)
	}

	// Apply migrations
	err = goose.Up(db, migrationsDir)
	if err != nil {
		log.Error("Failed to apply migrations with Goose", "error", err)
		return fmt.Errorf("failed to apply migrations with goose: %w", err)
	}

	log.Info("Database migrations applied successfully")
	log.Info("Database initialized successfully")
	return nil
}
