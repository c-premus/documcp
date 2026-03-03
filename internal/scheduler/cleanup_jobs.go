package scheduler

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"time"
)

// cleanupOAuthTokens purges expired and revoked OAuth tokens older than 7 days.
func (s *Scheduler) cleanupOAuthTokens() {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	if s.deps.OAuthRepo == nil {
		s.logger.Warn("skipping OAuth cleanup: oauth repository not configured")
		return
	}

	s.logger.Info("starting OAuth token cleanup")

	count, err := s.deps.OAuthRepo.PurgeExpiredTokens(ctx, 7)
	if err != nil {
		s.logger.Error("OAuth token cleanup failed", "error", err)
		return
	}

	s.logger.Info("OAuth token cleanup completed", "purged_count", count)
}

// cleanupOrphanedFiles removes files on disk that are not referenced by any
// active document in the database.
func (s *Scheduler) cleanupOrphanedFiles() {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	if s.deps.DocRepo == nil || s.deps.StoragePath == "" {
		s.logger.Warn("skipping orphaned files cleanup: document repository or storage path not configured")
		return
	}

	logger := s.logger.With("job", "orphaned-files")
	logger.Info("starting orphaned files cleanup")

	activePaths, err := s.deps.DocRepo.ListActiveFilePaths(ctx)
	if err != nil {
		logger.Error("listing active file paths", "error", err)
		return
	}

	activeSet := make(map[string]bool, len(activePaths))
	for _, fp := range activePaths {
		absPath := filepath.Join(s.deps.StoragePath, fp.FilePath)
		activeSet[absPath] = true
	}

	var deletedCount int
	walkErr := filepath.WalkDir(s.deps.StoragePath, func(path string, d os.DirEntry, err error) error {
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
		logger.Error("walking storage directory", "error", walkErr)
		return
	}

	logger.Info("orphaned files cleanup completed", "deleted_count", deletedCount)
}

// verifySearchIndex checks consistency between the database and the search
// index, logging documents missing from the index and removing orphaned entries.
func (s *Scheduler) verifySearchIndex() {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	if s.deps.DocRepo == nil || s.deps.Indexer == nil {
		s.logger.Warn("skipping search index verification: document repository or indexer not configured")
		return
	}

	logger := s.logger.With("job", "search-verify")
	logger.Info("starting search index verification")

	dbUUIDs, err := s.deps.DocRepo.ListAllUUIDs(ctx)
	if err != nil {
		logger.Error("listing database document UUIDs", "error", err)
		return
	}

	dbSet := make(map[string]bool, len(dbUUIDs))
	for _, uuid := range dbUUIDs {
		dbSet[uuid] = true
	}

	indexedSet, err := s.deps.Indexer.ListIndexedDocumentUUIDs(ctx)
	if err != nil {
		logger.Error("listing indexed document UUIDs", "error", err)
		return
	}

	// Find documents in DB but missing from search index.
	var missingCount int
	for _, uuid := range dbUUIDs {
		if !indexedSet[uuid] {
			missingCount++
			logger.Warn("document missing from search index", "uuid", uuid)
		}
	}

	// Find orphaned documents in search index but not in DB.
	var orphanedCount int
	for uuid := range indexedSet {
		if !dbSet[uuid] {
			orphanedCount++
			if err := s.deps.Indexer.DeleteDocument(ctx, uuid); err != nil {
				logger.Error("removing orphaned document from search index", "uuid", uuid, "error", err)
			}
		}
	}

	logger.Info("search index verification completed",
		"missing_from_index", missingCount,
		"orphaned_in_index", orphanedCount,
	)
}

// purgeSoftDeleted permanently removes documents that have been soft-deleted
// for more than 30 days, cleaning up associated files and search index entries.
func (s *Scheduler) purgeSoftDeleted() {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	if s.deps.DocRepo == nil {
		s.logger.Warn("skipping soft-delete purge: document repository not configured")
		return
	}

	logger := s.logger.With("job", "soft-delete-purge")
	logger.Info("starting soft-delete purge")

	purged, err := s.deps.DocRepo.PurgeSoftDeleted(ctx, 30*24*time.Hour)
	if err != nil {
		logger.Error("purging soft-deleted documents", "error", err)
		return
	}

	for _, fp := range purged {
		if fp.FilePath != "" {
			absPath := filepath.Join(s.deps.StoragePath, fp.FilePath)
			if removeErr := os.Remove(absPath); removeErr != nil && !os.IsNotExist(removeErr) {
				logger.Error("removing purged document file", "path", absPath, "error", removeErr)
			}
		}

		if s.deps.Indexer != nil {
			if deleteErr := s.deps.Indexer.DeleteDocument(ctx, fp.UUID); deleteErr != nil {
				logger.Error("removing purged document from search index", "uuid", fp.UUID, "error", deleteErr)
			}
		}
	}

	logger.Info("soft-delete purge completed", "purged_count", len(purged))
}

// cleanupDisabledZim removes disabled ZIM archives from the search index.
func (s *Scheduler) cleanupDisabledZim() {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	if s.deps.ZimRepo == nil || s.deps.Indexer == nil {
		s.logger.Warn("skipping disabled ZIM cleanup: zim repository or indexer not configured")
		return
	}

	logger := s.logger.With("job", "zim-cleanup")
	logger.Info("starting disabled ZIM archive cleanup")

	archives, err := s.deps.ZimRepo.FindDisabled(ctx)
	if err != nil {
		logger.Error("finding disabled ZIM archives", "error", err)
		return
	}

	var cleanedCount int
	for _, archive := range archives {
		if err := s.deps.Indexer.DeleteZimArchive(ctx, archive.UUID); err != nil {
			logger.Error("removing disabled ZIM archive from search index", "uuid", archive.UUID, "error", err)
			continue
		}
		cleanedCount++
	}

	logger.Info("disabled ZIM archive cleanup completed", "cleaned_count", cleanedCount)
}

// healthCheckServices performs HTTP health checks against all enabled external
// services and updates their health status in the database.
func (s *Scheduler) healthCheckServices() {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	if s.deps.HealthChecker == nil {
		s.logger.Warn("skipping health check: health checker not configured")
		return
	}

	logger := s.logger.With("job", "health-check")
	logger.Info("starting external service health checks")

	services, err := s.deps.HealthChecker.FindAllEnabled(ctx)
	if err != nil {
		logger.Error("finding enabled external services for health check", "error", err)
		return
	}

	httpClient := &http.Client{Timeout: 10 * time.Second}

	var healthyCount, unhealthyCount int
	for _, svc := range services {
		svcLogger := logger.With("service_id", svc.ID, "base_url", svc.BaseURL)

		req, err := http.NewRequestWithContext(ctx, http.MethodGet, svc.BaseURL, nil)
		if err != nil {
			svcLogger.Error("creating health check request", "error", err)
			if updateErr := s.deps.HealthChecker.UpdateHealthStatus(ctx, svc.ID, "unhealthy", 0, fmt.Sprintf("creating request: %v", err)); updateErr != nil {
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
			if updateErr := s.deps.HealthChecker.UpdateHealthStatus(ctx, svc.ID, "unhealthy", latencyMs, err.Error()); updateErr != nil {
				svcLogger.Error("updating health status", "error", updateErr)
			}
			unhealthyCount++
			continue
		}
		_ = resp.Body.Close()

		if resp.StatusCode >= 200 && resp.StatusCode < 300 {
			if updateErr := s.deps.HealthChecker.UpdateHealthStatus(ctx, svc.ID, "healthy", latencyMs, ""); updateErr != nil {
				svcLogger.Error("updating health status", "error", updateErr)
			}
			healthyCount++
		} else {
			errMsg := fmt.Sprintf("unexpected status code: %d", resp.StatusCode)
			svcLogger.Warn("health check returned non-2xx", "status_code", resp.StatusCode, "latency_ms", latencyMs)
			if updateErr := s.deps.HealthChecker.UpdateHealthStatus(ctx, svc.ID, "unhealthy", latencyMs, errMsg); updateErr != nil {
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
}
