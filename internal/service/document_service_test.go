package service

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/c-premus/documcp/internal/model"
	"github.com/c-premus/documcp/internal/repository"
	"github.com/c-premus/documcp/internal/testutil"
)

// ---------------------------------------------------------------------------
// Mock
// ---------------------------------------------------------------------------

type mockDocumentRepo struct {
	findByUUIDFn                 func(ctx context.Context, uuid string) (*model.Document, error)
	findByUUIDIncludingDeletedFn func(ctx context.Context, uuid string) (*model.Document, error)
	findByIDFn                   func(ctx context.Context, id int64) (*model.Document, error)
	createFn                     func(ctx context.Context, doc *model.Document) error
	updateFn                     func(ctx context.Context, doc *model.Document) error
	softDeleteFn                 func(ctx context.Context, id int64) error
	restoreFn                    func(ctx context.Context, id int64) error
	purgeSingleFn                func(ctx context.Context, id int64) (string, error)
	purgeSoftDeletedFn           func(ctx context.Context, olderThan time.Duration) ([]repository.DocumentFilePath, error)
	listFn                       func(ctx context.Context, params repository.DocumentListParams) (*repository.DocumentListResult, error)
	listDeletedFn                func(ctx context.Context, limit, offset int, userID *int64) ([]model.Document, int, error)
	listDistinctTagsFn           func(ctx context.Context, prefix string, limit int, userID *int64) ([]string, error)
	tagsForDocFn                 func(ctx context.Context, documentID int64) ([]model.DocumentTag, error)
	tagsForDocsFn                func(ctx context.Context, documentIDs []int64) (map[int64][]model.DocumentTag, error)
	replaceTagsFn                func(ctx context.Context, documentID int64, tags []string) error
	createVersionFn              func(ctx context.Context, version *model.DocumentVersion) error
}

func (m *mockDocumentRepo) FindByUUID(ctx context.Context, uuid string) (*model.Document, error) {
	if m.findByUUIDFn != nil {
		return m.findByUUIDFn(ctx, uuid)
	}
	return nil, nil
}

func (m *mockDocumentRepo) FindByUUIDIncludingDeleted(ctx context.Context, uuid string) (*model.Document, error) {
	if m.findByUUIDIncludingDeletedFn != nil {
		return m.findByUUIDIncludingDeletedFn(ctx, uuid)
	}
	return nil, nil
}

func (m *mockDocumentRepo) FindByID(ctx context.Context, id int64) (*model.Document, error) {
	if m.findByIDFn != nil {
		return m.findByIDFn(ctx, id)
	}
	return nil, nil
}

func (m *mockDocumentRepo) Create(ctx context.Context, doc *model.Document) error {
	if m.createFn != nil {
		return m.createFn(ctx, doc)
	}
	return nil
}

func (m *mockDocumentRepo) Update(ctx context.Context, doc *model.Document) error {
	if m.updateFn != nil {
		return m.updateFn(ctx, doc)
	}
	return nil
}

func (m *mockDocumentRepo) SoftDelete(ctx context.Context, id int64) error {
	if m.softDeleteFn != nil {
		return m.softDeleteFn(ctx, id)
	}
	return nil
}

func (m *mockDocumentRepo) Restore(ctx context.Context, id int64) error {
	if m.restoreFn != nil {
		return m.restoreFn(ctx, id)
	}
	return nil
}

func (m *mockDocumentRepo) PurgeSingle(ctx context.Context, id int64) (string, error) {
	if m.purgeSingleFn != nil {
		return m.purgeSingleFn(ctx, id)
	}
	return "", nil
}

func (m *mockDocumentRepo) PurgeSoftDeleted(ctx context.Context, olderThan time.Duration) ([]repository.DocumentFilePath, error) {
	if m.purgeSoftDeletedFn != nil {
		return m.purgeSoftDeletedFn(ctx, olderThan)
	}
	return nil, nil
}

func (m *mockDocumentRepo) List(ctx context.Context, params repository.DocumentListParams) (*repository.DocumentListResult, error) {
	if m.listFn != nil {
		return m.listFn(ctx, params)
	}
	return &repository.DocumentListResult{}, nil
}

func (m *mockDocumentRepo) ListDeleted(ctx context.Context, limit, offset int, userID *int64) ([]model.Document, int, error) {
	if m.listDeletedFn != nil {
		return m.listDeletedFn(ctx, limit, offset, userID)
	}
	return nil, 0, nil
}

func (m *mockDocumentRepo) ListDistinctTags(ctx context.Context, prefix string, limit int, userID *int64) ([]string, error) {
	if m.listDistinctTagsFn != nil {
		return m.listDistinctTagsFn(ctx, prefix, limit, userID)
	}
	return nil, nil
}

func (m *mockDocumentRepo) TagsForDocument(ctx context.Context, documentID int64) ([]model.DocumentTag, error) {
	if m.tagsForDocFn != nil {
		return m.tagsForDocFn(ctx, documentID)
	}
	return nil, nil
}

func (m *mockDocumentRepo) TagsForDocuments(ctx context.Context, documentIDs []int64) (map[int64][]model.DocumentTag, error) {
	if m.tagsForDocsFn != nil {
		return m.tagsForDocsFn(ctx, documentIDs)
	}
	return nil, nil
}

