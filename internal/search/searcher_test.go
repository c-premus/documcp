package search_test

import (
	"encoding/json"
	"log/slog"
	"testing"

	"github.com/prometheus/client_golang/prometheus"

	"github.com/c-premus/documcp/internal/observability"
	"github.com/c-premus/documcp/internal/search"
)

func TestNewSearcher(t *testing.T) {
	t.Parallel()

	t.Run("returns non-nil searcher with valid args", func(t *testing.T) {
		t.Parallel()

		logger := slog.Default()
		s := search.NewSearcher(nil, logger)

		if s == nil {
			t.Fatal("NewSearcher returned nil")
		}
	})

	t.Run("returns non-nil searcher with nil logger", func(t *testing.T) {
		t.Parallel()

		s := search.NewSearcher(nil, nil)

		if s == nil {
			t.Fatal("NewSearcher returned nil with nil logger")
		}
	})
}

func TestSetMetrics(t *testing.T) {
	t.Run("does not panic with valid metrics", func(t *testing.T) {
		logger := slog.Default()
		s := search.NewSearcher(nil, logger)

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

		s := search.NewSearcher(nil, slog.Default())
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

func TestSearchIndexConstants(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		got  string
		want string
	}{
		{"IndexDocuments", search.IndexDocuments, "documents"},
		{"IndexZimArchives", search.IndexZimArchives, "zim_archives"},
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

func TestFileSearchResult_JSONFieldNames(t *testing.T) {
	t.Parallel()

	result := search.FileSearchResult{
		TemplateUUID: "tpl-uuid-123",
		TemplateName: "My Template",
		FilePath:     "src/main.go",
		Filename:     "main.go",
		Score:        0.92,
	}

	data, err := json.Marshal(result)
	if err != nil {
		t.Fatalf("json.Marshal() unexpected error: %v", err)
	}

	var m map[string]any
	if err := json.Unmarshal(data, &m); err != nil {
		t.Fatalf("json.Unmarshal() unexpected error: %v", err)
	}

	requiredKeys := []string{"template_uuid", "template_name", "file_path", "filename", "score"}
	for _, key := range requiredKeys {
		if _, ok := m[key]; !ok {
			t.Errorf("JSON output missing expected key %q", key)
		}
	}

	// Verify no extra keys exist beyond the expected ones.
	if len(m) != len(requiredKeys) {
		t.Errorf("JSON output has %d keys, want %d", len(m), len(requiredKeys))
	}
}

func TestFileSearchResult_JSONRoundTrip(t *testing.T) {
	t.Parallel()

	original := search.FileSearchResult{
		TemplateUUID: "tpl-uuid-456",
		TemplateName: "Another Template",
		FilePath:     "pkg/util/helper.go",
		Filename:     "helper.go",
		Score:        0.77,
	}

	data, err := json.Marshal(original)
	if err != nil {
		t.Fatalf("json.Marshal() unexpected error: %v", err)
	}

	var decoded search.FileSearchResult
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("json.Unmarshal() unexpected error: %v", err)
	}

	if decoded.TemplateUUID != original.TemplateUUID {
		t.Errorf("TemplateUUID = %q, want %q", decoded.TemplateUUID, original.TemplateUUID)
	}
	if decoded.TemplateName != original.TemplateName {
		t.Errorf("TemplateName = %q, want %q", decoded.TemplateName, original.TemplateName)
	}
	if decoded.FilePath != original.FilePath {
		t.Errorf("FilePath = %q, want %q", decoded.FilePath, original.FilePath)
	}
	if decoded.Filename != original.Filename {
		t.Errorf("Filename = %q, want %q", decoded.Filename, original.Filename)
	}
	if decoded.Score != original.Score {
		t.Errorf("Score = %f, want %f", decoded.Score, original.Score)
	}
}

func TestFileSearchResult_ZeroValues(t *testing.T) {
	t.Parallel()

	result := search.FileSearchResult{}

	data, err := json.Marshal(result)
	if err != nil {
		t.Fatalf("json.Marshal() unexpected error: %v", err)
	}

	var m map[string]any
	if err := json.Unmarshal(data, &m); err != nil {
		t.Fatalf("json.Unmarshal() unexpected error: %v", err)
	}

	// All fields should still be present (no omitempty on FileSearchResult).
	requiredKeys := []string{"template_uuid", "template_name", "file_path", "filename", "score"}
	for _, key := range requiredKeys {
		if _, ok := m[key]; !ok {
			t.Errorf("JSON output missing expected key %q for zero-value struct", key)
		}
	}
}

func TestSearchResponse_JSONRoundTrip(t *testing.T) {
	t.Parallel()

	original := search.SearchResponse{
		Hits: []search.SearchResult{
			{
				UUID:   "sr-1",
				Title:  "First Result",
				Source: "documents",
				Score:  0.9,
			},
			{
				UUID:        "sr-2",
				Title:       "Second Result",
				Description: "Has a description",
				Source:      "documents",
				Score:       0.7,
				Extra:       map[string]any{"file_type": "pdf"},
			},
		},
		EstimatedTotal:   2,
		ProcessingTimeMs: 42,
	}

	data, err := json.Marshal(original)
	if err != nil {
		t.Fatalf("json.Marshal() unexpected error: %v", err)
	}

	var decoded search.SearchResponse
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("json.Unmarshal() unexpected error: %v", err)
	}

	if decoded.EstimatedTotal != original.EstimatedTotal {
		t.Errorf("EstimatedTotal = %d, want %d", decoded.EstimatedTotal, original.EstimatedTotal)
	}
	if decoded.ProcessingTimeMs != original.ProcessingTimeMs {
		t.Errorf("ProcessingTimeMs = %d, want %d", decoded.ProcessingTimeMs, original.ProcessingTimeMs)
	}
	if len(decoded.Hits) != len(original.Hits) {
		t.Fatalf("Hits length = %d, want %d", len(decoded.Hits), len(original.Hits))
	}
	if decoded.Hits[0].UUID != original.Hits[0].UUID {
		t.Errorf("Hits[0].UUID = %q, want %q", decoded.Hits[0].UUID, original.Hits[0].UUID)
	}
	if decoded.Hits[1].Description != original.Hits[1].Description {
		t.Errorf("Hits[1].Description = %q, want %q", decoded.Hits[1].Description, original.Hits[1].Description)
	}
}

func TestSearchResponse_EmptyHits(t *testing.T) {
	t.Parallel()

	original := search.SearchResponse{
		Hits:             []search.SearchResult{},
		EstimatedTotal:   0,
		ProcessingTimeMs: 1,
	}

	data, err := json.Marshal(original)
	if err != nil {
		t.Fatalf("json.Marshal() unexpected error: %v", err)
	}

	var decoded search.SearchResponse
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("json.Unmarshal() unexpected error: %v", err)
	}

	if decoded.Hits == nil {
		t.Error("Hits should be empty slice, not nil")
	}
	if len(decoded.Hits) != 0 {
		t.Errorf("Hits length = %d, want 0", len(decoded.Hits))
	}
}

func TestFederatedSearchResponse_JSONRoundTrip(t *testing.T) {
	t.Parallel()

	original := search.FederatedSearchResponse{
		Hits: []search.SearchResult{
			{
				UUID:   "fed-1",
				Title:  "Document Hit",
				Source: "documents",
				Score:  0.95,
			},
			{
				UUID:   "fed-2",
				Title:  "Zim Hit",
				Source: "zim_archives",
				Score:  0.80,
			},
			{
				UUID:   "fed-3",
				Title:  "Template Hit",
				Source: "git_templates",
				Score:  0.60,
			},
		},
		EstimatedTotal:   3,
		ProcessingTimeMs: 55,
	}

	data, err := json.Marshal(original)
	if err != nil {
		t.Fatalf("json.Marshal() unexpected error: %v", err)
	}

	var decoded search.FederatedSearchResponse
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("json.Unmarshal() unexpected error: %v", err)
	}

	if decoded.EstimatedTotal != original.EstimatedTotal {
		t.Errorf("EstimatedTotal = %d, want %d", decoded.EstimatedTotal, original.EstimatedTotal)
	}
	if decoded.ProcessingTimeMs != original.ProcessingTimeMs {
		t.Errorf("ProcessingTimeMs = %d, want %d", decoded.ProcessingTimeMs, original.ProcessingTimeMs)
	}
	if len(decoded.Hits) != len(original.Hits) {
		t.Fatalf("Hits length = %d, want %d", len(decoded.Hits), len(original.Hits))
	}

	sources := map[string]bool{}
	for _, h := range decoded.Hits {
		sources[h.Source] = true
	}
	for _, want := range []string{"documents", "zim_archives", "git_templates"} {
		if !sources[want] {
			t.Errorf("expected source %q in federated results", want)
		}
	}
}

