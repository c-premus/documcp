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

	"github.com/prometheus/client_golang/prometheus"
	"github.com/riverqueue/river"

	"github.com/c-premus/documcp/internal/client/git"
	"github.com/c-premus/documcp/internal/client/kiwix"
	"github.com/c-premus/documcp/internal/model"
	"github.com/c-premus/documcp/internal/repository"
	"github.com/c-premus/documcp/internal/search"
	"github.com/c-premus/documcp/internal/security"
)

// reconciliationActions tracks search index reconciliation operations.
var reconciliationActions = prometheus.NewCounterVec(
	prometheus.CounterOpts{
		Namespace: "documcp",
		Subsystem: "search",
		Name:      "reconciliation_actions_total",
		Help:      "Total number of search index reconciliation actions by index and action type.",
	},
	[]string{"index", "action"},
)

func init() {
	prometheus.MustRegister(reconciliationActions)
}

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
	FindByUUIDs(ctx context.Context, uuids []string) ([]model.Document, error)
	TagsForDocument(ctx context.Context, documentID int64) ([]model.DocumentTag, error)
	PurgeSoftDeleted(ctx context.Context, olderThan time.Duration) ([]repository.DocumentFilePath, error)
}

// ZimArchiveRepoDeps provides ZIM archive repository methods needed by scheduler workers.
type ZimArchiveRepoDeps interface {
	FindDisabled(ctx context.Context) ([]model.ZimArchive, error)
	ListAllUUIDs(ctx context.Context) ([]string, error)
	FindByUUIDs(ctx context.Context, uuids []string) ([]model.ZimArchive, error)
	UpsertFromCatalog(ctx context.Context, serviceID int64, upsert repository.ZimArchiveUpsert) error
	DisableOrphaned(ctx context.Context, serviceID int64, activeNames []string) (int, error)
}

