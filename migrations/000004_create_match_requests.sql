-- +goose Up
-- match_requests stores requests for organizing matches initiated by users
CREATE TABLE IF NOT EXISTS match_requests (
    id TEXT PRIMARY KEY,
    requester_id TEXT NOT NULL,
    requester_name TEXT NOT NULL,
    created_at INTEGER NOT NULL,
    updated_at INTEGER NOT NULL,
    status TEXT NOT NULL DEFAULT 'COLLECTING_AVAILABILITY', -- COLLECTING_AVAILABILITY, PROPOSING_MATCH, CONFIRMED, CANCELLED
    channel_id TEXT NOT NULL,
    thread_ts TEXT,
    availability_message_ts TEXT,
    proposed_date TEXT, -- YYYY-MM-DD format when match is proposed
    proposed_start_time TEXT, -- HH:MM format when match is proposed
    proposed_end_time TEXT, -- HH:MM format when match is proposed
    booking_responsible_id TEXT, -- player assigned to book via playtomic
    booking_responsible_name TEXT,
    team_assignments_blob BLOB, -- JSON blob containing team assignments when confirmed
    
    FOREIGN KEY (requester_id) REFERENCES players(id),
    FOREIGN KEY (booking_responsible_id) REFERENCES players(id) ON DELETE SET NULL
);

-- match_request_availability stores player availability responses
CREATE TABLE IF NOT EXISTS match_request_availability (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    match_request_id TEXT NOT NULL,
    player_id TEXT NOT NULL,
    player_name TEXT NOT NULL,
    available_date TEXT NOT NULL, -- YYYY-MM-DD format
    responded_at INTEGER NOT NULL,
    
    FOREIGN KEY (match_request_id) REFERENCES match_requests(id) ON DELETE CASCADE,
    FOREIGN KEY (player_id) REFERENCES players(id) ON DELETE CASCADE,
    UNIQUE(match_request_id, player_id, available_date)
);

-- Indexes for efficient queries
CREATE INDEX IF NOT EXISTS idx_match_requests_status ON match_requests (status);
CREATE INDEX IF NOT EXISTS idx_match_requests_created_at ON match_requests (created_at DESC);
CREATE INDEX IF NOT EXISTS idx_match_request_availability_request_id ON match_request_availability (match_request_id);
CREATE INDEX IF NOT EXISTS idx_match_request_availability_date ON match_request_availability (available_date);

-- +goose Down
DROP TABLE IF EXISTS match_request_availability;
DROP TABLE IF EXISTS match_requests;