package search_test

import (
	"bytes"
	"encoding/json"
	"log/slog"
	"testing"

	"github.com/c-premus/documcp/internal/search"
)

func TestNewIndexer(t *testing.T) {
	t.Parallel()

	t.Run("returns non-nil indexer", func(t *testing.T) {
		t.Parallel()

		logger := slog.New(slog.NewTextHandler(&bytes.Buffer{}, nil))
		client := search.NewClient("http://localhost:7700", "test-key", logger)
		ix := search.NewIndexer(client, logger)

		if ix == nil {
			t.Fatal("NewIndexer returned nil")
		}
	})

	t.Run("returns non-nil indexer with nil logger", func(t *testing.T) {
		t.Parallel()

		client := search.NewClient("http://localhost:7700", "", nil)
		ix := search.NewIndexer(client, nil)

		if ix == nil {
			t.Fatal("NewIndexer returned nil with nil logger")
		}
	})
}

func TestIndexer_Searcher(t *testing.T) {
	t.Parallel()

	logger := slog.New(slog.NewTextHandler(&bytes.Buffer{}, nil))
	client := search.NewClient("http://localhost:7700", "key", logger)
	ix := search.NewIndexer(client, logger)

	s := ix.Searcher()
	if s == nil {
		t.Fatal("Indexer.Searcher() returned nil")
	}
}

func TestDocumentRecord_JSONSerialization(t *testing.T) {
	t.Parallel()

	userID := int64(42)
	doc := search.DocumentRecord{
		UUID:        "doc-uuid-123",
		Title:       "Test Document",
		Description: "A test document description",
		Content:     "Some content here",
		FileType:    "pdf",
		Tags:        []string{"test", "sample"},
		Status:      "published",
		UserID:      &userID,
		IsPublic:    true,
		WordCount:   150,
		CreatedAt:   "2026-01-01T00:00:00Z",
		UpdatedAt:   "2026-03-01T00:00:00Z",
		SoftDeleted: false,
	}

	data, err := json.Marshal(doc)
	if err != nil {
		t.Fatalf("json.Marshal() unexpected error: %v", err)
	}

	var m map[string]any
	if err := json.Unmarshal(data, &m); err != nil {
		t.Fatalf("json.Unmarshal() unexpected error: %v", err)
	}

	wantKeys := []string{
		"uuid", "title", "description", "content", "file_type",
		"tags", "status", "user_id", "is_public", "word_count",
		"created_at", "updated_at", "__soft_deleted",
	}
	for _, key := range wantKeys {
		if _, ok := m[key]; !ok {
			t.Errorf("JSON output missing expected key %q", key)
		}
	}

	if m["__soft_deleted"] != false {
		t.Errorf("__soft_deleted = %v, want false", m["__soft_deleted"])
	}
}

func TestDocumentRecord_OmitEmptyFields(t *testing.T) {
	t.Parallel()

	doc := search.DocumentRecord{
		UUID:     "minimal-doc",
		Title:    "Minimal",
		FileType: "txt",
		Status:   "draft",
	}

	data, err := json.Marshal(doc)
	if err != nil {
		t.Fatalf("json.Marshal() unexpected error: %v", err)
	}

	var m map[string]any
	if err := json.Unmarshal(data, &m); err != nil {
		t.Fatalf("json.Unmarshal() unexpected error: %v", err)
	}

	omitKeys := []string{"description", "content", "tags", "user_id", "created_at", "updated_at"}
	for _, key := range omitKeys {
		if _, ok := m[key]; ok {
			t.Errorf("JSON output should omit empty %q field", key)
		}
	}
}

func TestDocumentRecord_JSONRoundTrip(t *testing.T) {
	t.Parallel()

	userID := int64(7)
	original := search.DocumentRecord{
		UUID:        "rt-uuid",
		Title:       "Round Trip",
		FileType:    "md",
		Tags:        []string{"a", "b"},
		Status:      "published",
		UserID:      &userID,
		IsPublic:    true,
		WordCount:   42,
		SoftDeleted: true,
	}

	data, err := json.Marshal(original)
	if err != nil {
		t.Fatalf("json.Marshal() unexpected error: %v", err)
	}

	var decoded search.DocumentRecord
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("json.Unmarshal() unexpected error: %v", err)
	}

	if decoded.UUID != original.UUID {
		t.Errorf("UUID = %q, want %q", decoded.UUID, original.UUID)
	}
	if decoded.SoftDeleted != original.SoftDeleted {
		t.Errorf("SoftDeleted = %v, want %v", decoded.SoftDeleted, original.SoftDeleted)
	}
	if decoded.UserID == nil || *decoded.UserID != *original.UserID {
		t.Errorf("UserID = %v, want %v", decoded.UserID, original.UserID)
	}
}

func TestZimArchiveRecord_JSONSerialization(t *testing.T) {
	t.Parallel()

	rec := search.ZimArchiveRecord{
		UUID:         "zim-uuid",
		Name:         "wikipedia_en",
		Title:        "Wikipedia English",
		Description:  "Full English Wikipedia",
		Language:     "en",
		Category:     "wikipedia",
		Creator:      "Kiwix",
		Tags:         []string{"encyclopedia", "reference"},
		ArticleCount: 6500000,
	}

	data, err := json.Marshal(rec)
	if err != nil {
		t.Fatalf("json.Marshal() unexpected error: %v", err)
	}

	var m map[string]any
	if err := json.Unmarshal(data, &m); err != nil {
		t.Fatalf("json.Unmarshal() unexpected error: %v", err)
	}

	wantKeys := []string{"uuid", "name", "title", "description", "language", "category", "creator", "tags", "article_count"}
	for _, key := range wantKeys {
		if _, ok := m[key]; !ok {
			t.Errorf("JSON output missing expected key %q", key)
		}
	}
}

func TestGitTemplateRecord_JSONSerialization(t *testing.T) {
	t.Parallel()

	userID := int64(99)
	rec := search.GitTemplateRecord{
		UUID:          "git-uuid",
		Name:          "go-starter",
		Slug:          "go-starter",
		Description:   "Go starter template",
		ReadmeContent: "# Go Starter",
		Category:      "backend",
		Tags:          []string{"go", "starter"},
		UserID:        &userID,
		IsPublic:      true,
		Status:        "active",
		SoftDeleted:   false,
	}

	data, err := json.Marshal(rec)
	if err != nil {
		t.Fatalf("json.Marshal() unexpected error: %v", err)
	}

	var m map[string]any
	if err := json.Unmarshal(data, &m); err != nil {
		t.Fatalf("json.Unmarshal() unexpected error: %v", err)
	}

	wantKeys := []string{
		"uuid", "name", "slug", "description", "readme_content",
		"category", "tags", "user_id", "is_public", "status", "__soft_deleted",
	}
	for _, key := range wantKeys {
		if _, ok := m[key]; !ok {
			t.Errorf("JSON output missing expected key %q", key)
		}
	}
}
