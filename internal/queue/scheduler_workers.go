package queue

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"github.com/riverqueue/river"

	"git.999.haus/chris/DocuMCP-go/internal/client/git"
	"git.999.haus/chris/DocuMCP-go/internal/client/kiwix"
	"git.999.haus/chris/DocuMCP-go/internal/model"
	"git.999.haus/chris/DocuMCP-go/internal/repository"
	"git.999.haus/chris/DocuMCP-go/internal/search"
	"git.999.haus/chris/DocuMCP-go/internal/security"
)

// --- Interfaces (defined where consumed) ---.

// ExternalServiceFinder retrieves enabled external services by type.
type ExternalServiceFinder interface {
	FindEnabledByType(ctx context.Context, serviceType string) ([]model.ExternalService, error)
}

// ExternalServiceHealthChecker checks health of external services.
type ExternalServiceHealthChecker interface {
	FindAllEnabled(ctx context.Context) ([]model.ExternalService, error)
	UpdateHealthStatus(ctx context.Context, id int64, status string, latencyMs int, lastError string) error
}

// OAuthTokenPurger purges expired OAuth tokens.
type OAuthTokenPurger interface {
	PurgeExpiredTokens(ctx context.Context, retentionDays int) (int64, error)
}

// DocumentRepoDeps provides document repository methods needed by cleanup workers.
type DocumentRepoDeps interface {
	ListActiveFilePaths(ctx context.Context) ([]repository.DocumentFilePath, error)
	ListAllUUIDs(ctx context.Context) ([]string, error)
	PurgeSoftDeleted(ctx context.Context, olderThan time.Duration) ([]repository.DocumentFilePath, error)
}

// ZimArchiveRepoDeps provides ZIM archive repository methods needed by scheduler workers.
type ZimArchiveRepoDeps interface {
	FindDisabled(ctx context.Context) ([]model.ZimArchive, error)
	ListAllUUIDs(ctx context.Context) ([]string, error)
	UpsertFromCatalog(ctx context.Context, serviceID int64, upsert repository.ZimArchiveUpsert) error
	DisableOrphaned(ctx context.Context, serviceID int64, activeNames []string) (int, error)
}

// GitTemplateRepoDeps provides Git template repository methods needed by scheduler workers.
type GitTemplateRepoDeps interface {
	List(ctx context.Context, category string, limit, offset int) ([]model.GitTemplate, error)
	ListAllUUIDs(ctx context.Context) ([]string, error)
	UpdateSyncStatus(ctx context.Context, templateID int64, status, commitSHA string, fileCount int, totalSize int64, errMsg string) error
	ReplaceFiles(ctx context.Context, templateID int64, files []repository.GitTemplateFileInsert) error
}

// SearchIndexDeps provides search indexer methods needed by cleanup workers.
type SearchIndexDeps interface {
	ListIndexedDocumentUUIDs(ctx context.Context) (map[string]bool, error)
	ListIndexedZimUUIDs(ctx context.Context) (map[string]bool, error)
	ListIndexedGitTemplateUUIDs(ctx context.Context) (map[string]bool, error)
	DeleteDocument(ctx context.Context, uuid string) error
	DeleteZimArchive(ctx context.Context, uuid string) error
	DeleteGitTemplate(ctx context.Context, uuid string) error
	IndexZimArchive(ctx context.Context, record search.ZimArchiveRecord) error
	IndexGitTemplate(ctx context.Context, record search.GitTemplateRecord) error
}

// SchedulerDeps holds all dependencies needed by scheduler workers.
type SchedulerDeps struct {
	Services          ExternalServiceFinder
	HealthChecker     ExternalServiceHealthChecker
	ZimRepo           ZimArchiveRepoDeps
	GitRepo           GitTemplateRepoDeps
	OAuthRepo         OAuthTokenPurger
	DocRepo           DocumentRepoDeps
	Indexer           SearchIndexDeps
	GitTempDir        string
	StoragePath       string
	Logger            *slog.Logger
	GitMaxFileSize    int64
	GitMaxTotalSize   int64
	SSRFDialerTimeout time.Duration
	KiwixConfig       kiwix.ClientConfig
}

// --- Sync Workers ---.

// SyncKiwixWorker syncs Kiwix ZIM archives from external services.
type SyncKiwixWorker struct {
	river.WorkerDefaults[SyncKiwixArgs]
	Deps SchedulerDeps
}

