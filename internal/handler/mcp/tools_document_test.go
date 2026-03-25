package mcphandler

import (
	"database/sql"
	"testing"
	"time"

	"github.com/c-premus/documcp/internal/model"
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
			got := tagNames(tt.tags)

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
			got := formatNullTime(tt.t)
			if got != tt.want {
				t.Errorf("formatNullTime() = %q, want %q", got, tt.want)
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
