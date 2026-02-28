-- +goose Up
-- Add source column to matches table to distinguish Playtomic vs manual matches
ALTER TABLE matches ADD COLUMN source TEXT NOT NULL DEFAULT 'playtomic';

-- Player aliases table for linking manual players to Playtomic players
CREATE TABLE IF NOT EXISTS player_aliases (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    manual_player_id TEXT NOT NULL UNIQUE,
    manual_player_name TEXT NOT NULL,
    playtomic_player_id TEXT,
    playtomic_player_name TEXT,
    confirmed INTEGER NOT NULL DEFAULT 0,
    confidence REAL,
    created_at INTEGER NOT NULL,
    updated_at INTEGER NOT NULL
);

CREATE INDEX idx_player_aliases_playtomic ON player_aliases(playtomic_player_id);
CREATE INDEX idx_player_aliases_name ON player_aliases(manual_player_name COLLATE NOCASE);
CREATE INDEX idx_matches_source ON matches(source);

-- +goose Down
DROP INDEX IF EXISTS idx_matches_source;
DROP INDEX IF EXISTS idx_player_aliases_name;
DROP INDEX IF EXISTS idx_player_aliases_playtomic;
DROP TABLE IF EXISTS player_aliases;