func (m *mockDocumentRepo) ReplaceTags(ctx context.Context, documentID int64, tags []string) error {
	if m.replaceTagsFn != nil {
		return m.replaceTagsFn(ctx, documentID, tags)
	}
	return nil
}

func (m *mockDocumentRepo) CreateVersion(ctx context.Context, version *model.DocumentVersion) error {
	if m.createVersionFn != nil {
		return m.createVersionFn(ctx, version)
	}
	return nil
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

// ---------------------------------------------------------------------------
// TestDocumentService_FindByUUID
// ---------------------------------------------------------------------------

func TestDocumentService_FindByUUID(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		repoFn    func(ctx context.Context, uuid string) (*model.Document, error)
		wantDoc   bool
		wantErr   bool
		wantIs    error  // sentinel to match via errors.Is (takes precedence over errSubstr)
		errSubstr string // fallback: wrapper-context substring for non-sentinel paths
	}{
		{
			name: "document found",
			repoFn: func(_ context.Context, _ string) (*model.Document, error) {
				return &model.Document{ID: 1, UUID: "abc-123", Title: "Found"}, nil
			},
			wantDoc: true,
		},
		{
			name: "document not found returns ErrNotFound",
			repoFn: func(_ context.Context, _ string) (*model.Document, error) {
				return nil, sql.ErrNoRows
			},
			wantDoc: false,
			wantErr: true,
			wantIs:  ErrNotFound,
		},
		{
			name: "repository error is wrapped",
			repoFn: func(_ context.Context, _ string) (*model.Document, error) {
				return nil, errors.New("connection refused")
			},
			wantErr:   true,
			errSubstr: "finding document by uuid",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			repo := &mockDocumentRepo{findByUUIDFn: tt.repoFn}
			svc := NewDocumentService(repo, testutil.DiscardLogger())

			doc, err := svc.FindByUUID(context.Background(), "abc-123")

			if tt.wantErr {
				if err == nil {
					t.Fatal("expected error, got nil")
				}
				if tt.wantIs != nil && !errors.Is(err, tt.wantIs) {
					t.Errorf("errors.Is(err, %v) = false, err = %q", tt.wantIs, err.Error())
				}
				if tt.errSubstr != "" && !strings.Contains(err.Error(), tt.errSubstr) {
					t.Errorf("error %q does not contain %q", err.Error(), tt.errSubstr)
				}
				return
			}

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if tt.wantDoc && doc == nil {
				t.Fatal("expected document, got nil")
			}
			if !tt.wantDoc && doc != nil {
				t.Fatalf("expected nil document, got %+v", doc)
			}
			if tt.wantDoc && doc.UUID != "abc-123" {
				t.Errorf("UUID = %q, want %q", doc.UUID, "abc-123")
			}
		})
	}
}

// ---------------------------------------------------------------------------
// TestDocumentService_Create
// ---------------------------------------------------------------------------

