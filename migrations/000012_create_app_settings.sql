-- +goose Up

-- App-wide settings key-value store
CREATE TABLE IF NOT EXISTS app_settings (
    key TEXT PRIMARY KEY,
    value TEXT NOT NULL,
    updated_at INTEGER NOT NULL
);

-- Initialize last fetch timestamp (0 = never fetched)
INSERT OR IGNORE INTO app_settings (key, value, updated_at)
VALUES ('last_fetch_timestamp', '0', 0);

-- +goose Down
DROP TABLE IF EXISTS app_settings;
