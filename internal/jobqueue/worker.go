package jobqueue

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"time"
)

// JobHandler is a function that processes a specific job type
type JobHandler func(payload json.RawMessage) error

// Worker processes jobs from the queue
type Worker struct {
	queue    JobQueue
	handlers map[string]JobHandler
	logger   *log.Logger
}

// NewWorker creates a new job worker
func NewWorker(queue JobQueue, logger *log.Logger) *Worker {
	return &Worker{
		queue:    queue,
		handlers: make(map[string]JobHandler),
		logger:   logger,
	}
}

// RegisterHandler registers a handler for a specific job type
func (w *Worker) RegisterHandler(jobType string, handler JobHandler) {
	w.handlers[jobType] = handler
}

// Start begins processing jobs in a loop
func (w *Worker) Start(ctx context.Context) error {
	w.logger.Println("Job worker started")
	
	ticker := time.NewTicker(5 * time.Second) // Poll every 5 seconds
	defer ticker.Stop()
	
	for {
		select {
		case <-ctx.Done():
			w.logger.Println("Job worker stopping...")
			return ctx.Err()
		case <-ticker.C:
			if err := w.processNextJob(); err != nil {
				w.logger.Printf("Error processing job: %v", err)
			}
		}
	}
}

// processNextJob gets and processes the next available job
func (w *Worker) processNextJob() error {
	job, err := w.queue.Dequeue()
	if err != nil {
		return fmt.Errorf("failed to dequeue job: %w", err)
	}
	
	if job == nil {
		return nil // No jobs available
	}
	
	w.logger.Printf("Processing job %s of type %s", job.ID, job.JobType)
	
	handler, exists := w.handlers[job.JobType]
	if !exists {
		err := fmt.Errorf("no handler registered for job type: %s", job.JobType)
		w.queue.Fail(job.ID, err.Error())
		return err
	}
	
	// Execute the job handler
	if err := handler(json.RawMessage(job.Payload)); err != nil {
		w.logger.Printf("Job %s failed: %v", job.ID, err)
		w.queue.Fail(job.ID, err.Error())
		return err
	}
	
	// Mark job as completed
	if err := w.queue.Complete(job.ID); err != nil {
		return fmt.Errorf("failed to mark job as completed: %w", err)
	}
	
	w.logger.Printf("Job %s completed successfully", job.ID)
	return nil
}

// StartCleanup starts a goroutine that periodically cleans up old jobs
func (w *Worker) StartCleanup(ctx context.Context, interval time.Duration, maxAge time.Duration) {
	go func() {
		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				if err := w.queue.Cleanup(maxAge); err != nil {
					w.logger.Printf("Error cleaning up jobs: %v", err)
				} else {
					w.logger.Printf("Cleaned up jobs older than %v", maxAge)
				}
			}
		}
	}()
}