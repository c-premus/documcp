package mcphandler

import (
	"context"
	"database/sql"
	"errors"
	"log/slog"
	"strings"
	"testing"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	authmiddleware "github.com/c-premus/documcp/internal/auth/middleware"
	"github.com/c-premus/documcp/internal/dto"
	"github.com/c-premus/documcp/internal/model"
	"github.com/c-premus/documcp/internal/repository"
	"github.com/c-premus/documcp/internal/service"
)

func TestTruncateContent(t *testing.T) {
	tests := []struct {
		name          string
		content       string
		summaryOnly   bool
		maxParagraphs int
		wantContent   string
		wantTruncated bool
	}{
		{
			name:          "empty content returns empty",
			content:       "",
			summaryOnly:   false,
			maxParagraphs: 0,
			wantContent:   "",
			wantTruncated: false,
		},
		{
			name:          "no options returns full content",
			content:       "Hello world\n\nSecond paragraph",
			summaryOnly:   false,
			maxParagraphs: 0,
			wantContent:   "Hello world\n\nSecond paragraph",
			wantTruncated: false,
		},
		{
			name:          "summaryOnly returns content before first heading",
			content:       "This is the intro.\n\n# First Section\n\nSection content here.",
			summaryOnly:   true,
			maxParagraphs: 0,
			wantContent:   "This is the intro.",
			wantTruncated: true,
		},
		{
			name:          "summaryOnly with ## heading",
			content:       "Intro text.\n\n## Sub Heading\n\nMore content.",
			summaryOnly:   true,
			maxParagraphs: 0,
			wantContent:   "Intro text.",
			wantTruncated: true,
		},
		{
			name:          "summaryOnly with no heading returns full content",
			content:       "Just some text\nwith multiple lines\nbut no headings.",
			summaryOnly:   true,
			maxParagraphs: 0,
			wantContent:   "Just some text\nwith multiple lines\nbut no headings.",
			wantTruncated: false,
		},
		{
			name:          "summaryOnly skips heading on first line",
			content:       "# Title\n\nIntro paragraph.\n\n# Next Section\n\nContent.",
			summaryOnly:   true,
			maxParagraphs: 0,
			wantContent:   "# Title\n\nIntro paragraph.",
			wantTruncated: true,
		},
		{
			name:          "maxParagraphs limits paragraphs",
			content:       "First paragraph.\n\nSecond paragraph.\n\nThird paragraph.",
			summaryOnly:   false,
			maxParagraphs: 2,
			wantContent:   "First paragraph.\n\nSecond paragraph.",
			wantTruncated: true,
		},
		{
			name:          "maxParagraphs equal to actual count returns all",
			content:       "First.\n\nSecond.",
			summaryOnly:   false,
			maxParagraphs: 2,
			wantContent:   "First.\n\nSecond.",
			wantTruncated: false,
		},
		{
			name:          "maxParagraphs greater than actual returns all",
			content:       "Only one paragraph.",
			summaryOnly:   false,
			maxParagraphs: 5,
			wantContent:   "Only one paragraph.",
			wantTruncated: false,
		},
		{
			name:          "maxParagraphs of 1 returns first paragraph",
			content:       "First.\n\nSecond.\n\nThird.",
			summaryOnly:   false,
			maxParagraphs: 1,
			wantContent:   "First.",
			wantTruncated: true,
		},
		{
			name:          "summaryOnly takes precedence over maxParagraphs",
			content:       "Intro.\n\n# Heading\n\nParagraph 1.\n\nParagraph 2.",
			summaryOnly:   true,
			maxParagraphs: 10,
			wantContent:   "Intro.",
			wantTruncated: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, truncated := truncateContent(tt.content, tt.summaryOnly, tt.maxParagraphs)

			if got != tt.wantContent {
				t.Errorf("content = %q, want %q", got, tt.wantContent)
			}
			if truncated != tt.wantTruncated {
				t.Errorf("truncated = %v, want %v", truncated, tt.wantTruncated)
			}
		})
	}
}

