package mcphandler

import (
	"context"
	"database/sql"
	"errors"
	"slices"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/c-premus/documcp/internal/client/kiwix"
	"github.com/c-premus/documcp/internal/dto"
	"github.com/c-premus/documcp/internal/model"
	"github.com/c-premus/documcp/internal/search"
)

// ===== searchKiwixArchives unit tests =====

func TestSearchKiwixArchives(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	t.Run("returns nil when factory is nil", func(t *testing.T) {
		t.Parallel()
		h := newHandlerWithMocks(struct {
			docSvc   *mockDocumentService
			docRepo  *mockDocumentRepo
			zimRepo  *mockZimArchiveRepo
			gitRepo  *mockGitTemplateRepo
			kiwixC   *mockKiwixClient
			searcher *mockSearcher
		}{
			// No kiwixC — factory stays nil.
			zimRepo: &mockZimArchiveRepo{
				listSearchableFn: func(_ context.Context) ([]model.ZimArchive, error) {
					t.Error("ListSearchable should not be called when factory is nil")
					return nil, nil
				},
			},
		})
		// Explicitly clear the factory set by newHandlerWithMocks (it only sets
		// when kiwixC != nil, so this is already nil, but be explicit).
		h.kiwixFactory = nil
		h.federatedSearchTimeout = 3 * time.Second
		h.federatedMaxArchives = 10
		h.federatedPerArchiveLimit = 3

		results, searched := h.searchKiwixArchives(ctx, "golang", 5)
		if results != nil {
			t.Errorf("expected nil results, got %v", results)
		}
		if searched != nil {
			t.Errorf("expected nil searched, got %v", searched)
		}
	})

	t.Run("returns nil when no searchable archives", func(t *testing.T) {
		t.Parallel()
		mc := &mockKiwixClient{
			fetchCatalogFn: func(_ context.Context) ([]kiwix.CatalogEntry, error) {
				t.Error("FetchCatalog should not be called when no archives exist")
				return nil, nil
			},
		}
		h := newHandlerWithMocks(struct {
			docSvc   *mockDocumentService
			docRepo  *mockDocumentRepo
			zimRepo  *mockZimArchiveRepo
			gitRepo  *mockGitTemplateRepo
			kiwixC   *mockKiwixClient
			searcher *mockSearcher
		}{
			kiwixC: mc,
			zimRepo: &mockZimArchiveRepo{
				listSearchableFn: func(_ context.Context) ([]model.ZimArchive, error) {
					return []model.ZimArchive{}, nil
				},
			},
		})
		h.federatedSearchTimeout = 3 * time.Second
		h.federatedMaxArchives = 10
		h.federatedPerArchiveLimit = 3

		results, searched := h.searchKiwixArchives(ctx, "golang", 5)
		if results != nil {
			t.Errorf("expected nil results, got %v", results)
		}
		if searched != nil {
			t.Errorf("expected nil searched, got %v", searched)
		}
	})

	t.Run("returns results from multiple archives", func(t *testing.T) {
		t.Parallel()
		mc := &mockKiwixClient{
			fetchCatalogFn: func(_ context.Context) ([]kiwix.CatalogEntry, error) {
				return []kiwix.CatalogEntry{
					{Name: "wiki_en", HasFulltextIndex: true},
					{Name: "devdocs_go", HasFulltextIndex: false},
				}, nil
			},
			searchFn: func(_ context.Context, archiveName, query, searchType string, limit int) ([]kiwix.SearchResult, error) {
				switch archiveName {
				case "wiki_en":
					return []kiwix.SearchResult{
						{Title: "Go (programming language)", Path: "/A/Go", Snippet: "Go is a language"},
					}, nil
				case "devdocs_go":
					return []kiwix.SearchResult{
						{Title: "Go Tutorial", Path: "/A/GoTutorial", Snippet: "Learn Go"},
					}, nil
				default:
					return nil, nil
				}
			},
		}
		h := newHandlerWithMocks(struct {
			docSvc   *mockDocumentService
			docRepo  *mockDocumentRepo
			zimRepo  *mockZimArchiveRepo
			gitRepo  *mockGitTemplateRepo
			kiwixC   *mockKiwixClient
			searcher *mockSearcher
		}{
			kiwixC: mc,
			zimRepo: &mockZimArchiveRepo{
				listSearchableFn: func(_ context.Context) ([]model.ZimArchive, error) {
					return []model.ZimArchive{
						{Name: "wiki_en", ArticleCount: 1000},
						{Name: "devdocs_go", ArticleCount: 500},
					}, nil
				},
			},
		})
		h.federatedSearchTimeout = 3 * time.Second
		h.federatedMaxArchives = 10
		h.federatedPerArchiveLimit = 3

		results, searched := h.searchKiwixArchives(ctx, "golang", 5)

		if len(results) != 2 {
			t.Fatalf("expected 2 results, got %d", len(results))
		}
		if len(searched) != 2 {
			t.Fatalf("expected 2 searched archives, got %d", len(searched))
		}

		// Verify all results have correct source and populated archive/path fields.
		for i, r := range results {
			if r.Source != "zim_article" {
				t.Errorf("result[%d].Source = %q, want %q", i, r.Source, "zim_article")
			}
			if r.Archive == "" {
				t.Errorf("result[%d].Archive is empty", i)
			}
			if r.Path == "" {
				t.Errorf("result[%d].Path is empty", i)
			}
		}
	})

	t.Run("uses fulltext for ftindex archives and suggest for others", func(t *testing.T) {
		t.Parallel()

		capturedSearchTypes := make(map[string]string)
		var mu sync.Mutex // not needed with atomic but let's track per-archive

		mc := &mockKiwixClient{
			fetchCatalogFn: func(_ context.Context) ([]kiwix.CatalogEntry, error) {
				return []kiwix.CatalogEntry{
					{Name: "wiki_en", HasFulltextIndex: true},
					{Name: "devdocs_go", HasFulltextIndex: false},
				}, nil
			},
			searchFn: func(_ context.Context, archiveName, _ string, searchType string, _ int) ([]kiwix.SearchResult, error) {
				mu.Lock()
				capturedSearchTypes[archiveName] = searchType
				mu.Unlock()
				return []kiwix.SearchResult{
					{Title: "Result", Path: "/A/Result"},
				}, nil
			},
		}
		h := newHandlerWithMocks(struct {
			docSvc   *mockDocumentService
			docRepo  *mockDocumentRepo
			zimRepo  *mockZimArchiveRepo
			gitRepo  *mockGitTemplateRepo
			kiwixC   *mockKiwixClient
			searcher *mockSearcher
		}{
			kiwixC: mc,
			zimRepo: &mockZimArchiveRepo{
				listSearchableFn: func(_ context.Context) ([]model.ZimArchive, error) {
					return []model.ZimArchive{
						{Name: "wiki_en", ArticleCount: 1000},
						{Name: "devdocs_go", ArticleCount: 500},
					}, nil
				},
			},
		})
		h.federatedSearchTimeout = 3 * time.Second
		h.federatedMaxArchives = 10
		h.federatedPerArchiveLimit = 3

		h.searchKiwixArchives(ctx, "test", 3)

		// Wait briefly for goroutines to complete (results are collected via channel).
		// The method is synchronous — it waits for all goroutines before returning.
		mu.Lock()
		defer mu.Unlock()

		if st, ok := capturedSearchTypes["wiki_en"]; !ok || st != "fulltext" {
			t.Errorf("wiki_en searchType = %q, want %q", st, "fulltext")
		}
		if st, ok := capturedSearchTypes["devdocs_go"]; !ok || st != "suggest" {
			t.Errorf("devdocs_go searchType = %q, want %q", st, "suggest")
		}
	})

	t.Run("handles partial failure gracefully", func(t *testing.T) {
		t.Parallel()
		mc := &mockKiwixClient{
			fetchCatalogFn: func(_ context.Context) ([]kiwix.CatalogEntry, error) {
				return []kiwix.CatalogEntry{
					{Name: "wiki_en", HasFulltextIndex: true},
					{Name: "broken_archive", HasFulltextIndex: true},
				}, nil
			},
			searchFn: func(_ context.Context, archiveName, _ string, _ string, _ int) ([]kiwix.SearchResult, error) {
				if archiveName == "broken_archive" {
					return nil, errors.New("connection refused")
				}
				return []kiwix.SearchResult{
					{Title: "Good Result", Path: "/A/Good", Snippet: "works"},
				}, nil
			},
		}
		h := newHandlerWithMocks(struct {
			docSvc   *mockDocumentService
			docRepo  *mockDocumentRepo
			zimRepo  *mockZimArchiveRepo
			gitRepo  *mockGitTemplateRepo
			kiwixC   *mockKiwixClient
			searcher *mockSearcher
		}{
			kiwixC: mc,
			zimRepo: &mockZimArchiveRepo{
				listSearchableFn: func(_ context.Context) ([]model.ZimArchive, error) {
					return []model.ZimArchive{
						{Name: "wiki_en", ArticleCount: 1000},
						{Name: "broken_archive", ArticleCount: 500},
					}, nil
				},
			},
		})
		h.federatedSearchTimeout = 3 * time.Second
		h.federatedMaxArchives = 10
		h.federatedPerArchiveLimit = 3

		results, searched := h.searchKiwixArchives(ctx, "test", 3)

		if len(results) != 1 {
			t.Fatalf("expected 1 result from successful archive, got %d", len(results))
		}
		if results[0].Title != "Good Result" {
			t.Errorf("Title = %q, want %q", results[0].Title, "Good Result")
		}
		if results[0].Archive != "wiki_en" {
			t.Errorf("Archive = %q, want %q", results[0].Archive, "wiki_en")
		}
		// Only the successful archive should appear in searched list.
		if len(searched) != 1 {
			t.Fatalf("expected 1 searched archive, got %d", len(searched))
		}
		if searched[0] != "wiki_en" {
			t.Errorf("searched[0] = %q, want %q", searched[0], "wiki_en")
		}
	})

	t.Run("caps at federatedMaxArchives", func(t *testing.T) {
		t.Parallel()

		// Create 15 archives but set max to 3.
		archives := make([]model.ZimArchive, 15)
		catalogEntries := make([]kiwix.CatalogEntry, 15)
		for i := range 15 {
			name := "archive_" + string(rune('a'+i))
			archives[i] = model.ZimArchive{Name: name, ArticleCount: int64(1000 - i)}
			catalogEntries[i] = kiwix.CatalogEntry{Name: name, HasFulltextIndex: true}
		}

		var searchCount atomic.Int32
		mc := &mockKiwixClient{
			fetchCatalogFn: func(_ context.Context) ([]kiwix.CatalogEntry, error) {
				return catalogEntries, nil
			},
			searchFn: func(_ context.Context, archiveName, _ string, _ string, _ int) ([]kiwix.SearchResult, error) {
				searchCount.Add(1)
				return []kiwix.SearchResult{
					{Title: "Hit from " + archiveName, Path: "/A/Hit"},
				}, nil
			},
		}
		h := newHandlerWithMocks(struct {
			docSvc   *mockDocumentService
			docRepo  *mockDocumentRepo
			zimRepo  *mockZimArchiveRepo
			gitRepo  *mockGitTemplateRepo
			kiwixC   *mockKiwixClient
			searcher *mockSearcher
		}{
			kiwixC: mc,
			zimRepo: &mockZimArchiveRepo{
				listSearchableFn: func(_ context.Context) ([]model.ZimArchive, error) {
					return archives, nil
				},
			},
		})
		h.federatedSearchTimeout = 3 * time.Second
		h.federatedMaxArchives = 3
		h.federatedPerArchiveLimit = 3

		results, searched := h.searchKiwixArchives(ctx, "test", 3)

		if got := int(searchCount.Load()); got != 3 {
			t.Errorf("Search called %d times, want 3 (maxArchives cap)", got)
		}
		if len(results) != 3 {
			t.Errorf("expected 3 results, got %d", len(results))
		}
		if len(searched) != 3 {
			t.Errorf("expected 3 searched, got %d", len(searched))
		}
	})
}

