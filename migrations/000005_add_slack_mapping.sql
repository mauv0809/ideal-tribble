-- +goose Up
-- Add Slack mapping fields to existing players table
ALTER TABLE players ADD COLUMN slack_user_id TEXT;
ALTER TABLE players ADD COLUMN slack_username TEXT;
ALTER TABLE players ADD COLUMN slack_display_name TEXT;
ALTER TABLE players ADD COLUMN mapping_status TEXT DEFAULT 'PENDING'; -- PENDING, CONFIRMED, AUTO_MATCHED
ALTER TABLE players ADD COLUMN mapping_confidence REAL DEFAULT 0.0;
ALTER TABLE players ADD COLUMN mapping_updated_at INTEGER;

-- Indexes for efficient Slack lookups
CREATE INDEX IF NOT EXISTS idx_players_slack_user_id ON players (slack_user_id);
CREATE INDEX IF NOT EXISTS idx_players_mapping_status ON players (mapping_status);
CREATE INDEX IF NOT EXISTS idx_players_slack_username ON players (slack_username COLLATE NOCASE);

-- +goose Down
-- Remove Slack mapping fields (SQLite doesn't support DROP COLUMN directly)
-- This would require creating a new table without these columns and copying data
-- For simplicity, we'll just note that a full down migration would be complex
CREATE TABLE players_backup AS SELECT id, name, level, ball_bringer_count FROM players;
DROP TABLE players;
CREATE TABLE players (
    id TEXT PRIMARY KEY,
    name TEXT,
    level DOUBLE NOT NULL DEFAULT 0,
    ball_bringer_count INTEGER NOT NULL DEFAULT 0
);
INSERT INTO players (id, name, level, ball_bringer_count) SELECT id, name, level, ball_bringer_count FROM players_backup;
DROP TABLE players_backup;