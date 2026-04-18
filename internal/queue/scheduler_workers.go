package queue

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"github.com/riverqueue/river"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"

	"github.com/c-premus/documcp/internal/client/git"
	"github.com/c-premus/documcp/internal/client/kiwix"
	"github.com/c-premus/documcp/internal/model"
	"github.com/c-premus/documcp/internal/observability"
	"github.com/c-premus/documcp/internal/repository"
	"github.com/c-premus/documcp/internal/security"
	"github.com/c-premus/documcp/internal/storage"
)

// --- Interfaces (defined where consumed) ---.

// ExternalServiceFinder retrieves enabled external services by type.
type ExternalServiceFinder interface {
	FindEnabledByType(ctx context.Context, serviceType string) ([]model.ExternalService, error)
}

// ExternalServiceHealthChecker checks health of external services.
type ExternalServiceHealthChecker interface {
	FindAllEnabled(ctx context.Context) ([]model.ExternalService, error)
	UpdateHealthStatus(ctx context.Context, id int64, status model.ExternalServiceStatus, latencyMs int, lastError string) error
}

// OAuthTokenPurger purges expired OAuth tokens and scope grants.
type OAuthTokenPurger interface {
	PurgeExpiredTokens(ctx context.Context, retentionDays int) (int64, error)
	DeleteExpiredScopeGrants(ctx context.Context) (int64, error)
}

// DocumentRepoDeps provides document repository methods needed by cleanup workers.
type DocumentRepoDeps interface {
	ListActiveFilePaths(ctx context.Context) ([]repository.DocumentFilePath, error)
	PurgeSoftDeleted(ctx context.Context, olderThan time.Duration) ([]repository.DocumentFilePath, error)
}

// ZimArchiveRepoDeps provides ZIM archive repository methods needed by scheduler workers.
type ZimArchiveRepoDeps interface {
	UpsertFromCatalog(ctx context.Context, serviceID int64, upsert repository.ZimArchiveUpsert) error
	DisableOrphaned(ctx context.Context, serviceID int64, activeNames []string) (int, error)
}

// GitTemplateRepoDeps provides Git template repository methods needed by scheduler workers.
type GitTemplateRepoDeps interface {
	List(ctx context.Context, category string, limit, offset int) ([]model.GitTemplate, error)
	UpdateSyncStatus(ctx context.Context, templateID int64, status model.GitTemplateStatus, commitSHA string, fileCount int, totalSize int64, errMsg string) error
	ReplaceFiles(ctx context.Context, templateID int64, files []repository.GitTemplateFileInsert) error
	UpdateSearchContent(ctx context.Context, templateID int64, readmeContent, filePaths string) error
}

// SchedulerDeps holds all dependencies needed by scheduler workers.
type SchedulerDeps struct {
	Services          ExternalServiceFinder
	HealthChecker     ExternalServiceHealthChecker
	ZimRepo           ZimArchiveRepoDeps
	GitRepo           GitTemplateRepoDeps
	OAuthRepo         OAuthTokenPurger
	DocRepo           DocumentRepoDeps
	Metrics           *observability.Metrics
	Blob              storage.Blob
	GitTempDir        string
	Logger            *slog.Logger
	GitMaxFileSize    int64
	GitMaxTotalSize   int64
	SSRFDialerTimeout time.Duration
	KiwixConfig       kiwix.ClientConfig
}

// workerTracer is the shared tracer for River worker spans.
var workerTracer = otel.Tracer("documcp/worker")

// --- Sync Workers ---.

// SyncKiwixWorker syncs Kiwix ZIM archives from external services.
type SyncKiwixWorker struct {
	river.WorkerDefaults[SyncKiwixArgs]
	Deps SchedulerDeps
}

