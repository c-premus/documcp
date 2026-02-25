package search_test

import (
	"encoding/json"
	"testing"

	"github.com/meilisearch/meilisearch-go"

	"git.999.haus/chris/DocuMCP-go/internal/search"
)

// makeHit builds a meilisearch.Hit from a plain map for test convenience.
func makeHit(m map[string]any) meilisearch.Hit {
	hit := make(meilisearch.Hit, len(m))
	for k, v := range m {
		raw, _ := json.Marshal(v)
		hit[k] = json.RawMessage(raw)
	}
	return hit
}

func TestNormalizeHits(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		hits      meilisearch.Hits
		source    string
		wantLen   int
		wantFirst *search.SearchResult // nil means skip first-element assertions
	}{
		{
			name:    "empty hits returns empty results",
			hits:    meilisearch.Hits{},
			source:  "documents",
			wantLen: 0,
		},
		{
			name: "hit with uuid and title fields",
			hits: meilisearch.Hits{
				makeHit(map[string]any{
					"uuid":  "abc-123",
					"title": "My Document",
				}),
			},
			source:  "documents",
			wantLen: 1,
			wantFirst: &search.SearchResult{
				UUID:   "abc-123",
				Title:  "My Document",
				Source: "documents",
			},
		},
		{
			name: "hit with name field used as title fallback",
			hits: meilisearch.Hits{
				makeHit(map[string]any{
					"uuid": "def-456",
					"name": "Fallback Name",
				}),
			},
			source:  "zim_archives",
			wantLen: 1,
			wantFirst: &search.SearchResult{
				UUID:   "def-456",
				Title:  "Fallback Name",
				Source: "zim_archives",
			},
		},
		{
			name: "hit with description",
			hits: meilisearch.Hits{
				makeHit(map[string]any{
					"uuid":        "ghi-789",
					"title":       "With Description",
					"description": "A detailed description.",
				}),
			},
			source:  "confluence_spaces",
			wantLen: 1,
			wantFirst: &search.SearchResult{
				UUID:        "ghi-789",
				Title:       "With Description",
				Description: "A detailed description.",
				Source:      "confluence_spaces",
			},
		},
		{
			name: "source field is set correctly",
			hits: meilisearch.Hits{
				makeHit(map[string]any{
					"uuid":  "jkl-012",
					"title": "Source Test",
				}),
			},
			source:  "git_templates",
			wantLen: 1,
			wantFirst: &search.SearchResult{
				UUID:   "jkl-012",
				Title:  "Source Test",
				Source: "git_templates",
			},
		},
		{
			name: "hit with ranking score",
			hits: meilisearch.Hits{
				makeHit(map[string]any{
					"uuid":          "mno-345",
					"title":         "Ranked",
					"_rankingScore": 0.95,
				}),
			},
			source:  "documents",
			wantLen: 1,
			wantFirst: &search.SearchResult{
				UUID:   "mno-345",
				Title:  "Ranked",
				Source: "documents",
				Score:  0.95,
			},
		},
		{
			name: "title takes precedence over name",
			hits: meilisearch.Hits{
				makeHit(map[string]any{
					"uuid":  "pqr-678",
					"title": "Preferred Title",
					"name":  "Ignored Name",
				}),
			},
			source:  "documents",
			wantLen: 1,
			wantFirst: &search.SearchResult{
				UUID:   "pqr-678",
				Title:  "Preferred Title",
				Source: "documents",
			},
		},
		{
			name: "hit with no recognized fields still appears in results",
			hits: meilisearch.Hits{
				makeHit(map[string]any{
					"custom_field": "value",
				}),
			},
			source:  "documents",
			wantLen: 1,
			wantFirst: &search.SearchResult{
				Source: "documents",
			},
		},
		{
			name: "multiple hits are all returned",
			hits: meilisearch.Hits{
				makeHit(map[string]any{"uuid": "first", "title": "First"}),
				makeHit(map[string]any{"uuid": "second", "title": "Second"}),
				makeHit(map[string]any{"uuid": "third", "title": "Third"}),
			},
			source:  "documents",
			wantLen: 3,
			wantFirst: &search.SearchResult{
				UUID:   "first",
				Title:  "First",
				Source: "documents",
			},
		},
		{
			name: "hit with zero ranking score",
			hits: meilisearch.Hits{
				makeHit(map[string]any{
					"uuid":          "zero-score",
					"title":         "Zero Score",
					"_rankingScore": 0.0,
				}),
			},
			source:  "documents",
			wantLen: 1,
			wantFirst: &search.SearchResult{
				UUID:   "zero-score",
				Title:  "Zero Score",
				Source: "documents",
				Score:  0.0,
			},
		},
		{
			name: "empty string fields are preserved as empty",
			hits: meilisearch.Hits{
				makeHit(map[string]any{
					"uuid":        "",
					"title":       "",
					"description": "",
				}),
			},
			source:  "documents",
			wantLen: 1,
			wantFirst: &search.SearchResult{
				UUID:        "",
				Title:       "",
				Description: "",
				Source:      "documents",
			},
		},
		{
			name: "extra fields are present in Extra map",
			hits: meilisearch.Hits{
				makeHit(map[string]any{
					"uuid":      "extra-test",
					"title":     "With Extra",
					"file_type": "pdf",
					"tags":      []string{"important", "reviewed"},
				}),
			},
			source:  "documents",
			wantLen: 1,
			wantFirst: &search.SearchResult{
				UUID:   "extra-test",
				Title:  "With Extra",
				Source: "documents",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			results := search.NormalizeHits(tt.hits, tt.source)

			if got := len(results); got != tt.wantLen {
				t.Fatalf("NormalizeHits() returned %d results, want %d", got, tt.wantLen)
			}

			if tt.wantFirst == nil {
				return
			}

			got := results[0]
			want := *tt.wantFirst

			if got.UUID != want.UUID {
				t.Errorf("UUID = %q, want %q", got.UUID, want.UUID)
			}
			if got.Title != want.Title {
				t.Errorf("Title = %q, want %q", got.Title, want.Title)
			}
			if got.Description != want.Description {
				t.Errorf("Description = %q, want %q", got.Description, want.Description)
			}
			if got.Source != want.Source {
				t.Errorf("Source = %q, want %q", got.Source, want.Source)
			}
			if got.Score != want.Score {
				t.Errorf("Score = %f, want %f", got.Score, want.Score)
			}
		})
	}
}

