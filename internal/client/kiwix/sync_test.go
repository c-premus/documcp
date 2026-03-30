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

func TestSync_Success(t *testing.T) {
	repo := &mockArchiveRepo{}

	entries := []CatalogEntry{
		{ID: "uuid-1", Name: "archive-a", Title: "Archive A", Language: "eng"},
		{ID: "uuid-2", Name: "archive-b", Title: "Archive B", Language: "fra"},
	}

	err := Sync(context.Background(), SyncParams{
		ServiceID: 1,
		Entries:   entries,
		Repo:      repo,
		Logger:    slog.Default(),
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(repo.upsertedNames) != 2 {
		t.Errorf("expected 2 upserts, got %d", len(repo.upsertedNames))
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
		Logger:    slog.Default(),
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if repo.disabledNames != nil {
		t.Error("DisableOrphaned should not be called for empty catalog")
	}
}

func TestSync_UpsertError(t *testing.T) {
	repo := &mockArchiveRepo{
		upsertErr:       errors.New("db write failed"),
		upsertFailNames: map[string]bool{"archive-b": true},
	}

	entries := []CatalogEntry{
		{ID: "uuid-1", Name: "archive-a", Title: "Archive A"},
		{ID: "uuid-2", Name: "archive-b", Title: "Archive B"},
		{ID: "uuid-3", Name: "archive-c", Title: "Archive C"},
	}

	err := Sync(context.Background(), SyncParams{
		ServiceID: 1,
		Entries:   entries,
		Repo:      repo,
		Logger:    slog.Default(),
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// archive-b should have failed, so only a and c succeed.
	if len(repo.upsertedNames) != 2 {
		t.Errorf("expected 2 successful upserts, got %d: %v", len(repo.upsertedNames), repo.upsertedNames)
	}
	if len(repo.disabledNames) != 2 {
		t.Errorf("expected 2 active names, got %d", len(repo.disabledNames))
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
		Logger:    slog.Default(),
	})
	if err == nil {
		t.Fatal("expected error from DisableOrphaned, got nil")
	}
	if !errors.Is(err, repo.disableErr) {
		t.Errorf("error should wrap DisableOrphaned error: %v", err)
	}
}
