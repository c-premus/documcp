package kiwix

import (
	"context"
	"errors"
	"log/slog"
	"testing"
)

// mockArchiveRepo is a test double for the ArchiveRepo interface.
type mockArchiveRepo struct {
	upsertErr       error
	upsertFailNames map[string]bool // names that should fail
	disableCount    int
	disableErr      error
	upsertedNames   []string
	disabledNames   []string
}

func (m *mockArchiveRepo) UpsertFromCatalog(_ context.Context, _ int64, entry CatalogEntry) error {
	if m.upsertFailNames != nil && m.upsertFailNames[entry.Name] {
		return m.upsertErr
	}
	m.upsertedNames = append(m.upsertedNames, entry.Name)
	return nil
}

func (m *mockArchiveRepo) DisableOrphaned(_ context.Context, _ int64, activeNames []string) (int, error) {
	m.disabledNames = activeNames
	return m.disableCount, m.disableErr
}

// mockArchiveIndexer is a test double for the ArchiveIndexer interface.
type mockArchiveIndexer struct {
	err         error
	failNames   map[string]bool
	indexedNames []string
}

func (m *mockArchiveIndexer) IndexZimArchive(_ context.Context, record ZimArchiveRecord) error {
	if m.failNames != nil && m.failNames[record.Name] {
		return m.err
	}
	m.indexedNames = append(m.indexedNames, record.Name)
	return nil
}

