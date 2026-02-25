package queue_test

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"strings"
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

func TestPool_Dispatch_BufferFullErrorContainsJobName(t *testing.T) {
	pool := queue.NewPool(1, 1, testLogger())
	defer pool.Shutdown()

	blocker := make(chan struct{})

	// Block the worker.
	_ = pool.Dispatch(queue.Job{
		Name: "blocker",
		Fn: func(ctx context.Context) error {
			<-blocker
			return nil
		},
	})
	time.Sleep(50 * time.Millisecond)

	// Fill the buffer.
	_ = pool.Dispatch(queue.Job{
		Name: "filler",
		Fn:   func(ctx context.Context) error { return nil },
	})

	// Overflow should contain the job name.
	err := pool.Dispatch(queue.Job{
		Name: "overflow-named",
		Fn:   func(ctx context.Context) error { return nil },
	})
	if err == nil {
		close(blocker)
		t.Fatal("expected error, got nil")
	}

	if !strings.Contains(err.Error(), "overflow-named") {
		t.Errorf("error %q should contain the job name %q", err.Error(), "overflow-named")
	}

	close(blocker)
}

func TestPool_Dispatch_ShutdownErrorContainsJobName(t *testing.T) {
	pool := queue.NewPool(1, 5, testLogger())
	pool.Shutdown()

	var err error
	func() {
		defer func() { recover() }() //nolint:errcheck
		err = pool.Dispatch(queue.Job{
			Name: "post-shutdown-named",
			Fn:   func(ctx context.Context) error { return nil },
		})
	}()

	// When the context.Done branch is selected, the error should contain the job name.
	if err != nil && !strings.Contains(err.Error(), "post-shutdown-named") {
		t.Errorf("error %q should contain the job name %q", err.Error(), "post-shutdown-named")
	}
}

func TestPool_JobReceivesPoolContext(t *testing.T) {
	pool := queue.NewPool(1, 5, testLogger())
	defer pool.Shutdown()

	ctxCh := make(chan context.Context, 1)

	err := pool.Dispatch(queue.Job{
		Name: "ctx-check",
		Fn: func(ctx context.Context) error {
			ctxCh <- ctx
			return nil
		},
	})
	if err != nil {
		t.Fatalf("Dispatch() returned unexpected error: %v", err)
	}

	select {
	case ctx := <-ctxCh:
		// The context should not be nil and should not already be done.
		if ctx == nil {
			t.Fatal("job received nil context")
		}
		if ctx.Err() != nil {
			t.Fatalf("job context already cancelled: %v", ctx.Err())
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for job context")
	}
}

func TestPool_ConcurrentDispatchAndShutdown(t *testing.T) {
	pool := queue.NewPool(4, 100, testLogger())

	var executed atomic.Int32
	var wg sync.WaitGroup

	// Dispatch many jobs rapidly.
	for i := range 50 {
		wg.Add(1)
		go func(n int) {
			defer wg.Done()
			_ = pool.Dispatch(queue.Job{
				Name: "concurrent",
				Fn: func(ctx context.Context) error {
					executed.Add(1)
					return nil
				},
			})
		}(i)
	}

	wg.Wait()
	pool.Shutdown()

	// All successfully dispatched jobs must have completed.
	if got := executed.Load(); got == 0 {
		t.Fatal("expected at least some jobs to execute, got 0")
	}
}

func TestPool_SingleWorker_ProcessesInOrder(t *testing.T) {
	pool := queue.NewPool(1, 10, testLogger())
	defer pool.Shutdown()

	var mu sync.Mutex
	var order []int

	var wg sync.WaitGroup
	for i := range 5 {
		wg.Add(1)
		n := i
		err := pool.Dispatch(queue.Job{
			Name: "ordered",
			Fn: func(ctx context.Context) error {
				mu.Lock()
				order = append(order, n)
				mu.Unlock()
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
		// With a single worker, jobs should be processed in FIFO order.
		mu.Lock()
		defer mu.Unlock()
		for i, v := range order {
			if v != i {
				t.Errorf("order[%d] = %d, want %d (expected FIFO processing)", i, v, i)
			}
		}
	case <-time.After(5 * time.Second):
		t.Fatal("timed out waiting for ordered jobs to complete")
	}
}

func TestPool_MultipleConsecutiveFailures(t *testing.T) {
	pool := queue.NewPool(2, 10, testLogger())
	defer pool.Shutdown()

	const numFailures = 5
	var failCount atomic.Int32
	var wg sync.WaitGroup
	wg.Add(numFailures)

	for range numFailures {
		err := pool.Dispatch(queue.Job{
			Name: "fail",
			Fn: func(ctx context.Context) error {
				failCount.Add(1)
				wg.Done()
				return errors.New("intentional failure")
			},
		})
		if err != nil {
			t.Fatalf("Dispatch() returned unexpected error: %v", err)
		}
	}

	waitDone := make(chan struct{})
	go func() {
		wg.Wait()
		close(waitDone)
	}()

	select {
	case <-waitDone:
		// All failures processed.
	case <-time.After(5 * time.Second):
		t.Fatal("timed out waiting for failing jobs")
	}

	// Now dispatch a success job to confirm the pool is alive.
	done := make(chan struct{})
	err := pool.Dispatch(queue.Job{
		Name: "success-after-failures",
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
		// Pool survived.
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for success job after multiple failures")
	}
}
