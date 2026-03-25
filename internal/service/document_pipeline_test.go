package service

import (
	"context"
	"database/sql"
	"errors"
	"io"
	"strings"
	"testing"

	"github.com/riverqueue/river"
	"github.com/riverqueue/river/rivertype"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/c-premus/documcp/internal/extractor"
	"github.com/c-premus/documcp/internal/model"
)

// ---------------------------------------------------------------------------
// Pipeline test mocks
// ---------------------------------------------------------------------------

// mockExtractor implements extractor.Extractor for testing.
type mockExtractor struct {
	extractFn  func(ctx context.Context, filePath string) (*extractor.ExtractedContent, error)
	supportsFn func(mimeType string) bool
}

func (m *mockExtractor) Extract(ctx context.Context, filePath string) (*extractor.ExtractedContent, error) {
	if m.extractFn != nil {
		return m.extractFn(ctx, filePath)
	}
	return &extractor.ExtractedContent{}, nil
}

func (m *mockExtractor) Supports(mimeType string) bool {
	if m.supportsFn != nil {
		return m.supportsFn(mimeType)
	}
	return false
}

// Compile-time check: mockExtractor implements extractor.Extractor.
var _ extractor.Extractor = (*mockExtractor)(nil)

// mockJobInserter implements JobInserter for testing.
type mockJobInserter struct {
	insertFn func(ctx context.Context, args river.JobArgs, opts *river.InsertOpts) (*rivertype.JobInsertResult, error)
}

func (m *mockJobInserter) Insert(ctx context.Context, args river.JobArgs, opts *river.InsertOpts) (*rivertype.JobInsertResult, error) {
	if m.insertFn != nil {
		return m.insertFn(ctx, args, opts)
	}
	return &rivertype.JobInsertResult{}, nil
}

// Compile-time check: mockJobInserter implements JobInserter.
var _ JobInserter = (*mockJobInserter)(nil)

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
		if !errors.Is(err, ErrFileTooLarge) {
			t.Errorf("expected ErrFileTooLarge, got: %v", err)
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
		if !errors.Is(err, ErrUnsupportedFileType) {
			t.Errorf("expected ErrUnsupportedFileType, got: %v", err)
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
// TestDocumentPipeline_ProcessDocument_FullPaths
// ---------------------------------------------------------------------------

func TestDocumentPipeline_ProcessDocument_NoExtractor(t *testing.T) {
	t.Parallel()

	storagePath := t.TempDir()
	repo := &mockDocumentRepo{
		findByIDFn: func(_ context.Context, _ int64) (*model.Document, error) {
			return &model.Document{
				ID:       1,
				UUID:     "proc-test",
				FilePath: "docs/test.pdf",
				MIMEType: "application/pdf",
				Status:   "uploaded",
			}, nil
		},
		updateFn: func(_ context.Context, doc *model.Document) error {
			return nil
		},
	}

	// Empty registry — no extractor for any MIME type.
	registry := extractor.NewRegistry()
	svc := NewDocumentService(repo, discardLogger())
	pipeline := NewDocumentPipeline(svc, registry, nil, nil, storagePath)

	err := pipeline.ProcessDocument(context.Background(), 1)
	if err == nil {
		t.Fatal("expected error for missing extractor")
	}
	if !strings.Contains(err.Error(), "no extractor for") {
		t.Errorf("error %q should contain 'no extractor for'", err.Error())
	}
}

func TestDocumentPipeline_ProcessDocument_ExtractorError(t *testing.T) {
	t.Parallel()

	storagePath := t.TempDir()
	repo := &mockDocumentRepo{
		findByIDFn: func(_ context.Context, _ int64) (*model.Document, error) {
			return &model.Document{
				ID:       2,
				UUID:     "ext-err",
				FilePath: "docs/test.md",
				MIMEType: "text/markdown",
				Status:   "uploaded",
			}, nil
		},
		updateFn: func(_ context.Context, doc *model.Document) error {
			return nil
		},
	}

	ext := &mockExtractor{
		supportsFn: func(mime string) bool { return mime == "text/markdown" },
		extractFn: func(_ context.Context, _ string) (*extractor.ExtractedContent, error) {
			return nil, errors.New("extraction boom")
		},
	}
	registry := extractor.NewRegistry(ext)

	svc := NewDocumentService(repo, discardLogger())
	pipeline := NewDocumentPipeline(svc, registry, nil, nil, storagePath)

	err := pipeline.ProcessDocument(context.Background(), 2)
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "extraction failed") {
		t.Errorf("error %q should contain 'extraction failed'", err.Error())
	}
}

func TestDocumentPipeline_ProcessDocument_Success(t *testing.T) {
	t.Parallel()

	storagePath := t.TempDir()
	var updatedDoc *model.Document
	repo := &mockDocumentRepo{
		findByIDFn: func(_ context.Context, _ int64) (*model.Document, error) {
			return &model.Document{
				ID:       3,
				UUID:     "proc-ok",
				FilePath: "docs/test.md",
				MIMEType: "text/markdown",
				Status:   "uploaded",
			}, nil
		},
		updateFn: func(_ context.Context, doc *model.Document) error {
			updatedDoc = doc
			return nil
		},
	}

	ext := &mockExtractor{
		supportsFn: func(mime string) bool { return mime == "text/markdown" },
		extractFn: func(_ context.Context, _ string) (*extractor.ExtractedContent, error) {
			return &extractor.ExtractedContent{
				Content:   "extracted content here",
				WordCount: 3,
			}, nil
		},
	}
	registry := extractor.NewRegistry(ext)

	svc := NewDocumentService(repo, discardLogger())
	pipeline := NewDocumentPipeline(svc, registry, nil, nil, storagePath)

	err := pipeline.ProcessDocument(context.Background(), 3)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if updatedDoc == nil {
		t.Fatal("expected document to be updated")
	}
	if updatedDoc.Status != "extracted" {
		t.Errorf("Status = %q, want %q", updatedDoc.Status, "extracted")
	}
	if !updatedDoc.Content.Valid || updatedDoc.Content.String != "extracted content here" {
		t.Errorf("Content = %q, want %q", updatedDoc.Content.String, "extracted content here")
	}
	if updatedDoc.WordCount.Int64 != 3 {
		t.Errorf("WordCount = %d, want 3", updatedDoc.WordCount.Int64)
	}
	if !updatedDoc.ContentHash.Valid || updatedDoc.ContentHash.String == "" {
		t.Error("expected ContentHash to be set")
	}
	if !updatedDoc.ProcessedAt.Valid {
		t.Error("expected ProcessedAt to be set")
	}
}

func TestDocumentPipeline_ProcessDocument_UpdateError(t *testing.T) {
	t.Parallel()

	storagePath := t.TempDir()
	repo := &mockDocumentRepo{
		findByIDFn: func(_ context.Context, _ int64) (*model.Document, error) {
			return &model.Document{
				ID:       4,
				UUID:     "upd-err",
				FilePath: "docs/test.md",
				MIMEType: "text/markdown",
				Status:   "uploaded",
			}, nil
		},
		updateFn: func(_ context.Context, _ *model.Document) error {
			return errors.New("db write failed")
		},
	}

	ext := &mockExtractor{
		supportsFn: func(mime string) bool { return mime == "text/markdown" },
		extractFn: func(_ context.Context, _ string) (*extractor.ExtractedContent, error) {
			return &extractor.ExtractedContent{Content: "ok", WordCount: 1}, nil
		},
	}
	registry := extractor.NewRegistry(ext)

	svc := NewDocumentService(repo, discardLogger())
	pipeline := NewDocumentPipeline(svc, registry, nil, nil, storagePath)

	err := pipeline.ProcessDocument(context.Background(), 4)
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "updating document 4 after extraction") {
		t.Errorf("error %q should contain 'updating document 4 after extraction'", err.Error())
	}
}