func TestSync_Success(t *testing.T) {
	repo := &mockArchiveRepo{}
	indexer := &mockArchiveIndexer{}

	entries := []CatalogEntry{
		{ID: "uuid-1", Name: "archive-a", Title: "Archive A", Language: "eng"},
		{ID: "uuid-2", Name: "archive-b", Title: "Archive B", Language: "fra"},
	}

	err := Sync(context.Background(), SyncParams{
		ServiceID: 1,
		Entries:   entries,
		Repo:      repo,
		Indexer:   indexer,
		Logger:    slog.Default(),
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(repo.upsertedNames) != 2 {
		t.Errorf("expected 2 upserts, got %d", len(repo.upsertedNames))
	}
	if len(indexer.indexedNames) != 2 {
		t.Errorf("expected 2 indexed, got %d", len(indexer.indexedNames))
	}
	if len(repo.disabledNames) != 2 {
		t.Errorf("expected 2 active names passed to DisableOrphaned, got %d", len(repo.disabledNames))
	}
}

func TestSync_EmptyCatalog(t *testing.T) {
	repo := &mockArchiveRepo{}

	err := Sync(context.Background(), SyncParams{
		ServiceID: 1,
		Entries:   []CatalogEntry{},
		Repo:      repo,
		Indexer:   nil,
		Logger:    slog.Default(),
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// DisableOrphaned should NOT be called when entries are empty (early return).
	if repo.disabledNames != nil {
		t.Error("DisableOrphaned should not be called for empty catalog")
	}
}

func TestSync_NilIndexer(t *testing.T) {
	repo := &mockArchiveRepo{}

	entries := []CatalogEntry{
		{ID: "uuid-1", Name: "archive-a", Title: "Archive A"},
	}

	err := Sync(context.Background(), SyncParams{
		ServiceID: 1,
		Entries:   entries,
		Repo:      repo,
		Indexer:   nil,
		Logger:    slog.Default(),
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(repo.upsertedNames) != 1 {
		t.Errorf("expected 1 upsert, got %d", len(repo.upsertedNames))
	}
}

func TestSync_UpsertError(t *testing.T) {
	repo := &mockArchiveRepo{
		upsertErr:       errors.New("db write failed"),
		upsertFailNames: map[string]bool{"archive-b": true},
	}
	indexer := &mockArchiveIndexer{}

	entries := []CatalogEntry{
		{ID: "uuid-1", Name: "archive-a", Title: "Archive A"},
		{ID: "uuid-2", Name: "archive-b", Title: "Archive B"},
		{ID: "uuid-3", Name: "archive-c", Title: "Archive C"},
	}

	err := Sync(context.Background(), SyncParams{
		ServiceID: 1,
		Entries:   entries,
		Repo:      repo,
		Indexer:   indexer,
		Logger:    slog.Default(),
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// archive-b should have failed, so only a and c succeed.
	if len(repo.upsertedNames) != 2 {
		t.Errorf("expected 2 successful upserts, got %d: %v", len(repo.upsertedNames), repo.upsertedNames)
	}
	// Only successful upserts get indexed.
	if len(indexer.indexedNames) != 2 {
		t.Errorf("expected 2 indexed, got %d", len(indexer.indexedNames))
	}
	// activeNames passed to DisableOrphaned should only include successful ones.
	if len(repo.disabledNames) != 2 {
		t.Errorf("expected 2 active names, got %d", len(repo.disabledNames))
	}
}

func TestSync_IndexError(t *testing.T) {
	repo := &mockArchiveRepo{}
	indexer := &mockArchiveIndexer{
		err:       errors.New("search index unavailable"),
		failNames: map[string]bool{"archive-a": true},
	}

	entries := []CatalogEntry{
		{ID: "uuid-1", Name: "archive-a", Title: "Archive A"},
		{ID: "uuid-2", Name: "archive-b", Title: "Archive B"},
	}

	err := Sync(context.Background(), SyncParams{
		ServiceID: 1,
		Entries:   entries,
		Repo:      repo,
		Indexer:   indexer,
		Logger:    slog.Default(),
	})
	if err != nil {
		t.Fatalf("unexpected error (index errors should not be fatal): %v", err)
	}

	// Both should be upserted despite index failure.
	if len(repo.upsertedNames) != 2 {
		t.Errorf("expected 2 upserts, got %d", len(repo.upsertedNames))
	}
	// Only archive-b should be indexed successfully.
	if len(indexer.indexedNames) != 1 {
		t.Errorf("expected 1 indexed, got %d", len(indexer.indexedNames))
	}
}

func TestSync_DisableOrphanedError(t *testing.T) {
	repo := &mockArchiveRepo{
		disableErr: errors.New("disable query failed"),
	}

	entries := []CatalogEntry{
		{ID: "uuid-1", Name: "archive-a", Title: "Archive A"},
	}

	err := Sync(context.Background(), SyncParams{
		ServiceID: 1,
		Entries:   entries,
		Repo:      repo,
		Indexer:   nil,
		Logger:    slog.Default(),
	})
	if err == nil {
		t.Fatal("expected error from DisableOrphaned, got nil")
	}
	if !errors.Is(err, repo.disableErr) {
		t.Errorf("error should wrap DisableOrphaned error: %v", err)
	}
}

func TestSync_RecordFieldMapping(t *testing.T) {
	var captured ZimArchiveRecord
	capturingIndexer := &capturingArchiveIndexer{captured: &captured}

	repo := &mockArchiveRepo{}

	entries := []CatalogEntry{
		{
			ID:           "uuid-1",
			Name:         "devdocs-go",
			Title:        "Go Docs",
			Description:  "Go programming language",
			Language:     "eng",
			Category:     "devdocs",
			Creator:      "DevDocs",
			Tags:         []string{"go", "programming"},
			ArticleCount: 1500,
		},
	}

	err := Sync(context.Background(), SyncParams{
		ServiceID: 1,
		Entries:   entries,
		Repo:      repo,
		Indexer:   capturingIndexer,
		Logger:    slog.Default(),
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if captured.UUID != "uuid-1" {
		t.Errorf("UUID = %q, want %q", captured.UUID, "uuid-1")
	}
	if captured.Name != "devdocs-go" {
		t.Errorf("Name = %q, want %q", captured.Name, "devdocs-go")
	}
	if captured.Title != "Go Docs" {
		t.Errorf("Title = %q, want %q", captured.Title, "Go Docs")
	}
	if captured.Description != "Go programming language" {
		t.Errorf("Description = %q, want %q", captured.Description, "Go programming language")
	}
	if captured.Language != "eng" {
		t.Errorf("Language = %q, want %q", captured.Language, "eng")
	}
	if captured.Category != "devdocs" {
		t.Errorf("Category = %q, want %q", captured.Category, "devdocs")
	}
	if captured.Creator != "DevDocs" {
		t.Errorf("Creator = %q, want %q", captured.Creator, "DevDocs")
	}
	if captured.ArticleCount != 1500 {
		t.Errorf("ArticleCount = %d, want 1500", captured.ArticleCount)
	}
	if len(captured.Tags) != 2 {
		t.Errorf("Tags length = %d, want 2", len(captured.Tags))
	}
}

// capturingArchiveIndexer captures the last record indexed.
type capturingArchiveIndexer struct {
	captured *ZimArchiveRecord
}

func (c *capturingArchiveIndexer) IndexZimArchive(_ context.Context, record ZimArchiveRecord) error {
	*c.captured = record
	return nil
}