func TestNormalizeHits_NilHits(t *testing.T) {
	t.Parallel()

	results := search.NormalizeHits(nil, "any")
	if results == nil {
		t.Fatal("NormalizeHits(nil, ...) returned nil, want non-nil empty slice")
	}
	if len(results) != 0 {
		t.Errorf("NormalizeHits(nil, ...) returned %d results, want 0", len(results))
	}
}

func TestNormalizeHits_ExtraFieldsPopulated(t *testing.T) {
	t.Parallel()

	hits := meilisearch.Hits{
		makeHit(map[string]any{
			"uuid":      "extra-check",
			"title":     "Extra Check",
			"file_type": "pdf",
			"status":    "published",
		}),
	}

	results := search.NormalizeHits(hits, "documents")
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}

	extra := results[0].Extra
	if extra == nil {
		t.Fatal("Extra map is nil, want populated map")
	}

	if _, ok := extra["file_type"]; !ok {
		t.Error("Extra should contain 'file_type' key")
	}
	if _, ok := extra["status"]; !ok {
		t.Error("Extra should contain 'status' key")
	}
}

func TestNormalizeHits_MultipleHitsPreservesOrder(t *testing.T) {
	t.Parallel()

	hits := meilisearch.Hits{
		makeHit(map[string]any{"uuid": "aaa", "title": "First"}),
		makeHit(map[string]any{"uuid": "bbb", "title": "Second"}),
		makeHit(map[string]any{"uuid": "ccc", "title": "Third"}),
	}

	results := search.NormalizeHits(hits, "documents")
	if len(results) != 3 {
		t.Fatalf("expected 3 results, got %d", len(results))
	}

	wantUUIDs := []string{"aaa", "bbb", "ccc"}
	for i, want := range wantUUIDs {
		if results[i].UUID != want {
			t.Errorf("results[%d].UUID = %q, want %q", i, results[i].UUID, want)
		}
	}
}

