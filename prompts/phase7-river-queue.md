# Phase 7: River Queue Migration

You are implementing DocuMCP-go. Run `/memory-bank` first to load project context.

## Your Task

Replace the in-process goroutine worker pool (`internal/queue/worker.go`) and cron scheduler (`internal/scheduler/`) with [River](https://riverqueue.com), a Postgres-native job queue for Go. Add SSE for real-time events, queue admin API, and Prometheus metrics for queue operations.

**Primary agent**: `go-writer`
**Secondary agents**: `test-generator`

## Why River

The current system has critical gaps:
- `queue.Pool` — 3 goroutines, 100-slot channel buffer, no persistence, no retry, jobs lost on restart
- `scheduler.Scheduler` — wraps `robfig/cron/v3`, no job history, no failure tracking
- No priority queues, no failed job storage, no auto-scaling, no real-time events

River provides: Postgres-backed persistence, exponential backoff retries, named priority queues, periodic jobs (replaces cron), job lifecycle hooks, and native pgx support.

## Architecture Decisions

1. **Dual pool connections**: River requires `pgxpool.Pool` (native pgx). The existing `sqlx.DB` (pgx stdlib driver) continues for all non-River database operations. Both share the same Postgres instance.
2. **Three named queues**: `high` (extraction, 10 workers), `default` (indexing, 5 workers), `low` (reindex/maintenance, 2 workers)
3. **Retry policy**: 3 attempts with exponential backoff — 60s, 120s, 300s — via custom `NextRetry` on each worker
4. **Scheduler replacement**: River's `PeriodicJob` replaces all 9 cron jobs. Schedule strings stay in `config.SchedulerConfig`.
5. **Event bus**: In-memory pub/sub (`EventBus`) for SSE. River job lifecycle hooks publish events; SSE handler subscribes.
6. **JobInserter interface**: Defined in `internal/service/` (where consumed, not where implemented) for testability

## Steps

### 1. Add Dependencies

```bash
go get github.com/riverqueue/river
go get github.com/riverqueue/river/riverdriver/riverpgxv5
go get github.com/riverqueue/river/rivermigrate
go get github.com/jackc/pgx/v5/pgxpool
```

### 2. pgxpool Factory — `internal/database/pgxpool.go`

Create a `NewPgxPool` function that creates a `*pgxpool.Pool`:

```go
package database

import (
    "context"
    "fmt"
    "time"

    "github.com/jackc/pgx/v5/pgxpool"
)

// NewPgxPool creates a pgx connection pool for use with River queue.
// It runs alongside the existing sqlx.DB; both connect to the same Postgres instance.
func NewPgxPool(ctx context.Context, dsn string, maxConns int32) (*pgxpool.Pool, error) {
    cfg, err := pgxpool.ParseConfig(dsn)
    if err != nil {
        return nil, fmt.Errorf("parsing pgxpool config: %w", err)
    }
    cfg.MaxConns = maxConns
    cfg.MinConns = 2
    cfg.MaxConnLifetime = 30 * time.Minute
    cfg.MaxConnIdleTime = 5 * time.Minute

    pool, err := pgxpool.NewWithConfig(ctx, cfg)
    if err != nil {
        return nil, fmt.Errorf("creating pgxpool: %w", err)
    }

    if err := pool.Ping(ctx); err != nil {
        pool.Close()
        return nil, fmt.Errorf("pinging pgxpool: %w", err)
    }
    return pool, nil
}
```

### 3. River Schema Migrations — `internal/database/river_migrate.go`

Run River's own migrations at startup (after goose migrations):

```go
package database

import (
    "context"
    "fmt"

    "github.com/jackc/pgx/v5/pgxpool"
    "github.com/riverqueue/river/rivermigrate"
    "github.com/riverqueue/river/riverdriver/riverpgxv5"
)

// RunRiverMigrations applies River's internal schema migrations.
func RunRiverMigrations(ctx context.Context, pool *pgxpool.Pool) error {
    migrator, err := rivermigrate.New(riverpgxv5.New(pool), nil)
    if err != nil {
        return fmt.Errorf("creating river migrator: %w", err)
    }
    _, err = migrator.Migrate(ctx, rivermigrate.DirectionUp, nil)
    if err != nil {
        return fmt.Errorf("running river migrations: %w", err)
    }
    return nil
}
```

### 4. Job Definitions — `internal/queue/jobs.go`

Define three job arg types. Each implements `river.JobArgs` (Kind, InsertOpts):

```go
package queue

import "github.com/riverqueue/river"

// DocumentExtractArgs dispatches document content extraction.
type DocumentExtractArgs struct {
    DocumentID int64  `json:"document_id"`
    DocUUID    string `json:"doc_uuid"`
}

func (DocumentExtractArgs) Kind() string { return "document_extract" }

func (DocumentExtractArgs) InsertOpts() river.InsertOpts {
    return river.InsertOpts{
        Queue:    "high",
        Priority: 1,
    }
}

// DocumentIndexArgs dispatches document search indexing.
type DocumentIndexArgs struct {
    DocumentID int64  `json:"document_id"`
    DocUUID    string `json:"doc_uuid"`
}

func (DocumentIndexArgs) Kind() string { return "document_index" }

func (DocumentIndexArgs) InsertOpts() river.InsertOpts {
    return river.InsertOpts{
        Queue:    "default",
        Priority: 2,
    }
}

// ReindexAllArgs dispatches a full reindex of all documents.
type ReindexAllArgs struct{}

func (ReindexAllArgs) Kind() string { return "reindex_all" }

func (ReindexAllArgs) InsertOpts() river.InsertOpts {
    return river.InsertOpts{
        Queue:    "low",
        Priority: 4,
        Unique:   true,
    }
}
```

Add additional job types for each scheduler job being migrated:
- `SyncKiwixArgs`, `SyncConfluenceArgs`, `SyncGitTemplatesArgs`
- `CleanupOAuthTokensArgs`, `CleanupOrphanedFilesArgs`, `VerifySearchIndexArgs`
- `PurgeSoftDeletedArgs`, `CleanupDisabledZimArgs`, `HealthCheckServicesArgs`

Each should use queue `"low"` with `Priority: 4` and `Unique: true`.

### 5. Workers — `internal/queue/workers.go`

Create workers that implement `river.Worker[T]`. Each provides `Work(ctx, *river.Job[T]) error` and `NextRetry(*river.Job[T]) time.Time`:

```go
package queue

import (
    "context"
    "time"

    "github.com/riverqueue/river"
)

// DocumentExtractWorker processes document extraction jobs.
type DocumentExtractWorker struct {
    river.WorkerDefaults[DocumentExtractArgs]
    Pipeline DocumentProcessor
}

// DocumentProcessor is the interface for document processing (extraction + indexing).
// Implemented by *service.DocumentPipeline.
type DocumentProcessor interface {
    ProcessDocument(ctx context.Context, docID int64) error
}

func (w *DocumentExtractWorker) Work(ctx context.Context, job *river.Job[DocumentExtractArgs]) error {
    return w.Pipeline.ProcessDocument(ctx, job.Args.DocumentID)
}

func (w *DocumentExtractWorker) NextRetry(job *river.Job[DocumentExtractArgs]) time.Time {
    backoffs := []time.Duration{60 * time.Second, 120 * time.Second, 300 * time.Second}
    attempt := max(0, min(job.Attempt-1, len(backoffs)-1))
    return time.Now().Add(backoffs[attempt])
}
```

Follow the same pattern for `DocumentIndexWorker` (needs a `DocumentIndexer` interface) and `ReindexAllWorker`.

For scheduler migration workers, inject the same dependencies the scheduler currently uses. Define minimal interfaces in this file (where consumed):

```go
// ExternalServiceFinder retrieves enabled external services by type.
type ExternalServiceFinder interface {
    FindEnabledByType(ctx context.Context, serviceType string) ([]model.ExternalService, error)
}
```

Create workers for all 9 scheduler jobs: `SyncKiwixWorker`, `SyncConfluenceWorker`, `SyncGitTemplatesWorker`, `CleanupOAuthTokensWorker`, `CleanupOrphanedFilesWorker`, `VerifySearchIndexWorker`, `PurgeSoftDeletedWorker`, `CleanupDisabledZimWorker`, `HealthCheckServicesWorker`.

Each worker's `Work` method should contain the same logic currently in the corresponding scheduler method (e.g., `scheduler.syncKiwix()` → `SyncKiwixWorker.Work()`), with a 5-minute timeout context.

### 6. Event Bus — `internal/queue/events.go`

In-memory pub/sub for SSE. Thread-safe, supports multiple subscribers:

```go
package queue

import (
    "encoding/json"
    "sync"
    "time"
)

// EventType identifies the kind of queue event.
type EventType string

const (
    EventJobDispatched EventType = "job.dispatched"
    EventJobCompleted  EventType = "job.completed"
    EventJobFailed     EventType = "job.failed"
    EventJobRetrying   EventType = "job.retrying"
)

// Event represents a queue lifecycle event.
type Event struct {
    Type      EventType `json:"type"`
    JobKind   string    `json:"job_kind"`
    JobID     int64     `json:"job_id"`
    Queue     string    `json:"queue"`
    Attempt   int       `json:"attempt,omitempty"`
    Error     string    `json:"error,omitempty"`
    Timestamp time.Time `json:"timestamp"`
}

// EventBus provides in-memory pub/sub for queue events.
type EventBus struct {
    mu          sync.RWMutex
    subscribers map[string]chan Event
}

func NewEventBus() *EventBus {
    return &EventBus{subscribers: make(map[string]chan Event)}
}

// Subscribe returns a channel that receives events. Call Unsubscribe with the
// returned ID when done.
func (eb *EventBus) Subscribe(id string) <-chan Event {
    eb.mu.Lock()
    defer eb.mu.Unlock()
    ch := make(chan Event, 64)
    eb.subscribers[id] = ch
    return ch
}

func (eb *EventBus) Unsubscribe(id string) {
    eb.mu.Lock()
    defer eb.mu.Unlock()
    if ch, ok := eb.subscribers[id]; ok {
        close(ch)
        delete(eb.subscribers, id)
    }
}

// Publish sends an event to all subscribers (non-blocking, drops if buffer full).
func (eb *EventBus) Publish(event Event) {
    eb.mu.RLock()
    defer eb.mu.RUnlock()
    data, _ := json.Marshal(event)
    _ = data // used by SSE handler
    for _, ch := range eb.subscribers {
        select {
        case ch <- event:
        default:
            // Drop event if subscriber is slow
        }
    }
}
```

### 7. River Client Wrapper — `internal/queue/river.go`

Wraps `*river.Client[pgx.Tx]`, registers workers, configures queues, wires event hooks:

```go
package queue

import (
    "context"
    "fmt"
    "log/slog"
    "time"

    "github.com/jackc/pgx/v5/pgxpool"
    "github.com/riverqueue/river"
    "github.com/riverqueue/river/riverdriver/riverpgxv5"
)

// RiverClient wraps River's client with application-specific configuration.
type RiverClient struct {
    client   *river.Client[pgx.Tx]
    pool     *pgxpool.Pool
    eventBus *EventBus
    logger   *slog.Logger
}

// RiverConfig holds configuration for the River queue client.
type RiverConfig struct {
    Pool     *pgxpool.Pool
    EventBus *EventBus
    Logger   *slog.Logger

    // Worker dependencies (injected into workers)
    Pipeline    DocumentProcessor
    Indexer     DocumentIndexer
    // ... other deps for scheduler workers
}

func NewRiverClient(cfg RiverConfig) (*RiverClient, error) {
    workers := river.NewWorkers()

    // Register document workers
    river.AddWorker(workers, &DocumentExtractWorker{Pipeline: cfg.Pipeline})
    river.AddWorker(workers, &DocumentIndexWorker{Indexer: cfg.Indexer})
    river.AddWorker(workers, &ReindexAllWorker{/* deps */})

    // Register scheduler migration workers
    // river.AddWorker(workers, &SyncKiwixWorker{...})
    // ... register all 9 scheduler workers

    client, err := river.NewClient(riverpgxv5.New(cfg.Pool), &river.Config{
        Queues: map[string]river.QueueConfig{
            "high":    {MaxWorkers: 10},
            "default": {MaxWorkers: 5},
            "low":     {MaxWorkers: 2},
        },
        Workers: workers,
        Logger:  slog.NewLogLogger(cfg.Logger.Handler(), slog.LevelInfo),
        ErrorHandler: &riverErrorHandler{
            eventBus: cfg.EventBus,
            logger:   cfg.Logger,
        },
        // PeriodicJobs configured separately via buildPeriodicJobs()
    })
    if err != nil {
        return nil, fmt.Errorf("creating river client: %w", err)
    }

    return &RiverClient{
        client:   client,
        pool:     cfg.Pool,
        eventBus: cfg.EventBus,
        logger:   cfg.Logger,
    }, nil
}

func (rc *RiverClient) Start(ctx context.Context) error {
    return rc.client.Start(ctx)
}

func (rc *RiverClient) Stop(ctx context.Context) error {
    return rc.client.Stop(ctx)
}

// Insert enqueues a job. Satisfies the JobInserter interface.
func (rc *RiverClient) Insert(ctx context.Context, args river.JobArgs, opts *river.InsertOpts) (*river.JobInsertResult, error) {
    result, err := rc.client.Insert(ctx, args, opts)
    if err == nil && rc.eventBus != nil {
        rc.eventBus.Publish(Event{
            Type:      EventJobDispatched,
            JobKind:   args.Kind(),
            JobID:     result.Job.ID,
            Timestamp: time.Now(),
        })
    }
    return result, err
}
```

### 8. Periodic Jobs — `internal/queue/periodic.go`

Convert all 9 cron schedule strings from `config.SchedulerConfig` into River `PeriodicJob` entries:

```go
package queue

import (
    "github.com/riverqueue/river"
    "git.999.haus/chris/DocuMCP-go/internal/config"
)

// BuildPeriodicJobs converts scheduler config into River periodic jobs.
// Empty schedule strings are skipped (same behavior as the old scheduler).
func BuildPeriodicJobs(cfg config.SchedulerConfig) []*river.PeriodicJob {
    var jobs []*river.PeriodicJob

    schedules := []struct {
        schedule string
        args     river.JobArgs
    }{
        {cfg.KiwixSchedule, SyncKiwixArgs{}},
        {cfg.ConfluenceSchedule, SyncConfluenceArgs{}},
        {cfg.GitSchedule, SyncGitTemplatesArgs{}},
        {cfg.OAuthCleanupSchedule, CleanupOAuthTokensArgs{}},
        {cfg.OrphanedFilesSchedule, CleanupOrphanedFilesArgs{}},
        {cfg.SearchVerifySchedule, VerifySearchIndexArgs{}},
        {cfg.SoftDeletePurgeSchedule, PurgeSoftDeletedArgs{}},
        {cfg.ZimCleanupSchedule, CleanupDisabledZimArgs{}},
        {cfg.HealthCheckSchedule, HealthCheckServicesArgs{}},
    }

    for _, s := range schedules {
        if s.schedule == "" {
            continue
        }
        args := s.args
        jobs = append(jobs, river.NewPeriodicJob(
            river.PeriodicInterval(parseCronToDuration(s.schedule)),
            func() (river.JobArgs, *river.InsertOpts) {
                return args, nil
            },
            &river.PeriodicJobOpts{RunOnStart: false},
        ))
    }

    return jobs
}
```

Note: River supports cron expressions natively via `river.PeriodicSchedule`. Use `river.CronSchedule(cronExpr)` instead of `PeriodicInterval` if preferred — check River docs for the exact API.

### 9. Document Recovery — `internal/queue/recovery.go`

On startup, re-dispatch jobs for documents stuck in intermediate states:

```go
package queue

import (
    "context"
    "log/slog"
)

// DocumentFinder finds documents by status for recovery.
type DocumentFinder interface {
    FindByStatus(ctx context.Context, status string) ([]struct{ ID int64; UUID string }, error)
}

// RecoverStuckDocuments re-dispatches jobs for documents in "uploaded" or
// "extracted" states. Call after River client starts.
func RecoverStuckDocuments(ctx context.Context, inserter JobInserter, finder DocumentFinder, logger *slog.Logger) {
    // Find documents stuck in "uploaded" → re-dispatch extraction
    // Find documents stuck in "extracted" → re-dispatch indexing
    // Log each recovery action
}
```

**Important**: The `DocumentFinder` interface should be defined here (where consumed). The `DocumentRepository` already has methods to query by status — you may need to add a `FindByStatus` method or use existing list methods with a status filter.

### 10. SSE Handler — `internal/handler/api/sse_handler.go`

Server-Sent Events endpoint at `GET /api/events/stream`:

```go
package api

import (
    "fmt"
    "net/http"

    "github.com/google/uuid"
    "git.999.haus/chris/DocuMCP-go/internal/queue"
)

// SSEHandler streams real-time queue events via Server-Sent Events.
type SSEHandler struct {
    eventBus *queue.EventBus
}

func NewSSEHandler(eventBus *queue.EventBus) *SSEHandler {
    return &SSEHandler{eventBus: eventBus}
}

func (h *SSEHandler) Stream(w http.ResponseWriter, r *http.Request) {
    flusher, ok := w.(http.Flusher)
    if !ok {
        http.Error(w, "streaming not supported", http.StatusInternalServerError)
        return
    }

    w.Header().Set("Content-Type", "text/event-stream")
    w.Header().Set("Cache-Control", "no-cache")
    w.Header().Set("Connection", "keep-alive")
    w.Header().Set("X-Accel-Buffering", "no")

    subID := uuid.New().String()
    events := h.eventBus.Subscribe(subID)
    defer h.eventBus.Unsubscribe(subID)

    for {
        select {
        case <-r.Context().Done():
            return
        case event, ok := <-events:
            if !ok {
                return
            }
            // Marshal event to JSON and write as SSE
            // fmt.Fprintf(w, "event: %s\ndata: %s\n\n", event.Type, jsonData)
            flusher.Flush()
        }
    }
}
```

### 11. Queue Admin API — `internal/handler/api/queue_handler.go`

Failed job inspection, retry, and deletion:

```go
package api

// QueueHandler provides REST API for queue management.
// GET  /api/admin/queue/stats   — queue depth, throughput, worker counts
// GET  /api/admin/queue/failed  — list failed jobs (paginated)
// POST /api/admin/queue/failed/{id}/retry — retry a specific failed job
// DELETE /api/admin/queue/failed/{id}     — delete a failed job
```

Use River's `rivertype.JobListParams` to query failed jobs. Use `river.Client.JobRetry` and `river.Client.JobCancel` for management operations.

### 12. Queue Prometheus Metrics — `internal/observability/metrics.go`

Add queue-specific counters to the existing `Metrics` struct:

```go
// Add to Metrics struct:
QueueJobsDispatched *prometheus.CounterVec  // labels: queue, job_kind
QueueJobsCompleted  *prometheus.CounterVec  // labels: queue, job_kind
QueueJobsFailed     *prometheus.CounterVec  // labels: queue, job_kind
QueueDepth          *prometheus.GaugeVec    // labels: queue
```

Register these in `NewMetrics()` with namespace `documcp`, subsystem `queue`.

Increment these counters from the River event hooks (in `RiverClient`) and from the `EventBus.Publish` calls.

### 13. Modify Document Pipeline — `internal/service/document_pipeline.go`

Replace `*queue.Pool` with a `JobInserter` interface:

```go
// JobInserter inserts jobs into the queue. Defined here (where consumed).
type JobInserter interface {
    Insert(ctx context.Context, args river.JobArgs, opts *river.InsertOpts) (*river.JobInsertResult, error)
}
```

Update `DocumentPipeline` struct:
- Change field `pool *queue.Pool` → `inserter JobInserter`
- Change `NewDocumentPipeline` parameter from `pool *queue.Pool` to `inserter JobInserter`
- Update `dispatchExtraction` to call `inserter.Insert(ctx, queue.DocumentExtractArgs{...}, nil)`
- Update `dispatchIndexing` to call `inserter.Insert(ctx, queue.DocumentIndexArgs{...}, nil)`
- Remove the import of `git.999.haus/chris/DocuMCP-go/internal/queue` — use the local `JobInserter` interface instead

**Important**: The `dispatchExtraction` method currently creates a closure around `p.ProcessDocument`. With River, the worker itself calls `ProcessDocument` — so dispatch just inserts the args. The `dispatchIndexing` method similarly just inserts `DocumentIndexArgs`.

### 14. Modify App Wiring — `internal/app/app.go`

Update the `App` struct and `New()` function:

1. **Add fields** to `App`:
   - `PgxPool *pgxpool.Pool`
   - `RiverClient *queue.RiverClient`
   - `EventBus *queue.EventBus`

2. **Remove fields**:
   - `WorkerPool *queue.Pool`
   - `Scheduler *scheduler.Scheduler`

3. **In `New()`**, after database setup:
   ```go
   // Create pgxpool for River
   pgxPool, err := database.NewPgxPool(context.Background(), cfg.DatabaseDSN(), 10)
   // Run River migrations
   database.RunRiverMigrations(context.Background(), pgxPool)
   // Create EventBus
   eventBus := queue.NewEventBus()
   // Create RiverClient with worker dependencies
   riverClient, err := queue.NewRiverClient(queue.RiverConfig{...})
   ```

4. **Replace** `workerPool` usage with `riverClient` when creating `DocumentPipeline`

5. **Create SSE and Queue handlers**:
   ```go
   sseH := apihandler.NewSSEHandler(eventBus)
   queueH := apihandler.NewQueueHandler(riverClient)
   ```

6. **Add to `Deps`** struct: `SSEHandler`, `QueueHandler`

7. **In `Start()`**: call `riverClient.Start(ctx)` instead of `scheduler.Start()`

8. **In `Close()`**: call `riverClient.Stop(ctx)` and `pgxPool.Close()` instead of scheduler/pool shutdown

9. **Remove imports**: `internal/queue` (old pool), `internal/scheduler`

### 15. Modify Routes — `internal/server/routes.go`

Add new endpoints:

```go
// Inside the /api route group:
// SSE events (bearer token protected like other API routes)
if deps.SSEHandler != nil {
    r.Get("/events/stream", deps.SSEHandler.Stream)
}

// Queue admin endpoints (require admin)
if deps.QueueHandler != nil {
    r.Route("/admin/queue", func(r chi.Router) {
        // Add admin auth middleware here
        r.Get("/stats", deps.QueueHandler.Stats)
        r.Get("/failed", deps.QueueHandler.ListFailed)
        r.Post("/failed/{id}/retry", deps.QueueHandler.RetryFailed)
        r.Delete("/failed/{id}", deps.QueueHandler.DeleteFailed)
    })
}
```

Add `SSEHandler` and `QueueHandler` to the `Deps` struct.

### 16. Delete Old Files

After everything works:

- `internal/queue/worker.go` — replaced by River
- `internal/queue/worker_test.go` — if it exists
- `internal/scheduler/scheduler.go` — replaced by periodic jobs
- `internal/scheduler/cleanup_jobs.go` — logic moved to workers
- `internal/scheduler/adapters.go` — logic moved to workers
- Any test files in `internal/scheduler/`

Run `go mod tidy` to remove `github.com/robfig/cron/v3`.

### 17. Tests

Write tests for:

- **Job definitions** — verify Kind(), InsertOpts() return correct values
- **EventBus** — subscribe, publish, unsubscribe, slow subscriber drops
- **Workers** — mock DocumentProcessor, verify Work() calls correct methods
- **Recovery** — mock finder, verify correct jobs dispatched
- **SSE handler** — httptest recorder, verify event stream format
- **Queue handler** — mock river client, verify CRUD operations
- **Integration** (if testcontainers available): real River + Postgres, insert job → verify completion

Use table-driven tests with `t.Run` subtests. Mock interfaces, not concrete types.

### 18. Verification

```bash
go build ./...
go test -race -cover ./...
golangci-lint run
```

Confirm:
- No references to `queue.Pool`, `queue.NewPool`, `queue.Job`, or `queue.Dispatch` remain
- No references to `scheduler.Scheduler`, `scheduler.New`, `robfig/cron` remain
- All 9 periodic jobs are registered
- SSE endpoint streams events
- Queue admin API returns failed jobs
- Prometheus metrics include queue counters

## Commit Checkpoints

1. **River setup + job definitions**: `pgxpool.go`, `river_migrate.go`, `jobs.go`, `events.go`, `river.go`
2. **Pipeline migration**: modify `document_pipeline.go`, create workers for extract/index
3. **Scheduler migration**: create all 9 scheduler workers, `periodic.go`, `recovery.go`
4. **SSE + queue API + metrics**: `sse_handler.go`, `queue_handler.go`, metrics additions
5. **Tests + cleanup**: delete old files, run `go mod tidy`, verify all tests pass

Use `/commit` after each checkpoint.
