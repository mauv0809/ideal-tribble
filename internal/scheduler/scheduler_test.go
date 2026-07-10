package scheduler

import (
	"context"
	"sync/atomic"
	"testing"
	"time"

	"github.com/charmbracelet/log"
)

func TestRegister_InvalidSpec(t *testing.T) {
	s := New(log.Default())
	if err := s.Register("not-a-cron-spec", "bad", func() {}); err == nil {
		t.Fatal("expected an error for an invalid cron spec, got nil")
	}
}

func TestJob_RunsAndDrainsOnStop(t *testing.T) {
	s := New(log.Default())

	// Signal on first fire and wait for it with a generous timeout, rather
	// than a fixed sleep — avoids flakiness when the machine is loaded.
	fired := make(chan struct{}, 1)
	if err := s.Register("@every 100ms", "tick", func() {
		select {
		case fired <- struct{}{}:
		default:
		}
	}); err != nil {
		t.Fatalf("Register: %v", err)
	}

	s.Start()

	select {
	case <-fired:
		// ran at least once
	case <-time.After(5 * time.Second):
		t.Fatal("expected the job to run at least once within 5s, it never fired")
	}

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	s.Stop(ctx)
}

func TestSkipIfStillRunning(t *testing.T) {
	s := New(log.Default())

	var concurrent, maxConcurrent int64
	if err := s.Register("@every 100ms", "slow", func() {
		c := atomic.AddInt64(&concurrent, 1)
		for {
			m := atomic.LoadInt64(&maxConcurrent)
			if c <= m || atomic.CompareAndSwapInt64(&maxConcurrent, m, c) {
				break
			}
		}
		time.Sleep(350 * time.Millisecond) // outlives several tick intervals
		atomic.AddInt64(&concurrent, -1)
	}); err != nil {
		t.Fatalf("Register: %v", err)
	}

	s.Start()
	time.Sleep(700 * time.Millisecond)

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	s.Stop(ctx)

	if got := atomic.LoadInt64(&maxConcurrent); got > 1 {
		t.Fatalf("SkipIfStillRunning failed: saw %d concurrent runs, want 1", got)
	}
}
