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

	"github.com/c-premus/documcp/internal/observability"
)

// RiverClient wraps River's client with application-specific configuration.
type RiverClient struct {
	client          *river.Client[pgx.Tx]
	pool            *pgxpool.Pool
	eventBus        EventPublisher
	metrics         *observability.Metrics
	logger          *slog.Logger
	cancelSubscribe func()
	forwardingDone  chan struct{}
	insertOnly      bool
}

// RiverConfig holds configuration for the River queue client.
type RiverConfig struct {
	Pool     *pgxpool.Pool
	EventBus EventPublisher
	Logger   *slog.Logger
	Metrics  *observability.Metrics
	Workers  *river.Workers

	PeriodicJobs []*river.PeriodicJob

	// InsertOnly creates a client that can only insert jobs, not process them.
	// When true, Queues are omitted and Start()/Stop() become no-ops.
	InsertOnly bool

	// QueueWorkers sets per-queue concurrency. Ignored when InsertOnly is true.
	// Zero values fall back to defaults (high=10, default=5, low=2).
	QueueWorkers map[string]int
}

// NewRiverClient creates a River client with the given workers and queue config.
// When cfg.InsertOnly is true, the client can insert jobs but will not process
// them — Start() and Stop() become no-ops.
func NewRiverClient(cfg RiverConfig) (*RiverClient, error) {
	logger := cfg.Logger
	if logger == nil {
		logger = slog.Default()
	}

	riverCfg := &river.Config{
		Workers:      cfg.Workers,
		Logger:       logger,
		ErrorHandler: &riverErrorHandler{eventBus: cfg.EventBus, metrics: cfg.Metrics, logger: logger},
		PeriodicJobs: cfg.PeriodicJobs,
	}

	if !cfg.InsertOnly {
		riverCfg.Queues = buildQueueConfig(cfg.QueueWorkers)
	}

	client, err := river.NewClient(riverpgxv5.New(cfg.Pool), riverCfg)
	if err != nil {
		return nil, fmt.Errorf("creating river client: %w", err)
	}

	return &RiverClient{
		client:     client,
		pool:       cfg.Pool,
		eventBus:   cfg.EventBus,
		metrics:    cfg.Metrics,
		logger:     logger,
		insertOnly: cfg.InsertOnly,
	}, nil
}

// buildQueueConfig creates the queue concurrency map with defaults for zero values.
func buildQueueConfig(workers map[string]int) map[string]river.QueueConfig {
	get := func(name string, fallback int) int {
		if v, ok := workers[name]; ok && v > 0 {
			return v
		}
		return fallback
	}
	return map[string]river.QueueConfig{
		"high":    {MaxWorkers: get("high", 10)},
		"default": {MaxWorkers: get("default", 5)},
		"low":     {MaxWorkers: get("low", 2)},
	}
}

// Start begins processing jobs. Call after creating the client.
// No-op when the client is in insert-only mode.
func (rc *RiverClient) Start(ctx context.Context) error {
	if rc.insertOnly {
		return nil
	}
	if err := rc.client.Start(ctx); err != nil {
		return fmt.Errorf("starting river client: %w", err)
	}
	return nil
}

// StartEventForwarding subscribes to River's job completion and snooze events
// and forwards them to the application EventBus so SSE clients receive them.
// Call after Start(). The goroutine exits when Stop() cancels the subscription.
// No-op when the client is in insert-only mode.
func (rc *RiverClient) StartEventForwarding() {
	if rc.insertOnly || rc.eventBus == nil {
		return
	}

	subscribeCh, cancel := rc.client.Subscribe(
		river.EventKindJobCompleted,
		river.EventKindJobSnoozed,
	)
	rc.cancelSubscribe = cancel
	rc.forwardingDone = make(chan struct{})

	go func() {
		defer close(rc.forwardingDone)
		for re := range subscribeCh {
			var eventType EventType
			switch re.Kind {
			case river.EventKindJobCompleted:
				eventType = EventJobCompleted
			case river.EventKindJobSnoozed:
				eventType = EventJobSnoozed
			default:
				continue
			}

			evt := Event{
				Type:      eventType,
				JobKind:   re.Job.Kind,
				JobID:     re.Job.ID,
				Queue:     re.Job.Queue,
				Attempt:   re.Job.Attempt,
				Timestamp: time.Now(),
			}
			evt.UserID, evt.DocUUID = extractDocumentUserID(re.Job.Kind, re.Job.EncodedArgs)
			rc.eventBus.Publish(evt)
		}
	}()
}

