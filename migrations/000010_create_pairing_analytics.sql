-- +goose Up

-- Tracked pairings to monitor
CREATE TABLE IF NOT EXISTS tracked_pairings (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    player1_id TEXT NOT NULL,
    player1_name TEXT NOT NULL,
    player2_id TEXT NOT NULL,
    player2_name TEXT NOT NULL,
    created_at INTEGER NOT NULL,
    active INTEGER NOT NULL DEFAULT 1,
    UNIQUE(player1_id, player2_id)
);

-- Matches involving tracked pairings
CREATE TABLE IF NOT EXISTS pairing_matches (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    pairing_id INTEGER NOT NULL,
    match_id TEXT NOT NULL,
    match_date INTEGER NOT NULL,
    day_of_week INTEGER NOT NULL,
    hour_of_day INTEGER NOT NULL,
    opponent1_id TEXT,
    opponent1_name TEXT,
    opponent2_id TEXT,
    opponent2_name TEXT,
    pairing_won INTEGER NOT NULL,
    sets_won INTEGER NOT NULL DEFAULT 0,
    sets_lost INTEGER NOT NULL DEFAULT 0,
    games_won INTEGER NOT NULL DEFAULT 0,
    games_lost INTEGER NOT NULL DEFAULT 0,
    tenant_id TEXT,
    tenant_name TEXT,
    FOREIGN KEY (pairing_id) REFERENCES tracked_pairings(id) ON DELETE CASCADE,
    UNIQUE(pairing_id, match_id)
);

-- Indexes for analytics queries
CREATE INDEX IF NOT EXISTS idx_pairing_matches_pairing ON pairing_matches(pairing_id);
CREATE INDEX IF NOT EXISTS idx_pairing_matches_opponents ON pairing_matches(opponent1_id, opponent2_id);
CREATE INDEX IF NOT EXISTS idx_pairing_matches_time ON pairing_matches(day_of_week, hour_of_day);
CREATE INDEX IF NOT EXISTS idx_pairing_matches_date ON pairing_matches(match_date DESC);
CREATE INDEX IF NOT EXISTS idx_tracked_pairings_active ON tracked_pairings(active);
CREATE INDEX IF NOT EXISTS idx_tracked_pairings_players ON tracked_pairings(player1_id, player2_id);

-- +goose Down
DROP INDEX IF EXISTS idx_tracked_pairings_players;
DROP INDEX IF EXISTS idx_tracked_pairings_active;
DROP INDEX IF EXISTS idx_pairing_matches_date;
DROP INDEX IF EXISTS idx_pairing_matches_time;
DROP INDEX IF EXISTS idx_pairing_matches_opponents;
DROP INDEX IF EXISTS idx_pairing_matches_pairing;
DROP TABLE IF EXISTS pairing_matches;
DROP TABLE IF EXISTS tracked_pairings;