func TestDocumentService_Create(t *testing.T) {
	t.Parallel()

	t.Run("success with computed fields", func(t *testing.T) {
		t.Parallel()

		var createdDoc *model.Document
		repo := &mockDocumentRepo{
			createFn: func(_ context.Context, doc *model.Document) error {
				createdDoc = doc
				doc.ID = 42
				return nil
			},
			findByIDFn: func(_ context.Context, id int64) (*model.Document, error) {
				if id != 42 {
					t.Errorf("FindByID called with %d, want 42", id)
				}
				return createdDoc, nil
			},
		}
		svc := NewDocumentService(repo, testutil.DiscardLogger())

		content := "hello world"
		params := CreateDocumentParams{
			Title:    "Test Doc",
			Content:  content,
			FileType: "markdown",
		}

		doc, err := svc.Create(context.Background(), params)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		// UUID is generated (non-empty, looks like a UUID)
		if doc.UUID == "" {
			t.Error("expected UUID to be generated, got empty string")
		}
		if len(doc.UUID) != 36 {
			t.Errorf("UUID length = %d, want 36", len(doc.UUID))
		}

		// Content hash is SHA-256 of content
		wantHash := sha256.Sum256([]byte(content))
		wantHashStr := hex.EncodeToString(wantHash[:])
		if doc.ContentHash.String != wantHashStr {
			t.Errorf("ContentHash = %q, want %q", doc.ContentHash.String, wantHashStr)
		}

		// Word count
		if doc.WordCount.Int64 != 2 {
			t.Errorf("WordCount = %d, want 2", doc.WordCount.Int64)
		}

		// Status
		if doc.Status != model.DocumentStatusIndexed {
			t.Errorf("Status = %q, want %q", doc.Status, model.DocumentStatusIndexed)
		}

		// MIME type
		if doc.MIMEType != "text/markdown" {
			t.Errorf("MIMEType = %q, want %q", doc.MIMEType, "text/markdown")
		}

		// FileSize
		if doc.FileSize != int64(len(content)) {
			t.Errorf("FileSize = %d, want %d", doc.FileSize, len(content))
		}
	})

	t.Run("with tags calls ReplaceTags", func(t *testing.T) {
		t.Parallel()

		replaceTagsCalled := false
		var replacedTags []string
		repo := &mockDocumentRepo{
			createFn: func(_ context.Context, doc *model.Document) error {
				doc.ID = 10
				return nil
			},
			replaceTagsFn: func(_ context.Context, docID int64, tags []string) error {
				replaceTagsCalled = true
				replacedTags = tags
				if docID != 10 {
					t.Errorf("ReplaceTags docID = %d, want 10", docID)
				}
				return nil
			},
			findByIDFn: func(_ context.Context, _ int64) (*model.Document, error) {
				return &model.Document{ID: 10}, nil
			},
		}
		svc := NewDocumentService(repo, testutil.DiscardLogger())

		params := CreateDocumentParams{
			Title:    "Tagged",
			Content:  "body",
			FileType: "md",
			Tags:     []string{"go", "testing"},
		}

		_, err := svc.Create(context.Background(), params)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if !replaceTagsCalled {
			t.Error("expected ReplaceTags to be called")
		}
		if len(replacedTags) != 2 || replacedTags[0] != "go" || replacedTags[1] != "testing" {
			t.Errorf("ReplaceTags tags = %v, want [go testing]", replacedTags)
		}
	})

	t.Run("without tags does not call ReplaceTags", func(t *testing.T) {
		t.Parallel()

		replaceTagsCalled := false
		repo := &mockDocumentRepo{
			createFn: func(_ context.Context, doc *model.Document) error {
				doc.ID = 11
				return nil
			},
			replaceTagsFn: func(_ context.Context, _ int64, _ []string) error {
				replaceTagsCalled = true
				return nil
			},
			findByIDFn: func(_ context.Context, _ int64) (*model.Document, error) {
				return &model.Document{ID: 11}, nil
			},
		}
		svc := NewDocumentService(repo, testutil.DiscardLogger())

		params := CreateDocumentParams{
			Title:    "No Tags",
			Content:  "body",
			FileType: "html",
		}

		_, err := svc.Create(context.Background(), params)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if replaceTagsCalled {
			t.Error("expected ReplaceTags NOT to be called when tags are empty")
		}
	})

	t.Run("repository create error is wrapped", func(t *testing.T) {
		t.Parallel()

		repo := &mockDocumentRepo{
			createFn: func(_ context.Context, _ *model.Document) error {
				return errors.New("disk full")
			},
		}
		svc := NewDocumentService(repo, testutil.DiscardLogger())

		params := CreateDocumentParams{
			Title:    "Fail",
			Content:  "x",
			FileType: "md",
		}

		_, err := svc.Create(context.Background(), params)
		if err == nil {
			t.Fatal("expected error, got nil")
		}
		if !strings.Contains(err.Error(), "creating document") {
			t.Errorf("error %q does not contain %q", err.Error(), "creating document")
		}
	})
}

// ---------------------------------------------------------------------------
// TestDocumentService_Update
// ---------------------------------------------------------------------------

func TestDocumentService_Update(t *testing.T) {
	t.Parallel()

	existingDoc := func() *model.Document {
		return &model.Document{
			ID:       5,
			UUID:     "update-uuid",
			Title:    "Original Title",
			IsPublic: false,
		}
	}

	t.Run("success applies title change", func(t *testing.T) {
		t.Parallel()

		var updatedDoc *model.Document
		repo := &mockDocumentRepo{
			findByUUIDFn: func(_ context.Context, _ string) (*model.Document, error) {
				return existingDoc(), nil
			},
			updateFn: func(_ context.Context, doc *model.Document) error {
				updatedDoc = doc
				return nil
			},
			findByIDFn: func(_ context.Context, _ int64) (*model.Document, error) {
				return updatedDoc, nil
			},
		}
		svc := NewDocumentService(repo, testutil.DiscardLogger())

		doc, err := svc.Update(context.Background(), "update-uuid", UpdateDocumentParams{
			Title: "New Title",
		})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if doc.Title != "New Title" {
			t.Errorf("Title = %q, want %q", doc.Title, "New Title")
		}
	})

	t.Run("applies isPublic change", func(t *testing.T) {
		t.Parallel()

		var updatedDoc *model.Document
		repo := &mockDocumentRepo{
			findByUUIDFn: func(_ context.Context, _ string) (*model.Document, error) {
				return existingDoc(), nil
			},
			updateFn: func(_ context.Context, doc *model.Document) error {
				updatedDoc = doc
				return nil
			},
			findByIDFn: func(_ context.Context, _ int64) (*model.Document, error) {
				return updatedDoc, nil
			},
		}
		svc := NewDocumentService(repo, testutil.DiscardLogger())

		doc, err := svc.Update(context.Background(), "update-uuid", UpdateDocumentParams{
			IsPublic: new(true),
		})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !doc.IsPublic {
			t.Error("expected IsPublic to be true")
		}
	})

	t.Run("applies tags change", func(t *testing.T) {
		t.Parallel()

		replaceTagsCalled := false
		var updatedDoc *model.Document
		repo := &mockDocumentRepo{
			findByUUIDFn: func(_ context.Context, _ string) (*model.Document, error) {
				return existingDoc(), nil
			},
			updateFn: func(_ context.Context, doc *model.Document) error {
				updatedDoc = doc
				return nil
			},
			replaceTagsFn: func(_ context.Context, docID int64, tags []string) error {
				replaceTagsCalled = true
				if docID != 5 {
					t.Errorf("ReplaceTags docID = %d, want 5", docID)
				}
				if len(tags) != 1 || tags[0] != "new-tag" {
					t.Errorf("ReplaceTags tags = %v, want [new-tag]", tags)
				}
				return nil
			},
			findByIDFn: func(_ context.Context, _ int64) (*model.Document, error) {
				return updatedDoc, nil
			},
		}
		svc := NewDocumentService(repo, testutil.DiscardLogger())

		_, err := svc.Update(context.Background(), "update-uuid", UpdateDocumentParams{
			Tags: []string{"new-tag"},
		})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !replaceTagsCalled {
			t.Error("expected ReplaceTags to be called")
		}
	})

	t.Run("document not found returns error", func(t *testing.T) {
		t.Parallel()

		repo := &mockDocumentRepo{
			findByUUIDFn: func(_ context.Context, _ string) (*model.Document, error) {
				return nil, sql.ErrNoRows
			},
		}
		svc := NewDocumentService(repo, testutil.DiscardLogger())

		_, err := svc.Update(context.Background(), "missing-uuid", UpdateDocumentParams{
			Title: "Nope",
		})
		if err == nil {
			t.Fatal("expected error, got nil")
		}
		if !errors.Is(err, ErrNotFound) {
			t.Errorf("expected ErrNotFound, got: %v", err)
		}
	})

	t.Run("repository update error is wrapped", func(t *testing.T) {
		t.Parallel()

		repo := &mockDocumentRepo{
			findByUUIDFn: func(_ context.Context, _ string) (*model.Document, error) {
				return existingDoc(), nil
			},
			updateFn: func(_ context.Context, _ *model.Document) error {
				return errors.New("constraint violation")
			},
		}
		svc := NewDocumentService(repo, testutil.DiscardLogger())

		_, err := svc.Update(context.Background(), "update-uuid", UpdateDocumentParams{
			Title: "Boom",
		})
		if err == nil {
			t.Fatal("expected error, got nil")
		}
		if !strings.Contains(err.Error(), "updating document") {
			t.Errorf("error %q does not contain %q", err.Error(), "updating document")
		}
	})
}

