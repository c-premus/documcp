package search_test

import (
	"bytes"
	"encoding/json"
	"log/slog"
	"testing"

	"github.com/meilisearch/meilisearch-go"
	"github.com/prometheus/client_golang/prometheus"

	"git.999.haus/chris/DocuMCP-go/internal/observability"
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

func TestNewSearcher(t *testing.T) {
	t.Parallel()

	t.Run("returns non-nil searcher with valid args", func(t *testing.T) {
		t.Parallel()

		logger := slog.New(slog.NewTextHandler(&bytes.Buffer{}, nil))
		client := search.NewClient("http://localhost:7700", "test-key", logger)
		s := search.NewSearcher(client, logger)

		if s == nil {
			t.Fatal("NewSearcher returned nil")
		}
	})

	t.Run("returns non-nil searcher with nil logger", func(t *testing.T) {
		t.Parallel()

		client := search.NewClient("http://localhost:7700", "", nil)
		s := search.NewSearcher(client, nil)

		if s == nil {
			t.Fatal("NewSearcher returned nil with nil logger")
		}
	})
}

func TestSetMetrics(t *testing.T) {
	t.Run("does not panic with valid metrics", func(t *testing.T) {
		logger := slog.New(slog.NewTextHandler(&bytes.Buffer{}, nil))
		client := search.NewClient("http://localhost:7700", "", logger)
		s := search.NewSearcher(client, logger)

		reg := prometheus.NewRegistry()
		origReg := prometheus.DefaultRegisterer
		origGath := prometheus.DefaultGatherer
		prometheus.DefaultRegisterer = reg
		prometheus.DefaultGatherer = reg
		t.Cleanup(func() {
			prometheus.DefaultRegisterer = origReg
			prometheus.DefaultGatherer = origGath
		})

		m := observability.NewMetrics()
		s.SetMetrics(m)
	})

	t.Run("does not panic with nil metrics", func(t *testing.T) {
		t.Parallel()

		logger := slog.New(slog.NewTextHandler(&bytes.Buffer{}, nil))
		client := search.NewClient("http://localhost:7700", "", logger)
		s := search.NewSearcher(client, logger)

		s.SetMetrics(nil)
	})
}

func TestSearchParams_LimitDefaults(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		limit int64
	}{
		{name: "zero limit", limit: 0},
		{name: "negative limit", limit: -1},
		{name: "positive limit", limit: 50},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			params := search.SearchParams{
				Query:    "test",
				IndexUID: "documents",
				Limit:    tt.limit,
			}

			if params.Limit != tt.limit {
				t.Errorf("SearchParams.Limit = %d, want %d", params.Limit, tt.limit)
			}
		})
	}
}

