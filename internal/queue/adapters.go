// Package queue provides River job workers, adapters, and scheduling for background tasks.
package queue

import (
	"context"

	"git.999.haus/chris/DocuMCP-go/internal/client/git"
	"git.999.haus/chris/DocuMCP-go/internal/client/kiwix"
	"git.999.haus/chris/DocuMCP-go/internal/repository"
	"git.999.haus/chris/DocuMCP-go/internal/search"
)

// --- Kiwix adapters ---.

// kiwixRepoAdapter adapts ZimArchiveRepoDeps to satisfy kiwix.ArchiveRepo.
type kiwixRepoAdapter struct {
	repo ZimArchiveRepoDeps
}

// UpsertFromCatalog creates or updates a ZIM archive record from a Kiwix catalog entry.
func (a *kiwixRepoAdapter) UpsertFromCatalog(ctx context.Context, serviceID int64, entry kiwix.CatalogEntry) error {
	return a.repo.UpsertFromCatalog(ctx, serviceID, repository.ZimArchiveUpsert{
		Name:         entry.Name,
		Title:        entry.Title,
		Description:  entry.Description,
		Language:     entry.Language,
		Category:     entry.Category,
		Creator:      entry.Creator,
		Publisher:    entry.Publisher,
		Favicon:      entry.Favicon,
		ArticleCount: entry.ArticleCount,
		MediaCount:   entry.MediaCount,
		FileSize:     entry.FileSize,
		Tags:         entry.Tags,
	})
}

// DisableOrphaned disables ZIM archives that are no longer in the active catalog.
func (a *kiwixRepoAdapter) DisableOrphaned(ctx context.Context, serviceID int64, activeNames []string) (int, error) {
	return a.repo.DisableOrphaned(ctx, serviceID, activeNames)
}

// kiwixIndexerAdapter adapts SearchIndexDeps to satisfy kiwix.ArchiveIndexer.
type kiwixIndexerAdapter struct {
	indexer SearchIndexDeps
}

// IndexZimArchive indexes a ZIM archive in the search engine.
func (a *kiwixIndexerAdapter) IndexZimArchive(ctx context.Context, record kiwix.ZimArchiveRecord) error {
	return a.indexer.IndexZimArchive(ctx, search.ZimArchiveRecord{
		UUID:         record.UUID,
		Name:         record.Name,
		Title:        record.Title,
		Description:  record.Description,
		Language:     record.Language,
		Category:     record.Category,
		Creator:      record.Creator,
		Tags:         record.Tags,
		ArticleCount: record.ArticleCount,
	})
}

// --- Git template adapters ---.

// gitRepoAdapter adapts GitTemplateRepoDeps to satisfy git.TemplateRepo.
type gitRepoAdapter struct {
	repo GitTemplateRepoDeps
}

// UpdateSyncStatus updates the sync status of a Git template after a sync attempt.
func (a *gitRepoAdapter) UpdateSyncStatus(ctx context.Context, templateID int64, status, commitSHA string, fileCount int, totalSize int64, errMsg string) error {
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

// gitIndexerAdapter adapts SearchIndexDeps to satisfy git.TemplateIndexer.
type gitIndexerAdapter struct {
	indexer SearchIndexDeps
}

// IndexGitTemplate indexes a Git template in the search engine.
func (a *gitIndexerAdapter) IndexGitTemplate(ctx context.Context, record git.GitTemplateRecord) error {
	return a.indexer.IndexGitTemplate(ctx, search.GitTemplateRecord{
		UUID:          record.UUID,
		Name:          record.Name,
		Slug:          record.Slug,
		Description:   record.Description,
		ReadmeContent: record.ReadmeContent,
		Category:      record.Category,
		Tags:          record.Tags,
		IsPublic:      record.IsPublic,
		Status:        record.Status,
		SoftDeleted:   record.SoftDeleted,
	})
}