// ---------------------------------------------------------------------------
// TestDocumentService_Delete
// ---------------------------------------------------------------------------

func TestDocumentService_Delete(t *testing.T) {
	t.Parallel()

	t.Run("success calls SoftDelete", func(t *testing.T) {
		t.Parallel()

		softDeleteCalled := false
		repo := &mockDocumentRepo{
			findByUUIDFn: func(_ context.Context, _ string) (*model.Document, error) {
				return &model.Document{ID: 7, UUID: "del-uuid"}, nil
			},
			softDeleteFn: func(_ context.Context, id int64) error {
				softDeleteCalled = true
				if id != 7 {
					t.Errorf("SoftDelete id = %d, want 7", id)
				}
				return nil
			},
		}
		svc := NewDocumentService(repo, testutil.DiscardLogger())

		err := svc.Delete(context.Background(), "del-uuid")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !softDeleteCalled {
			t.Error("expected SoftDelete to be called")
		}
	})

	t.Run("document not found returns error", func(t *testing.T) {
		t.Parallel()

		repo := &mockDocumentRepo{
			findByUUIDFn: func(_ context.Context, _ string) (*model.Document, error) {
				return nil, sql.ErrNoRows
			},
		}
		svc := NewDocumentService(repo, testutil.DiscardLogger())

		err := svc.Delete(context.Background(), "ghost-uuid")
		if err == nil {
			t.Fatal("expected error, got nil")
		}
		if !errors.Is(err, ErrNotFound) {
			t.Errorf("expected ErrNotFound, got: %v", err)
		}
	})
}

// ---------------------------------------------------------------------------
// TestDocumentService_Delete_SoftDeleteError
// ---------------------------------------------------------------------------

func TestDocumentService_Delete_SoftDeleteError(t *testing.T) {
	t.Parallel()

	repo := &mockDocumentRepo{
		findByUUIDFn: func(_ context.Context, _ string) (*model.Document, error) {
			return &model.Document{ID: 9, UUID: "err-uuid"}, nil
		},
		softDeleteFn: func(_ context.Context, _ int64) error {
			return errors.New("foreign key constraint")
		},
	}
	svc := NewDocumentService(repo, testutil.DiscardLogger())

	err := svc.Delete(context.Background(), "err-uuid")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "soft deleting document") {
		t.Errorf("error %q does not contain %q", err.Error(), "soft deleting document")
	}
}

// ---------------------------------------------------------------------------
// TestDocumentService_Create_WithUserID
// ---------------------------------------------------------------------------

func TestDocumentService_Create_WithUserID(t *testing.T) {
	t.Parallel()

	var createdDoc *model.Document
	repo := &mockDocumentRepo{
		createFn: func(_ context.Context, doc *model.Document) error {
			createdDoc = doc
			doc.ID = 50
			return nil
		},
		findByIDFn: func(_ context.Context, _ int64) (*model.Document, error) {
			return createdDoc, nil
		},
	}
	svc := NewDocumentService(repo, testutil.DiscardLogger())

	uid := int64(42)
	params := CreateDocumentParams{
		Title:    "With User",
		Content:  "content",
		FileType: "md",
		UserID:   &uid,
	}

	doc, err := svc.Create(context.Background(), params)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !doc.UserID.Valid {
		t.Fatal("expected UserID to be valid")
	}
	if doc.UserID.Int64 != 42 {
		t.Errorf("UserID = %d, want 42", doc.UserID.Int64)
	}
}