// GitTemplateRepoDeps provides Git template repository methods needed by scheduler workers.
type GitTemplateRepoDeps interface {
	List(ctx context.Context, category string, limit, offset int) ([]model.GitTemplate, error)
	ListAllUUIDs(ctx context.Context) ([]string, error)
	FindByUUIDs(ctx context.Context, uuids []string) ([]model.GitTemplate, error)
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
	IndexDocument(ctx context.Context, doc search.DocumentRecord) error
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
		if d.IsDir() || d.Type()&os.ModeSymlink != 0 {
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

// Work executes the VerifySearchIndexWorker job, performing two-way reconciliation
// between the database and Meilisearch indexes: removing orphaned index entries and
// re-indexing database records missing from the search index.
func (w *VerifySearchIndexWorker) Work(ctx context.Context, _ *river.Job[VerifySearchIndexArgs]) error {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Minute)
	defer cancel()

	if w.Deps.DocRepo == nil || w.Deps.Indexer == nil {
		w.Deps.Logger.Warn("skipping search index verification: document repository or indexer not configured")
		return nil
	}

	logger := w.Deps.Logger.With("job", "search-verify")
	logger.Info("starting search index verification")

	w.reconcileDocuments(ctx, logger)

	if w.Deps.ZimRepo != nil {
		w.reconcileZimArchives(ctx, logger)
	}

	if w.Deps.GitRepo != nil {
		w.reconcileGitTemplates(ctx, logger)
	}

	return nil
}

// reconcileDocuments performs two-way sync between the documents DB table and Meilisearch index.
func (w *VerifySearchIndexWorker) reconcileDocuments(ctx context.Context, logger *slog.Logger) {
	dbUUIDs, err := w.Deps.DocRepo.ListAllUUIDs(ctx)
	if err != nil {
		logger.Error("listing database document UUIDs", "error", err)
		return
	}

	dbSet := make(map[string]bool, len(dbUUIDs))
	for _, uuid := range dbUUIDs {
		dbSet[uuid] = true
	}

	indexedSet, err := w.Deps.Indexer.ListIndexedDocumentUUIDs(ctx)
	if err != nil {
		logger.Error("listing indexed document UUIDs", "error", err)
		return
	}

	// Find missing: in DB but not in index.
	var missingUUIDs []string
	for _, uuid := range dbUUIDs {
		if !indexedSet[uuid] {
			missingUUIDs = append(missingUUIDs, uuid)
		}
	}

	// Re-index missing documents.
	var reindexedCount int
	if len(missingUUIDs) > 0 {
		docs, fetchErr := w.Deps.DocRepo.FindByUUIDs(ctx, missingUUIDs)
		if fetchErr != nil {
			logger.Error("fetching documents for re-indexing", "error", fetchErr)
		} else {
			for i := range docs {
				record := w.documentToRecord(ctx, &docs[i], logger)
				if indexErr := w.Deps.Indexer.IndexDocument(ctx, record); indexErr != nil {
					logger.Error("re-indexing missing document", "uuid", docs[i].UUID, "error", indexErr)
				} else {
					reindexedCount++
				}
			}
		}
	}

	// Remove orphaned: in index but not in DB.
	var orphanedCount int
	for uuid := range indexedSet {
		if !dbSet[uuid] {
			orphanedCount++
			if err := w.Deps.Indexer.DeleteDocument(ctx, uuid); err != nil {
				logger.Error("removing orphaned document from search index", "uuid", uuid, "error", err)
			}
		}
	}

	reconciliationActions.WithLabelValues("documents", "orphaned_deleted").Add(float64(orphanedCount))
	reconciliationActions.WithLabelValues("documents", "missing_reindexed").Add(float64(reindexedCount))

	logger.Info("document index verification completed",
		"missing_reindexed", reindexedCount,
		"orphaned_deleted", orphanedCount,
	)
}

// documentToRecord converts a model.Document to a search.DocumentRecord, loading tags.
func (w *VerifySearchIndexWorker) documentToRecord(ctx context.Context, doc *model.Document, logger *slog.Logger) search.DocumentRecord {
	record := search.DocumentRecord{
		UUID:        doc.UUID,
		Title:       doc.Title,
		Description: doc.Description.String,
		Content:     doc.Content.String,
		FileType:    doc.FileType,
		Status:      doc.Status,
		IsPublic:    doc.IsPublic,
		WordCount:   int(doc.WordCount.Int64),
		SoftDeleted: doc.DeletedAt.Valid,
	}
	if doc.UserID.Valid {
		uid := doc.UserID.Int64
		record.UserID = &uid
	}
	if doc.CreatedAt.Valid {
		record.CreatedAt = doc.CreatedAt.Time.Format(time.RFC3339)
	}
	if doc.UpdatedAt.Valid {
		record.UpdatedAt = doc.UpdatedAt.Time.Format(time.RFC3339)
	}

	tags, err := w.Deps.DocRepo.TagsForDocument(ctx, doc.ID)
	if err != nil {
		logger.Warn("loading tags for document re-index", "uuid", doc.UUID, "error", err)
	} else {
		tagNames := make([]string, len(tags))
		for i, t := range tags {
			tagNames[i] = t.Tag
		}
		record.Tags = tagNames
	}

	return record
}

// reconcileZimArchives performs two-way sync between the zim_archives DB table and Meilisearch index.
func (w *VerifySearchIndexWorker) reconcileZimArchives(ctx context.Context, logger *slog.Logger) {
	zimDBUUIDs, err := w.Deps.ZimRepo.ListAllUUIDs(ctx)
	if err != nil {
		logger.Error("listing zim archive UUIDs from database", "error", err)
		return
	}

	zimDBSet := make(map[string]bool, len(zimDBUUIDs))
	for _, uuid := range zimDBUUIDs {
		zimDBSet[uuid] = true
	}

	zimIndexed, err := w.Deps.Indexer.ListIndexedZimUUIDs(ctx)
	if err != nil {
		logger.Error("listing indexed zim archive UUIDs", "error", err)
		return
	}

	// Find missing: in DB but not in index.
	var missingUUIDs []string
	for _, uuid := range zimDBUUIDs {
		if !zimIndexed[uuid] {
			missingUUIDs = append(missingUUIDs, uuid)
		}
	}

	// Re-index missing ZIM archives.
	var reindexedCount int
	if len(missingUUIDs) > 0 {
		archives, fetchErr := w.Deps.ZimRepo.FindByUUIDs(ctx, missingUUIDs)
		if fetchErr != nil {
			logger.Error("fetching zim archives for re-indexing", "error", fetchErr)
		} else {
			for i := range archives {
				record := zimArchiveToRecord(&archives[i])
				if indexErr := w.Deps.Indexer.IndexZimArchive(ctx, record); indexErr != nil {
					logger.Error("re-indexing missing zim archive", "uuid", archives[i].UUID, "error", indexErr)
				} else {
					reindexedCount++
				}
			}
		}
	}

	// Remove orphaned: in index but not in DB.
	var orphanedCount int
	for uuid := range zimIndexed {
		if !zimDBSet[uuid] {
			orphanedCount++
			if err := w.Deps.Indexer.DeleteZimArchive(ctx, uuid); err != nil {
				logger.Error("removing orphaned zim archive from index", "uuid", uuid, "error", err)
			}
		}
	}

	reconciliationActions.WithLabelValues("zim_archives", "orphaned_deleted").Add(float64(orphanedCount))
	reconciliationActions.WithLabelValues("zim_archives", "missing_reindexed").Add(float64(reindexedCount))

	logger.Info("zim archive index verification completed",
		"missing_reindexed", reindexedCount,
		"orphaned_deleted", orphanedCount,
	)
}

// reconcileGitTemplates performs two-way sync between the git_templates DB table and Meilisearch index.
func (w *VerifySearchIndexWorker) reconcileGitTemplates(ctx context.Context, logger *slog.Logger) {
	gitDBUUIDs, err := w.Deps.GitRepo.ListAllUUIDs(ctx)
	if err != nil {
		logger.Error("listing git template UUIDs from database", "error", err)
		return
	}

	gitDBSet := make(map[string]bool, len(gitDBUUIDs))
	for _, uuid := range gitDBUUIDs {
		gitDBSet[uuid] = true
	}

	gitIndexed, err := w.Deps.Indexer.ListIndexedGitTemplateUUIDs(ctx)
	if err != nil {
		logger.Error("listing indexed git template UUIDs", "error", err)
		return
	}

	// Find missing: in DB but not in index.
	var missingUUIDs []string
	for _, uuid := range gitDBUUIDs {
		if !gitIndexed[uuid] {
			missingUUIDs = append(missingUUIDs, uuid)
		}
	}

	// Re-index missing Git templates.
	var reindexedCount int
	if len(missingUUIDs) > 0 {
		templates, fetchErr := w.Deps.GitRepo.FindByUUIDs(ctx, missingUUIDs)
		if fetchErr != nil {
			logger.Error("fetching git templates for re-indexing", "error", fetchErr)
		} else {
			for i := range templates {
				record := gitTemplateToRecord(&templates[i])
				if indexErr := w.Deps.Indexer.IndexGitTemplate(ctx, record); indexErr != nil {
					logger.Error("re-indexing missing git template", "uuid", templates[i].UUID, "error", indexErr)
				} else {
					reindexedCount++
				}
			}
		}
	}

	// Remove orphaned: in index but not in DB.
	var orphanedCount int
	for uuid := range gitIndexed {
		if !gitDBSet[uuid] {
			orphanedCount++
			if err := w.Deps.Indexer.DeleteGitTemplate(ctx, uuid); err != nil {
				logger.Error("removing orphaned git template from index", "uuid", uuid, "error", err)
			}
		}
	}

	reconciliationActions.WithLabelValues("git_templates", "orphaned_deleted").Add(float64(orphanedCount))
	reconciliationActions.WithLabelValues("git_templates", "missing_reindexed").Add(float64(reindexedCount))

	logger.Info("git template index verification completed",
		"missing_reindexed", reindexedCount,
		"orphaned_deleted", orphanedCount,
	)
}

// zimArchiveToRecord converts a model.ZimArchive to a search.ZimArchiveRecord.
func zimArchiveToRecord(za *model.ZimArchive) search.ZimArchiveRecord {
	rec := search.ZimArchiveRecord{
		UUID:         za.UUID,
		Name:         za.Name,
		Title:        za.Title,
		Language:     za.Language,
		ArticleCount: za.ArticleCount,
	}
	if za.Description.Valid {
		rec.Description = za.Description.String
	}
	if za.Category.Valid {
		rec.Category = za.Category.String
	}
	if za.Creator.Valid {
		rec.Creator = za.Creator.String
	}
	if za.Tags.Valid && za.Tags.String != "" {
		tags, err := za.ParseTags()
		if err == nil {
			rec.Tags = tags
		}
	}
	return rec
}

// gitTemplateToRecord converts a model.GitTemplate to a search.GitTemplateRecord.
func gitTemplateToRecord(gt *model.GitTemplate) search.GitTemplateRecord {
	rec := search.GitTemplateRecord{
		UUID:        gt.UUID,
		Name:        gt.Name,
		Slug:        gt.Slug,
		IsPublic:    gt.IsPublic,
		Status:      gt.Status,
		SoftDeleted: gt.DeletedAt.Valid,
	}
	if gt.Description.Valid {
		rec.Description = gt.Description.String
	}
	if gt.ReadmeContent.Valid {
		rec.ReadmeContent = gt.ReadmeContent.String
	}
	if gt.Category.Valid {
		rec.Category = gt.Category.String
	}
	if gt.UserID.Valid {
		uid := gt.UserID.Int64
		rec.UserID = &uid
	}
	if gt.Tags.Valid && gt.Tags.String != "" {
		tags, err := gt.ParseTags()
		if err == nil {
			rec.Tags = tags
		}
	}
	return rec
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
