-- +goose Up
-- Add booking count fields to players table
ALTER TABLE players ADD COLUMN booking_count_singles INTEGER DEFAULT 0 NOT NULL;
ALTER TABLE players ADD COLUMN booking_count_doubles INTEGER DEFAULT 0 NOT NULL;

-- +goose Down
-- Remove booking count fields from players table
ALTER TABLE players DROP COLUMN booking_count_singles;
ALTER TABLE players DROP COLUMN booking_count_doubles;