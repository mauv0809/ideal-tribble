package jobqueue

import (
	"database/sql"
	"time"
)

// JobQueue interface replaces PubSub functionality with SQLite-based job queue
type JobQueue interface {
	// Enqueue adds a new job to the queue
	Enqueue(jobType string, payload interface{}) error

	// EnqueueDelayed adds a job to be processed after a delay
	EnqueueDelayed(jobType string, payload interface{}, delay time.Duration) error

	// Dequeue gets the next available job for processing
	Dequeue() (*Job, error)

	// Complete marks a job as successfully completed
	Complete(jobID string) error

	// Fail marks a job as failed and potentially retries it
	Fail(jobID string, errorMsg string) error

	// Cleanup removes old completed/failed jobs
	Cleanup(olderThan time.Duration) error
}

// Job represents a queued job
type Job struct {
	ID           string         `json:"id"`
	JobType      string         `json:"job_type"`
	Payload      string         `json:"payload"` // JSON payload
	Status       string         `json:"status"`
	Priority     int            `json:"priority"`
	RetryCount   int            `json:"retry_count"`
	MaxRetries   int            `json:"max_retries"`
	CreatedAt    time.Time      `json:"created_at"`
	UpdatedAt    time.Time      `json:"updated_at"`
	ScheduledAt  time.Time      `json:"scheduled_at"`
	ProcessedAt  *time.Time     `json:"processed_at,omitempty"`
	ErrorMessage sql.NullString `json:"error_message,omitempty"`
}

// Job types (replacing PubSub EventTypes)
const (
	JobTypeAssignBallBoy     = "assign_ball_boy"
	JobTypeUpdatePlayerStats = "update_player_stats"
	JobTypeUpdateWeeklyStats = "update_weekly_stats"
	JobTypeNotifyBooking     = "notify_booking"
	JobTypeNotifyResult      = "notify_result"
)

// Job statuses
const (
	JobStatusPending    = "pending"
	JobStatusProcessing = "processing"
	JobStatusCompleted  = "completed"
	JobStatusFailed     = "failed"
)
