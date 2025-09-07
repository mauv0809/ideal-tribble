package jobqueue

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"time"
)

type SQLiteQueue struct {
	db *sql.DB
}

// New creates a new SQLite-based job queue
func New(db *sql.DB) JobQueue {
	return &SQLiteQueue{db: db}
}

// Enqueue adds a new job to the queue
func (q *SQLiteQueue) Enqueue(jobType string, payload interface{}) error {
	return q.EnqueueDelayed(jobType, payload, 0)
}

// EnqueueDelayed adds a job to be processed after a delay
func (q *SQLiteQueue) EnqueueDelayed(jobType string, payload interface{}, delay time.Duration) error {
	payloadJSON, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("failed to marshal payload: %w", err)
	}

	scheduledAt := time.Now().Add(delay).Unix()

	_, err = q.db.Exec(`
		INSERT INTO job_queue (job_type, payload, scheduled_at)
		VALUES (?, ?, ?)
	`, jobType, string(payloadJSON), scheduledAt)

	if err != nil {
		return fmt.Errorf("failed to enqueue job: %w", err)
	}

	return nil
}

// Dequeue gets the next available job for processing
func (q *SQLiteQueue) Dequeue() (*Job, error) {
	tx, err := q.db.Begin()
	if err != nil {
		return nil, fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback()

	// Find next available job
	row := tx.QueryRow(`
		SELECT id, job_type, payload, status, priority, retry_count, max_retries,
			   created_at, updated_at, scheduled_at, processed_at, error_message
		FROM job_queue
		WHERE status = ? AND scheduled_at <= ?
		ORDER BY priority DESC, scheduled_at ASC
		LIMIT 1
	`, JobStatusPending, time.Now().Unix())

	var job Job
	var createdAt, updatedAt, scheduledAt int64
	var processedAt sql.NullInt64

	err = row.Scan(
		&job.ID, &job.JobType, &job.Payload, &job.Status, &job.Priority,
		&job.RetryCount, &job.MaxRetries, &createdAt, &updatedAt, &scheduledAt,
		&processedAt, &job.ErrorMessage,
	)

	if err == sql.ErrNoRows {
		return nil, nil // No jobs available
	}
	if err != nil {
		return nil, fmt.Errorf("failed to query job: %w", err)
	}

	// Convert timestamps
	job.CreatedAt = time.Unix(createdAt, 0)
	job.UpdatedAt = time.Unix(updatedAt, 0)
	job.ScheduledAt = time.Unix(scheduledAt, 0)
	if processedAt.Valid {
		t := time.Unix(processedAt.Int64, 0)
		job.ProcessedAt = &t
	}

	// Mark job as processing
	_, err = tx.Exec(`
		UPDATE job_queue 
		SET status = ?, updated_at = ?
		WHERE id = ?
	`, JobStatusProcessing, time.Now().Unix(), job.ID)

	if err != nil {
		return nil, fmt.Errorf("failed to mark job as processing: %w", err)
	}
	job.Status = JobStatusProcessing

	if err = tx.Commit(); err != nil {
		return nil, fmt.Errorf("failed to commit transaction: %w", err)
	}

	return &job, nil
}

// Complete marks a job as successfully completed
func (q *SQLiteQueue) Complete(jobID string) error {
	now := time.Now().Unix()
	_, err := q.db.Exec(`
		UPDATE job_queue 
		SET status = ?, updated_at = ?, processed_at = ?
		WHERE id = ?
	`, JobStatusCompleted, now, now, jobID)

	if err != nil {
		return fmt.Errorf("failed to mark job as completed: %w", err)
	}

	return nil
}

// Fail marks a job as failed and potentially retries it
func (q *SQLiteQueue) Fail(jobID string, errorMsg string) error {
	tx, err := q.db.Begin()
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback()

	// Get current job info
	var retryCount, maxRetries int
	err = tx.QueryRow(`
		SELECT retry_count, max_retries FROM job_queue WHERE id = ?
	`, jobID).Scan(&retryCount, &maxRetries)

	if err != nil {
		return fmt.Errorf("failed to get job info: %w", err)
	}

	now := time.Now().Unix()

	// Check if we should retry
	if retryCount < maxRetries {
		// Schedule for retry with exponential backoff
		retryDelay := time.Duration(retryCount+1) * time.Minute
		scheduledAt := time.Now().Add(retryDelay).Unix()

		_, err = tx.Exec(`
			UPDATE job_queue 
			SET status = ?, retry_count = ?, updated_at = ?, scheduled_at = ?, error_message = ?
			WHERE id = ?
		`, JobStatusPending, retryCount+1, now, scheduledAt, errorMsg, jobID)
	} else {
		// Max retries exceeded, mark as failed
		_, err = tx.Exec(`
			UPDATE job_queue 
			SET status = ?, updated_at = ?, processed_at = ?, error_message = ?
			WHERE id = ?
		`, JobStatusFailed, now, now, errorMsg, jobID)
	}

	if err != nil {
		return fmt.Errorf("failed to update job: %w", err)
	}

	return tx.Commit()
}

// Cleanup removes old completed/failed jobs
func (q *SQLiteQueue) Cleanup(olderThan time.Duration) error {
	cutoff := time.Now().Add(-olderThan).Unix()

	_, err := q.db.Exec(`
		DELETE FROM job_queue 
		WHERE status IN (?, ?) AND processed_at <= ?
	`, JobStatusCompleted, JobStatusFailed, cutoff)

	if err != nil {
		return fmt.Errorf("failed to cleanup jobs: %w", err)
	}

	return nil
}