// Work executes the SyncKiwixWorker job, syncing ZIM archives from the Kiwix catalog.
func (w *SyncKiwixWorker) Work(ctx context.Context, job *river.Job[SyncKiwixArgs]) (retErr error) {
	ctx, span := workerTracer.Start(ctx, "job."+job.Kind,
		trace.WithSpanKind(trace.SpanKindInternal),
		trace.WithAttributes(attribute.String("job.kind", job.Kind)),
	)
	defer func() {
		if retErr != nil {
			span.RecordError(retErr)
			span.SetStatus(codes.Error, retErr.Error())
		}
		span.End()
	}()

	start := time.Now()
	ctx, cancel := context.WithTimeout(ctx, 5*time.Minute)
	defer cancel()

	logger := w.Deps.Logger.With("job", "kiwix")
	logger.Info("starting kiwix sync")

	services, err := w.Deps.Services.FindEnabledByType(ctx, "kiwix")
	if err != nil {
		return fmt.Errorf("finding enabled kiwix services: %w", err)
	}

	if len(services) == 0 {
		logger.Info("no enabled kiwix services found")
		return nil
	}

	for i := range services {
		svc := &services[i]
		svcLogger := logger.With("service_id", svc.ID, "base_url", svc.BaseURL)

		kiwixCfg := w.Deps.KiwixConfig
		kiwixCfg.BaseURL = svc.BaseURL
		client, clientErr := kiwix.NewClient(kiwixCfg, svcLogger)
		if clientErr != nil {
			svcLogger.Error("kiwix client URL rejected", "error", clientErr)
			continue
		}

		entries, err := client.FetchCatalog(ctx)
		if err != nil {
			svcLogger.Error("fetching kiwix catalog", "error", err)
			if w.Deps.HealthChecker != nil {
				if hErr := w.Deps.HealthChecker.UpdateHealthStatus(ctx, svc.ID, model.ExternalServiceStatusUnhealthy, 0, err.Error()); hErr != nil {
					svcLogger.Error("updating health status", "error", hErr)
				}
			}
			continue
		}

		if err := kiwix.Sync(ctx, kiwix.SyncParams{
			ServiceID: svc.ID,
			Entries:   entries,
			Repo:      &kiwixRepoAdapter{repo: w.Deps.ZimRepo},
			Logger:    svcLogger,
		}); err != nil {
			svcLogger.Error("kiwix sync failed", "error", err)
			if w.Deps.HealthChecker != nil {
				if hErr := w.Deps.HealthChecker.UpdateHealthStatus(ctx, svc.ID, model.ExternalServiceStatusUnhealthy, 0, err.Error()); hErr != nil {
					svcLogger.Error("updating health status", "error", hErr)
				}
			}
			continue
		}

		if w.Deps.HealthChecker != nil {
			if hErr := w.Deps.HealthChecker.UpdateHealthStatus(ctx, svc.ID, model.ExternalServiceStatusHealthy, 0, ""); hErr != nil {
				svcLogger.Error("updating health status", "error", hErr)
			}
		}
		svcLogger.Info("kiwix service sync completed", "entries", len(entries))
	}
	recordJobCompleted(w.Deps.Metrics, job.Queue, job.Kind, time.Since(start))
	return nil
}

// SyncGitTemplatesWorker syncs Git template repositories.
type SyncGitTemplatesWorker struct {
	river.WorkerDefaults[SyncGitTemplatesArgs]
	Deps SchedulerDeps
}

// Work executes the SyncGitTemplatesWorker job, syncing Git template repositories.
func (w *SyncGitTemplatesWorker) Work(ctx context.Context, job *river.Job[SyncGitTemplatesArgs]) (retErr error) {
	ctx, span := workerTracer.Start(ctx, "job."+job.Kind,
		trace.WithSpanKind(trace.SpanKindInternal),
		trace.WithAttributes(attribute.String("job.kind", job.Kind)),
	)
	defer func() {
		if retErr != nil {
			span.RecordError(retErr)
			span.SetStatus(codes.Error, retErr.Error())
		}
		span.End()
	}()

	start := time.Now()
	ctx, cancel := context.WithTimeout(ctx, 5*time.Minute)
	defer cancel()

	logger := w.Deps.Logger.With("job", "git")
	logger.Info("starting git template sync")

	templates, err := w.Deps.GitRepo.List(ctx, "", 100, 0)
	if err != nil {
		return fmt.Errorf("listing git templates: %w", err)
	}

	if len(templates) == 0 {
		logger.Info("no git templates found")
		return nil
	}

	client := git.NewClient(w.Deps.GitTempDir, w.Deps.GitMaxFileSize, w.Deps.GitMaxTotalSize, logger)

	for i := range templates {
		t := &templates[i]
		tmplLogger := logger.With("template_id", t.ID, "slug", t.Slug)

		syncTmpl, err := toSyncTemplate(*t)
		if err != nil {
			tmplLogger.Error("converting git template", "error", err)
			continue
		}

		if err := git.Sync(ctx, git.SyncParams{
			Template: syncTmpl,
			Client:   client,
			Repo:     &gitRepoAdapter{repo: w.Deps.GitRepo},
			Logger:   tmplLogger,
		}); err != nil {
			tmplLogger.Error("git template sync failed", "error", err)
			continue
		}

		tmplLogger.Info("git template sync completed")
	}
	recordJobCompleted(w.Deps.Metrics, job.Queue, job.Kind, time.Since(start))
	return nil
}