// Work executes the SyncKiwixWorker job, syncing ZIM archives from the Kiwix catalog.
func (w *SyncKiwixWorker) Work(ctx context.Context, _ *river.Job[SyncKiwixArgs]) error {
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
				if hErr := w.Deps.HealthChecker.UpdateHealthStatus(ctx, svc.ID, "unhealthy", 0, err.Error()); hErr != nil {
					svcLogger.Error("updating health status", "error", hErr)
				}
			}
			continue
		}

		var indexer kiwix.ArchiveIndexer
		if w.Deps.Indexer != nil {
			indexer = &kiwixIndexerAdapter{indexer: w.Deps.Indexer}
		}

		if err := kiwix.Sync(ctx, kiwix.SyncParams{
			ServiceID: svc.ID,
			Entries:   entries,
			Repo:      &kiwixRepoAdapter{repo: w.Deps.ZimRepo},
			Indexer:   indexer,
			Logger:    svcLogger,
		}); err != nil {
			svcLogger.Error("kiwix sync failed", "error", err)
			if w.Deps.HealthChecker != nil {
				if hErr := w.Deps.HealthChecker.UpdateHealthStatus(ctx, svc.ID, "unhealthy", 0, err.Error()); hErr != nil {
					svcLogger.Error("updating health status", "error", hErr)
				}
			}
			continue
		}

		if w.Deps.HealthChecker != nil {
			if hErr := w.Deps.HealthChecker.UpdateHealthStatus(ctx, svc.ID, "healthy", 0, ""); hErr != nil {
				svcLogger.Error("updating health status", "error", hErr)
			}
		}
		svcLogger.Info("kiwix service sync completed", "entries", len(entries))
	}
	return nil
}

// SyncGitTemplatesWorker syncs Git template repositories.
type SyncGitTemplatesWorker struct {
	river.WorkerDefaults[SyncGitTemplatesArgs]
	Deps SchedulerDeps
}

// Work executes the SyncGitTemplatesWorker job, syncing Git template repositories.
func (w *SyncGitTemplatesWorker) Work(ctx context.Context, _ *river.Job[SyncGitTemplatesArgs]) error {
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

		var indexer git.TemplateIndexer
		if w.Deps.Indexer != nil {
			indexer = &gitIndexerAdapter{indexer: w.Deps.Indexer}
		}

		if err := git.Sync(ctx, git.SyncParams{
			Template: syncTmpl,
			Client:   client,
			Repo:     &gitRepoAdapter{repo: w.Deps.GitRepo},
			Indexer:  indexer,
			Logger:   tmplLogger,
		}); err != nil {
			tmplLogger.Error("git template sync failed", "error", err)
			continue
		}

		tmplLogger.Info("git template sync completed")
	}
	return nil
}

// --- Cleanup Workers ---.

// CleanupOAuthTokensWorker purges expired OAuth tokens.
type CleanupOAuthTokensWorker struct {
	river.WorkerDefaults[CleanupOAuthTokensArgs]
	Deps SchedulerDeps
}

// Work executes the CleanupOAuthTokensWorker job, purging expired OAuth tokens.
func (w *CleanupOAuthTokensWorker) Work(ctx context.Context, _ *river.Job[CleanupOAuthTokensArgs]) error {
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
	return nil
}

// CleanupOrphanedFilesWorker removes files not referenced by any active document.
type CleanupOrphanedFilesWorker struct {
	river.WorkerDefaults[CleanupOrphanedFilesArgs]
	Deps SchedulerDeps
}

// Work executes the CleanupOrphanedFilesWorker job, removing unreferenced files from storage.
func (w *CleanupOrphanedFilesWorker) Work(ctx context.Context, _ *river.Job[CleanupOrphanedFilesArgs]) error {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Minute)
	defer cancel()

	if w.Deps.DocRepo == nil || w.Deps.StoragePath == "" {
		w.Deps.Logger.Warn("skipping orphaned files cleanup: document repository or storage path not configured")
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
		absPath := filepath.Join(w.Deps.StoragePath, fp.FilePath)
		activeSet[absPath] = true
	}

	var deletedCount int
	walkErr := filepath.WalkDir(w.Deps.StoragePath, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		if !activeSet[path] {
			if removeErr := os.Remove(path); removeErr != nil {
				logger.Error("removing orphaned file", "path", path, "error", removeErr)
			} else {
				deletedCount++
			}
		}
		return nil
	})
	if walkErr != nil {
		return fmt.Errorf("walking storage directory: %w", walkErr)
	}

	logger.Info("orphaned files cleanup completed", "deleted_count", deletedCount)
	return nil
}

// VerifySearchIndexWorker checks consistency between DB and search index.
type VerifySearchIndexWorker struct {
	river.WorkerDefaults[VerifySearchIndexArgs]
	Deps SchedulerDeps
}