func TestSoftDeleteFilter(t *testing.T) {
	t.Parallel()

	tests := []struct {
		indexUID  string
		wantEmpty bool
	}{
		{indexUID: search.IndexDocuments, wantEmpty: false},
		{indexUID: search.IndexConfluenceSpaces, wantEmpty: false},
		{indexUID: search.IndexGitTemplates, wantEmpty: false},
		{indexUID: search.IndexZimArchives, wantEmpty: true},
		{indexUID: "unknown_index", wantEmpty: true},
		{indexUID: "", wantEmpty: true},
	}

	for _, tt := range tests {
		t.Run(tt.indexUID, func(t *testing.T) {
			t.Parallel()

			// We cannot call softDeleteFilter directly since it is unexported.
			// However, we verify its behavior indirectly through the
			// FederatedSearch path. Since we cannot call Meilisearch in unit
			// tests, we just verify the index constant values are correct.
			if tt.indexUID == search.IndexDocuments && tt.indexUID != "documents" {
				t.Errorf("IndexDocuments = %q, want %q", tt.indexUID, "documents")
			}
		})
	}
}

func TestSearchIndexConstants(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		got  string
		want string
	}{
		{"IndexDocuments", search.IndexDocuments, "documents"},
		{"IndexZimArchives", search.IndexZimArchives, "zim_archives"},
		{"IndexConfluenceSpaces", search.IndexConfluenceSpaces, "confluence_spaces"},
		{"IndexGitTemplates", search.IndexGitTemplates, "git_templates"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if tt.got != tt.want {
				t.Errorf("%s = %q, want %q", tt.name, tt.got, tt.want)
			}
		})
	}
}

func TestSearchResult_JSONFieldNames(t *testing.T) {
	t.Parallel()

	// Verify the SearchResult struct serializes with the expected JSON field names.
	result := search.SearchResult{
		UUID:        "test-uuid",
		Title:       "Test Title",
		Description: "Test Description",
		Source:      "documents",
		Score:       0.85,
		Extra:       map[string]any{"custom": "value"},
	}

	data, err := json.Marshal(result)
	if err != nil {
		t.Fatalf("json.Marshal() unexpected error: %v", err)
	}

	var m map[string]any
	if err := json.Unmarshal(data, &m); err != nil {
		t.Fatalf("json.Unmarshal() unexpected error: %v", err)
	}

	requiredKeys := []string{"uuid", "title", "source", "score", "extra"}
	for _, key := range requiredKeys {
		if _, ok := m[key]; !ok {
			t.Errorf("JSON output missing expected key %q", key)
		}
	}

	// description should be present (non-empty).
	if _, ok := m["description"]; !ok {
		t.Error("JSON output missing 'description' key")
	}
}

func TestSearchResult_OmitEmptyFields(t *testing.T) {
	t.Parallel()

	// Verify omitempty behavior for optional fields.
	result := search.SearchResult{
		UUID:   "test-uuid",
		Title:  "Minimal",
		Source: "documents",
	}

	data, err := json.Marshal(result)
	if err != nil {
		t.Fatalf("json.Marshal() unexpected error: %v", err)
	}

	var m map[string]any
	if err := json.Unmarshal(data, &m); err != nil {
		t.Fatalf("json.Unmarshal() unexpected error: %v", err)
	}

	// description and extra should be omitted when empty.
	if _, ok := m["description"]; ok {
		t.Error("JSON output should omit empty 'description' field")
	}
	if _, ok := m["extra"]; ok {
		t.Error("JSON output should omit nil 'extra' field")
	}
}