// ===== handleUnifiedSearch fan-out integration tests =====

func TestHandleUnifiedSearchFanOut(t *testing.T) {
	t.Parallel()
	ctx := ctxWithAdminUser()

	t.Run("merges search and Kiwix results sorted by score", func(t *testing.T) {
		t.Parallel()

		// Search returns a high-score document hit.
		s := &mockSearcher{
			federatedSearchFn: func(_ context.Context, _ search.FederatedSearchParams) (*search.FederatedSearchResponse, error) {
				return &search.FederatedSearchResponse{
					Hits: []search.SearchResult{
						makeMCPHit(map[string]any{
							"uuid":        "doc-001",
							"title":       "Go Best Practices",
							"description": "A guide to Go",
							"_federation": map[string]any{
								"indexUid": search.IndexDocuments,
							},
							"_rankingScore": 1.0,
						}),
					},
				}, nil
			},
		}

		// Kiwix returns a lower-score article hit.
		mc := &mockKiwixClient{
			fetchCatalogFn: func(_ context.Context) ([]kiwix.CatalogEntry, error) {
				return []kiwix.CatalogEntry{
					{Name: "wiki_en", HasFulltextIndex: true},
				}, nil
			},
			searchFn: func(_ context.Context, _, _, _ string, _ int) ([]kiwix.SearchResult, error) {
				return []kiwix.SearchResult{
					{Title: "Go Language", Path: "/A/Go", Snippet: "Go wiki article"},
				}, nil
			},
		}

		h := newHandlerWithMocks(struct {
			docSvc   *mockDocumentService
			docRepo  *mockDocumentRepo
			zimRepo  *mockZimArchiveRepo
			gitRepo  *mockGitTemplateRepo
			kiwixC   *mockKiwixClient
			searcher *mockSearcher
		}{
			searcher: s,
			kiwixC:   mc,
			zimRepo: &mockZimArchiveRepo{
				listSearchableFn: func(_ context.Context) ([]model.ZimArchive, error) {
					return []model.ZimArchive{
						{Name: "wiki_en", ArticleCount: 1000},
					}, nil
				},
			},
		})
		h.federatedSearchTimeout = 3 * time.Second
		h.federatedMaxArchives = 10
		h.federatedPerArchiveLimit = 3

		_, resp, err := h.handleUnifiedSearch(ctx, nil, dto.UnifiedSearchInput{Query: "golang"})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !resp.Success {
			t.Fatal("expected Success=true")
		}
		if resp.Total < 2 {
			t.Fatalf("Total = %d, want >= 2", resp.Total)
		}

		// Document (score 1.0) should sort before Kiwix article (score 0.5).
		if resp.Results[0].Source != "document" {
			t.Errorf("Results[0].Source = %q, want %q (highest score)", resp.Results[0].Source, "document")
		}
		if resp.Results[0].Score != 1.0 {
			t.Errorf("Results[0].Score = %v, want 1.0", resp.Results[0].Score)
		}

		// Find the zim_article result.
		found := false
		for _, r := range resp.Results {
			if r.Source == "zim_article" {
				found = true
				if r.Archive != "wiki_en" {
					t.Errorf("zim_article Archive = %q, want %q", r.Archive, "wiki_en")
				}
				if r.Path != "/A/Go" {
					t.Errorf("zim_article Path = %q, want %q", r.Path, "/A/Go")
				}
				if r.Score != 0.5 {
					t.Errorf("zim_article Score = %v, want 0.5", r.Score)
				}
			}
		}
		if !found {
			t.Error("expected zim_article result in merged output")
		}
	})

	t.Run("skips fan-out when types filter excludes zim_article", func(t *testing.T) {
		t.Parallel()

		searchCalled := false
		mc := &mockKiwixClient{
			fetchCatalogFn: func(_ context.Context) ([]kiwix.CatalogEntry, error) {
				t.Error("FetchCatalog should not be called when zim_article is excluded")
				return nil, nil
			},
			searchFn: func(_ context.Context, _, _, _ string, _ int) ([]kiwix.SearchResult, error) {
				searchCalled = true
				t.Error("Search should not be called when zim_article is excluded")
				return nil, nil
			},
		}
		s := &mockSearcher{
			federatedSearchFn: func(_ context.Context, _ search.FederatedSearchParams) (*search.FederatedSearchResponse, error) {
				return &search.FederatedSearchResponse{}, nil
			},
		}
		h := newHandlerWithMocks(struct {
			docSvc   *mockDocumentService
			docRepo  *mockDocumentRepo
			zimRepo  *mockZimArchiveRepo
			gitRepo  *mockGitTemplateRepo
			kiwixC   *mockKiwixClient
			searcher *mockSearcher
		}{
			searcher: s,
			kiwixC:   mc,
			zimRepo: &mockZimArchiveRepo{
				listSearchableFn: func(_ context.Context) ([]model.ZimArchive, error) {
					return []model.ZimArchive{{Name: "wiki_en", ArticleCount: 100}}, nil
				},
			},
		})
		h.federatedSearchTimeout = 3 * time.Second
		h.federatedMaxArchives = 10
		h.federatedPerArchiveLimit = 3

		_, resp, err := h.handleUnifiedSearch(ctx, nil, dto.UnifiedSearchInput{
			Query: "test",
			Types: []string{"document"},
		})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if searchCalled {
			t.Error("Kiwix search was called despite types filter excluding zim_article")
		}

		// sources_searched should not include zim_article.
		for _, src := range resp.SourcesSearched {
			if src == "zim_article" {
				t.Error("sources_searched should not include zim_article when filtered out")
			}
		}
	})

	t.Run("with types=zim_article only skips search", func(t *testing.T) {
		t.Parallel()

		searchCalled := false
		s := &mockSearcher{
			federatedSearchFn: func(_ context.Context, _ search.FederatedSearchParams) (*search.FederatedSearchResponse, error) {
				searchCalled = true
				return &search.FederatedSearchResponse{}, nil
			},
		}
		mc := &mockKiwixClient{
			fetchCatalogFn: func(_ context.Context) ([]kiwix.CatalogEntry, error) {
				return []kiwix.CatalogEntry{
					{Name: "wiki_en", HasFulltextIndex: true},
				}, nil
			},
			searchFn: func(_ context.Context, _, _, _ string, _ int) ([]kiwix.SearchResult, error) {
				return []kiwix.SearchResult{
					{Title: "Only Kiwix", Path: "/A/Only", Snippet: "Kiwix result"},
				}, nil
			},
		}
		h := newHandlerWithMocks(struct {
			docSvc   *mockDocumentService
			docRepo  *mockDocumentRepo
			zimRepo  *mockZimArchiveRepo
			gitRepo  *mockGitTemplateRepo
			kiwixC   *mockKiwixClient
			searcher *mockSearcher
		}{
			searcher: s,
			kiwixC:   mc,
			zimRepo: &mockZimArchiveRepo{
				listSearchableFn: func(_ context.Context) ([]model.ZimArchive, error) {
					return []model.ZimArchive{
						{Name: "wiki_en", ArticleCount: 1000},
					}, nil
				},
			},
		})
		h.federatedSearchTimeout = 3 * time.Second
		h.federatedMaxArchives = 10
		h.federatedPerArchiveLimit = 3

		_, resp, err := h.handleUnifiedSearch(ctx, nil, dto.UnifiedSearchInput{
			Query: "test",
			Types: []string{"zim_article"},
		})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if searchCalled {
			t.Error("FederatedSearch was called despite types=[zim_article]")
		}
		if !resp.Success {
			t.Fatal("expected Success=true")
		}
		if resp.Total != 1 {
			t.Fatalf("Total = %d, want 1", resp.Total)
		}
		if resp.Results[0].Source != "zim_article" {
			t.Errorf("Results[0].Source = %q, want %q", resp.Results[0].Source, "zim_article")
		}

		// sources_searched should only contain zim_article.
		if len(resp.SourcesSearched) != 1 || resp.SourcesSearched[0] != "zim_article" {
			t.Errorf("SourcesSearched = %v, want [zim_article]", resp.SourcesSearched)
		}
	})

	t.Run("includes zim_article in sources_searched when archives found", func(t *testing.T) {
		t.Parallel()

		s := &mockSearcher{
			federatedSearchFn: func(_ context.Context, _ search.FederatedSearchParams) (*search.FederatedSearchResponse, error) {
				return &search.FederatedSearchResponse{}, nil
			},
		}
		mc := &mockKiwixClient{
			fetchCatalogFn: func(_ context.Context) ([]kiwix.CatalogEntry, error) {
				return []kiwix.CatalogEntry{
					{Name: "wiki_en", HasFulltextIndex: true},
				}, nil
			},
			searchFn: func(_ context.Context, _, _, _ string, _ int) ([]kiwix.SearchResult, error) {
				return []kiwix.SearchResult{
					{Title: "Some Article", Path: "/A/Some"},
				}, nil
			},
		}
		h := newHandlerWithMocks(struct {
			docSvc   *mockDocumentService
			docRepo  *mockDocumentRepo
			zimRepo  *mockZimArchiveRepo
			gitRepo  *mockGitTemplateRepo
			kiwixC   *mockKiwixClient
			searcher *mockSearcher
		}{
			searcher: s,
			kiwixC:   mc,
			zimRepo: &mockZimArchiveRepo{
				listSearchableFn: func(_ context.Context) ([]model.ZimArchive, error) {
					return []model.ZimArchive{
						{Name: "wiki_en", ArticleCount: 1000},
					}, nil
				},
			},
		})
		h.federatedSearchTimeout = 3 * time.Second
		h.federatedMaxArchives = 10
		h.federatedPerArchiveLimit = 3

		// No types filter — search all sources.
		_, resp, err := h.handleUnifiedSearch(ctx, nil, dto.UnifiedSearchInput{Query: "test"})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if !slices.Contains(resp.SourcesSearched, "zim_article") {
			t.Errorf("SourcesSearched = %v, expected to contain %q", resp.SourcesSearched, "zim_article")
		}
	})
}