// Work executes the VerifySearchIndexWorker job, checking consistency between the database and search index.
func (w *VerifySearchIndexWorker) Work(ctx context.Context, _ *river.Job[VerifySearchIndexArgs]) error {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Minute)
	defer cancel()

	if w.Deps.DocRepo == nil || w.Deps.Indexer == nil {
		w.Deps.Logger.Warn("skipping search index verification: document repository or indexer not configured")
		return nil
	}

	logger := w.Deps.Logger.With("job", "search-verify")
	logger.Info("starting search index verification")

	dbUUIDs, err := w.Deps.DocRepo.ListAllUUIDs(ctx)
	if err != nil {
		return fmt.Errorf("listing database document UUIDs: %w", err)
	}

	dbSet := make(map[string]bool, len(dbUUIDs))
	for _, uuid := range dbUUIDs {
		dbSet[uuid] = true
	}

	indexedSet, err := w.Deps.Indexer.ListIndexedDocumentUUIDs(ctx)
	if err != nil {
		return fmt.Errorf("listing indexed document UUIDs: %w", err)
	}

	var missingCount int
	for _, uuid := range dbUUIDs {
		if !indexedSet[uuid] {
			missingCount++
			logger.Warn("document missing from search index", "uuid", uuid)
		}
	}

	var orphanedCount int
	for uuid := range indexedSet {
		if !dbSet[uuid] {
			orphanedCount++
			if err := w.Deps.Indexer.DeleteDocument(ctx, uuid); err != nil {
				logger.Error("removing orphaned document from search index", "uuid", uuid, "error", err)
			}
		}
	}

	logger.Info("document index verification completed",
		"missing_from_index", missingCount,
		"orphaned_in_index", orphanedCount,
	)

	// Verify ZIM archives index.
	if w.Deps.ZimRepo != nil {
		zimOrphaned := 0
		zimDBUUIDs, err := w.Deps.ZimRepo.ListAllUUIDs(ctx)
		if err != nil {
			logger.Error("listing zim archive UUIDs from database", "error", err)
		} else {
			zimDBSet := make(map[string]bool, len(zimDBUUIDs))
			for _, uuid := range zimDBUUIDs {
				zimDBSet[uuid] = true
			}
			zimIndexed, err := w.Deps.Indexer.ListIndexedZimUUIDs(ctx)
			if err != nil {
				logger.Error("listing indexed zim archive UUIDs", "error", err)
			} else {
				for uuid := range zimIndexed {
					if !zimDBSet[uuid] {
						zimOrphaned++
						if err := w.Deps.Indexer.DeleteZimArchive(ctx, uuid); err != nil {
							logger.Error("removing orphaned zim archive from index", "uuid", uuid, "error", err)
						}
					}
				}
				logger.Info("zim archive index verification completed", "orphaned_in_index", zimOrphaned)
			}
		}
	}

	// Verify Git templates index.
	if w.Deps.GitRepo != nil {
		gitOrphaned := 0
		gitDBUUIDs, err := w.Deps.GitRepo.ListAllUUIDs(ctx)
		if err != nil {
			logger.Error("listing git template UUIDs from database", "error", err)
		} else {
			gitDBSet := make(map[string]bool, len(gitDBUUIDs))
			for _, uuid := range gitDBUUIDs {
				gitDBSet[uuid] = true
			}
			gitIndexed, err := w.Deps.Indexer.ListIndexedGitTemplateUUIDs(ctx)
			if err != nil {
				logger.Error("listing indexed git template UUIDs", "error", err)
			} else {
				for uuid := range gitIndexed {
					if !gitDBSet[uuid] {
						gitOrphaned++
						if err := w.Deps.Indexer.DeleteGitTemplate(ctx, uuid); err != nil {
							logger.Error("removing orphaned git template from index", "uuid", uuid, "error", err)
						}
					}
				}
				logger.Info("git template index verification completed", "orphaned_in_index", gitOrphaned)
			}
		}
	}

	return nil
}

// PurgeSoftDeletedWorker permanently removes documents soft-deleted >30 days.
type PurgeSoftDeletedWorker struct {
	river.WorkerDefaults[PurgeSoftDeletedArgs]
	Deps SchedulerDeps
}

// Work executes the PurgeSoftDeletedWorker job, permanently removing documents soft-deleted over 30 days ago.
func (w *PurgeSoftDeletedWorker) Work(ctx context.Context, _ *river.Job[PurgeSoftDeletedArgs]) error {
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
		if fp.FilePath != "" {
			absPath := filepath.Join(w.Deps.StoragePath, fp.FilePath)
			if removeErr := os.Remove(absPath); removeErr != nil && !os.IsNotExist(removeErr) {
				logger.Error("removing purged document file", "path", absPath, "error", removeErr)
			}
		}

		if w.Deps.Indexer != nil {
			if deleteErr := w.Deps.Indexer.DeleteDocument(ctx, fp.UUID); deleteErr != nil {
				logger.Error("removing purged document from search index", "uuid", fp.UUID, "error", deleteErr)
			}
		}
	}

	logger.Info("soft-delete purge completed", "purged_count", len(purged))
	return nil
}