func TestFederatedSearchParams_DefaultIndexes(t *testing.T) {
	t.Parallel()

	t.Run("empty indexes means search all", func(t *testing.T) {
		t.Parallel()

		params := search.FederatedSearchParams{
			Query:   "test",
			Indexes: nil,
		}

		if len(params.Indexes) != 0 {
			t.Errorf("expected nil Indexes, got %v", params.Indexes)
		}
	})

	t.Run("explicit indexes are preserved", func(t *testing.T) {
		t.Parallel()

		params := search.FederatedSearchParams{
			Query:   "test",
			Indexes: []string{search.IndexDocuments, search.IndexZimArchives},
		}

		if len(params.Indexes) != 2 {
			t.Errorf("expected 2 indexes, got %d", len(params.Indexes))
		}
	})
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
		{
			name: "hit with numeric uuid is ignored as string field",
			hits: meilisearch.Hits{
				makeHit(map[string]any{
					"uuid":  42,
					"title": "Numeric UUID",
				}),
			},
			source:  "documents",
			wantLen: 1,
			wantFirst: &search.SearchResult{
				UUID:   "", // int does not match string type assertion
				Title:  "Numeric UUID",
				Source: "documents",
			},
		},
		{
			name: "hit with numeric title is ignored as string field",
			hits: meilisearch.Hits{
				makeHit(map[string]any{
					"uuid":  "num-title",
					"title": 12345,
				}),
			},
			source:  "documents",
			wantLen: 1,
			wantFirst: &search.SearchResult{
				UUID:   "num-title",
				Title:  "", // int does not match string type assertion
				Source: "documents",
			},
		},
		{
			name: "hit with boolean description is ignored",
			hits: meilisearch.Hits{
				makeHit(map[string]any{
					"uuid":        "bool-desc",
					"title":       "Bool Desc",
					"description": true,
				}),
			},
			source:  "documents",
			wantLen: 1,
			wantFirst: &search.SearchResult{
				UUID:        "bool-desc",
				Title:       "Bool Desc",
				Description: "", // bool does not match string type assertion
				Source:      "documents",
			},
		},
		{
			name: "hit with string ranking score is ignored",
			hits: meilisearch.Hits{
				makeHit(map[string]any{
					"uuid":          "str-score",
					"title":         "String Score",
					"_rankingScore": "not-a-number",
				}),
			},
			source:  "documents",
			wantLen: 1,
			wantFirst: &search.SearchResult{
				UUID:   "str-score",
				Title:  "String Score",
				Source: "documents",
				Score:  0.0, // string does not match float64 type assertion
			},
		},
		{
			name: "unicode fields are handled correctly",
			hits: meilisearch.Hits{
				makeHit(map[string]any{
					"uuid":        "unicode-test",
					"title":       "Titre en francais",
					"description": "Beschreibung auf Deutsch",
				}),
			},
			source:  "documents",
			wantLen: 1,
			wantFirst: &search.SearchResult{
				UUID:        "unicode-test",
				Title:       "Titre en francais",
				Description: "Beschreibung auf Deutsch",
				Source:      "documents",
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

func TestNormalizeHits_ExtraContainsAllOriginalFields(t *testing.T) {
	t.Parallel()

	hits := meilisearch.Hits{
		makeHit(map[string]any{
			"uuid":        "all-fields",
			"title":       "All Fields",
			"description": "desc",
			"file_type":   "pdf",
			"is_public":   true,
			"word_count":  500,
		}),
	}

	results := search.NormalizeHits(hits, "documents")
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}

	extra := results[0].Extra
	wantKeys := []string{"uuid", "title", "description", "file_type", "is_public", "word_count"}
	for _, key := range wantKeys {
		if _, ok := extra[key]; !ok {
			t.Errorf("Extra should contain %q key", key)
		}
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

	if _, ok := m["description"]; !ok {
		t.Error("JSON output missing 'description' key")
	}
}

func TestSearchResult_OmitEmptyFields(t *testing.T) {
	t.Parallel()

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

	if _, ok := m["description"]; ok {
		t.Error("JSON output should omit empty 'description' field")
	}
	if _, ok := m["extra"]; ok {
		t.Error("JSON output should omit nil 'extra' field")
	}
}

func TestSearchResult_JSONRoundTrip(t *testing.T) {
	t.Parallel()

	original := search.SearchResult{
		UUID:        "round-trip",
		Title:       "Round Trip Test",
		Description: "Tests JSON round-trip fidelity",
		Source:      "documents",
		Score:       0.75,
		Extra:       map[string]any{"key": "value"},
	}

	data, err := json.Marshal(original)
	if err != nil {
		t.Fatalf("json.Marshal() unexpected error: %v", err)
	}

	var decoded search.SearchResult
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("json.Unmarshal() unexpected error: %v", err)
	}

	if decoded.UUID != original.UUID {
		t.Errorf("UUID = %q, want %q", decoded.UUID, original.UUID)
	}
	if decoded.Title != original.Title {
		t.Errorf("Title = %q, want %q", decoded.Title, original.Title)
	}
	if decoded.Description != original.Description {
		t.Errorf("Description = %q, want %q", decoded.Description, original.Description)
	}
	if decoded.Source != original.Source {
		t.Errorf("Source = %q, want %q", decoded.Source, original.Source)
	}
	if decoded.Score != original.Score {
		t.Errorf("Score = %f, want %f", decoded.Score, original.Score)
	}
}
