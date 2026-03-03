package queue

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/riverqueue/river"
	"github.com/riverqueue/river/riverdriver/riverpgxv5"
	"github.com/riverqueue/river/rivertype"

	"git.999.haus/chris/DocuMCP-go/internal/observability"
)

// RiverClient wraps River's client with application-specific configuration.
type RiverClient struct {
	client   *river.Client[pgx.Tx]
	pool     *pgxpool.Pool
	eventBus *EventBus
	metrics  *observability.Metrics
	logger   *slog.Logger
}

// RiverConfig holds configuration for the River queue client.
type RiverConfig struct {
	Pool     *pgxpool.Pool
	EventBus *EventBus
	Logger   *slog.Logger
	Metrics  *observability.Metrics
	Workers  *river.Workers

	PeriodicJobs []*river.PeriodicJob
}

// NewRiverClient creates a River client with the given workers and queue config.
func NewRiverClient(cfg RiverConfig) (*RiverClient, error) {
	logger := cfg.Logger
	if logger == nil {
		logger = slog.Default()
	}

	client, err := river.NewClient(riverpgxv5.New(cfg.Pool), &river.Config{
		Queues: map[string]river.QueueConfig{
			"high":    {MaxWorkers: 10},
			"default": {MaxWorkers: 5},
			"low":     {MaxWorkers: 2},
		},
		Workers:      cfg.Workers,
		Logger:       logger,
		ErrorHandler: &riverErrorHandler{eventBus: cfg.EventBus, metrics: cfg.Metrics, logger: logger},
		PeriodicJobs: cfg.PeriodicJobs,
	})
	if err != nil {
		return nil, fmt.Errorf("creating river client: %w", err)
	}

	return &RiverClient{
		client:   client,
		pool:     cfg.Pool,
		eventBus: cfg.EventBus,
		metrics:  cfg.Metrics,
		logger:   logger,
	}, nil
}

// Start begins processing jobs. Call after creating the client.
func (rc *RiverClient) Start(ctx context.Context) error {
	return rc.client.Start(ctx)
}

// Stop gracefully shuts down the River client, waiting for in-progress jobs.
func (rc *RiverClient) Stop(ctx context.Context) error {
	return rc.client.Stop(ctx)
}

// Insert enqueues a job. Satisfies the service.JobInserter interface.
func (rc *RiverClient) Insert(ctx context.Context, args river.JobArgs, opts *river.InsertOpts) (*rivertype.JobInsertResult, error) {
	result, err := rc.client.Insert(ctx, args, opts)
	if err == nil {
		if rc.eventBus != nil {
			rc.eventBus.Publish(Event{
				Type:      EventJobDispatched,
				JobKind:   args.Kind(),
				JobID:     result.Job.ID,
				Queue:     result.Job.Queue,
				Timestamp: time.Now(),
			})
		}
		if rc.metrics != nil {
			rc.metrics.QueueJobsDispatched.WithLabelValues(result.Job.Queue, args.Kind()).Inc()
		}
	}
	return result, err
}

// Client returns the underlying River client for admin operations.
func (rc *RiverClient) Client() *river.Client[pgx.Tx] {
	return rc.client
}

// QueueStats returns job counts grouped by state, queried directly from the
// river_job table for accuracy (avoids fetching full job rows).
func (rc *RiverClient) QueueStats(ctx context.Context) (map[string]int, error) {
	rows, err := rc.pool.Query(ctx,
		`SELECT state, count(*) FROM river_job WHERE state = ANY($1) GROUP BY state`,
		[]string{"available", "running", "retryable", "discarded", "cancelled"},
	)
	if err != nil {
		return nil, fmt.Errorf("querying queue stats: %w", err)
	}
	defer rows.Close()

	stats := make(map[string]int, 5)
	for rows.Next() {
		var state string
		var count int
		if err := rows.Scan(&state, &count); err != nil {
			return nil, fmt.Errorf("scanning queue stats row: %w", err)
		}
		stats[state] = count
	}
	return stats, rows.Err()
}

// riverErrorHandler implements river.ErrorHandler to publish failure events.
type riverErrorHandler struct {
	eventBus *EventBus
	metrics  *observability.Metrics
	logger   *slog.Logger
}

func (h *riverErrorHandler) HandleError(ctx context.Context, job *rivertype.JobRow, jobErr error) *river.ErrorHandlerResult {
	h.logger.Error("job failed",
		"job_id", job.ID,
		"kind", job.Kind,
		"queue", job.Queue,
		"attempt", job.Attempt,
		"error", jobErr,
	)
	if h.eventBus != nil {
		h.eventBus.Publish(Event{
			Type:      EventJobFailed,
			JobKind:   job.Kind,
			JobID:     job.ID,
			Queue:     job.Queue,
			Attempt:   job.Attempt,
			Error:     jobErr.Error(),
			Timestamp: time.Now(),
		})
	}
	if h.metrics != nil {
		h.metrics.QueueJobsFailed.WithLabelValues(job.Queue, job.Kind).Inc()
	}
	return nil
}

func (h *riverErrorHandler) HandlePanic(ctx context.Context, job *rivertype.JobRow, panicVal any, trace string) *river.ErrorHandlerResult {
	h.logger.Error("job panicked",
		"job_id", job.ID,
		"kind", job.Kind,
		"queue", job.Queue,
		"attempt", job.Attempt,
		"panic", panicVal,
	)
	if h.eventBus != nil {
		h.eventBus.Publish(Event{
			Type:      EventJobFailed,
			JobKind:   job.Kind,
			JobID:     job.ID,
			Queue:     job.Queue,
			Attempt:   job.Attempt,
			Error:     fmt.Sprintf("panic: %v", panicVal),
			Timestamp: time.Now(),
		})
	}
	return nil
}
