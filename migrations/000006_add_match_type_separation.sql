-- +goose Up
-- Add match_type_enum column to distinguish between singles and doubles matches
-- Match type will be determined by the application based on team size during processing
ALTER TABLE matches ADD COLUMN match_type_enum TEXT CHECK (match_type_enum IN ('SINGLES', 'DOUBLES'));

-- Create separate player stats tables for singles and doubles
CREATE TABLE IF NOT EXISTS player_stats_singles (
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

CREATE TABLE IF NOT EXISTS player_stats_doubles (
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

-- Add separate ball bringer counts for each match type
ALTER TABLE players ADD COLUMN ball_bringer_count_singles INTEGER NOT NULL DEFAULT 0;
ALTER TABLE players ADD COLUMN ball_bringer_count_doubles INTEGER NOT NULL DEFAULT 0;

-- Drop old coloumn
ALTER TABLE players DROP COLUMN ball_bringer_count;
-- Create indexes for performance
CREATE INDEX IF NOT EXISTS idx_matches_match_type_enum ON matches (match_type_enum);
CREATE INDEX IF NOT EXISTS idx_player_stats_singles_rank ON player_stats_singles (matches_won DESC, sets_won DESC, games_won DESC);
CREATE INDEX IF NOT EXISTS idx_player_stats_doubles_rank ON player_stats_doubles (matches_won DESC, sets_won DESC, games_won DESC);
CREATE INDEX IF NOT EXISTS idx_players_ball_bringer_singles ON players (ball_bringer_count_singles ASC, name ASC);
CREATE INDEX IF NOT EXISTS idx_players_ball_bringer_doubles ON players (ball_bringer_count_doubles ASC, name ASC);

-- Update weekly_player_stats to include match_type
-- SQLite doesn't support modifying primary keys directly, so we need to recreate the table
CREATE TABLE IF NOT EXISTS weekly_player_stats_new (
    -- The start date of the week (e.g., Sunday at 00:00:00) stored as a Unix timestamp.
    -- Part of the composite primary key to ensure one entry per player per week per match type.
    week_start_date INTEGER NOT NULL,
    
    -- Foreign key to the players table.
    player_id TEXT NOT NULL,
    
    -- Match type (SINGLES or DOUBLES)
    match_type_enum TEXT NOT NULL CHECK (match_type_enum IN ('SINGLES', 'DOUBLES')),

    matches_played INTEGER NOT NULL DEFAULT 0,
    matches_won INTEGER NOT NULL DEFAULT 0,
    matches_lost INTEGER NOT NULL DEFAULT 0,
    sets_won INTEGER NOT NULL DEFAULT 0,
    sets_lost INTEGER NOT NULL DEFAULT 0,
    games_won INTEGER NOT NULL DEFAULT 0,
    games_lost INTEGER NOT NULL DEFAULT 0,

    PRIMARY KEY (week_start_date, player_id, match_type_enum),
    FOREIGN KEY (player_id) REFERENCES players(id) ON DELETE CASCADE
);

-- Drop old table and rename new one (we'll start fresh with weekly stats)
DROP TABLE weekly_player_stats;
ALTER TABLE weekly_player_stats_new RENAME TO weekly_player_stats;

-- Recreate the index with the new structure
CREATE INDEX IF NOT EXISTS idx_weekly_player_stats_week_type ON weekly_player_stats (week_start_date DESC, match_type_enum);

-- Note: After this migration, run the fetch endpoint with a large days parameter 
-- to re-import historical matches and populate the new statistics tables.

-- +goose Down
-- Remove the new columns and tables
DROP TABLE IF EXISTS player_stats_singles;
DROP TABLE IF EXISTS player_stats_doubles;

-- Recreate original weekly_player_stats table
CREATE TABLE IF NOT EXISTS weekly_player_stats_original (
    week_start_date INTEGER NOT NULL,
    player_id TEXT NOT NULL,
    matches_played INTEGER NOT NULL DEFAULT 0,
    matches_won INTEGER NOT NULL DEFAULT 0,
    matches_lost INTEGER NOT NULL DEFAULT 0,
    sets_won INTEGER NOT NULL DEFAULT 0,
    sets_lost INTEGER NOT NULL DEFAULT 0,
    games_won INTEGER NOT NULL DEFAULT 0,
    games_lost INTEGER NOT NULL DEFAULT 0,
    PRIMARY KEY (week_start_date, player_id),
    FOREIGN KEY (player_id) REFERENCES players(id) ON DELETE CASCADE
);

-- Copy back doubles data only (if any exists)
INSERT INTO weekly_player_stats_original (
    week_start_date, player_id, matches_played, matches_won, matches_lost, 
    sets_won, sets_lost, games_won, games_lost
)
SELECT 
    week_start_date, player_id, matches_played, matches_won, matches_lost,
    sets_won, sets_lost, games_won, games_lost
FROM weekly_player_stats
WHERE match_type_enum = 'DOUBLES';

-- Drop new table and rename original
DROP TABLE weekly_player_stats;
ALTER TABLE weekly_player_stats_original RENAME TO weekly_player_stats;

-- Recreate original index
CREATE INDEX IF NOT EXISTS idx_weekly_player_stats_week ON weekly_player_stats (week_start_date DESC);

-- Remove the new columns (handled in a separate script if needed)
-- SQLite doesn't support dropping columns directly in older versions