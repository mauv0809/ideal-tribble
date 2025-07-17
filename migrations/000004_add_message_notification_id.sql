-- +goose Up
ALTER TABLE matches ADD COLUMN booking_message_id TEXT;
ALTER TABLE matches ADD COLUMN result_message_id TEXT;

-- +goose Down
-- This is the down migration for 000002_add_match_notification_timestamps.up.sql
-- SQLite does not support ALTER TABLE DROP COLUMN directly.
-- Reverting this change would typically involve recreating the table without these columns.
-- Due to the complexity, a direct 'down' migration is not provided for simple column drops in SQLite. 