// ---------------------------------------------------------------------------
// TestDocumentService_Create_WithDescription
// ---------------------------------------------------------------------------

func TestDocumentService_Create_WithDescription(t *testing.T) {
	t.Parallel()

	var createdDoc *model.Document
	repo := &mockDocumentRepo{
		createFn: func(_ context.Context, doc *model.Document) error {
			createdDoc = doc
			doc.ID = 51
			return nil
		},
		findByIDFn: func(_ context.Context, _ int64) (*model.Document, error) {
			return createdDoc, nil
		},
	}
	svc := NewDocumentService(repo, testutil.DiscardLogger())

	params := CreateDocumentParams{
		Title:       "Described",
		Content:     "body text",
		FileType:    "html",
		Description: "A helpful description",
	}

	doc, err := svc.Create(context.Background(), params)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !doc.Description.Valid {
		t.Fatal("expected Description to be valid")
	}
	if doc.Description.String != "A helpful description" {
		t.Errorf("Description = %q, want %q", doc.Description.String, "A helpful description")
	}
}

// ---------------------------------------------------------------------------
// TestDocumentService_Create_EmptyDescription
// ---------------------------------------------------------------------------

func TestDocumentService_Create_EmptyDescription(t *testing.T) {
	t.Parallel()

	var createdDoc *model.Document
	repo := &mockDocumentRepo{
		createFn: func(_ context.Context, doc *model.Document) error {
			createdDoc = doc
			doc.ID = 52
			return nil
		},
		findByIDFn: func(_ context.Context, _ int64) (*model.Document, error) {
			return createdDoc, nil
		},
	}
	svc := NewDocumentService(repo, testutil.DiscardLogger())

	params := CreateDocumentParams{
		Title:    "No Description",
		Content:  "body",
		FileType: "md",
	}

	doc, err := svc.Create(context.Background(), params)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if doc.Description.Valid {
		t.Errorf("expected Description to be invalid (empty), got %q", doc.Description.String)
	}
}

// ---------------------------------------------------------------------------
// TestDocumentService_Create_ReplaceTagsError
// ---------------------------------------------------------------------------

func TestDocumentService_Create_ReplaceTagsError(t *testing.T) {
	t.Parallel()

	repo := &mockDocumentRepo{
		createFn: func(_ context.Context, doc *model.Document) error {
			doc.ID = 53
			return nil
		},
		replaceTagsFn: func(_ context.Context, _ int64, _ []string) error {
			return errors.New("tag constraint")
		},
	}
	svc := NewDocumentService(repo, testutil.DiscardLogger())

	params := CreateDocumentParams{
		Title:    "Tag Fail",
		Content:  "body",
		FileType: "md",
		Tags:     []string{"bad-tag"},
	}

	_, err := svc.Create(context.Background(), params)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "setting tags on new document") {
		t.Errorf("error %q does not contain %q", err.Error(), "setting tags on new document")
	}
}

// ---------------------------------------------------------------------------
// TestDocumentService_Create_FindByIDError
// ---------------------------------------------------------------------------

func TestDocumentService_Create_FindByIDError(t *testing.T) {
	t.Parallel()

	repo := &mockDocumentRepo{
		createFn: func(_ context.Context, doc *model.Document) error {
			doc.ID = 54
			return nil
		},
		findByIDFn: func(_ context.Context, _ int64) (*model.Document, error) {
			return nil, errors.New("unexpected connection loss")
		},
	}
	svc := NewDocumentService(repo, testutil.DiscardLogger())

	params := CreateDocumentParams{
		Title:    "Refetch Fail",
		Content:  "body",
		FileType: "md",
	}

	_, err := svc.Create(context.Background(), params)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "re-fetching created document") {
		t.Errorf("error %q does not contain %q", err.Error(), "re-fetching created document")
	}
}

// ---------------------------------------------------------------------------
// TestDocumentService_Update_DescriptionChange
// ---------------------------------------------------------------------------

