// Package queue provides a simple in-process goroutine worker pool for
// background job processing. It dispatches extraction and indexing jobs
// asynchronously after document uploads.
package queue

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
)

// Job represents a unit of work to be processed by the worker pool.
type Job struct {
	// Name identifies the job type for logging.
	Name string

	// Fn is the function to execute. It receives the pool's base context.
	Fn func(ctx context.Context) error
}

// Pool is a simple goroutine worker pool that processes jobs from a channel.
type Pool struct {
	jobs   chan Job
	wg     sync.WaitGroup
	ctx    context.Context
	cancel context.CancelFunc
	logger *slog.Logger
}

// NewPool creates a worker pool with the given number of workers and job buffer size.
func NewPool(workers, bufferSize int, logger *slog.Logger) *Pool {
	ctx, cancel := context.WithCancel(context.Background())

	p := &Pool{
		jobs:   make(chan Job, bufferSize),
		ctx:    ctx,
		cancel: cancel,
		logger: logger,
	}

	p.wg.Add(workers)
	for i := range workers {
		go p.worker(i)
	}

	logger.Info("worker pool started", "workers", workers, "buffer", bufferSize)
	return p
}

// Dispatch sends a job to the worker pool. It returns an error if the pool
// is full (non-blocking) or the pool has been shut down.
func (p *Pool) Dispatch(job Job) error {
	select {
	case <-p.ctx.Done():
		return fmt.Errorf("dispatching job %q: pool is shut down", job.Name)
	case p.jobs <- job:
		p.logger.Debug("job dispatched", "job", job.Name)
		return nil
	default:
		return fmt.Errorf("dispatching job %q: pool buffer full", job.Name)
	}
}

// Shutdown stops accepting new jobs and waits for all in-progress jobs to
// complete. It should be called during application shutdown.
func (p *Pool) Shutdown() {
	p.logger.Info("worker pool shutting down")
	p.cancel()
	close(p.jobs)
	p.wg.Wait()
	p.logger.Info("worker pool stopped")
}

// worker processes jobs from the channel until it is closed.
func (p *Pool) worker(id int) {
	defer p.wg.Done()

	for job := range p.jobs {
		p.logger.Debug("processing job", "worker", id, "job", job.Name)

		if err := job.Fn(p.ctx); err != nil {
			p.logger.Error("job failed", "worker", id, "job", job.Name, "error", err)
		} else {
			p.logger.Debug("job completed", "worker", id, "job", job.Name)
		}
	}
}
