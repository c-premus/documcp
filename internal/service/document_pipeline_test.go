package service

import (
	"context"
	"database/sql"
	"errors"
	"io"
	"strings"
	"testing"

	"git.999.haus/chris/DocuMCP-go/internal/model"
)

// ---------------------------------------------------------------------------
// TestDocumentPipeline_Upload
// ---------------------------------------------------------------------------

func TestDocumentPipeline_Upload(t *testing.T) {
	t.Parallel()

	t.Run("success stores file and creates record", func(t *testing.T) {
		t.Parallel()

		storagePath := t.TempDir()

		var createdDoc *model.Document
		repo := &mockDocumentRepo{
			createFn: func(_ context.Context, doc *model.Document) error {
				createdDoc = doc
				doc.ID = 100
				return nil
			},
			findByIDFn: func(_ context.Context, id int64) (*model.Document, error) {
				if id != 100 {
					t.Errorf("FindByID called with %d, want 100", id)
				}
				return createdDoc, nil
			},
		}
		svc := NewDocumentService(repo, discardLogger())
		pipeline := NewDocumentPipeline(svc, nil, nil, nil, storagePath)

		content := "hello pipeline"
		params := UploadDocumentParams{
			Title:    "Pipeline Doc",
			FileName: "test.md",
			FileSize: int64(len(content)),
			Reader:   strings.NewReader(content),
			IsPublic: true,
		}

		doc, err := pipeline.Upload(context.Background(), params)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if doc.Title != "Pipeline Doc" {
			t.Errorf("Title = %q, want %q", doc.Title, "Pipeline Doc")
		}
		if doc.Status != "uploaded" {
			t.Errorf("Status = %q, want %q", doc.Status, "uploaded")
		}
		if doc.MIMEType != "text/markdown" {
			t.Errorf("MIMEType = %q, want %q", doc.MIMEType, "text/markdown")
		}
		if doc.FileType != "md" {
			t.Errorf("FileType = %q, want %q", doc.FileType, "md")
		}
		if !doc.IsPublic {
			t.Error("expected IsPublic to be true")
		}
		if doc.UUID == "" || len(doc.UUID) != 36 {
			t.Errorf("UUID = %q, want a 36-char UUID", doc.UUID)
		}
		if doc.FilePath == "" {
			t.Error("expected FilePath to be set")
		}
		if doc.FileSize != int64(len(content)) {
			t.Errorf("FileSize = %d, want %d", doc.FileSize, len(content))
		}
	})

	t.Run("file exceeds maximum size", func(t *testing.T) {
		t.Parallel()

		svc := NewDocumentService(&mockDocumentRepo{}, discardLogger())
		pipeline := NewDocumentPipeline(svc, nil, nil, nil, t.TempDir())

		params := UploadDocumentParams{
			Title:    "Too Big",
			FileName: "big.pdf",
			FileSize: maxUploadSize + 1,
			Reader:   strings.NewReader(""),
		}

		_, err := pipeline.Upload(context.Background(), params)
		if err == nil {
			t.Fatal("expected error, got nil")
		}
		if !strings.Contains(err.Error(), "file exceeds maximum size") {
			t.Errorf("error %q does not contain %q", err.Error(), "file exceeds maximum size")
		}
	})

	t.Run("unsupported file type", func(t *testing.T) {
		t.Parallel()

		svc := NewDocumentService(&mockDocumentRepo{}, discardLogger())
		pipeline := NewDocumentPipeline(svc, nil, nil, nil, t.TempDir())

		params := UploadDocumentParams{
			Title:    "Bad Type",
			FileName: "virus.exe",
			FileSize: 100,
			Reader:   strings.NewReader("data"),
		}

		_, err := pipeline.Upload(context.Background(), params)
		if err == nil {
			t.Fatal("expected error, got nil")
		}
		if !strings.Contains(err.Error(), "unsupported file type") {
			t.Errorf("error %q does not contain %q", err.Error(), "unsupported file type")
		}
	})

	t.Run("repository create error cleans up file", func(t *testing.T) {
		t.Parallel()

		repo := &mockDocumentRepo{
			createFn: func(_ context.Context, _ *model.Document) error {
				return errors.New("insert failed")
			},
		}
		svc := NewDocumentService(repo, discardLogger())
		pipeline := NewDocumentPipeline(svc, nil, nil, nil, t.TempDir())

		params := UploadDocumentParams{
			Title:    "Create Fail",
			FileName: "doc.html",
			FileSize: 5,
			Reader:   strings.NewReader("hello"),
		}

		_, err := pipeline.Upload(context.Background(), params)
		if err == nil {
			t.Fatal("expected error, got nil")
		}
		if !strings.Contains(err.Error(), "creating document record") {
			t.Errorf("error %q does not contain %q", err.Error(), "creating document record")
		}
	})

	t.Run("with tags calls ReplaceTags", func(t *testing.T) {
		t.Parallel()

		replaceTagsCalled := false
		var replacedTags []string
		var createdDoc *model.Document
		repo := &mockDocumentRepo{
			createFn: func(_ context.Context, doc *model.Document) error {
				createdDoc = doc
				doc.ID = 101
				return nil
			},
			replaceTagsFn: func(_ context.Context, docID int64, tags []string) error {
				replaceTagsCalled = true
				replacedTags = tags
				if docID != 101 {
					t.Errorf("ReplaceTags docID = %d, want 101", docID)
				}
				return nil
			},
			findByIDFn: func(_ context.Context, _ int64) (*model.Document, error) {
				return createdDoc, nil
			},
		}
		svc := NewDocumentService(repo, discardLogger())
		pipeline := NewDocumentPipeline(svc, nil, nil, nil, t.TempDir())

		params := UploadDocumentParams{
			Title:    "Tagged Upload",
			FileName: "tagged.md",
			FileSize: 4,
			Reader:   strings.NewReader("body"),
			Tags:     []string{"upload", "test"},
		}

		_, err := pipeline.Upload(context.Background(), params)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !replaceTagsCalled {
			t.Error("expected ReplaceTags to be called")
		}
		if len(replacedTags) != 2 || replacedTags[0] != "upload" || replacedTags[1] != "test" {
			t.Errorf("ReplaceTags tags = %v, want [upload test]", replacedTags)
		}
	})

	t.Run("ReplaceTags error propagates", func(t *testing.T) {
		t.Parallel()

		repo := &mockDocumentRepo{
			createFn: func(_ context.Context, doc *model.Document) error {
				doc.ID = 102
				return nil
			},
			replaceTagsFn: func(_ context.Context, _ int64, _ []string) error {
				return errors.New("tag insert failed")
			},
		}
		svc := NewDocumentService(repo, discardLogger())
		pipeline := NewDocumentPipeline(svc, nil, nil, nil, t.TempDir())

		params := UploadDocumentParams{
			Title:    "Tag Fail Upload",
			FileName: "fail.pdf",
			FileSize: 3,
			Reader:   strings.NewReader("abc"),
			Tags:     []string{"bad"},
		}

		_, err := pipeline.Upload(context.Background(), params)
		if err == nil {
			t.Fatal("expected error, got nil")
		}
		if !strings.Contains(err.Error(), "setting tags on uploaded document") {
			t.Errorf("error %q does not contain %q", err.Error(), "setting tags on uploaded document")
		}
	})

	t.Run("FindByID re-fetch error propagates", func(t *testing.T) {
		t.Parallel()

		repo := &mockDocumentRepo{
			createFn: func(_ context.Context, doc *model.Document) error {
				doc.ID = 103
				return nil
			},
			findByIDFn: func(_ context.Context, _ int64) (*model.Document, error) {
				return nil, errors.New("refetch boom")
			},
		}
		svc := NewDocumentService(repo, discardLogger())
		pipeline := NewDocumentPipeline(svc, nil, nil, nil, t.TempDir())

		params := UploadDocumentParams{
			Title:    "Refetch Fail",
			FileName: "ok.htm",
			FileSize: 2,
			Reader:   strings.NewReader("hi"),
		}

		_, err := pipeline.Upload(context.Background(), params)
		if err == nil {
			t.Fatal("expected error, got nil")
		}
		if !strings.Contains(err.Error(), "re-fetching uploaded document") {
			t.Errorf("error %q does not contain %q", err.Error(), "re-fetching uploaded document")
		}
	})

	t.Run("with UserID sets UserID on document", func(t *testing.T) {
		t.Parallel()

		var createdDoc *model.Document
		repo := &mockDocumentRepo{
			createFn: func(_ context.Context, doc *model.Document) error {
				createdDoc = doc
				doc.ID = 104
				return nil
			},
			findByIDFn: func(_ context.Context, _ int64) (*model.Document, error) {
				return createdDoc, nil
			},
		}
		svc := NewDocumentService(repo, discardLogger())
		pipeline := NewDocumentPipeline(svc, nil, nil, nil, t.TempDir())

		uid := int64(77)
		params := UploadDocumentParams{
			Title:    "User Upload",
			FileName: "user.md",
			FileSize: 5,
			Reader:   strings.NewReader("hello"),
			UserID:   &uid,
		}

		doc, err := pipeline.Upload(context.Background(), params)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !doc.UserID.Valid {
			t.Fatal("expected UserID to be valid")
		}
		if doc.UserID.Int64 != 77 {
			t.Errorf("UserID = %d, want 77", doc.UserID.Int64)
		}
	})

	t.Run("with description sets description on document", func(t *testing.T) {
		t.Parallel()

		var createdDoc *model.Document
		repo := &mockDocumentRepo{
			createFn: func(_ context.Context, doc *model.Document) error {
				createdDoc = doc
				doc.ID = 105
				return nil
			},
			findByIDFn: func(_ context.Context, _ int64) (*model.Document, error) {
				return createdDoc, nil
			},
		}
		svc := NewDocumentService(repo, discardLogger())
		pipeline := NewDocumentPipeline(svc, nil, nil, nil, t.TempDir())

		params := UploadDocumentParams{
			Title:       "Described Upload",
			Description: "Important doc",
			FileName:    "desc.md",
			FileSize:    4,
			Reader:      strings.NewReader("body"),
		}

		doc, err := pipeline.Upload(context.Background(), params)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !doc.Description.Valid || doc.Description.String != "Important doc" {
			t.Errorf("Description = %q (valid=%v), want %q", doc.Description.String, doc.Description.Valid, "Important doc")
		}
	})

	t.Run("empty description is not set", func(t *testing.T) {
		t.Parallel()

		var createdDoc *model.Document
		repo := &mockDocumentRepo{
			createFn: func(_ context.Context, doc *model.Document) error {
				createdDoc = doc
				doc.ID = 106
				return nil
			},
			findByIDFn: func(_ context.Context, _ int64) (*model.Document, error) {
				return createdDoc, nil
			},
		}
		svc := NewDocumentService(repo, discardLogger())
		pipeline := NewDocumentPipeline(svc, nil, nil, nil, t.TempDir())

		params := UploadDocumentParams{
			Title:    "No Desc",
			FileName: "nodesc.md",
			FileSize: 3,
			Reader:   strings.NewReader("abc"),
		}

		doc, err := pipeline.Upload(context.Background(), params)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if doc.Description.Valid {
			t.Errorf("expected Description to be invalid (empty), got %q", doc.Description.String)
		}
	})

	t.Run("io read error cleans up", func(t *testing.T) {
		t.Parallel()

		repo := &mockDocumentRepo{}
		svc := NewDocumentService(repo, discardLogger())
		pipeline := NewDocumentPipeline(svc, nil, nil, nil, t.TempDir())

		params := UploadDocumentParams{
			Title:    "Read Error",
			FileName: "err.md",
			FileSize: 100,
			Reader:   &failingReader{err: errors.New("read failure")},
		}

		_, err := pipeline.Upload(context.Background(), params)
		if err == nil {
			t.Fatal("expected error, got nil")
		}
		if !strings.Contains(err.Error(), "writing uploaded file") {
			t.Errorf("error %q does not contain %q", err.Error(), "writing uploaded file")
		}
	})
}

