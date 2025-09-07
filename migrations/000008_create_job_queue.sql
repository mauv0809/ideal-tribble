-- +goose Up
-- Job queue table to replace Google Cloud Pub/Sub
CREATE TABLE IF NOT EXISTS job_queue (
    id TEXT PRIMARY KEY DEFAULT (lower(hex(randomblob(16)))),
    job_type TEXT NOT NULL,
    payload TEXT NOT NULL, -- JSON payload
    status TEXT NOT NULL DEFAULT 'pending', -- pending, processing, completed, failed
    priority INTEGER NOT NULL DEFAULT 0, -- higher number = higher priority
    retry_count INTEGER NOT NULL DEFAULT 0,
    max_retries INTEGER NOT NULL DEFAULT 3,
    created_at INTEGER NOT NULL DEFAULT (unixepoch()),
    updated_at INTEGER NOT NULL DEFAULT (unixepoch()),
    scheduled_at INTEGER NOT NULL DEFAULT (unixepoch()), -- when to process the job
    processed_at INTEGER, -- when job was completed/failed
    error_message TEXT
);

-- Index for efficient job processing
CREATE INDEX IF NOT EXISTS idx_job_queue_status_priority ON job_queue (status, priority DESC, scheduled_at ASC);

-- Index for cleanup queries  
CREATE INDEX IF NOT EXISTS idx_job_queue_processed_at ON job_queue (processed_at);

-- +goose Down
DROP TABLE IF EXISTS job_queue;
DROP INDEX IF EXISTS idx_job_queue_status_priority;
DROP INDEX IF EXISTS idx_job_queue_processed_at;