// Stop gracefully shuts down the River client, waiting for in-progress jobs.
// No-op when the client is in insert-only mode.
func (rc *RiverClient) Stop(ctx context.Context) error {
	if rc.insertOnly {
		return nil
	}
	if rc.cancelSubscribe != nil {
		rc.cancelSubscribe()
		<-rc.forwardingDone
	}
	if err := rc.client.Stop(ctx); err != nil {
		return fmt.Errorf("stopping river client: %w", err)
	}
	return nil
}

// Insert enqueues a job. Satisfies the service.JobInserter interface.
func (rc *RiverClient) Insert(ctx context.Context, args river.JobArgs, opts *river.InsertOpts) (*rivertype.JobInsertResult, error) {
	result, err := rc.client.Insert(ctx, args, opts)
	if err == nil {
		if rc.eventBus != nil {
			evt := Event{
				Type:      EventJobDispatched,
				JobKind:   args.Kind(),
				JobID:     result.Job.ID,
				Queue:     result.Job.Queue,
				Timestamp: time.Now(),
			}
			if dea, ok := args.(DocumentExtractArgs); ok {
				evt.UserID = dea.UserID
				evt.DocUUID = dea.DocUUID
			}
			rc.eventBus.Publish(evt)
		}
		if rc.metrics != nil {
			rc.metrics.QueueJobsDispatched.WithLabelValues(result.Job.Queue, args.Kind()).Inc()
		}
	}
	if err != nil {
		return nil, fmt.Errorf("inserting river job: %w", err)
	}
	return result, nil
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
		[]string{"available", "running", "retryable", "discarded", "cancelled"}, //nolint:misspell // "cancelled" is the River queue state name
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
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterating queue stats rows: %w", err)
	}
	return stats, nil
}

// riverErrorHandler implements river.ErrorHandler to publish failure events.
type riverErrorHandler struct {
	eventBus EventPublisher
	metrics  *observability.Metrics
	logger   *slog.Logger
}

// HandleError logs and records metrics for a failed job.
func (h *riverErrorHandler) HandleError(ctx context.Context, job *rivertype.JobRow, jobErr error) *river.ErrorHandlerResult {
	h.logger.Error("job failed",
		"job_id", job.ID,
		"kind", job.Kind,
		"queue", job.Queue,
		"attempt", job.Attempt,
		"error", jobErr,
	)
	if h.eventBus != nil {
		evt := Event{
			Type:      EventJobFailed,
			JobKind:   job.Kind,
			JobID:     job.ID,
			Queue:     job.Queue,
			Attempt:   job.Attempt,
			Error:     "job processing failed",
			Timestamp: time.Now(),
		}
		evt.UserID, evt.DocUUID = extractDocumentUserID(job.Kind, job.EncodedArgs)
		h.eventBus.Publish(evt)
	}
	if h.metrics != nil {
		h.metrics.QueueJobsFailed.WithLabelValues(job.Queue, job.Kind).Inc()
	}
	return nil
}

// HandlePanic logs and records metrics for a panicking job.
func (h *riverErrorHandler) HandlePanic(ctx context.Context, job *rivertype.JobRow, panicVal any, trace string) *river.ErrorHandlerResult {
	h.logger.Error("job panicked",
		"job_id", job.ID,
		"kind", job.Kind,
		"queue", job.Queue,
		"attempt", job.Attempt,
		"panic", panicVal,
	)
	if h.eventBus != nil {
		evt := Event{
			Type:      EventJobFailed,
			JobKind:   job.Kind,
			JobID:     job.ID,
			Queue:     job.Queue,
			Attempt:   job.Attempt,
			Error:     "job panicked",
			Timestamp: time.Now(),
		}
		evt.UserID, evt.DocUUID = extractDocumentUserID(job.Kind, job.EncodedArgs)
		h.eventBus.Publish(evt)
	}
	return nil
}
