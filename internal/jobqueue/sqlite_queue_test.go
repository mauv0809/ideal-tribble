package jobqueue

import (
	"database/sql"
	"testing"
	"time"

	"github.com/mauv0809/ideal-tribble/internal/database"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	_ "github.com/tursodatabase/libsql-client-go/libsql"
)

func setupTestDB(t *testing.T) (*sql.DB, func()) {
	t.Helper()

	db, dbTeardown, err := database.InitDB(":memory:", "", "", "../../migrations")
	require.NoError(t, err)

	teardown := func() {
		dbTeardown()
		db.Close()
	}

	return db, teardown
}

func TestSQLiteQueue_Enqueue(t *testing.T) {
	db, teardown := setupTestDB(t)
	defer teardown()

	queue := New(db)

	payload := map[string]interface{}{
		"match_id": "test-match-123",
		"data":     "test data",
	}

	err := queue.Enqueue(JobTypeAssignBallBoy, payload)
	require.NoError(t, err)

	// Verify job was inserted
	var count int
	err = db.QueryRow("SELECT COUNT(*) FROM job_queue WHERE job_type = ?", JobTypeAssignBallBoy).Scan(&count)
	require.NoError(t, err)
	assert.Equal(t, 1, count)

	// Verify job details
	var job Job
	var createdAt, updatedAt, scheduledAt int64
	err = db.QueryRow(`
		SELECT id, job_type, payload, status, priority, retry_count, max_retries, 
			   created_at, updated_at, scheduled_at
		FROM job_queue WHERE job_type = ?
	`, JobTypeAssignBallBoy).Scan(
		&job.ID, &job.JobType, &job.Payload, &job.Status, &job.Priority,
		&job.RetryCount, &job.MaxRetries, &createdAt, &updatedAt, &scheduledAt,
	)
	require.NoError(t, err)

	assert.Equal(t, JobTypeAssignBallBoy, job.JobType)
	assert.Equal(t, JobStatusPending, job.Status)
	assert.Equal(t, 0, job.Priority)
	assert.Equal(t, 0, job.RetryCount)
	assert.Equal(t, 3, job.MaxRetries)
	assert.Contains(t, job.Payload, "match_id")
}

func TestSQLiteQueue_EnqueueDelayed(t *testing.T) {
	db, teardown := setupTestDB(t)
	defer teardown()

	queue := New(db)

	payload := map[string]string{"test": "data"}
	delay := 5 * time.Minute

	startTime := time.Now()
	err := queue.EnqueueDelayed(JobTypeNotifyBooking, payload, delay)
	require.NoError(t, err)

	// Verify job was scheduled for future
	var scheduledAt int64
	err = db.QueryRow("SELECT scheduled_at FROM job_queue WHERE job_type = ?", JobTypeNotifyBooking).Scan(&scheduledAt)
	require.NoError(t, err)

	expectedTime := startTime.Add(delay).Unix()
	// Allow for 1 second variance due to test execution time
	assert.InDelta(t, expectedTime, scheduledAt, 1)
}

func TestSQLiteQueue_Dequeue(t *testing.T) {
	db, teardown := setupTestDB(t)
	defer teardown()

	queue := New(db)

	t.Run("returns nil when no jobs available", func(t *testing.T) {
		job, err := queue.Dequeue()
		require.NoError(t, err)
		assert.Nil(t, job)
	})

	t.Run("returns job and marks as processing", func(t *testing.T) {
		// Add a job
		payload := map[string]string{"test": "data"}
		err := queue.Enqueue(JobTypeAssignBallBoy, payload)
		require.NoError(t, err)

		// Dequeue it
		job, err := queue.Dequeue()
		require.NoError(t, err)
		require.NotNil(t, job)

		assert.Equal(t, JobTypeAssignBallBoy, job.JobType)
		assert.Equal(t, JobStatusProcessing, job.Status)
		assert.Contains(t, job.Payload, "test")

		// Verify status in database
		var status string
		err = db.QueryRow("SELECT status FROM job_queue WHERE id = ?", job.ID).Scan(&status)
		require.NoError(t, err)
		assert.Equal(t, JobStatusProcessing, status)
	})

	t.Run("respects scheduled_at time", func(t *testing.T) {
		// Clear previous jobs
		_, err := db.Exec("DELETE FROM job_queue")
		require.NoError(t, err)

		// Add a job scheduled for the future
		err = queue.EnqueueDelayed(JobTypeNotifyBooking, map[string]string{"test": "future"}, 1*time.Hour)
		require.NoError(t, err)

		// Try to dequeue - should get nothing
		job, err := queue.Dequeue()
		require.NoError(t, err)
		assert.Nil(t, job)
	})

	t.Run("respects priority order", func(t *testing.T) {
		// Clear previous jobs
		_, err := db.Exec("DELETE FROM job_queue")
		require.NoError(t, err)

		// Insert jobs with different priorities manually
		_, err = db.Exec(`
			INSERT INTO job_queue (job_type, payload, priority) VALUES 
			(?, '{"priority": "low"}', 1),
			(?, '{"priority": "high"}', 10),
			(?, '{"priority": "medium"}', 5)
		`, JobTypeAssignBallBoy, JobTypeNotifyBooking, JobTypeUpdatePlayerStats)
		require.NoError(t, err)

		// Dequeue should get highest priority first
		job, err := queue.Dequeue()
		require.NoError(t, err)
		require.NotNil(t, job)

		assert.Equal(t, JobTypeNotifyBooking, job.JobType)
		assert.Contains(t, job.Payload, "high")
	})
}

