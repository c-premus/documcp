package service

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"errors"
	"io"
	"log/slog"
	"strings"
	"testing"

	"git.999.haus/chris/DocuMCP-go/internal/model"
)

// ---------------------------------------------------------------------------
// Mock
// ---------------------------------------------------------------------------

type mockDocumentRepo struct {
	findByUUIDFn    func(ctx context.Context, uuid string) (*model.Document, error)
	findByIDFn      func(ctx context.Context, id int64) (*model.Document, error)
	createFn        func(ctx context.Context, doc *model.Document) error
	updateFn        func(ctx context.Context, doc *model.Document) error
	softDeleteFn    func(ctx context.Context, id int64) error
	tagsForDocFn    func(ctx context.Context, documentID int64) ([]model.DocumentTag, error)
	replaceTagsFn   func(ctx context.Context, documentID int64, tags []string) error
	createVersionFn func(ctx context.Context, version *model.DocumentVersion) error
}

func (m *mockDocumentRepo) FindByUUID(ctx context.Context, uuid string) (*model.Document, error) {
	if m.findByUUIDFn != nil {
		return m.findByUUIDFn(ctx, uuid)
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

func (m *mockDocumentRepo) TagsForDocument(ctx context.Context, documentID int64) ([]model.DocumentTag, error) {
	if m.tagsForDocFn != nil {
		return m.tagsForDocFn(ctx, documentID)
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

func discardLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

func ptrBool(b bool) *bool { return &b }

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
		errSubstr string
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
			errSubstr: "not found",
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
			svc := NewDocumentService(repo, discardLogger())

			doc, err := svc.FindByUUID(context.Background(), "abc-123")

			if tt.wantErr {
				if err == nil {
					t.Fatal("expected error, got nil")
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
		svc := NewDocumentService(repo, discardLogger())

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
		if doc.Status != "processed" {
			t.Errorf("Status = %q, want %q", doc.Status, "processed")
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
		svc := NewDocumentService(repo, discardLogger())

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
		svc := NewDocumentService(repo, discardLogger())

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
		svc := NewDocumentService(repo, discardLogger())

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
		svc := NewDocumentService(repo, discardLogger())

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
		svc := NewDocumentService(repo, discardLogger())

		doc, err := svc.Update(context.Background(), "update-uuid", UpdateDocumentParams{
			IsPublic: ptrBool(true),
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
		svc := NewDocumentService(repo, discardLogger())

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
		svc := NewDocumentService(repo, discardLogger())

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
		svc := NewDocumentService(repo, discardLogger())

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
		svc := NewDocumentService(repo, discardLogger())

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
		svc := NewDocumentService(repo, discardLogger())

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
	svc := NewDocumentService(repo, discardLogger())

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
	svc := NewDocumentService(repo, discardLogger())

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
	svc := NewDocumentService(repo, discardLogger())

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
	svc := NewDocumentService(repo, discardLogger())

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
	svc := NewDocumentService(repo, discardLogger())

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
	svc := NewDocumentService(repo, discardLogger())

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
	svc := NewDocumentService(repo, discardLogger())

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
	svc := NewDocumentService(repo, discardLogger())

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
	svc := NewDocumentService(repo, discardLogger())

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
			svc := NewDocumentService(repo, discardLogger())

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
	svc := NewDocumentService(repo, discardLogger())

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
	svc := NewDocumentService(repo, discardLogger())

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
		{"html", "text/html"},
		{"other", "application/octet-stream"},
		{"", "application/octet-stream"},
		{"MARKDOWN", "text/markdown"},
		{"Html", "text/html"},
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