// CleanupDisabledZimWorker removes disabled ZIM archives from search.
type CleanupDisabledZimWorker struct {
	river.WorkerDefaults[CleanupDisabledZimArgs]
	Deps SchedulerDeps
}

// Work executes the CleanupDisabledZimWorker job, removing disabled ZIM archives from the search index.
func (w *CleanupDisabledZimWorker) Work(ctx context.Context, _ *river.Job[CleanupDisabledZimArgs]) error {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Minute)
	defer cancel()

	if w.Deps.ZimRepo == nil || w.Deps.Indexer == nil {
		w.Deps.Logger.Warn("skipping disabled ZIM cleanup: zim repository or indexer not configured")
		return nil
	}

	logger := w.Deps.Logger.With("job", "zim-cleanup")
	logger.Info("starting disabled ZIM archive cleanup")

	archives, err := w.Deps.ZimRepo.FindDisabled(ctx)
	if err != nil {
		return fmt.Errorf("finding disabled ZIM archives: %w", err)
	}

	var cleanedCount int
	for i := range archives {
		if err := w.Deps.Indexer.DeleteZimArchive(ctx, archives[i].UUID); err != nil {
			logger.Error("removing disabled ZIM archive from search index", "uuid", archives[i].UUID, "error", err)
			continue
		}
		cleanedCount++
	}

	logger.Info("disabled ZIM archive cleanup completed", "cleaned_count", cleanedCount)
	return nil
}

// HealthCheckServicesWorker performs HTTP health checks on external services.
type HealthCheckServicesWorker struct {
	river.WorkerDefaults[HealthCheckServicesArgs]
	Deps SchedulerDeps
}

// Work executes the HealthCheckServicesWorker job, performing HTTP health checks on external services.
func (w *HealthCheckServicesWorker) Work(ctx context.Context, _ *river.Job[HealthCheckServicesArgs]) error {
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

	httpClient := &http.Client{Timeout: 10 * time.Second, Transport: security.SafeTransport(w.Deps.SSRFDialerTimeout)}

	var healthyCount, unhealthyCount int
	for i := range services {
		svc := &services[i]
		svcLogger := logger.With("service_id", svc.ID, "base_url", svc.BaseURL)

		req, err := http.NewRequestWithContext(ctx, http.MethodGet, svc.BaseURL, http.NoBody)
		if err != nil {
			svcLogger.Error("creating health check request", "error", err)
			if updateErr := w.Deps.HealthChecker.UpdateHealthStatus(ctx, svc.ID, "unhealthy", 0, fmt.Sprintf("creating request: %v", err)); updateErr != nil {
				svcLogger.Error("updating health status", "error", updateErr)
			}
			unhealthyCount++
			continue
		}

		start := time.Now()
		resp, err := httpClient.Do(req)
		latencyMs := int(time.Since(start).Milliseconds())

		if err != nil {
			svcLogger.Warn("health check failed", "error", err, "latency_ms", latencyMs)
			if updateErr := w.Deps.HealthChecker.UpdateHealthStatus(ctx, svc.ID, "unhealthy", latencyMs, err.Error()); updateErr != nil {
				svcLogger.Error("updating health status", "error", updateErr)
			}
			unhealthyCount++
			continue
		}
		_ = resp.Body.Close()

		if resp.StatusCode >= 200 && resp.StatusCode < 300 {
			if updateErr := w.Deps.HealthChecker.UpdateHealthStatus(ctx, svc.ID, "healthy", latencyMs, ""); updateErr != nil {
				svcLogger.Error("updating health status", "error", updateErr)
			}
			healthyCount++
		} else {
			errMsg := fmt.Sprintf("unexpected status code: %d", resp.StatusCode)
			svcLogger.Warn("health check returned non-2xx", "status_code", resp.StatusCode, "latency_ms", latencyMs)
			if updateErr := w.Deps.HealthChecker.UpdateHealthStatus(ctx, svc.ID, "unhealthy", latencyMs, errMsg); updateErr != nil {
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
	return nil
}

// --- Helpers (moved from scheduler package) ---.

// toSyncTemplate converts a model.GitTemplate to the git.SyncTemplate type.
func toSyncTemplate(t model.GitTemplate) (git.SyncTemplate, error) {
	var tags []string
	if t.Tags.Valid && t.Tags.String != "" {
		if err := json.Unmarshal([]byte(t.Tags.String), &tags); err != nil {
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
