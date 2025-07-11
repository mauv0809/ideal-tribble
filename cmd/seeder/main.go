package main

import (
	"database/sql"
	"fmt"
	"math/rand"
	"os"
	"strings"
	"time"

	"github.com/charmbracelet/log"
	"github.com/google/uuid"
	"github.com/joho/godotenv"
	"github.com/mauv0809/ideal-tribble/internal/playtomic"
	_ "github.com/tursodatabase/go-libsql"
	"github.com/vmihailenco/msgpack/v5"
)

// Simplified config loading for the script
func loadConfig() map[string]string {
	err := godotenv.Load()
	if err != nil {
		log.Warn("No .env file found, reading from environment variables")
	}

	config := make(map[string]string)
	required := []string{"TURSO_PRIMARY_URL", "TURSO_AUTH_TOKEN"}

	for _, key := range required {
		if value, ok := os.LookupEnv(key); ok {
			config[key] = value
		} else {
			log.Fatalf("Error: Required environment variable %s is not set.", key)
		}
	}
	return config
}

func main() {
	log.Info("Starting database seeder...")
	cfg := loadConfig()

	// Connect directly to the primary database
	dbURL := fmt.Sprintf("%s?authToken=%s", cfg["TURSO_PRIMARY_URL"], cfg["TURSO_AUTH_TOKEN"])
	db, err := sql.Open("libsql", dbURL)
	if err != nil {
		log.Fatalf("Failed to open primary database: %s", err)
	}
	defer db.Close()

	if err := db.Ping(); err != nil {
		log.Fatalf("Failed to connect to primary database: %s", err)
	}

	log.Info("Successfully connected to the primary database.")

	// Create 4 dummy players to use in matches
	dummyPlayers := []playtomic.Player{
		{UserID: "player-1", Name: "Seeder Player A"},
		{UserID: "player-2", Name: "Seeder Player B"},
		{UserID: "player-3", Name: "Seeder Player C"},
		{UserID: "player-4", Name: "Seeder Player D"},
	}

	for _, p := range dummyPlayers {
		_, err := db.Exec("INSERT OR IGNORE INTO players (id, name, level) VALUES (?, ?, ?)", p.UserID, p.Name, 4.0)
		if err != nil {
			log.Fatalf("Failed to insert dummy player %s: %s", p.Name, err)
		}
	}
	log.Info("Ensured dummy players exist.")

	const batchSize = 100 // Insert 100 matches at a time
	const numMatches = 10000

	log.Info("Preparing to insert dummy matches...", "total", numMatches, "batch_size", batchSize)
	startTime := time.Now()

	tx, err := db.Begin()
	if err != nil {
		log.Fatalf("Failed to begin transaction: %s", err)
	}

	valueStrings := make([]string, 0, batchSize)
	valueArgs := make([]interface{}, 0, batchSize*20) // 20 columns per match

	for i := 0; i < numMatches; i++ {
		matchTime := time.Now().Add(-time.Duration(rand.Intn(365*24)) * time.Hour)
		teams := []playtomic.Team{
			{ID: "t1", Players: []playtomic.Player{dummyPlayers[0], dummyPlayers[1]}},
			{ID: "t2", Players: []playtomic.Player{dummyPlayers[2], dummyPlayers[3]}},
		}
		teamsBlob, _ := msgpack.Marshal(teams)
		resultsBlob, _ := msgpack.Marshal([]playtomic.SetResult{})

		valueStrings = append(valueStrings, "(?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)")
		valueArgs = append(valueArgs,
			uuid.NewString(),
			dummyPlayers[0].UserID,
			dummyPlayers[0].Name,
			matchTime.Unix(),
			matchTime.Add(90*time.Minute).Unix(),
			matchTime.Add(-24*time.Hour).Unix(),
			"CONFIRMED",
			"ENDED",
			"ENDED",
			"Seeded Court",
			"",
			"10 EUR",
			"tenant-id-placeholder",
			"Seeded Tenant",
			"PROCESSED",
			playtomic.MatchTypePractice,
			teamsBlob,
			resultsBlob,
			nil, // ball_bringer_id
			nil, // ball_bringer_name
		)

		if (i+1)%batchSize == 0 || (i+1) == numMatches {
			stmt := fmt.Sprintf(`
				INSERT INTO matches (id, owner_id, owner_name, start_time, end_time, created_at, status, 
					game_status, results_status, resource_name, access_code, price, tenant_id, tenant_name, 
					processing_status, match_type, teams_blob, results_blob, ball_bringer_id, ball_bringer_name)
				VALUES %s;`, strings.Join(valueStrings, ","))

			_, err := tx.Exec(stmt, valueArgs...)
			if err != nil {
				tx.Rollback()
				log.Fatalf("Failed to execute batch insert: %s", err)
			}

			// Reset for the next batch
			valueStrings = make([]string, 0, batchSize)
			valueArgs = make([]interface{}, 0, batchSize*20)
			log.Info("Inserted batch", "completed", i+1, "total", numMatches)
		}
	}

	if err := tx.Commit(); err != nil {
		log.Fatalf("Failed to commit transaction: %s", err)
	}

	duration := time.Since(startTime)
	log.Info("Successfully inserted all dummy matches.", "duration", duration)
}