// ===== FTS-based archive selection tests =====

func TestSearchKiwixArchives_FTSSelection(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	// catalogForNames returns a catalog with fulltext support for all named archives.
	catalogForNames := func(names ...string) func(context.Context) ([]kiwix.CatalogEntry, error) {
		return func(_ context.Context) ([]kiwix.CatalogEntry, error) {
			entries := make([]kiwix.CatalogEntry, len(names))
			for i, n := range names {
				entries[i] = kiwix.CatalogEntry{Name: n, HasFulltextIndex: true}
			}
			return entries, nil
		}
	}

	t.Run("selects archives via FTS", func(t *testing.T) {
		t.Parallel()

		var searchedArchives sync.Map
		mc := &mockKiwixClient{
			fetchCatalogFn: catalogForNames("archive_a", "archive_b", "archive_c", "archive_d", "archive_e"),
			searchFn: func(_ context.Context, archiveName, _, _ string, _ int) ([]kiwix.SearchResult, error) {
				searchedArchives.Store(archiveName, true)
				return []kiwix.SearchResult{{Title: "Article", Path: "A/Article"}}, nil
			},
		}

		h := newHandlerWithMocks(struct {
			docSvc   *mockDocumentService
			docRepo  *mockDocumentRepo
			zimRepo  *mockZimArchiveRepo
			gitRepo  *mockGitTemplateRepo
			kiwixC   *mockKiwixClient
			searcher *mockSearcher
		}{
			kiwixC: mc,
			searcher: &mockSearcher{
				searchFn: func(_ context.Context, params search.SearchParams) (*search.SearchResponse, error) {
					if params.IndexUID != search.IndexZimArchives {
						t.Errorf("expected index %q, got %q", search.IndexZimArchives, params.IndexUID)
					}
					return &search.SearchResponse{
						Hits: []search.SearchResult{
							makeHit(map[string]any{"name": "archive_a", "title": "Archive A"}),
							makeHit(map[string]any{"name": "archive_b", "title": "Archive B"}),
						},
					}, nil
				},
			},
			zimRepo: &mockZimArchiveRepo{
				listSearchableFn: func(_ context.Context) ([]model.ZimArchive, error) {
					return []model.ZimArchive{
						{Name: "archive_a", ArticleCount: 100},
						{Name: "archive_b", ArticleCount: 200},
						{Name: "archive_c", ArticleCount: 300},
						{Name: "archive_d", ArticleCount: 400},
						{Name: "archive_e", ArticleCount: 500},
					}, nil
				},
			},
		})
		h.federatedSearchTimeout = 3 * time.Second
		h.federatedMaxArchives = 10
		h.federatedPerArchiveLimit = 3

		results, searched := h.searchKiwixArchives(ctx, "test query", 3)
		if len(results) == 0 {
			t.Fatal("expected results, got none")
		}

		// Only archive_a and archive_b should have been fanned out to.
		if len(searched) != 2 {
			t.Errorf("expected 2 archives searched, got %d: %v", len(searched), searched)
		}
		for _, name := range searched {
			if name != "archive_a" && name != "archive_b" {
				t.Errorf("unexpected archive searched: %s", name)
			}
		}

		// Verify irrelevant archives were NOT called.
		for _, name := range []string{"archive_c", "archive_d", "archive_e"} {
			if _, ok := searchedArchives.Load(name); ok {
				t.Errorf("archive %s should not have been searched", name)
			}
		}
	})

	t.Run("falls back when FTS returns no hits", func(t *testing.T) {
		t.Parallel()

		var callCount atomic.Int32
		mc := &mockKiwixClient{
			fetchCatalogFn: catalogForNames("archive_a", "archive_b", "archive_c"),
			searchFn: func(_ context.Context, _, _, _ string, _ int) ([]kiwix.SearchResult, error) {
				callCount.Add(1)
				return []kiwix.SearchResult{{Title: "Article", Path: "A/Article"}}, nil
			},
		}

		h := newHandlerWithMocks(struct {
			docSvc   *mockDocumentService
			docRepo  *mockDocumentRepo
			zimRepo  *mockZimArchiveRepo
			gitRepo  *mockGitTemplateRepo
			kiwixC   *mockKiwixClient
			searcher *mockSearcher
		}{
			kiwixC: mc,
			searcher: &mockSearcher{
				searchFn: func(_ context.Context, _ search.SearchParams) (*search.SearchResponse, error) {
					return &search.SearchResponse{Hits: []search.SearchResult{}}, nil
				},
			},
			zimRepo: &mockZimArchiveRepo{
				listSearchableFn: func(_ context.Context) ([]model.ZimArchive, error) {
					return []model.ZimArchive{
						{Name: "archive_a", ArticleCount: 100},
						{Name: "archive_b", ArticleCount: 200},
						{Name: "archive_c", ArticleCount: 300},
					}, nil
				},
			},
		})
		h.federatedSearchTimeout = 3 * time.Second
		h.federatedMaxArchives = 10
		h.federatedPerArchiveLimit = 3

		_, searched := h.searchKiwixArchives(ctx, "test query", 3)

		// All 3 archives should be searched (fallback to DB list).
		if len(searched) != 3 {
			t.Errorf("expected 3 archives searched (fallback), got %d: %v", len(searched), searched)
		}
	})

	t.Run("falls back when searcher is nil", func(t *testing.T) {
		t.Parallel()

		var callCount atomic.Int32
		mc := &mockKiwixClient{
			fetchCatalogFn: catalogForNames("archive_a", "archive_b", "archive_c"),
			searchFn: func(_ context.Context, _, _, _ string, _ int) ([]kiwix.SearchResult, error) {
				callCount.Add(1)
				return []kiwix.SearchResult{{Title: "Article", Path: "A/Article"}}, nil
			},
		}

		h := newHandlerWithMocks(struct {
			docSvc   *mockDocumentService
			docRepo  *mockDocumentRepo
			zimRepo  *mockZimArchiveRepo
			gitRepo  *mockGitTemplateRepo
			kiwixC   *mockKiwixClient
			searcher *mockSearcher
		}{
			kiwixC: mc,
			// searcher intentionally nil
			zimRepo: &mockZimArchiveRepo{
				listSearchableFn: func(_ context.Context) ([]model.ZimArchive, error) {
					return []model.ZimArchive{
						{Name: "archive_a", ArticleCount: 100},
						{Name: "archive_b", ArticleCount: 200},
						{Name: "archive_c", ArticleCount: 300},
					}, nil
				},
			},
		})
		h.federatedSearchTimeout = 3 * time.Second
		h.federatedMaxArchives = 10
		h.federatedPerArchiveLimit = 3

		_, searched := h.searchKiwixArchives(ctx, "test query", 3)

		// All 3 archives should be searched (no FTS filtering).
		if len(searched) != 3 {
			t.Errorf("expected 3 archives searched, got %d: %v", len(searched), searched)
		}
	})

	t.Run("always includes devdocs archives even when not selected by FTS", func(t *testing.T) {
		t.Parallel()

		var searchedArchives sync.Map
		mc := &mockKiwixClient{
			fetchCatalogFn: catalogForNames("archive_a", "devdocs_en_go", "archive_c"),
			searchFn: func(_ context.Context, archiveName, _, _ string, _ int) ([]kiwix.SearchResult, error) {
				searchedArchives.Store(archiveName, true)
				return []kiwix.SearchResult{{Title: "Article", Path: "A/Article"}}, nil
			},
		}

		h := newHandlerWithMocks(struct {
			docSvc   *mockDocumentService
			docRepo  *mockDocumentRepo
			zimRepo  *mockZimArchiveRepo
			gitRepo  *mockGitTemplateRepo
			kiwixC   *mockKiwixClient
			searcher *mockSearcher
		}{
			kiwixC: mc,
			searcher: &mockSearcher{
				searchFn: func(_ context.Context, _ search.SearchParams) (*search.SearchResponse, error) {
					// FTS only selects archive_a — devdocs_en_go metadata doesn't match.
					return &search.SearchResponse{
						Hits: []search.SearchResult{
							makeHit(map[string]any{"name": "archive_a", "title": "Archive A"}),
						},
					}, nil
				},
			},
			zimRepo: &mockZimArchiveRepo{
				listSearchableFn: func(_ context.Context) ([]model.ZimArchive, error) {
					return []model.ZimArchive{
						{Name: "archive_a", ArticleCount: 100},
						{Name: "devdocs_en_go", ArticleCount: 183, Category: sql.NullString{String: "devdocs", Valid: true}},
						{Name: "archive_c", ArticleCount: 300},
					}, nil
				},
			},
		})
		h.federatedSearchTimeout = 3 * time.Second
		h.federatedMaxArchives = 10
		h.federatedPerArchiveLimit = 3

		_, searched := h.searchKiwixArchives(ctx, "os/exec Package exec", 3)

		// Both archive_a (FTS-selected) and devdocs_en_go (DevDocs) should be searched.
		if len(searched) != 2 {
			t.Errorf("expected 2 archives searched, got %d: %v", len(searched), searched)
		}
		for _, name := range searched {
			if name != "archive_a" && name != "devdocs_en_go" {
				t.Errorf("unexpected archive searched: %s", name)
			}
		}

		// archive_c should NOT have been searched.
		if _, ok := searchedArchives.Load("archive_c"); ok {
			t.Error("archive_c should not have been searched")
		}
	})

	t.Run("falls back on search error", func(t *testing.T) {
		t.Parallel()

		var callCount atomic.Int32
		mc := &mockKiwixClient{
			fetchCatalogFn: catalogForNames("archive_a", "archive_b", "archive_c"),
			searchFn: func(_ context.Context, _, _, _ string, _ int) ([]kiwix.SearchResult, error) {
				callCount.Add(1)
				return []kiwix.SearchResult{{Title: "Article", Path: "A/Article"}}, nil
			},
		}

		h := newHandlerWithMocks(struct {
			docSvc   *mockDocumentService
			docRepo  *mockDocumentRepo
			zimRepo  *mockZimArchiveRepo
			gitRepo  *mockGitTemplateRepo
			kiwixC   *mockKiwixClient
			searcher *mockSearcher
		}{
			kiwixC: mc,
			searcher: &mockSearcher{
				searchFn: func(_ context.Context, _ search.SearchParams) (*search.SearchResponse, error) {
					return nil, errors.New("search unavailable")
				},
			},
			zimRepo: &mockZimArchiveRepo{
				listSearchableFn: func(_ context.Context) ([]model.ZimArchive, error) {
					return []model.ZimArchive{
						{Name: "archive_a", ArticleCount: 100},
						{Name: "archive_b", ArticleCount: 200},
						{Name: "archive_c", ArticleCount: 300},
					}, nil
				},
			},
		})
		h.federatedSearchTimeout = 3 * time.Second
		h.federatedMaxArchives = 10
		h.federatedPerArchiveLimit = 3

		_, searched := h.searchKiwixArchives(ctx, "test query", 3)

		// All 3 archives should be searched (graceful fallback).
		if len(searched) != 3 {
			t.Errorf("expected 3 archives searched (fallback), got %d: %v", len(searched), searched)
		}
	})
}
