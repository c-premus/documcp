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
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
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
	results := search.NormalizeHits(nil, "any")
	if results == nil {
		t.Fatal("NormalizeHits(nil, ...) returned nil, want non-nil empty slice")
	}
	if len(results) != 0 {
		t.Errorf("NormalizeHits(nil, ...) returned %d results, want 0", len(results))
	}
}