func TestTagNames(t *testing.T) {
	tests := []struct {
		name string
		tags []model.DocumentTag
		want []string
	}{
		{
			name: "empty slice returns empty slice",
			tags: []model.DocumentTag{},
			want: []string{},
		},
		{
			name: "single tag",
			tags: []model.DocumentTag{
				{Tag: "golang"},
			},
			want: []string{"golang"},
		},
		{
			name: "multiple tags preserves order",
			tags: []model.DocumentTag{
				{Tag: "golang"},
				{Tag: "testing"},
				{Tag: "mcp"},
			},
			want: []string{"golang", "testing", "mcp"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := dto.TagNames(tt.tags)

			if len(got) != len(tt.want) {
				t.Fatalf("len = %d, want %d", len(got), len(tt.want))
			}
			for i, name := range got {
				if name != tt.want[i] {
					t.Errorf("tag[%d] = %q, want %q", i, name, tt.want[i])
				}
			}
		})
	}
}

func TestFormatNullTime(t *testing.T) {
	fixedTime := time.Date(2025, 6, 15, 10, 30, 0, 0, time.UTC)

	tests := []struct {
		name string
		t    sql.NullTime
		want string
	}{
		{
			name: "invalid NullTime returns empty string",
			t:    sql.NullTime{Valid: false},
			want: "",
		},
		{
			name: "valid NullTime returns RFC3339",
			t:    sql.NullTime{Time: fixedTime, Valid: true},
			want: "2025-06-15T10:30:00Z",
		},
		{
			name: "valid NullTime with non-UTC timezone",
			t: sql.NullTime{
				Time:  time.Date(2025, 1, 1, 12, 0, 0, 0, time.FixedZone("EST", -5*60*60)),
				Valid: true,
			},
			want: "2025-01-01T12:00:00-05:00",
		},
		{
			name: "zero time but valid",
			t:    sql.NullTime{Time: time.Time{}, Valid: true},
			want: "0001-01-01T00:00:00Z",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := dto.FormatNullTime(tt.t)
			if got != tt.want {
				t.Errorf("dto.FormatNullTime() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestBuildDocumentMeta(t *testing.T) {
	fixedTime := time.Date(2025, 3, 20, 14, 0, 0, 0, time.UTC)

	t.Run("maps all fields from document and tags", func(t *testing.T) {
		doc := &model.Document{
			UUID:        "abc-123",
			Title:       "Test Document",
			Description: sql.NullString{String: "A test doc", Valid: true},
			FileType:    "markdown",
			WordCount:   sql.NullInt64{Int64: 42, Valid: true},
			IsPublic:    true,
			ContentHash: sql.NullString{String: "sha256hash", Valid: true},
			CreatedAt:   sql.NullTime{Time: fixedTime, Valid: true},
			UpdatedAt:   sql.NullTime{Time: fixedTime.Add(time.Hour), Valid: true},
			ProcessedAt: sql.NullTime{Time: fixedTime.Add(2 * time.Hour), Valid: true},
		}
		tags := []model.DocumentTag{
			{Tag: "go"},
			{Tag: "testing"},
		}

		got := buildDocumentMeta(doc, tags)

		if got.UUID != "abc-123" {
			t.Errorf("UUID = %q, want %q", got.UUID, "abc-123")
		}
		if got.Title != "Test Document" {
			t.Errorf("Title = %q, want %q", got.Title, "Test Document")
		}
		if got.Description != "A test doc" {
			t.Errorf("Description = %q, want %q", got.Description, "A test doc")
		}
		if got.FileType != "markdown" {
			t.Errorf("FileType = %q, want %q", got.FileType, "markdown")
		}
		if got.WordCount != 42 {
			t.Errorf("WordCount = %d, want %d", got.WordCount, 42)
		}
		if !got.IsPublic {
			t.Error("IsPublic = false, want true")
		}
		if got.ContentHash != "sha256hash" {
			t.Errorf("ContentHash = %q, want %q", got.ContentHash, "sha256hash")
		}
		if got.CreatedAt != "2025-03-20T14:00:00Z" {
			t.Errorf("CreatedAt = %q, want %q", got.CreatedAt, "2025-03-20T14:00:00Z")
		}
		if got.UpdatedAt != "2025-03-20T15:00:00Z" {
			t.Errorf("UpdatedAt = %q, want %q", got.UpdatedAt, "2025-03-20T15:00:00Z")
		}
		if got.ProcessedAt != "2025-03-20T16:00:00Z" {
			t.Errorf("ProcessedAt = %q, want %q", got.ProcessedAt, "2025-03-20T16:00:00Z")
		}
		if len(got.Tags) != 2 || got.Tags[0] != "go" || got.Tags[1] != "testing" {
			t.Errorf("Tags = %v, want [go testing]", got.Tags)
		}
	})

	t.Run("handles null optional fields", func(t *testing.T) {
		doc := &model.Document{
			UUID:        "def-456",
			Title:       "Minimal Doc",
			Description: sql.NullString{Valid: false},
			FileType:    "html",
			WordCount:   sql.NullInt64{Valid: false},
			IsPublic:    false,
			ContentHash: sql.NullString{Valid: false},
			CreatedAt:   sql.NullTime{Valid: false},
			UpdatedAt:   sql.NullTime{Valid: false},
			ProcessedAt: sql.NullTime{Valid: false},
		}

		got := buildDocumentMeta(doc, nil)

		if got.Description != "" {
			t.Errorf("Description = %q, want empty", got.Description)
		}
		if got.WordCount != 0 {
			t.Errorf("WordCount = %d, want 0", got.WordCount)
		}
		if got.ContentHash != "" {
			t.Errorf("ContentHash = %q, want empty", got.ContentHash)
		}
		if got.CreatedAt != "" {
			t.Errorf("CreatedAt = %q, want empty", got.CreatedAt)
		}
		if got.UpdatedAt != "" {
			t.Errorf("UpdatedAt = %q, want empty", got.UpdatedAt)
		}
		if got.ProcessedAt != "" {
			t.Errorf("ProcessedAt = %q, want empty", got.ProcessedAt)
		}
	})

	t.Run("nil tags produces empty slice", func(t *testing.T) {
		doc := &model.Document{
			UUID:     "ghi-789",
			Title:    "No Tags",
			FileType: "markdown",
		}

		got := buildDocumentMeta(doc, nil)

		if len(got.Tags) != 0 {
			t.Errorf("Tags length = %d, want 0", len(got.Tags))
		}
	})
}

// testHandler creates a Handler with the given documentRepo mock for list_documents tests.
func testHandler(repo *mockDocumentRepo) *Handler {
	return &Handler{
		server:       mcp.NewServer(&mcp.Implementation{Name: "test", Version: "v0.0.0"}, nil),
		logger:       slog.Default(),
		documentRepo: repo,
	}
}

// twoDocResult returns a DocumentListResult with two sample documents and a matching tag map.
func twoDocResult() (result *repository.DocumentListResult, tags map[int64][]model.DocumentTag) {
	now := time.Date(2025, 6, 15, 10, 0, 0, 0, time.UTC)
	result = &repository.DocumentListResult{
		Documents: []model.Document{
			{
				ID:          1,
				UUID:        "uuid-1",
				Title:       "First Doc",
				Description: sql.NullString{String: "desc one", Valid: true},
				FileType:    "markdown",
				FileSize:    1024,
				WordCount:   sql.NullInt64{Int64: 200, Valid: true},
				IsPublic:    true,
				Status:      model.DocumentStatusIndexed,
				CreatedAt:   sql.NullTime{Time: now, Valid: true},
				UpdatedAt:   sql.NullTime{Time: now.Add(time.Hour), Valid: true},
			},
			{
				ID:          2,
				UUID:        "uuid-2",
				Title:       "Second Doc",
				Description: sql.NullString{Valid: false},
				FileType:    "pdf",
				FileSize:    2048,
				WordCount:   sql.NullInt64{Valid: false},
				IsPublic:    false,
				Status:      model.DocumentStatusPending,
				CreatedAt:   sql.NullTime{Time: now.Add(2 * time.Hour), Valid: true},
				UpdatedAt:   sql.NullTime{Valid: false},
			},
		},
		Total: 2,
	}
	tags = map[int64][]model.DocumentTag{
		1: {{Tag: "golang"}, {Tag: "testing"}},
		2: {{Tag: "report"}},
	}
	return result, tags
}

func TestHandleListDocuments(t *testing.T) {
	t.Run("admin sees all documents", func(t *testing.T) {
		result, tags := twoDocResult()
		var capturedParams repository.DocumentListParams

		repo := &mockDocumentRepo{
			listFn: func(_ context.Context, params repository.DocumentListParams) (*repository.DocumentListResult, error) {
				capturedParams = params
				return result, nil
			},
			tagsForDocumentsFn: func(_ context.Context, _ []int64) (map[int64][]model.DocumentTag, error) {
				return tags, nil
			},
		}
		h := testHandler(repo)
		ctx := ctxWithAdminUser()

		_, resp, err := h.handleListDocuments(ctx, nil, dto.ListDocumentsInput{})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		// Admin should have no visibility filter.
		if capturedParams.IsPublic != nil {
			t.Error("expected IsPublic to be nil for admin")
		}
		if capturedParams.OwnerOrPublic != nil {
			t.Error("expected OwnerOrPublic to be nil for admin")
		}
		if !resp.Success {
			t.Error("expected Success to be true")
		}
		if resp.Total != 2 {
			t.Errorf("Total = %d, want 2", resp.Total)
		}
		if resp.Count != 2 {
			t.Errorf("Count = %d, want 2", resp.Count)
		}
		if len(resp.Documents) != 2 {
			t.Fatalf("Documents length = %d, want 2", len(resp.Documents))
		}
		doc0 := resp.Documents[0]
		if doc0.UUID != "uuid-1" {
			t.Errorf("doc[0].UUID = %q, want %q", doc0.UUID, "uuid-1")
		}
		if doc0.Title != "First Doc" {
			t.Errorf("doc[0].Title = %q, want %q", doc0.Title, "First Doc")
		}
		if doc0.Description != "desc one" {
			t.Errorf("doc[0].Description = %q, want %q", doc0.Description, "desc one")
		}
		if doc0.FileType != "markdown" {
			t.Errorf("doc[0].FileType = %q, want %q", doc0.FileType, "markdown")
		}
		if doc0.FileSize != 1024 {
			t.Errorf("doc[0].FileSize = %d, want 1024", doc0.FileSize)
		}
		if doc0.WordCount != 200 {
			t.Errorf("doc[0].WordCount = %d, want 200", doc0.WordCount)
		}
		if doc0.CreatedAt != "2025-06-15T10:00:00Z" {
			t.Errorf("doc[0].CreatedAt = %q, want %q", doc0.CreatedAt, "2025-06-15T10:00:00Z")
		}
		if doc0.UpdatedAt != "2025-06-15T11:00:00Z" {
			t.Errorf("doc[0].UpdatedAt = %q, want %q", doc0.UpdatedAt, "2025-06-15T11:00:00Z")
		}
	})

	t.Run("M2M token sees public only", func(t *testing.T) {
		var capturedParams repository.DocumentListParams
		repo := &mockDocumentRepo{
			listFn: func(_ context.Context, params repository.DocumentListParams) (*repository.DocumentListResult, error) {
				capturedParams = params
				return &repository.DocumentListResult{}, nil
			},
		}
		h := testHandler(repo)
		ctx := ctxWithTokenOnly()

		_, _, err := h.handleListDocuments(ctx, nil, dto.ListDocumentsInput{})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if capturedParams.IsPublic == nil || !*capturedParams.IsPublic {
			t.Error("expected IsPublic = true for M2M token")
		}
		if capturedParams.OwnerOrPublic != nil {
			t.Error("expected OwnerOrPublic to be nil for M2M token")
		}
	})

	t.Run("non-admin user gets OwnerOrPublic filter", func(t *testing.T) {
		var capturedParams repository.DocumentListParams
		repo := &mockDocumentRepo{
			listFn: func(_ context.Context, params repository.DocumentListParams) (*repository.DocumentListResult, error) {
				capturedParams = params
				return &repository.DocumentListResult{}, nil
			},
		}
		h := testHandler(repo)

		ctx := context.Background()
		ctx = context.WithValue(ctx, authmiddleware.UserContextKey, &model.User{ID: 42, IsAdmin: false})
		ctx = context.WithValue(ctx, authmiddleware.AccessTokenContextKey, mcpToken())

		_, _, err := h.handleListDocuments(ctx, nil, dto.ListDocumentsInput{})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if capturedParams.OwnerOrPublic == nil {
			t.Fatal("expected OwnerOrPublic to be set")
		}
		if *capturedParams.OwnerOrPublic != 42 {
			t.Errorf("OwnerOrPublic = %d, want 42", *capturedParams.OwnerOrPublic)
		}
		if capturedParams.IsPublic != nil {
			t.Error("expected IsPublic to be nil for non-admin user")
		}
	})

	t.Run("default limit is 50", func(t *testing.T) {
		var capturedParams repository.DocumentListParams
		repo := &mockDocumentRepo{
			listFn: func(_ context.Context, params repository.DocumentListParams) (*repository.DocumentListResult, error) {
				capturedParams = params
				return &repository.DocumentListResult{}, nil
			},
		}
		h := testHandler(repo)

		_, resp, err := h.handleListDocuments(ctxWithAdminUser(), nil, dto.ListDocumentsInput{})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if capturedParams.Limit != 50 {
			t.Errorf("params.Limit = %d, want 50", capturedParams.Limit)
		}
		if resp.Limit != 50 {
			t.Errorf("resp.Limit = %d, want 50", resp.Limit)
		}
	})

	t.Run("limit clamped to 100", func(t *testing.T) {
		var capturedParams repository.DocumentListParams
		repo := &mockDocumentRepo{
			listFn: func(_ context.Context, params repository.DocumentListParams) (*repository.DocumentListResult, error) {
				capturedParams = params
				return &repository.DocumentListResult{}, nil
			},
		}
		h := testHandler(repo)

		_, resp, err := h.handleListDocuments(ctxWithAdminUser(), nil, dto.ListDocumentsInput{Limit: 200})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if capturedParams.Limit != 100 {
			t.Errorf("params.Limit = %d, want 100", capturedParams.Limit)
		}
		if resp.Limit != 100 {
			t.Errorf("resp.Limit = %d, want 100", resp.Limit)
		}
	})

	t.Run("negative offset becomes 0", func(t *testing.T) {
		var capturedParams repository.DocumentListParams
		repo := &mockDocumentRepo{
			listFn: func(_ context.Context, params repository.DocumentListParams) (*repository.DocumentListResult, error) {
				capturedParams = params
				return &repository.DocumentListResult{}, nil
			},
		}
		h := testHandler(repo)

		_, resp, err := h.handleListDocuments(ctxWithAdminUser(), nil, dto.ListDocumentsInput{Offset: -5})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if capturedParams.Offset != 0 {
			t.Errorf("params.Offset = %d, want 0", capturedParams.Offset)
		}
		if resp.Offset != 0 {
			t.Errorf("resp.Offset = %d, want 0", resp.Offset)
		}
	})

	t.Run("invalid file_type ignored", func(t *testing.T) {
		var capturedParams repository.DocumentListParams
		repo := &mockDocumentRepo{
			listFn: func(_ context.Context, params repository.DocumentListParams) (*repository.DocumentListResult, error) {
				capturedParams = params
				return &repository.DocumentListResult{}, nil
			},
		}
		h := testHandler(repo)

		_, _, err := h.handleListDocuments(ctxWithAdminUser(), nil, dto.ListDocumentsInput{FileType: "invalid"})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if capturedParams.FileType != "" {
			t.Errorf("params.FileType = %q, want empty", capturedParams.FileType)
		}
	})

	t.Run("valid file_type passed through", func(t *testing.T) {
		var capturedParams repository.DocumentListParams
		repo := &mockDocumentRepo{
			listFn: func(_ context.Context, params repository.DocumentListParams) (*repository.DocumentListResult, error) {
				capturedParams = params
				return &repository.DocumentListResult{}, nil
			},
		}
		h := testHandler(repo)

		_, _, err := h.handleListDocuments(ctxWithAdminUser(), nil, dto.ListDocumentsInput{FileType: "pdf"})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if capturedParams.FileType != "pdf" {
			t.Errorf("params.FileType = %q, want %q", capturedParams.FileType, "pdf")
		}
	})

	t.Run("repo List error propagated", func(t *testing.T) {
		repo := &mockDocumentRepo{
			listFn: func(_ context.Context, _ repository.DocumentListParams) (*repository.DocumentListResult, error) {
				return nil, errors.New("db connection lost")
			},
		}
		h := testHandler(repo)

		_, _, err := h.handleListDocuments(ctxWithAdminUser(), nil, dto.ListDocumentsInput{})
		if err == nil {
			t.Fatal("expected error, got nil")
		}
		if !errors.Is(err, errors.Unwrap(err)) && err.Error() != "listing documents: db connection lost" {
			t.Errorf("error = %q, want wrapped db error", err.Error())
		}
	})

	t.Run("empty result set", func(t *testing.T) {
		repo := &mockDocumentRepo{
			listFn: func(_ context.Context, _ repository.DocumentListParams) (*repository.DocumentListResult, error) {
				return &repository.DocumentListResult{Documents: []model.Document{}, Total: 0}, nil
			},
		}
		h := testHandler(repo)

		_, resp, err := h.handleListDocuments(ctxWithAdminUser(), nil, dto.ListDocumentsInput{})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if !resp.Success {
			t.Error("expected Success to be true")
		}
		if resp.Total != 0 {
			t.Errorf("Total = %d, want 0", resp.Total)
		}
		if resp.Count != 0 {
			t.Errorf("Count = %d, want 0", resp.Count)
		}
		if len(resp.Documents) != 0 {
			t.Errorf("Documents length = %d, want 0", len(resp.Documents))
		}
	})

	t.Run("no scope returns error", func(t *testing.T) {
		repo := &mockDocumentRepo{}
		h := testHandler(repo)

		// Context with no bearer token at all.
		ctx := context.Background()

		_, _, err := h.handleListDocuments(ctx, nil, dto.ListDocumentsInput{})
		if err == nil {
			t.Fatal("expected error, got nil")
		}
		if err.Error() != "mcp:read scope required for listing documents" {
			t.Errorf("error = %q, want scope error message", err.Error())
		}
	})

	t.Run("tags loaded and mapped correctly", func(t *testing.T) {
		result, tags := twoDocResult()

		var capturedDocIDs []int64
		repo := &mockDocumentRepo{
			listFn: func(_ context.Context, _ repository.DocumentListParams) (*repository.DocumentListResult, error) {
				return result, nil
			},
			tagsForDocumentsFn: func(_ context.Context, docIDs []int64) (map[int64][]model.DocumentTag, error) {
				capturedDocIDs = docIDs
				return tags, nil
			},
		}
		h := testHandler(repo)

		_, resp, err := h.handleListDocuments(ctxWithAdminUser(), nil, dto.ListDocumentsInput{})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		// Verify TagsForDocuments was called with correct IDs.
		if len(capturedDocIDs) != 2 {
			t.Fatalf("TagsForDocuments called with %d IDs, want 2", len(capturedDocIDs))
		}
		if capturedDocIDs[0] != 1 || capturedDocIDs[1] != 2 {
			t.Errorf("TagsForDocuments IDs = %v, want [1 2]", capturedDocIDs)
		}

		// First doc should have two tags.
		doc0 := resp.Documents[0]
		if len(doc0.Tags) != 2 {
			t.Fatalf("doc[0].Tags length = %d, want 2", len(doc0.Tags))
		}
		if doc0.Tags[0] != "golang" || doc0.Tags[1] != "testing" {
			t.Errorf("doc[0].Tags = %v, want [golang testing]", doc0.Tags)
		}

		// Second doc should have one tag.
		doc1 := resp.Documents[1]
		if len(doc1.Tags) != 1 {
			t.Fatalf("doc[1].Tags length = %d, want 1", len(doc1.Tags))
		}
		if doc1.Tags[0] != "report" {
			t.Errorf("doc[1].Tags = %v, want [report]", doc1.Tags)
		}
	})
}

// TestHandleReplaceDocumentContent covers the success path and every error
// branch of handleReplaceDocumentContent: input validation, ownership
// rejection, the file-backed-document schema-shape rejection, and service
// error mapping. Scope rejection lives in tools_document_scope_test.go.
func TestHandleReplaceDocumentContent(t *testing.T) {
	t.Parallel()

	const ownerID int64 = 7
	const docUUID = "aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa"

	// writeAccessToken carries mcp:write so the per-handler scope guard passes.
	writeAccessToken := &model.OAuthAccessToken{
		Scope: sql.NullString{String: "mcp:access mcp:read mcp:write", Valid: true},
	}
	ownerCtx := func() context.Context {
		c := context.WithValue(context.Background(), authmiddleware.AccessTokenContextKey, writeAccessToken)
		return context.WithValue(c, authmiddleware.UserContextKey, &model.User{ID: ownerID, IsAdmin: false})
	}
	adminCtx := func() context.Context {
		c := context.WithValue(context.Background(), authmiddleware.AccessTokenContextKey, writeAccessToken)
		return context.WithValue(c, authmiddleware.UserContextKey, &model.User{ID: 999, IsAdmin: true})
	}
	m2mCtx := func() context.Context {
		// M2M = token without a user context.
		return context.WithValue(context.Background(), authmiddleware.AccessTokenContextKey, writeAccessToken)
	}

	inlineDoc := func() *model.Document {
		return &model.Document{
			ID:          1,
			UUID:        docUUID,
			Title:       "Replaceable",
			FileType:    "markdown",
			FilePath:    "",
			IsPublic:    true,
			UserID:      sql.NullInt64{Int64: ownerID, Valid: true},
			Description: sql.NullString{String: "desc", Valid: true},
			Status:      model.DocumentStatusIndexed,
		}
	}

	t.Run("success returns documentMeta with refreshed body", func(t *testing.T) {
		t.Parallel()

		var calledWith service.ReplaceInlineContentParams
		svc := &mockDocumentService{
			findByUUIDFn: func(_ context.Context, _ string) (*model.Document, error) {
				return inlineDoc(), nil
			},
			replaceInlineFn: func(_ context.Context, _ string, params service.ReplaceInlineContentParams) (*model.Document, error) {
				calledWith = params
				doc := inlineDoc()
				doc.Content = sql.NullString{String: params.Content, Valid: true}
				doc.WordCount = sql.NullInt64{Int64: 3, Valid: true}
				doc.ContentHash = sql.NullString{String: "newhash", Valid: true}
				doc.ProcessedAt = sql.NullTime{Time: time.Now(), Valid: true}
				return doc, nil
			},
			tagsForDocFn: func(_ context.Context, _ int64) ([]model.DocumentTag, error) {
				return []model.DocumentTag{{Tag: "go"}}, nil
			},
		}
		h := &Handler{documentService: svc, documentRepo: &mockDocumentRepo{}}

		_, resp, err := h.handleReplaceDocumentContent(ownerCtx(), nil, dto.ReplaceDocumentContentInput{
			UUID:    docUUID,
			Content: "new body here",
		})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !resp.Success {
			t.Error("response Success = false")
		}
		if resp.Document == nil {
			t.Fatal("response Document is nil")
		}
		if resp.Document.ContentHash != "newhash" {
			t.Errorf("Document.ContentHash = %q, want %q", resp.Document.ContentHash, "newhash")
		}
		if len(resp.Document.Tags) != 1 || resp.Document.Tags[0] != "go" {
			t.Errorf("Document.Tags = %v, want [go]", resp.Document.Tags)
		}
		if calledWith.Content != "new body here" {
			t.Errorf("service called with content %q, want %q", calledWith.Content, "new body here")
		}
	})

	t.Run("admin bypass: non-owner admin succeeds", func(t *testing.T) {
		t.Parallel()

		svc := &mockDocumentService{
			findByUUIDFn: func(_ context.Context, _ string) (*model.Document, error) { return inlineDoc(), nil },
			replaceInlineFn: func(_ context.Context, _ string, _ service.ReplaceInlineContentParams) (*model.Document, error) {
				return inlineDoc(), nil
			},
			tagsForDocFn: func(_ context.Context, _ int64) ([]model.DocumentTag, error) { return nil, nil },
		}
		h := &Handler{documentService: svc, documentRepo: &mockDocumentRepo{}}

		_, _, err := h.handleReplaceDocumentContent(adminCtx(), nil, dto.ReplaceDocumentContentInput{
			UUID:    docUUID,
			Content: "ok",
		})
		if err != nil {
			t.Fatalf("admin should pass ownership check: %v", err)
		}
	})

	t.Run("empty content rejected", func(t *testing.T) {
		t.Parallel()

		h := &Handler{documentService: &mockDocumentService{}, documentRepo: &mockDocumentRepo{}}
		_, _, err := h.handleReplaceDocumentContent(ownerCtx(), nil, dto.ReplaceDocumentContentInput{
			UUID:    docUUID,
			Content: "",
		})
		if err == nil || !strings.Contains(err.Error(), "content is required") {
			t.Errorf("expected content-required error, got %v", err)
		}
	})

	t.Run("oversize content rejected", func(t *testing.T) {
		t.Parallel()

		h := &Handler{documentService: &mockDocumentService{}, documentRepo: &mockDocumentRepo{}}
		// One byte over the 10 MB cap.
		body := strings.Repeat("x", maxInlineDocumentContentBytes+1)
		_, _, err := h.handleReplaceDocumentContent(ownerCtx(), nil, dto.ReplaceDocumentContentInput{
			UUID:    docUUID,
			Content: body,
		})
		if err == nil || !strings.Contains(err.Error(), "10 MB") {
			t.Errorf("expected 10 MB cap error, got %v", err)
		}
	})

	t.Run("M2M token rejected as not found", func(t *testing.T) {
		t.Parallel()

		svc := &mockDocumentService{
			findByUUIDFn: func(_ context.Context, _ string) (*model.Document, error) { return inlineDoc(), nil },
			replaceInlineFn: func(_ context.Context, _ string, _ service.ReplaceInlineContentParams) (*model.Document, error) {
				t.Fatal("replaceInline must not be called when ownership rejects")
				return nil, nil
			},
		}
		h := &Handler{documentService: svc, documentRepo: &mockDocumentRepo{}}

		_, _, err := h.handleReplaceDocumentContent(m2mCtx(), nil, dto.ReplaceDocumentContentInput{
			UUID:    docUUID,
			Content: "ok",
		})
		if err == nil || err.Error() != "document not found" {
			t.Errorf("expected exact \"document not found\", got %v", err)
		}
	})

	t.Run("non-owner rejected as not found", func(t *testing.T) {
		t.Parallel()

		svc := &mockDocumentService{
			findByUUIDFn: func(_ context.Context, _ string) (*model.Document, error) { return inlineDoc(), nil },
			replaceInlineFn: func(_ context.Context, _ string, _ service.ReplaceInlineContentParams) (*model.Document, error) {
				t.Fatal("replaceInline must not be called when ownership rejects")
				return nil, nil
			},
		}
		h := &Handler{documentService: svc, documentRepo: &mockDocumentRepo{}}

		// Non-admin, non-owner user.
		ctx := context.WithValue(context.Background(), authmiddleware.AccessTokenContextKey, writeAccessToken)
		ctx = context.WithValue(ctx, authmiddleware.UserContextKey, &model.User{ID: ownerID + 1, IsAdmin: false})

		_, _, err := h.handleReplaceDocumentContent(ctx, nil, dto.ReplaceDocumentContentInput{
			UUID:    docUUID,
			Content: "ok",
		})
		if err == nil || err.Error() != "document not found" {
			t.Errorf("expected exact \"document not found\", got %v", err)
		}
	})

	t.Run("file-backed document rejected with schema-shape error", func(t *testing.T) {
		t.Parallel()

		svc := &mockDocumentService{
			findByUUIDFn: func(_ context.Context, _ string) (*model.Document, error) { return inlineDoc(), nil },
			replaceInlineFn: func(_ context.Context, _ string, _ service.ReplaceInlineContentParams) (*model.Document, error) {
				return nil, service.ErrFileBackedDocument
			},
		}
		h := &Handler{documentService: svc, documentRepo: &mockDocumentRepo{}}

		_, _, err := h.handleReplaceDocumentContent(ownerCtx(), nil, dto.ReplaceDocumentContentInput{
			UUID:    docUUID,
			Content: "ok",
		})
		if err == nil {
			t.Fatal("expected error, got nil")
		}
		// Message must steer the caller toward the REST endpoint.
		if !strings.Contains(err.Error(), "markdown or html") {
			t.Errorf("error %q does not mention the markdown/html scope", err.Error())
		}
		if !strings.Contains(err.Error(), "/api/documents/{uuid}/content") {
			t.Errorf("error %q does not point at the REST endpoint", err.Error())
		}
	})

	t.Run("service ErrNotFound maps to not found", func(t *testing.T) {
		t.Parallel()

		svc := &mockDocumentService{
			findByUUIDFn: func(_ context.Context, _ string) (*model.Document, error) { return inlineDoc(), nil },
			replaceInlineFn: func(_ context.Context, _ string, _ service.ReplaceInlineContentParams) (*model.Document, error) {
				return nil, service.ErrNotFound
			},
		}
		h := &Handler{documentService: svc, documentRepo: &mockDocumentRepo{}}

		_, _, err := h.handleReplaceDocumentContent(ownerCtx(), nil, dto.ReplaceDocumentContentInput{
			UUID:    docUUID,
			Content: "ok",
		})
		if err == nil || err.Error() != "document not found" {
			t.Errorf("expected exact \"document not found\", got %v", err)
		}
	})

	t.Run("other service error is wrapped", func(t *testing.T) {
		t.Parallel()

		svc := &mockDocumentService{
			findByUUIDFn: func(_ context.Context, _ string) (*model.Document, error) { return inlineDoc(), nil },
			replaceInlineFn: func(_ context.Context, _ string, _ service.ReplaceInlineContentParams) (*model.Document, error) {
				return nil, errors.New("db gone")
			},
		}
		h := &Handler{documentService: svc, documentRepo: &mockDocumentRepo{}}

		_, _, err := h.handleReplaceDocumentContent(ownerCtx(), nil, dto.ReplaceDocumentContentInput{
			UUID:    docUUID,
			Content: "ok",
		})
		if err == nil {
			t.Fatal("expected error, got nil")
		}
		if !strings.Contains(err.Error(), "replacing document content") {
			t.Errorf("error %q does not contain expected wrapper", err.Error())
		}
	})
}
