// Package scheduler runs registered jobs on wall-clock cron schedules.
//
// It replaces the host crontab that previously POSTed to /fetch and /process:
// scheduling now lives inside the application process, so no external cron (or
// publicly reachable trigger endpoint) is required. It wraps robfig/cron with
// panic recovery and overlap protection, so a job that panics or runs long can
// never take down the process or stack up against its previous run.
package scheduler

import (
	"context"
	"fmt"
	"time"

	"github.com/charmbracelet/log"
	"github.com/robfig/cron/v3"
)

// Scheduler owns a cron runtime. Jobs are added with Register and only begin
// firing once Start is called.
type Scheduler struct {
	cron   *cron.Cron
	logger *log.Logger
}

// New creates a Scheduler. Every job runs through a wrapper chain that recovers
// panics and skips a run if the previous one is still executing.
func New(logger *log.Logger) *Scheduler {
	cl := &cronLogger{logger: logger}
	c := cron.New(cron.WithChain(
		cron.Recover(cl),            // a panicking job is logged, not fatal
		cron.SkipIfStillRunning(cl), // never overlap a job with its previous run
	))
	return &Scheduler{cron: c, logger: logger}
}

// Register schedules fn on spec, which uses standard 5-field cron syntax
// (e.g. "0 * * * *" for the top of every hour). name is used only for logging.
// It returns an error if spec is invalid.
func (s *Scheduler) Register(spec, name string, fn func()) error {
	if _, err := s.cron.AddFunc(spec, s.wrap(name, fn)); err != nil {
		return fmt.Errorf("scheduler: invalid spec %q for job %q: %w", spec, name, err)
	}
	s.logger.Info("Registered scheduled job", "job", name, "spec", spec)
	return nil
}

// wrap adds start/finish logging with timing around a job.
func (s *Scheduler) wrap(name string, fn func()) func() {
	return func() {
		start := time.Now()
		s.logger.Info("Scheduled job starting", "job", name)
		fn()
		s.logger.Info("Scheduled job finished", "job", name, "duration_ms", time.Since(start).Milliseconds())
	}
}

// Start begins firing jobs. It does not block.
func (s *Scheduler) Start() {
	s.cron.Start()
}

// Stop halts scheduling of new runs and waits for any in-flight job to finish,
// bounded by ctx. Call it during graceful shutdown.
func (s *Scheduler) Stop(ctx context.Context) {
	stopped := s.cron.Stop() // done when currently-running jobs complete
	select {
	case <-stopped.Done():
		s.logger.Info("Scheduler stopped; in-flight jobs drained")
	case <-ctx.Done():
		s.logger.Warn("Scheduler stop timed out waiting for in-flight jobs")
	}
}

// cronLogger adapts charmbracelet/log to the cron.Logger interface.
type cronLogger struct {
	logger *log.Logger
}

func (l *cronLogger) Info(msg string, keysAndValues ...any) {
	l.logger.Info(msg, keysAndValues...)
}

func (l *cronLogger) Error(err error, msg string, keysAndValues ...any) {
	l.logger.Error(msg, append([]any{"error", err}, keysAndValues...)...)
}