func TestFederatedSearchResponse_EmptyHits(t *testing.T) {
	t.Parallel()

	original := search.FederatedSearchResponse{
		Hits:             []search.SearchResult{},
		EstimatedTotal:   0,
		ProcessingTimeMs: 3,
	}

	data, err := json.Marshal(original)
	if err != nil {
		t.Fatalf("json.Marshal() unexpected error: %v", err)
	}

	var decoded search.FederatedSearchResponse
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("json.Unmarshal() unexpected error: %v", err)
	}

	if decoded.Hits == nil {
		t.Error("Hits should be empty slice, not nil")
	}
	if len(decoded.Hits) != 0 {
		t.Errorf("Hits length = %d, want 0", len(decoded.Hits))
	}
}

func TestExtraString(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		extra map[string]any
		key   string
		want  string
	}{
		{"present with correct type", map[string]any{"k": "hello"}, "k", "hello"},
		{"present with wrong type", map[string]any{"k": 42}, "k", ""},
		{"key absent", map[string]any{"other": "val"}, "k", ""},
		{"nil map", nil, "k", ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := search.ExtraString(tt.extra, tt.key); got != tt.want {
				t.Errorf("ExtraString() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestExtraFloat64(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		extra map[string]any
		key   string
		want  float64
	}{
		{"present with correct type", map[string]any{"k": 3.14}, "k", 3.14},
		{"present with wrong type", map[string]any{"k": "nope"}, "k", 0},
		{"key absent", map[string]any{"other": 1.0}, "k", 0},
		{"nil map", nil, "k", 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := search.ExtraFloat64(tt.extra, tt.key); got != tt.want {
				t.Errorf("ExtraFloat64() = %f, want %f", got, tt.want)
			}
		})
	}
}

func TestExtraInt(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		extra map[string]any
		key   string
		want  int
	}{
		{"present with float64", map[string]any{"k": float64(7)}, "k", 7},
		{"present with wrong type", map[string]any{"k": "seven"}, "k", 0},
		{"key absent", map[string]any{}, "k", 0},
		{"nil map", nil, "k", 0},
		{"truncates decimal", map[string]any{"k": 7.9}, "k", 7},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := search.ExtraInt(tt.extra, tt.key); got != tt.want {
				t.Errorf("ExtraInt() = %d, want %d", got, tt.want)
			}
		})
	}
}

func TestExtraStringSlice(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		extra map[string]any
		key   string
		want  []string
	}{
		{
			"present with correct type",
			map[string]any{"k": []any{"a", "b", "c"}},
			"k",
			[]string{"a", "b", "c"},
		},
		{
			"mixed types filters non-strings",
			map[string]any{"k": []any{"a", 42, "b"}},
			"k",
			[]string{"a", "b"},
		},
		{
			"present with wrong type",
			map[string]any{"k": "not-a-slice"},
			"k",
			nil,
		},
		{"key absent", map[string]any{}, "k", nil},
		{"nil map", nil, "k", nil},
		{
			"empty array",
			map[string]any{"k": []any{}},
			"k",
			[]string{},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := search.ExtraStringSlice(tt.extra, tt.key)
			if tt.want == nil {
				if got != nil {
					t.Errorf("ExtraStringSlice() = %v, want nil", got)
				}
				return
			}
			if len(got) != len(tt.want) {
				t.Fatalf("ExtraStringSlice() len = %d, want %d", len(got), len(tt.want))
			}
			for i := range tt.want {
				if got[i] != tt.want[i] {
					t.Errorf("ExtraStringSlice()[%d] = %q, want %q", i, got[i], tt.want[i])
				}
			}
		})
	}
}