// --- Cleanup Workers ---.

// CleanupOAuthTokensWorker purges expired OAuth tokens.
type CleanupOAuthTokensWorker struct {
	river.WorkerDefaults[CleanupOAuthTokensArgs]
	Deps SchedulerDeps
}

// Work executes the CleanupOAuthTokensWorker job, purging expired OAuth tokens.
func (w *CleanupOAuthTokensWorker) Work(ctx context.Context, job *river.Job[CleanupOAuthTokensArgs]) (retErr error) {
	ctx, span := workerTracer.Start(ctx, "job."+job.Kind,
		trace.WithSpanKind(trace.SpanKindInternal),
		trace.WithAttributes(attribute.String("job.kind", job.Kind)),
	)
	defer func() {
		if retErr != nil {
			span.RecordError(retErr)
			span.SetStatus(codes.Error, retErr.Error())
		}
		span.End()
	}()

	start := time.Now()
	ctx, cancel := context.WithTimeout(ctx, 5*time.Minute)
	defer cancel()

	if w.Deps.OAuthRepo == nil {
		w.Deps.Logger.Warn("skipping OAuth cleanup: oauth repository not configured")
		return nil
	}

	w.Deps.Logger.Info("starting OAuth token cleanup")

	count, err := w.Deps.OAuthRepo.PurgeExpiredTokens(ctx, 7)
	if err != nil {
		return fmt.Errorf("purging expired OAuth tokens: %w", err)
	}

	w.Deps.Logger.Info("OAuth token cleanup completed", "purged_count", count)

	// Also purge expired scope grants.
	grantCount, grantErr := w.Deps.OAuthRepo.DeleteExpiredScopeGrants(ctx)
	if grantErr != nil {
		w.Deps.Logger.Error("purging expired scope grants", "error", grantErr)
	} else if grantCount > 0 {
		w.Deps.Logger.Info("expired scope grants cleanup completed", "purged_count", grantCount)
	}

	recordJobCompleted(w.Deps.Metrics, job.Queue, job.Kind, time.Since(start))
	return nil
}

// CleanupOrphanedFilesWorker removes files not referenced by any active document.
type CleanupOrphanedFilesWorker struct {
	river.WorkerDefaults[CleanupOrphanedFilesArgs]
	Deps SchedulerDeps
}

// Work executes the CleanupOrphanedFilesWorker job, removing unreferenced files from storage.
func (w *CleanupOrphanedFilesWorker) Work(ctx context.Context, job *river.Job[CleanupOrphanedFilesArgs]) (retErr error) {
	ctx, span := workerTracer.Start(ctx, "job."+job.Kind,
		trace.WithSpanKind(trace.SpanKindInternal),
		trace.WithAttributes(attribute.String("job.kind", job.Kind)),
	)
	defer func() {
		if retErr != nil {
			span.RecordError(retErr)
			span.SetStatus(codes.Error, retErr.Error())
		}
		span.End()
	}()

	start := time.Now()
	ctx, cancel := context.WithTimeout(ctx, 5*time.Minute)
	defer cancel()

	if w.Deps.DocRepo == nil || w.Deps.Blob == nil {
		w.Deps.Logger.Warn("skipping orphaned files cleanup: document repository or blob store not configured")
		return nil
	}

	logger := w.Deps.Logger.With("job", "orphaned-files")
	logger.Info("starting orphaned files cleanup")

	activePaths, err := w.Deps.DocRepo.ListActiveFilePaths(ctx)
	if err != nil {
		return fmt.Errorf("listing active file paths: %w", err)
	}

	activeSet := make(map[string]bool, len(activePaths))
	for _, fp := range activePaths {
		activeSet[fp.FilePath] = true
	}

	var deletedCount int
	it := w.Deps.Blob.List(ctx, "")
	for it.Next(ctx) {
		key := it.Key()
		if activeSet[key] {
			continue
		}
		if removeErr := w.Deps.Blob.Delete(ctx, key); removeErr != nil {
			logger.Error("removing orphaned blob", "key", key, "error", removeErr)
			continue
		}
		deletedCount++
	}
	if iterErr := it.Err(); iterErr != nil {
		return fmt.Errorf("listing blobs: %w", iterErr)
	}

	logger.Info("orphaned files cleanup completed", "deleted_count", deletedCount)
	recordJobCompleted(w.Deps.Metrics, job.Queue, job.Kind, time.Since(start))
	return nil
}

