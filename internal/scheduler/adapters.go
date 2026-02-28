package scheduler

import (
	"context"

	"git.999.haus/chris/DocuMCP-go/internal/client/confluence"
	"git.999.haus/chris/DocuMCP-go/internal/client/git"
	"git.999.haus/chris/DocuMCP-go/internal/client/kiwix"
	"git.999.haus/chris/DocuMCP-go/internal/repository"
	"git.999.haus/chris/DocuMCP-go/internal/search"
)

// --- Kiwix adapters ---

// kiwixRepoAdapter adapts *repository.ZimArchiveRepository to satisfy kiwix.ArchiveRepo.
type kiwixRepoAdapter struct {
	repo *repository.ZimArchiveRepository
}

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

func (a *kiwixRepoAdapter) DisableOrphaned(ctx context.Context, serviceID int64, activeNames []string) (int, error) {
	return a.repo.DisableOrphaned(ctx, serviceID, activeNames)
}

// kiwixIndexerAdapter adapts *search.Indexer to satisfy kiwix.ArchiveIndexer.
type kiwixIndexerAdapter struct {
	indexer *search.Indexer
}

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

// --- Confluence adapters ---

// confluenceRepoAdapter adapts *repository.ConfluenceSpaceRepository to satisfy confluence.SpaceRepo.
type confluenceRepoAdapter struct {
	repo *repository.ConfluenceSpaceRepository
}

func (a *confluenceRepoAdapter) UpsertFromAPI(ctx context.Context, serviceID int64, space confluence.Space) error {
	return a.repo.UpsertFromAPI(ctx, serviceID, repository.ConfluenceSpaceUpsert{
		ConfluenceID: space.ID,
		Key:          space.Key,
		Name:         space.Name,
		Description:  space.Description,
		Type:         space.Type,
		Status:       space.Status,
		HomepageID:   space.HomepageID,
		IconURL:      space.IconURL,
	})
}

func (a *confluenceRepoAdapter) DisableOrphaned(ctx context.Context, serviceID int64, activeKeys []string) (int, error) {
	return a.repo.DisableOrphaned(ctx, serviceID, activeKeys)
}

// confluenceIndexerAdapter adapts *search.Indexer to satisfy confluence.SpaceIndexer.
type confluenceIndexerAdapter struct {
	indexer *search.Indexer
}

func (a *confluenceIndexerAdapter) IndexConfluenceSpace(ctx context.Context, record confluence.ConfluenceSpaceRecord) error {
	return a.indexer.IndexConfluenceSpace(ctx, search.ConfluenceSpaceRecord{
		UUID:              record.UUID,
		ConfluenceID:      record.ConfluenceID,
		Key:               record.Key,
		Name:              record.Name,
		Description:       record.Description,
		Type:              record.Type,
		Status:            record.Status,
		ExternalServiceID: record.ExternalServiceID,
		IsEnabled:         record.IsEnabled,
	})
}

// --- Git template adapters ---

// gitRepoAdapter adapts *repository.GitTemplateRepository to satisfy git.TemplateRepo.
type gitRepoAdapter struct {
	repo *repository.GitTemplateRepository
}

func (a *gitRepoAdapter) UpdateSyncStatus(ctx context.Context, templateID int64, status, commitSHA string, fileCount int, totalSize int64, errMsg string) error {
	return a.repo.UpdateSyncStatus(ctx, templateID, status, commitSHA, fileCount, totalSize, errMsg)
}

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

// gitIndexerAdapter adapts *search.Indexer to satisfy git.TemplateIndexer.
type gitIndexerAdapter struct {
	indexer *search.Indexer
}

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
