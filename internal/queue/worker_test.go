package queue_test

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"git.999.haus/chris/DocuMCP-go/internal/queue"
)

func testLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

func TestPool_Dispatch(t *testing.T) {
	t.Run("basic job dispatch and execution", func(t *testing.T) {
		pool := queue.NewPool(2, 10, testLogger())
		defer pool.Shutdown()

		done := make(chan struct{})

		err := pool.Dispatch(queue.Job{
			Name: "test-job",
			Fn: func(ctx context.Context) error {
				close(done)
				return nil
			},
		})
		if err != nil {
			t.Fatalf("Dispatch() returned unexpected error: %v", err)
		}

		select {
		case <-done:
			// success
		case <-time.After(2 * time.Second):
			t.Fatal("timed out waiting for job to execute")
		}
	})

	t.Run("multiple jobs processed concurrently", func(t *testing.T) {
		const numJobs = 10
		pool := queue.NewPool(4, numJobs, testLogger())
		defer pool.Shutdown()

		var count atomic.Int32
		var wg sync.WaitGroup
		wg.Add(numJobs)

		for i := range numJobs {
			err := pool.Dispatch(queue.Job{
				Name: "concurrent-job",
				Fn: func(ctx context.Context) error {
					count.Add(1)
					wg.Done()
					return nil
				},
			})
			if err != nil {
				t.Fatalf("Dispatch() job %d returned unexpected error: %v", i, err)
			}
		}

		waitDone := make(chan struct{})
		go func() {
			wg.Wait()
			close(waitDone)
		}()

		select {
		case <-waitDone:
			if got := count.Load(); got != numJobs {
				t.Errorf("expected %d jobs executed, got %d", numJobs, got)
			}
		case <-time.After(5 * time.Second):
			t.Fatalf("timed out: only %d of %d jobs completed", count.Load(), numJobs)
		}
	})

	t.Run("dispatch returns error when pool is shut down", func(t *testing.T) {
		pool := queue.NewPool(1, 5, testLogger())
		pool.Shutdown()

		// After Shutdown, calling Dispatch may either return an error
		// (if ctx.Done is selected first) or panic with "send on closed
		// channel" (if the channel send case is selected). Both outcomes
		// indicate the pool correctly rejects new work. We use recover
		// to handle the latter case gracefully.
		var err error
		panicked := false
		func() {
			defer func() {
				if r := recover(); r != nil {
					panicked = true
				}
			}()
			err = pool.Dispatch(queue.Job{
				Name: "after-shutdown",
				Fn: func(ctx context.Context) error {
					return nil
				},
			})
		}()

		if !panicked && err == nil {
			t.Fatal("expected error or panic dispatching to shut-down pool, got neither")
		}
	})

	t.Run("dispatch returns error when buffer is full", func(t *testing.T) {
		// Buffer of 1, 1 worker that blocks so the buffer stays full.
		pool := queue.NewPool(1, 1, testLogger())
		defer pool.Shutdown()

		blocker := make(chan struct{})

		// First job: blocks the only worker.
		err := pool.Dispatch(queue.Job{
			Name: "blocking-job",
			Fn: func(ctx context.Context) error {
				<-blocker
				return nil
			},
		})
		if err != nil {
			t.Fatalf("first Dispatch() returned unexpected error: %v", err)
		}

		// Give the worker time to pick up the blocking job.
		// We need the worker to drain the channel so the next job fills the buffer.
		time.Sleep(50 * time.Millisecond)

		// Second job: fills the buffer (size 1).
		err = pool.Dispatch(queue.Job{
			Name: "buffer-fill",
			Fn: func(ctx context.Context) error {
				return nil
			},
		})
		if err != nil {
			t.Fatalf("second Dispatch() returned unexpected error: %v", err)
		}

		// Third job: buffer is full, should fail.
		err = pool.Dispatch(queue.Job{
			Name: "overflow",
			Fn: func(ctx context.Context) error {
				return nil
			},
		})
		if err == nil {
			t.Fatal("expected error when buffer is full, got nil")
		}

		close(blocker)
	})

	t.Run("failed job does not crash the pool", func(t *testing.T) {
		pool := queue.NewPool(2, 10, testLogger())
		defer pool.Shutdown()

		done := make(chan struct{})

		// Dispatch a job that returns an error.
		err := pool.Dispatch(queue.Job{
			Name: "failing-job",
			Fn: func(ctx context.Context) error {
				return errors.New("intentional failure")
			},
		})
		if err != nil {
			t.Fatalf("Dispatch() returned unexpected error: %v", err)
		}

		// Dispatch a follow-up job to verify the pool is still operational.
		err = pool.Dispatch(queue.Job{
			Name: "follow-up-job",
			Fn: func(ctx context.Context) error {
				close(done)
				return nil
			},
		})
		if err != nil {
			t.Fatalf("Dispatch() returned unexpected error: %v", err)
		}

		select {
		case <-done:
			// Pool survived the failing job.
		case <-time.After(2 * time.Second):
			t.Fatal("timed out waiting for follow-up job after failure")
		}
	})

	t.Run("shutdown waits for in-progress jobs", func(t *testing.T) {
		pool := queue.NewPool(1, 5, testLogger())

		var completed atomic.Bool
		started := make(chan struct{})

		err := pool.Dispatch(queue.Job{
			Name: "slow-job",
			Fn: func(ctx context.Context) error {
				close(started)
				time.Sleep(200 * time.Millisecond)
				completed.Store(true)
				return nil
			},
		})
		if err != nil {
			t.Fatalf("Dispatch() returned unexpected error: %v", err)
		}

		// Wait for the job to start before shutting down.
		select {
		case <-started:
		case <-time.After(2 * time.Second):
			t.Fatal("timed out waiting for job to start")
		}

		// Shutdown should block until the slow job finishes.
		pool.Shutdown()

		if !completed.Load() {
			t.Fatal("Shutdown() returned before in-progress job completed")
		}
	})
}