func TestDocumentService_Update_DescriptionChange(t *testing.T) {
	t.Parallel()

	var updatedDoc *model.Document
	repo := &mockDocumentRepo{
		findByUUIDFn: func(_ context.Context, _ string) (*model.Document, error) {
			return &model.Document{
				ID:    6,
				UUID:  "desc-uuid",
				Title: "Original",
			}, nil
		},
		updateFn: func(_ context.Context, doc *model.Document) error {
			updatedDoc = doc
			return nil
		},
		findByIDFn: func(_ context.Context, _ int64) (*model.Document, error) {
			return updatedDoc, nil
		},
	}
	svc := NewDocumentService(repo, testutil.DiscardLogger())

	doc, err := svc.Update(context.Background(), "desc-uuid", UpdateDocumentParams{
		Description: "Updated description",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !doc.Description.Valid || doc.Description.String != "Updated description" {
		t.Errorf("Description = %q (valid=%v), want %q", doc.Description.String, doc.Description.Valid, "Updated description")
	}
	// Title should remain unchanged.
	if doc.Title != "Original" {
		t.Errorf("Title = %q, want %q (should not change)", doc.Title, "Original")
	}
}

// ---------------------------------------------------------------------------
// TestDocumentService_Update_ReplaceTagsError
// ---------------------------------------------------------------------------

func TestDocumentService_Update_ReplaceTagsError(t *testing.T) {
	t.Parallel()

	repo := &mockDocumentRepo{
		findByUUIDFn: func(_ context.Context, _ string) (*model.Document, error) {
			return &model.Document{ID: 8, UUID: "tag-err-uuid"}, nil
		},
		updateFn: func(_ context.Context, _ *model.Document) error {
			return nil
		},
		replaceTagsFn: func(_ context.Context, _ int64, _ []string) error {
			return errors.New("tag replace failure")
		},
	}
	svc := NewDocumentService(repo, testutil.DiscardLogger())

	_, err := svc.Update(context.Background(), "tag-err-uuid", UpdateDocumentParams{
		Tags: []string{"fail"},
	})
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "replacing tags on document") {
		t.Errorf("error %q does not contain %q", err.Error(), "replacing tags on document")
	}
}

// ---------------------------------------------------------------------------
// TestDocumentService_Update_FindByIDError
// ---------------------------------------------------------------------------

func TestDocumentService_Update_FindByIDError(t *testing.T) {
	t.Parallel()

	repo := &mockDocumentRepo{
		findByUUIDFn: func(_ context.Context, _ string) (*model.Document, error) {
			return &model.Document{ID: 9, UUID: "refetch-err"}, nil
		},
		updateFn: func(_ context.Context, _ *model.Document) error {
			return nil
		},
		findByIDFn: func(_ context.Context, _ int64) (*model.Document, error) {
			return nil, errors.New("db gone")
		},
	}
	svc := NewDocumentService(repo, testutil.DiscardLogger())

	_, err := svc.Update(context.Background(), "refetch-err", UpdateDocumentParams{
		Title: "Whatever",
	})
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "re-fetching updated document") {
		t.Errorf("error %q does not contain %q", err.Error(), "re-fetching updated document")
	}
}

// ---------------------------------------------------------------------------
// TestDocumentService_TagsForDocument
// ---------------------------------------------------------------------------