func TestSQLiteQueue_Complete(t *testing.T) {
	db, teardown := setupTestDB(t)
	defer teardown()

	queue := New(db)

	// Add and dequeue a job
	err := queue.Enqueue(JobTypeAssignBallBoy, map[string]string{"test": "data"})
	require.NoError(t, err)

	job, err := queue.Dequeue()
	require.NoError(t, err)
	require.NotNil(t, job)

	// Complete the job
	err = queue.Complete(job.ID)
	require.NoError(t, err)

	// Verify job status and processed_at
	var status string
	var processedAt sql.NullInt64
	err = db.QueryRow("SELECT status, processed_at FROM job_queue WHERE id = ?", job.ID).Scan(&status, &processedAt)
	require.NoError(t, err)

	assert.Equal(t, JobStatusCompleted, status)
	assert.True(t, processedAt.Valid)
	assert.InDelta(t, time.Now().Unix(), processedAt.Int64, 2)
}

func TestSQLiteQueue_Fail(t *testing.T) {
	db, teardown := setupTestDB(t)
	defer teardown()

	queue := New(db)

	t.Run("retries job when under max retries", func(t *testing.T) {
		// Add and dequeue a job
		err := queue.Enqueue(JobTypeAssignBallBoy, map[string]string{"test": "data"})
		require.NoError(t, err)

		job, err := queue.Dequeue()
		require.NoError(t, err)
		require.NotNil(t, job)

		// Fail the job
		err = queue.Fail(job.ID, "test error")
		require.NoError(t, err)

		// Verify job is back to pending with retry count incremented
		var status string
		var retryCount int
		var errorMsg string
		var scheduledAt int64
		err = db.QueryRow(`
			SELECT status, retry_count, error_message, scheduled_at 
			FROM job_queue WHERE id = ?
		`, job.ID).Scan(&status, &retryCount, &errorMsg, &scheduledAt)
		require.NoError(t, err)

		assert.Equal(t, JobStatusPending, status)
		assert.Equal(t, 1, retryCount)
		assert.Equal(t, "test error", errorMsg)
		// Should be scheduled for retry with delay
		assert.Greater(t, scheduledAt, time.Now().Unix())
	})

	t.Run("marks job as failed when max retries exceeded", func(t *testing.T) {
		// Clear previous jobs
		_, err := db.Exec("DELETE FROM job_queue")
		require.NoError(t, err)

		// Insert a job with max retries already reached
		_, err = db.Exec(`
			INSERT INTO job_queue (job_type, payload, status, retry_count, max_retries) 
			VALUES (?, '{"test": "data"}', ?, 3, 3)
		`, JobTypeAssignBallBoy, JobStatusProcessing)
		require.NoError(t, err)

		var jobID string
		err = db.QueryRow("SELECT id FROM job_queue").Scan(&jobID)
		require.NoError(t, err)

		// Fail the job
		err = queue.Fail(jobID, "final error")
		require.NoError(t, err)

		// Verify job is marked as failed
		var status string
		var processedAt sql.NullInt64
		var errorMsg string
		err = db.QueryRow(`
			SELECT status, processed_at, error_message 
			FROM job_queue WHERE id = ?
		`, jobID).Scan(&status, &processedAt, &errorMsg)
		require.NoError(t, err)

		assert.Equal(t, JobStatusFailed, status)
		assert.True(t, processedAt.Valid)
		assert.Equal(t, "final error", errorMsg)
	})
}

func TestSQLiteQueue_Cleanup(t *testing.T) {
	db, teardown := setupTestDB(t)
	defer teardown()

	queue := New(db)

	// Insert jobs with different statuses and ages
	pastTime := time.Now().Add(-2 * time.Hour).Unix()
	recentTime := time.Now().Unix()

	_, err := db.Exec(`
		INSERT INTO job_queue (job_type, payload, status, processed_at) VALUES 
		(?, '{"old": "completed"}', ?, ?),
		(?, '{"old": "failed"}', ?, ?),
		(?, '{"new": "completed"}', ?, ?),
		(?, '{"pending": "job"}', ?, NULL)
	`, JobTypeAssignBallBoy, JobStatusCompleted, pastTime,
		JobTypeNotifyBooking, JobStatusFailed, pastTime,
		JobTypeUpdatePlayerStats, JobStatusCompleted, recentTime,
		JobTypeNotifyResult, JobStatusPending)
	require.NoError(t, err)

	// Cleanup jobs older than 1 hour
	err = queue.Cleanup(1 * time.Hour)
	require.NoError(t, err)

	// Verify only old completed/failed jobs were removed
	var count int
	err = db.QueryRow("SELECT COUNT(*) FROM job_queue").Scan(&count)
	require.NoError(t, err)
	assert.Equal(t, 2, count) // Recent completed job + pending job should remain

	// Verify the remaining jobs
	rows, err := db.Query("SELECT payload, status FROM job_queue ORDER BY payload")
	require.NoError(t, err)
	defer rows.Close()

	var payloads []string
	var statuses []string
	for rows.Next() {
		var payload, status string
		err = rows.Scan(&payload, &status)
		require.NoError(t, err)
		payloads = append(payloads, payload)
		statuses = append(statuses, status)
	}

	assert.Contains(t, payloads, `{"new": "completed"}`)
	assert.Contains(t, payloads, `{"pending": "job"}`)
	assert.Contains(t, statuses, JobStatusCompleted)
	assert.Contains(t, statuses, JobStatusPending)
}