// PurgeSoftDeletedWorker permanently removes documents soft-deleted >30 days.
type PurgeSoftDeletedWorker struct {
	river.WorkerDefaults[PurgeSoftDeletedArgs]
	Deps SchedulerDeps
}

// Work executes the PurgeSoftDeletedWorker job, permanently removing documents soft-deleted over 30 days ago.
func (w *PurgeSoftDeletedWorker) Work(ctx context.Context, job *river.Job[PurgeSoftDeletedArgs]) (retErr error) {
	ctx, span := workerTracer.Start(ctx, "job."+job.Kind,
		trace.WithSpanKind(trace.SpanKindInternal),
		trace.WithAttributes(attribute.String("job.kind", job.Kind)),
	)
	defer func() {
		if retErr != nil {
			span.RecordError(retErr)
			span.SetStatus(codes.Error, retErr.Error())
		}
		span.End()
	}()

	start := time.Now()
	ctx, cancel := context.WithTimeout(ctx, 5*time.Minute)
	defer cancel()

	if w.Deps.DocRepo == nil {
		w.Deps.Logger.Warn("skipping soft-delete purge: document repository not configured")
		return nil
	}

	logger := w.Deps.Logger.With("job", "soft-delete-purge")
	logger.Info("starting soft-delete purge")

	purged, err := w.Deps.DocRepo.PurgeSoftDeleted(ctx, 30*24*time.Hour)
	if err != nil {
		return fmt.Errorf("purging soft-deleted documents: %w", err)
	}

	for _, fp := range purged {
		if fp.FilePath == "" {
			continue
		}
		if w.Deps.Blob == nil {
			logger.Warn("skipping purged document removal: blob store not configured", "key", fp.FilePath)
			continue
		}
		if removeErr := w.Deps.Blob.Delete(ctx, fp.FilePath); removeErr != nil {
			logger.Error("removing purged document blob", "key", fp.FilePath, "error", removeErr)
		}
	}

	logger.Info("soft-delete purge completed", "purged_count", len(purged))
	recordJobCompleted(w.Deps.Metrics, job.Queue, job.Kind, time.Since(start))
	return nil
}

// HealthCheckServicesWorker performs HTTP health checks on external services.
type HealthCheckServicesWorker struct {
	river.WorkerDefaults[HealthCheckServicesArgs]
	Deps SchedulerDeps
}

