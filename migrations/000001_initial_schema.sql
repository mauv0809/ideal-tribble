-- +goose Up
PRAGMA foreign_keys = ON;

CREATE TABLE IF NOT EXISTS players (
    id TEXT PRIMARY KEY,
    name TEXT,
    level DOUBLE NOT NULL DEFAULT 0,
    ball_bringer_count INTEGER NOT NULL DEFAULT 0
);

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
);

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
);

CREATE INDEX IF NOT EXISTS idx_matches_processing_game_results
ON matches (processing_status, game_status, results_status);

CREATE INDEX IF NOT EXISTS idx_players_name ON players (name COLLATE NOCASE);

CREATE INDEX IF NOT EXISTS idx_player_stats_rank ON player_stats (matches_won DESC, sets_won DESC, games_won DESC);

CREATE INDEX IF NOT EXISTS idx_players_ball_bringer_rank ON players (ball_bringer_count ASC, name ASC);

CREATE INDEX IF NOT EXISTS idx_players_level ON players (level DESC);

-- +goose Down
-- This is the down migration for 000001_initial_schema.sql
-- SQLite does not support ALTER TABLE DROP COLUMN directly.
-- Reverting this change would typically involve:
-- 1. Creating a new temporary table with the old schema (without booking_notified_ts and result_notified_ts).
-- 2. Copying data from the 'matches' table to the temporary table, excluding the new columns.
-- 3. Dropping the original 'matches' table.
-- 4. Renaming the temporary table to 'matches'.
-- Due to the complexity, a direct 'down' migration is not provided for simple column drops in SQLite.
DROP TABLE IF EXISTS matches;
DROP TABLE IF EXISTS players;
DROP TABLE IF EXISTS player_stats; 