func TestDocumentService_TagsForDocument(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		repoFn    func(ctx context.Context, documentID int64) ([]model.DocumentTag, error)
		wantCount int
		wantErr   bool
	}{
		{
			name: "returns tags",
			repoFn: func(_ context.Context, _ int64) ([]model.DocumentTag, error) {
				return []model.DocumentTag{
					{ID: 1, DocumentID: 10, Tag: "go"},
					{ID: 2, DocumentID: 10, Tag: "testing"},
				}, nil
			},
			wantCount: 2,
		},
		{
			name: "returns empty slice when no tags",
			repoFn: func(_ context.Context, _ int64) ([]model.DocumentTag, error) {
				return []model.DocumentTag{}, nil
			},
			wantCount: 0,
		},
		{
			name: "returns nil when repo returns nil",
			repoFn: func(_ context.Context, _ int64) ([]model.DocumentTag, error) {
				return nil, nil
			},
			wantCount: 0,
		},
		{
			name: "propagates repo error",
			repoFn: func(_ context.Context, _ int64) ([]model.DocumentTag, error) {
				return nil, errors.New("db error")
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			repo := &mockDocumentRepo{tagsForDocFn: tt.repoFn}
			svc := NewDocumentService(repo, testutil.DiscardLogger())

			tags, err := svc.TagsForDocument(context.Background(), 10)

			if tt.wantErr {
				if err == nil {
					t.Fatal("expected error, got nil")
				}
				return
			}

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if len(tags) != tt.wantCount {
				t.Errorf("len(tags) = %d, want %d", len(tags), tt.wantCount)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// TestDocumentService_Update_NilTagsDoesNotCallReplaceTags
// ---------------------------------------------------------------------------

func TestDocumentService_Update_NilTagsDoesNotCallReplaceTags(t *testing.T) {
	t.Parallel()

	replaceTagsCalled := false
	var updatedDoc *model.Document
	repo := &mockDocumentRepo{
		findByUUIDFn: func(_ context.Context, _ string) (*model.Document, error) {
			return &model.Document{ID: 20, UUID: "no-tags-uuid", Title: "Old"}, nil
		},
		updateFn: func(_ context.Context, doc *model.Document) error {
			updatedDoc = doc
			return nil
		},
		replaceTagsFn: func(_ context.Context, _ int64, _ []string) error {
			replaceTagsCalled = true
			return nil
		},
		findByIDFn: func(_ context.Context, _ int64) (*model.Document, error) {
			return updatedDoc, nil
		},
	}
	svc := NewDocumentService(repo, testutil.DiscardLogger())

	// Tags field is nil (not provided), should not call ReplaceTags.
	_, err := svc.Update(context.Background(), "no-tags-uuid", UpdateDocumentParams{
		Title: "New Title",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if replaceTagsCalled {
		t.Error("expected ReplaceTags NOT to be called when Tags is nil")
	}
}

// ---------------------------------------------------------------------------
// TestDocumentService_Update_EmptyTagsCallsReplaceTags
// ---------------------------------------------------------------------------

func TestDocumentService_Update_EmptyTagsCallsReplaceTags(t *testing.T) {
	t.Parallel()

	replaceTagsCalled := false
	var replacedTags []string
	var updatedDoc *model.Document
	repo := &mockDocumentRepo{
		findByUUIDFn: func(_ context.Context, _ string) (*model.Document, error) {
			return &model.Document{ID: 21, UUID: "empty-tags-uuid"}, nil
		},
		updateFn: func(_ context.Context, doc *model.Document) error {
			updatedDoc = doc
			return nil
		},
		replaceTagsFn: func(_ context.Context, _ int64, tags []string) error {
			replaceTagsCalled = true
			replacedTags = tags
			return nil
		},
		findByIDFn: func(_ context.Context, _ int64) (*model.Document, error) {
			return updatedDoc, nil
		},
	}
	svc := NewDocumentService(repo, testutil.DiscardLogger())

	// Tags is an empty (non-nil) slice -- should clear all tags.
	_, err := svc.Update(context.Background(), "empty-tags-uuid", UpdateDocumentParams{
		Tags: []string{},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !replaceTagsCalled {
		t.Error("expected ReplaceTags to be called with empty slice to clear tags")
	}
	if len(replacedTags) != 0 {
		t.Errorf("ReplaceTags tags = %v, want empty", replacedTags)
	}
}

// ---------------------------------------------------------------------------
// TestMimeTypeForFileType
// ---------------------------------------------------------------------------

func TestMimeTypeForFileType(t *testing.T) {
	t.Parallel()

	tests := []struct {
		fileType string
		want     string
	}{
		{"markdown", "text/markdown"},
		{"md", "text/markdown"},
		{"txt", "text/markdown"},
		{"html", "text/html"},
		{"htm", "text/html"},
		{"pdf", "application/pdf"},
		{"docx", "application/vnd.openxmlformats-officedocument.wordprocessingml.document"},
		{"xlsx", "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet"},
		{"other", "application/octet-stream"},
		{"", "application/octet-stream"},
		{"MARKDOWN", "text/markdown"},
		{"Html", "text/html"},
		{"DOCX", "application/vnd.openxmlformats-officedocument.wordprocessingml.document"},
		{"PDF", "application/pdf"},
	}

	for _, tt := range tests {
		t.Run(tt.fileType, func(t *testing.T) {
			t.Parallel()

			got := mimeTypeForFileType(tt.fileType)
			if got != tt.want {
				t.Errorf("mimeTypeForFileType(%q) = %q, want %q", tt.fileType, got, tt.want)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// TestDocumentService_ReplaceInlineContent
// ---------------------------------------------------------------------------

func TestDocumentService_ReplaceInlineContent(t *testing.T) {
	t.Parallel()

	// existingInlineDoc returns a fresh markdown document with metadata that
	// must survive a content replacement. Each subtest gets its own copy so
	// in-place mutation by the service under test does not leak between runs.
	existingInlineDoc := func() *model.Document {
		return &model.Document{
			ID:          7,
			UUID:        "inline-uuid",
			Title:       "Original Title",
			FileType:    "markdown",
			FilePath:    "", // inline
			MIMEType:    "text/markdown",
			Content:     sql.NullString{String: "old body", Valid: true},
			ContentHash: sql.NullString{String: "oldhash", Valid: true},
			WordCount:   sql.NullInt64{Int64: 2, Valid: true},
			ProcessedAt: sql.NullTime{Time: time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC), Valid: true},
			FileSize:    int64(len("old body")),
			IsPublic:    true,
			UserID:      sql.NullInt64{Int64: 42, Valid: true},
			Description: sql.NullString{String: "untouched desc", Valid: true},
			Status:      model.DocumentStatusIndexed,
		}
	}

	t.Run("success recomputes derived fields and preserves metadata", func(t *testing.T) {
		t.Parallel()

		var updatedDoc *model.Document
		repo := &mockDocumentRepo{
			findByUUIDFn: func(_ context.Context, _ string) (*model.Document, error) {
				return existingInlineDoc(), nil
			},
			updateFn: func(_ context.Context, doc *model.Document) error {
				updatedDoc = doc
				return nil
			},
			findByIDFn: func(_ context.Context, id int64) (*model.Document, error) {
				if id != 7 {
					t.Errorf("FindByID called with %d, want 7", id)
				}
				return updatedDoc, nil
			},
		}
		svc := NewDocumentService(repo, testutil.DiscardLogger())

		newContent := "freshly written body with five words"
		doc, err := svc.ReplaceInlineContent(context.Background(), "inline-uuid", ReplaceInlineContentParams{
			Content: newContent,
		})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		// Body actually changed.
		if doc.Content.String != newContent {
			t.Errorf("Content = %q, want %q", doc.Content.String, newContent)
		}

		// Hash recomputed from new content.
		wantHash := sha256.Sum256([]byte(newContent))
		wantHashStr := hex.EncodeToString(wantHash[:])
		if doc.ContentHash.String != wantHashStr {
			t.Errorf("ContentHash = %q, want %q", doc.ContentHash.String, wantHashStr)
		}

		// Word count = strings.Fields-based count (6 words in "freshly written body with five words").
		if doc.WordCount.Int64 != 6 {
			t.Errorf("WordCount = %d, want 6", doc.WordCount.Int64)
		}

		// FileSize matches the new byte length.
		if doc.FileSize != int64(len(newContent)) {
			t.Errorf("FileSize = %d, want %d", doc.FileSize, len(newContent))
		}

		// ProcessedAt updated (newer than the seed timestamp).
		if !doc.ProcessedAt.Valid {
			t.Fatal("ProcessedAt is not valid; expected updated timestamp")
		}
		seed := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
		if !doc.ProcessedAt.Time.After(seed) {
			t.Errorf("ProcessedAt = %v, want after %v", doc.ProcessedAt.Time, seed)
		}

		// Status stays indexed (no extraction worker).
		if doc.Status != model.DocumentStatusIndexed {
			t.Errorf("Status = %q, want %q", doc.Status, model.DocumentStatusIndexed)
		}

		// Metadata fields preserved.
		if doc.Title != "Original Title" {
			t.Errorf("Title mutated to %q", doc.Title)
		}
		if doc.FileType != "markdown" {
			t.Errorf("FileType mutated to %q", doc.FileType)
		}
		if doc.Description.String != "untouched desc" {
			t.Errorf("Description mutated to %q", doc.Description.String)
		}
		if !doc.IsPublic {
			t.Error("IsPublic flipped to false")
		}
		if !doc.UserID.Valid || doc.UserID.Int64 != 42 {
			t.Errorf("UserID mutated to %+v", doc.UserID)
		}

		// ErrorMessage cleared even though we didn't seed one (defensive).
		if doc.ErrorMessage.Valid {
			t.Errorf("ErrorMessage left set: %+v", doc.ErrorMessage)
		}
	})

	t.Run("file-backed document returns ErrFileBackedDocument", func(t *testing.T) {
		t.Parallel()

		repo := &mockDocumentRepo{
			findByUUIDFn: func(_ context.Context, _ string) (*model.Document, error) {
				doc := existingInlineDoc()
				doc.FilePath = "pdf/inline-uuid.pdf" // file-backed
				doc.FileType = "pdf"
				return doc, nil
			},
			updateFn: func(_ context.Context, _ *model.Document) error {
				t.Fatal("Update must not be called for file-backed docs")
				return nil
			},
		}
		svc := NewDocumentService(repo, testutil.DiscardLogger())

		_, err := svc.ReplaceInlineContent(context.Background(), "inline-uuid", ReplaceInlineContentParams{Content: "new"})
		if err == nil {
			t.Fatal("expected error, got nil")
		}
		if !errors.Is(err, ErrFileBackedDocument) {
			t.Errorf("errors.Is(err, ErrFileBackedDocument) = false, err = %q", err.Error())
		}
	})

	t.Run("not found returns ErrNotFound", func(t *testing.T) {
		t.Parallel()

		repo := &mockDocumentRepo{
			findByUUIDFn: func(_ context.Context, _ string) (*model.Document, error) {
				return nil, sql.ErrNoRows
			},
		}
		svc := NewDocumentService(repo, testutil.DiscardLogger())

		_, err := svc.ReplaceInlineContent(context.Background(), "missing", ReplaceInlineContentParams{Content: "new"})
		if !errors.Is(err, ErrNotFound) {
			t.Errorf("errors.Is(err, ErrNotFound) = false, err = %v", err)
		}
	})

	t.Run("repository update error is wrapped", func(t *testing.T) {
		t.Parallel()

		repo := &mockDocumentRepo{
			findByUUIDFn: func(_ context.Context, _ string) (*model.Document, error) {
				return existingInlineDoc(), nil
			},
			updateFn: func(_ context.Context, _ *model.Document) error {
				return errors.New("disk full")
			},
		}
		svc := NewDocumentService(repo, testutil.DiscardLogger())

		_, err := svc.ReplaceInlineContent(context.Background(), "inline-uuid", ReplaceInlineContentParams{Content: "new"})
		if err == nil {
			t.Fatal("expected error, got nil")
		}
		if !strings.Contains(err.Error(), "updating document content") {
			t.Errorf("error %q does not contain %q", err.Error(), "updating document content")
		}
	})

	t.Run("FindByID error after update is wrapped", func(t *testing.T) {
		t.Parallel()

		repo := &mockDocumentRepo{
			findByUUIDFn: func(_ context.Context, _ string) (*model.Document, error) {
				return existingInlineDoc(), nil
			},
			updateFn: func(_ context.Context, _ *model.Document) error { return nil },
			findByIDFn: func(_ context.Context, _ int64) (*model.Document, error) {
				return nil, errors.New("connection closed")
			},
		}
		svc := NewDocumentService(repo, testutil.DiscardLogger())

		_, err := svc.ReplaceInlineContent(context.Background(), "inline-uuid", ReplaceInlineContentParams{Content: "new"})
		if err == nil {
			t.Fatal("expected error, got nil")
		}
		if !strings.Contains(err.Error(), "re-fetching updated document") {
			t.Errorf("error %q does not contain %q", err.Error(), "re-fetching updated document")
		}
	})
}