// Work executes the HealthCheckServicesWorker job, performing HTTP health checks on external services.
func (w *HealthCheckServicesWorker) Work(ctx context.Context, job *river.Job[HealthCheckServicesArgs]) (retErr error) {
	ctx, span := workerTracer.Start(ctx, "job."+job.Kind,
		trace.WithSpanKind(trace.SpanKindInternal),
		trace.WithAttributes(attribute.String("job.kind", job.Kind)),
	)
	defer func() {
		if retErr != nil {
			span.RecordError(retErr)
			span.SetStatus(codes.Error, retErr.Error())
		}
		span.End()
	}()

	start := time.Now()
	ctx, cancel := context.WithTimeout(ctx, 5*time.Minute)
	defer cancel()

	if w.Deps.HealthChecker == nil {
		w.Deps.Logger.Warn("skipping health check: health checker not configured")
		return nil
	}

	logger := w.Deps.Logger.With("job", "health-check")
	logger.Info("starting external service health checks")

	services, err := w.Deps.HealthChecker.FindAllEnabled(ctx)
	if err != nil {
		return fmt.Errorf("finding enabled external services: %w", err)
	}

	httpClient := &http.Client{Timeout: 10 * time.Second, Transport: security.SafeTransportAllowPrivate(w.Deps.SSRFDialerTimeout)}

	var healthyCount, unhealthyCount int
	for i := range services {
		svc := &services[i]
		svcLogger := logger.With("service_id", svc.ID, "base_url", svc.BaseURL)

		req, err := http.NewRequestWithContext(ctx, http.MethodGet, svc.BaseURL, http.NoBody)
		if err != nil {
			svcLogger.Error("creating health check request", "error", err)
			if updateErr := w.Deps.HealthChecker.UpdateHealthStatus(ctx, svc.ID, model.ExternalServiceStatusUnhealthy, 0, fmt.Sprintf("creating request: %v", err)); updateErr != nil {
				svcLogger.Error("updating health status", "error", updateErr)
			}
			unhealthyCount++
			continue
		}

		reqStart := time.Now()
		resp, err := httpClient.Do(req)
		latencyMs := int(time.Since(reqStart).Milliseconds())

		if err != nil {
			svcLogger.Warn("health check failed", "error", err, "latency_ms", latencyMs)
			if updateErr := w.Deps.HealthChecker.UpdateHealthStatus(ctx, svc.ID, model.ExternalServiceStatusUnhealthy, latencyMs, err.Error()); updateErr != nil {
				svcLogger.Error("updating health status", "error", updateErr)
			}
			unhealthyCount++
			continue
		}
		_ = resp.Body.Close()

		if resp.StatusCode >= 200 && resp.StatusCode < 300 {
			if updateErr := w.Deps.HealthChecker.UpdateHealthStatus(ctx, svc.ID, model.ExternalServiceStatusHealthy, latencyMs, ""); updateErr != nil {
				svcLogger.Error("updating health status", "error", updateErr)
			}
			healthyCount++
		} else {
			errMsg := fmt.Sprintf("unexpected status code: %d", resp.StatusCode)
			svcLogger.Warn("health check returned non-2xx", "status_code", resp.StatusCode, "latency_ms", latencyMs)
			if updateErr := w.Deps.HealthChecker.UpdateHealthStatus(ctx, svc.ID, model.ExternalServiceStatusUnhealthy, latencyMs, errMsg); updateErr != nil {
				svcLogger.Error("updating health status", "error", updateErr)
			}
			unhealthyCount++
		}
	}

	logger.Info("external service health checks completed",
		"total", len(services),
		"healthy", healthyCount,
		"unhealthy", unhealthyCount,
	)
	recordJobCompleted(w.Deps.Metrics, job.Queue, job.Kind, time.Since(start))
	return nil
}

// --- Helpers (moved from scheduler package) ---.

// toSyncTemplate converts a model.GitTemplate to the git.SyncTemplate type.
func toSyncTemplate(t model.GitTemplate) (git.SyncTemplate, error) {
	var tags []string
	if len(t.Tags) > 0 {
		if err := json.Unmarshal(t.Tags, &tags); err != nil {
			return git.SyncTemplate{}, fmt.Errorf("parsing tags for template %d: %w", t.ID, err)
		}
	}

	description := ""
	if t.Description.Valid {
		description = t.Description.String
	}

	category := ""
	if t.Category.Valid {
		category = t.Category.String
	}

	token := ""
	if t.GitToken.Valid {
		token = t.GitToken.String
	}

	lastCommitSHA := ""
	if t.LastCommitSHA.Valid {
		lastCommitSHA = t.LastCommitSHA.String
	}

	return git.SyncTemplate{
		ID:            t.ID,
		UUID:          t.UUID,
		Name:          t.Name,
		Slug:          t.Slug,
		Description:   description,
		RepositoryURL: t.RepositoryURL,
		Branch:        t.Branch,
		Token:         token,
		Category:      category,
		Tags:          tags,
		LastCommitSHA: lastCommitSHA,
	}, nil
}