// ---------------------------------------------------------------------------
// TestDocumentPipeline_Upload_FileTypeMapping
// ---------------------------------------------------------------------------

func TestDocumentPipeline_Upload_FileTypeMapping(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name         string
		fileName     string
		wantFileType string
		wantMIME     string
	}{
		{
			name:         "markdown file",
			fileName:     "readme.md",
			wantFileType: "md",
			wantMIME:     "text/markdown",
		},
		{
			name:         "txt maps to markdown",
			fileName:     "notes.txt",
			wantFileType: "markdown",
			wantMIME:     "text/plain",
		},
		{
			name:         "html file",
			fileName:     "page.html",
			wantFileType: "html",
			wantMIME:     "text/html",
		},
		{
			name:         "htm maps to html",
			fileName:     "page.htm",
			wantFileType: "html",
			wantMIME:     "text/html",
		},
		{
			name:         "pdf file",
			fileName:     "doc.pdf",
			wantFileType: "pdf",
			wantMIME:     "application/pdf",
		},
		{
			name:         "docx file",
			fileName:     "report.docx",
			wantFileType: "docx",
			wantMIME:     "application/vnd.openxmlformats-officedocument.wordprocessingml.document",
		},
		{
			name:         "xlsx file",
			fileName:     "data.xlsx",
			wantFileType: "xlsx",
			wantMIME:     "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet",
		},
		{
			name:         "uppercase extension normalized",
			fileName:     "CAPS.PDF",
			wantFileType: "pdf",
			wantMIME:     "application/pdf",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			var createdDoc *model.Document
			repo := &mockDocumentRepo{
				createFn: func(_ context.Context, doc *model.Document) error {
					createdDoc = doc
					doc.ID = 200
					return nil
				},
				findByIDFn: func(_ context.Context, _ int64) (*model.Document, error) {
					return createdDoc, nil
				},
			}
			svc := NewDocumentService(repo, discardLogger())
			pipeline := NewDocumentPipeline(svc, nil, nil, nil, t.TempDir())

			params := UploadDocumentParams{
				Title:    "Type Test",
				FileName: tt.fileName,
				FileSize: 1,
				Reader:   strings.NewReader("x"),
			}

			doc, err := pipeline.Upload(context.Background(), params)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if doc.FileType != tt.wantFileType {
				t.Errorf("FileType = %q, want %q", doc.FileType, tt.wantFileType)
			}
			if doc.MIMEType != tt.wantMIME {
				t.Errorf("MIMEType = %q, want %q", doc.MIMEType, tt.wantMIME)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// TestDocumentPipeline_Upload_MaxUploadSizeBoundary
// ---------------------------------------------------------------------------

func TestDocumentPipeline_Upload_MaxUploadSizeBoundary(t *testing.T) {
	t.Parallel()

	t.Run("exactly at limit succeeds", func(t *testing.T) {
		t.Parallel()

		var createdDoc *model.Document
		repo := &mockDocumentRepo{
			createFn: func(_ context.Context, doc *model.Document) error {
				createdDoc = doc
				doc.ID = 300
				return nil
			},
			findByIDFn: func(_ context.Context, _ int64) (*model.Document, error) {
				return createdDoc, nil
			},
		}
		svc := NewDocumentService(repo, discardLogger())
		pipeline := NewDocumentPipeline(svc, nil, nil, nil, t.TempDir())

		params := UploadDocumentParams{
			Title:    "At Limit",
			FileName: "limit.md",
			FileSize: maxUploadSize, // exactly at the limit
			Reader:   strings.NewReader("x"),
		}

		_, err := pipeline.Upload(context.Background(), params)
		if err != nil {
			t.Fatalf("expected no error at max size boundary, got: %v", err)
		}
	})

	t.Run("one byte over limit fails", func(t *testing.T) {
		t.Parallel()

		svc := NewDocumentService(&mockDocumentRepo{}, discardLogger())
		pipeline := NewDocumentPipeline(svc, nil, nil, nil, t.TempDir())

		params := UploadDocumentParams{
			Title:    "Over Limit",
			FileName: "big.md",
			FileSize: maxUploadSize + 1,
			Reader:   strings.NewReader("x"),
		}

		_, err := pipeline.Upload(context.Background(), params)
		if err == nil {
			t.Fatal("expected error, got nil")
		}
	})
}

// ---------------------------------------------------------------------------
// TestDocumentPipeline_ProcessDocument
// ---------------------------------------------------------------------------

func TestDocumentPipeline_ProcessDocument(t *testing.T) {
	t.Parallel()

	t.Run("FindByID error propagates", func(t *testing.T) {
		t.Parallel()

		repo := &mockDocumentRepo{
			findByIDFn: func(_ context.Context, _ int64) (*model.Document, error) {
				return nil, errors.New("not found")
			},
		}
		svc := NewDocumentService(repo, discardLogger())
		pipeline := NewDocumentPipeline(svc, nil, nil, nil, t.TempDir())

		err := pipeline.ProcessDocument(context.Background(), 999)
		if err == nil {
			t.Fatal("expected error, got nil")
		}
		if !strings.Contains(err.Error(), "finding document 999 for processing") {
			t.Errorf("error %q does not contain expected prefix", err.Error())
		}
	})
}

// ---------------------------------------------------------------------------
// TestDocumentPipeline_IndexDocument_NilIndexer
// ---------------------------------------------------------------------------

func TestDocumentPipeline_IndexDocument_NilIndexer(t *testing.T) {
	t.Parallel()

	svc := NewDocumentService(&mockDocumentRepo{}, discardLogger())
	pipeline := NewDocumentPipeline(svc, nil, nil, nil, t.TempDir())

	doc := &model.Document{
		ID:   1,
		UUID: "idx-test",
		Content: sql.NullString{
			String: "content",
			Valid:  true,
		},
	}

	err := pipeline.IndexDocument(context.Background(), doc)
	if err != nil {
		t.Fatalf("expected nil error when indexer is nil, got: %v", err)
	}
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

// failingReader is an io.Reader that always returns an error.
type failingReader struct {
	err error
}

func (r *failingReader) Read(_ []byte) (int, error) {
	return 0, r.err
}

// Compile-time check: failingReader implements io.Reader.
var _ io.Reader = (*failingReader)(nil)
