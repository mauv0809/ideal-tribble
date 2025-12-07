-- +goose Up

-- Add individual set scores to pairing_matches for situational analytics
-- (e.g., performance after winning/losing first set)
ALTER TABLE pairing_matches ADD COLUMN set1_pairing_score INTEGER;
ALTER TABLE pairing_matches ADD COLUMN set1_opponent_score INTEGER;
ALTER TABLE pairing_matches ADD COLUMN set2_pairing_score INTEGER;
ALTER TABLE pairing_matches ADD COLUMN set2_opponent_score INTEGER;
ALTER TABLE pairing_matches ADD COLUMN set3_pairing_score INTEGER;
ALTER TABLE pairing_matches ADD COLUMN set3_opponent_score INTEGER;

-- +goose Down
-- SQLite doesn't support DROP COLUMN directly, but goose handles this
-- For rollback, we'd need to recreate the table without these columns
-- This is a one-way migration for simplicity
