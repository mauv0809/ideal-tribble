-- +goose Up
-- weekly_player_stats stores a snapshot of a player's performance for a given week.
-- This allows for efficient generation of weekly reports and historical trend analysis.
CREATE TABLE IF NOT EXISTS weekly_player_stats (
    -- The start date of the week (e.g., Sunday at 00:00:00) stored as a Unix timestamp.
    -- Part of the composite primary key to ensure one entry per player per week.
    week_start_date INTEGER NOT NULL,
    
    -- Foreign key to the players table.
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

-- An index to quickly query all stats for a specific week, which is the primary use case.
CREATE INDEX IF NOT EXISTS idx_weekly_player_stats_week ON weekly_player_stats (week_start_date DESC);

-- +goose Down
DROP TABLE IF EXISTS weekly_player_stats;