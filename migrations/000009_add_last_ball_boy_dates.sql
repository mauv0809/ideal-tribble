-- +goose Up
-- Add columns to track the last time each player was assigned as ball boy
-- NULL values will sort first in ascending order, making new players eligible immediately
-- but the tiebreaker (ball_bringer_count) will ensure fairness

ALTER TABLE players ADD COLUMN last_ball_boy_date_singles INTEGER;
ALTER TABLE players ADD COLUMN last_ball_boy_date_doubles INTEGER;

-- Backfill dates from historical match data
-- This finds the most recent match where each player was ball boy
-- Only updates NULL dates, so it's safe even if migration is re-run
-- Players with no ball boy history remain NULL (correct - they've never been ball boy)
UPDATE players
SET last_ball_boy_date_singles = (
    SELECT start_time
    FROM matches
    WHERE ball_bringer_id = players.id
    AND match_type_enum = 'SINGLES'
    ORDER BY start_time DESC
    LIMIT 1
)
WHERE last_ball_boy_date_singles IS NULL
AND EXISTS (
    SELECT 1
    FROM matches
    WHERE ball_bringer_id = players.id
    AND match_type_enum = 'SINGLES'
);

UPDATE players
SET last_ball_boy_date_doubles = (
    SELECT start_time
    FROM matches
    WHERE ball_bringer_id = players.id
    AND match_type_enum = 'DOUBLES'
    ORDER BY start_time DESC
    LIMIT 1
)
WHERE last_ball_boy_date_doubles IS NULL
AND EXISTS (
    SELECT 1
    FROM matches
    WHERE ball_bringer_id = players.id
    AND match_type_enum = 'DOUBLES'
);

-- Create indexes for efficient ball boy selection queries
-- These indexes support the ORDER BY clause in AssignBallBringerAtomically
CREATE INDEX IF NOT EXISTS idx_players_ball_boy_selection_singles
ON players (last_ball_boy_date_singles ASC, ball_bringer_count_singles ASC, name ASC);

CREATE INDEX IF NOT EXISTS idx_players_ball_boy_selection_doubles
ON players (last_ball_boy_date_doubles ASC, ball_bringer_count_doubles ASC, name ASC);

-- +goose Down
-- Remove the indexes
DROP INDEX IF EXISTS idx_players_ball_boy_selection_singles;
DROP INDEX IF EXISTS idx_players_ball_boy_selection_doubles;

-- SQLite doesn't support DROP COLUMN directly in older versions
-- In production, you might need to recreate the table without these columns
-- For now, we'll just comment that these columns would need manual removal:
-- ALTER TABLE players DROP COLUMN last_ball_boy_date_singles;
-- ALTER TABLE players DROP COLUMN last_ball_boy_date_doubles;