func TestDocumentPipeline_ProcessDocument_MarkFailedUpdateError(t *testing.T) {
	t.Parallel()

	storagePath := t.TempDir()
	repo := &mockDocumentRepo{
		findByIDFn: func(_ context.Context, _ int64) (*model.Document, error) {
			return &model.Document{
				ID:       5,
				UUID:     "mark-fail",
				FilePath: "docs/test.md",
				MIMEType: "text/markdown",
				Status:   "uploaded",
			}, nil
		},
		updateFn: func(_ context.Context, _ *model.Document) error {
			return errors.New("cannot update")
		},
	}

	ext := &mockExtractor{
		supportsFn: func(mime string) bool { return mime == "text/markdown" },
		extractFn: func(_ context.Context, _ string) (*extractor.ExtractedContent, error) {
			return nil, errors.New("extraction boom")
		},
	}
	registry := extractor.NewRegistry(ext)

	svc := NewDocumentService(repo, discardLogger())
	pipeline := NewDocumentPipeline(svc, registry, nil, nil, storagePath)

	err := pipeline.ProcessDocument(context.Background(), 5)
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "marking document 5 as failed") {
		t.Errorf("error %q should contain 'marking document 5 as failed'", err.Error())
	}
}

// ---------------------------------------------------------------------------
// TestDocumentPipeline_IndexDocument
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

// NOTE: IndexDocument with non-nil indexer requires integration tests
// (search.Indexer depends on Meilisearch client).

// ---------------------------------------------------------------------------
// TestDocumentPipeline_IndexDocumentByID
// ---------------------------------------------------------------------------

func TestDocumentPipeline_IndexDocumentByID_FindError(t *testing.T) {
	t.Parallel()

	repo := &mockDocumentRepo{
		findByIDFn: func(_ context.Context, _ int64) (*model.Document, error) {
			return nil, errors.New("not found")
		},
	}
	svc := NewDocumentService(repo, discardLogger())
	pipeline := NewDocumentPipeline(svc, nil, nil, nil, t.TempDir())

	err := pipeline.IndexDocumentByID(context.Background(), 999)
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "finding document 999 for indexing") {
		t.Errorf("error = %q, want containing 'finding document 999 for indexing'", err.Error())
	}
}

