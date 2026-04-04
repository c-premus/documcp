// Package queue provides River job workers, adapters, and scheduling for background tasks.
package queue

import (
	"context"

	"github.com/c-premus/documcp/internal/client/git"
	"github.com/c-premus/documcp/internal/client/kiwix"
	"github.com/c-premus/documcp/internal/model"
	"github.com/c-premus/documcp/internal/repository"
)

// --- Kiwix adapters ---.

// kiwixRepoAdapter adapts ZimArchiveRepoDeps to satisfy kiwix.ArchiveRepo.
type kiwixRepoAdapter struct {
	repo ZimArchiveRepoDeps
}

// UpsertFromCatalog creates or updates a ZIM archive record from a Kiwix catalog entry.
func (a *kiwixRepoAdapter) UpsertFromCatalog(ctx context.Context, serviceID int64, entry kiwix.CatalogEntry) error {
	return a.repo.UpsertFromCatalog(ctx, serviceID, repository.ZimArchiveUpsert{
		Name:             entry.Name,
		Title:            entry.Title,
		Description:      entry.Description,
		Language:         entry.Language,
		Category:         entry.Category,
		Creator:          entry.Creator,
		Publisher:        entry.Publisher,
		Favicon:          entry.Favicon,
		ArticleCount:     entry.ArticleCount,
		MediaCount:       entry.MediaCount,
		FileSize:         entry.FileSize,
		Tags:             entry.Tags,
		HasFulltextIndex: entry.HasFulltextIndex,
	})
}

// DisableOrphaned disables ZIM archives that are no longer in the active catalog.
func (a *kiwixRepoAdapter) DisableOrphaned(ctx context.Context, serviceID int64, activeNames []string) (int, error) {
	return a.repo.DisableOrphaned(ctx, serviceID, activeNames)
}

// --- Git template adapters ---.

// gitRepoAdapter adapts GitTemplateRepoDeps to satisfy git.TemplateRepo.
type gitRepoAdapter struct {
	repo GitTemplateRepoDeps
}

// UpdateSyncStatus updates the sync status of a Git template after a sync attempt.
func (a *gitRepoAdapter) UpdateSyncStatus(ctx context.Context, templateID int64, status model.GitTemplateStatus, commitSHA string, fileCount int, totalSize int64, errMsg string) error {
	return a.repo.UpdateSyncStatus(ctx, templateID, status, commitSHA, fileCount, totalSize, errMsg)
}

// ReplaceFiles replaces all files for a Git template with the provided set.
func (a *gitRepoAdapter) ReplaceFiles(ctx context.Context, templateID int64, files []git.TemplateFile) error {
	converted := make([]repository.GitTemplateFileInsert, len(files))
	for i, f := range files {
		converted[i] = repository.GitTemplateFileInsert{
			Path:        f.Path,
			Filename:    f.Filename,
			Extension:   f.Extension,
			Content:     f.Content,
			ContentHash: f.ContentHash,
			SizeBytes:   f.SizeBytes,
			IsEssential: f.IsEssential,
			Variables:   f.Variables,
		}
	}
	return a.repo.ReplaceFiles(ctx, templateID, converted)
}

// UpdateSearchContent updates the readme_content and file_paths for FTS indexing.
func (a *gitRepoAdapter) UpdateSearchContent(ctx context.Context, templateID int64, readmeContent, filePaths string) error {
	return a.repo.UpdateSearchContent(ctx, templateID, readmeContent, filePaths)
}