func TestDocumentPipeline_IndexDocumentByID_NilIndexer(t *testing.T) {
	t.Parallel()

	repo := &mockDocumentRepo{
		findByIDFn: func(_ context.Context, _ int64) (*model.Document, error) {
			return &model.Document{ID: 1, UUID: "idx-by-id"}, nil
		},
	}
	svc := NewDocumentService(repo, discardLogger())
	pipeline := NewDocumentPipeline(svc, nil, nil, nil, t.TempDir())

	err := pipeline.IndexDocumentByID(context.Background(), 1)
	if err != nil {
		t.Fatalf("expected nil (indexer nil), got: %v", err)
	}
}

// ---------------------------------------------------------------------------
// TestDocumentPipeline_DispatchExtraction
// ---------------------------------------------------------------------------

func TestDocumentPipeline_DispatchExtraction_NilInserter(t *testing.T) {
	t.Parallel()

	svc := NewDocumentService(&mockDocumentRepo{}, discardLogger())
	pipeline := NewDocumentPipeline(svc, nil, nil, nil, t.TempDir())

	// Should not panic with nil inserter.
	err := pipeline.dispatchExtraction(context.Background(), 1, "test-uuid")
	require.NoError(t, err)
}

func TestDocumentPipeline_DispatchExtraction_Success(t *testing.T) {
	t.Parallel()

	inserted := false
	inserter := &mockJobInserter{
		insertFn: func(_ context.Context, args river.JobArgs, _ *river.InsertOpts) (*rivertype.JobInsertResult, error) {
			inserted = true
			return &rivertype.JobInsertResult{}, nil
		},
	}

	svc := NewDocumentService(&mockDocumentRepo{}, discardLogger())
	pipeline := NewDocumentPipeline(svc, nil, nil, inserter, t.TempDir())

	err := pipeline.dispatchExtraction(context.Background(), 42, "doc-uuid")
	require.NoError(t, err)

	if !inserted {
		t.Error("expected inserter.Insert to be called")
	}
}

func TestDocumentPipeline_DispatchExtraction_Error(t *testing.T) {
	t.Parallel()

	inserter := &mockJobInserter{
		insertFn: func(_ context.Context, _ river.JobArgs, _ *river.InsertOpts) (*rivertype.JobInsertResult, error) {
			return nil, errors.New("insert failed")
		},
	}

	svc := NewDocumentService(&mockDocumentRepo{}, discardLogger())
	pipeline := NewDocumentPipeline(svc, nil, nil, inserter, t.TempDir())

	// Should return error.
	err := pipeline.dispatchExtraction(context.Background(), 42, "doc-uuid")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "dispatching extraction job")
}

// ---------------------------------------------------------------------------
// TestDocumentPipeline_DispatchIndexing
// ---------------------------------------------------------------------------

func TestDocumentPipeline_DispatchIndexing_NilInserter(t *testing.T) {
	t.Parallel()

	svc := NewDocumentService(&mockDocumentRepo{}, discardLogger())
	pipeline := NewDocumentPipeline(svc, nil, nil, nil, t.TempDir())

	err := pipeline.dispatchIndexing(context.Background(), &model.Document{ID: 1, UUID: "test"})
	require.NoError(t, err)
}

// ---------------------------------------------------------------------------
// TestDocumentPipeline_StoragePath_ExtractorRegistry
// ---------------------------------------------------------------------------

func TestDocumentPipeline_StoragePath(t *testing.T) {
	t.Parallel()

	svc := NewDocumentService(&mockDocumentRepo{}, discardLogger())
	pipeline := NewDocumentPipeline(svc, nil, nil, nil, "/data/uploads")

	if pipeline.StoragePath() != "/data/uploads" {
		t.Errorf("StoragePath() = %q, want %q", pipeline.StoragePath(), "/data/uploads")
	}
}

func TestDocumentPipeline_ExtractorRegistry(t *testing.T) {
	t.Parallel()

	registry := extractor.NewRegistry()
	svc := NewDocumentService(&mockDocumentRepo{}, discardLogger())
	pipeline := NewDocumentPipeline(svc, registry, nil, nil, t.TempDir())

	if pipeline.ExtractorRegistry() != registry {
		t.Error("expected ExtractorRegistry to return the injected registry")
	}
}

func TestDocumentPipeline_ExtractorRegistry_Nil(t *testing.T) {
	t.Parallel()

	svc := NewDocumentService(&mockDocumentRepo{}, discardLogger())
	pipeline := NewDocumentPipeline(svc, nil, nil, nil, t.TempDir())

	if pipeline.ExtractorRegistry() != nil {
		t.Error("expected nil registry")
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
