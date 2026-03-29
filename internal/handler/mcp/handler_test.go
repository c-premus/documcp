package mcphandler

import (
	"context"
	"database/sql"
	"errors"
	"log/slog"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	authmiddleware "github.com/c-premus/documcp/internal/auth/middleware"
	"github.com/c-premus/documcp/internal/client/kiwix"
	"github.com/c-premus/documcp/internal/dto"
	"github.com/c-premus/documcp/internal/model"
	"github.com/c-premus/documcp/internal/search"
	"github.com/c-premus/documcp/internal/service"
)

// mcpToken returns an access token with all MCP scopes for use in test contexts.
func mcpToken() *model.OAuthAccessToken {
	return &model.OAuthAccessToken{
		Scope: sql.NullString{String: "mcp:access mcp:read mcp:write", Valid: true},
	}
}

// ctxWithAdminUser returns a context with an admin user and bearer token,
// simulating an authenticated bearer-token request to MCP tools.
func ctxWithAdminUser() context.Context {
	ctx := context.Background()
	ctx = context.WithValue(ctx, authmiddleware.UserContextKey, &model.User{ID: 1, IsAdmin: true})
	ctx = context.WithValue(ctx, authmiddleware.AccessTokenContextKey, mcpToken())
	return ctx
}

// ctxWithTokenOnly returns a context with a bearer token but no user (M2M flow).
func ctxWithTokenOnly() context.Context {
	return context.WithValue(context.Background(), authmiddleware.AccessTokenContextKey, mcpToken())
}

// makeMCPHit builds a search.SearchResult from a plain map for test convenience.
func makeMCPHit(m map[string]any) search.SearchResult {
	return mapToSearchResult(m)
}

// mapToSearchResult converts a map of arbitrary values to a SearchResult.
func mapToSearchResult(m map[string]any) search.SearchResult {
	r := search.SearchResult{Extra: make(map[string]any)}
	for k, v := range m {
		switch k {
		case "uuid":
			if s, ok := v.(string); ok {
				r.UUID = s
			}
		case "title":
			if s, ok := v.(string); ok {
				r.Title = s
			}
		case "name":
			if s, ok := v.(string); ok {
				if r.Title == "" {
					r.Title = s
				}
				r.Extra["name"] = v // also store in Extra for lookups
			}
		case "description":
			if s, ok := v.(string); ok {
				r.Description = s
			}
		case "source":
			if s, ok := v.(string); ok {
				r.Source = s
			}
		case "_federation":
			// Extract indexUid from federation metadata map.
			switch fv := v.(type) {
			case map[string]any:
				if uid, ok := fv["indexUid"].(string); ok {
					r.Source = uid
				}
			case string:
				r.Source = fv
			}
		case "_rankingScore":
			if f, ok := v.(float64); ok {
				r.Score = f
			}
		default:
			r.Extra[k] = v
		}
	}
	return r
}

// --- Mock implementations ---

type mockDocumentService struct {
	findByUUIDFn func(ctx context.Context, uuid string) (*model.Document, error)
	tagsForDocFn func(ctx context.Context, docID int64) ([]model.DocumentTag, error)
	createFn     func(ctx context.Context, params service.CreateDocumentParams) (*model.Document, error)
	updateFn     func(ctx context.Context, uuid string, params service.UpdateDocumentParams) (*model.Document, error)
	deleteFn     func(ctx context.Context, uuid string) error
}

func (m *mockDocumentService) FindByUUID(ctx context.Context, uuid string) (*model.Document, error) {
	if m.findByUUIDFn != nil {
		return m.findByUUIDFn(ctx, uuid)
	}
	return nil, nil
}

func (m *mockDocumentService) TagsForDocument(ctx context.Context, docID int64) ([]model.DocumentTag, error) {
	if m.tagsForDocFn != nil {
		return m.tagsForDocFn(ctx, docID)
	}
	return nil, nil
}

func (m *mockDocumentService) Create(ctx context.Context, params service.CreateDocumentParams) (*model.Document, error) {
	if m.createFn != nil {
		return m.createFn(ctx, params)
	}
	return nil, nil
}

func (m *mockDocumentService) Update(ctx context.Context, uuid string, params service.UpdateDocumentParams) (*model.Document, error) {
	if m.updateFn != nil {
		return m.updateFn(ctx, uuid, params)
	}
	return nil, nil
}

func (m *mockDocumentService) Delete(ctx context.Context, uuid string) error {
	if m.deleteFn != nil {
		return m.deleteFn(ctx, uuid)
	}
	return nil
}

type mockZimArchiveRepo struct {
	listFn           func(ctx context.Context, category, language, query string, limit, offset int) ([]model.ZimArchive, error)
	listSearchableFn func(ctx context.Context) ([]model.ZimArchive, error)
}

func (m *mockZimArchiveRepo) List(ctx context.Context, category, language, query string, limit, offset int) ([]model.ZimArchive, error) {
	if m.listFn != nil {
		return m.listFn(ctx, category, language, query, limit, offset)
	}
	return nil, nil
}

func (m *mockZimArchiveRepo) ListSearchable(ctx context.Context) ([]model.ZimArchive, error) {
	if m.listSearchableFn != nil {
		return m.listSearchableFn(ctx)
	}
	return nil, nil
}

type mockGitTemplateRepo struct {
	listFn           func(ctx context.Context, category string, limit, offset int) ([]model.GitTemplate, error)
	searchFn         func(ctx context.Context, query, category string, limit int) ([]model.GitTemplate, error)
	findByUUIDFn     func(ctx context.Context, uuid string) (*model.GitTemplate, error)
	filesForTmplFn   func(ctx context.Context, templateID int64) ([]model.GitTemplateFile, error)
	findFileByPathFn func(ctx context.Context, templateID int64, path string) (*model.GitTemplateFile, error)
}

func (m *mockGitTemplateRepo) List(ctx context.Context, category string, limit, offset int) ([]model.GitTemplate, error) {
	if m.listFn != nil {
		return m.listFn(ctx, category, limit, offset)
	}
	return nil, nil
}

func (m *mockGitTemplateRepo) Search(ctx context.Context, query, category string, limit int) ([]model.GitTemplate, error) {
	if m.searchFn != nil {
		return m.searchFn(ctx, query, category, limit)
	}
	return nil, nil
}

func (m *mockGitTemplateRepo) FindByUUID(ctx context.Context, uuid string) (*model.GitTemplate, error) {
	if m.findByUUIDFn != nil {
		return m.findByUUIDFn(ctx, uuid)
	}
	return nil, nil
}

func (m *mockGitTemplateRepo) FilesForTemplate(ctx context.Context, templateID int64) ([]model.GitTemplateFile, error) {
	if m.filesForTmplFn != nil {
		return m.filesForTmplFn(ctx, templateID)
	}
	return nil, nil
}

func (m *mockGitTemplateRepo) FindFileByPath(ctx context.Context, templateID int64, path string) (*model.GitTemplateFile, error) {
	if m.findFileByPathFn != nil {
		return m.findFileByPathFn(ctx, templateID, path)
	}
	return nil, nil
}

type mockKiwixClient struct {
	searchFn       func(ctx context.Context, archiveName, query, searchType string, limit int) ([]kiwix.SearchResult, error)
	readArticleFn  func(ctx context.Context, archiveName, articlePath string) (*kiwix.Article, error)
	fetchCatalogFn func(ctx context.Context) ([]kiwix.CatalogEntry, error)
}

func (m *mockKiwixClient) Search(ctx context.Context, archiveName, query, searchType string, limit int) ([]kiwix.SearchResult, error) {
	if m.searchFn != nil {
		return m.searchFn(ctx, archiveName, query, searchType, limit)
	}
	return nil, nil
}

func (m *mockKiwixClient) ReadArticle(ctx context.Context, archiveName, articlePath string) (*kiwix.Article, error) {
	if m.readArticleFn != nil {
		return m.readArticleFn(ctx, archiveName, articlePath)
	}
	return nil, nil
}

func (m *mockKiwixClient) FetchCatalog(ctx context.Context) ([]kiwix.CatalogEntry, error) {
	if m.fetchCatalogFn != nil {
		return m.fetchCatalogFn(ctx)
	}
	return nil, nil
}

// mockKiwixFactory wraps a mockKiwixClient as a kiwixClientFactory for tests.
type mockKiwixFactory struct {
	client *mockKiwixClient
}

func (f *mockKiwixFactory) Get(_ context.Context) (kiwixSearcher, error) {
	if f.client == nil {
		return nil, errors.New("kiwix not configured")
	}
	return f.client, nil
}

type mockSearcher struct {
	searchFn          func(ctx context.Context, params search.SearchParams) (*search.SearchResponse, error)
	federatedSearchFn func(ctx context.Context, params search.FederatedSearchParams) (*search.FederatedSearchResponse, error)
}

func (m *mockSearcher) Search(ctx context.Context, params search.SearchParams) (*search.SearchResponse, error) {
	if m.searchFn != nil {
		return m.searchFn(ctx, params)
	}
	return &search.SearchResponse{}, nil
}

func (m *mockSearcher) FederatedSearch(ctx context.Context, params search.FederatedSearchParams) (*search.FederatedSearchResponse, error) {
	if m.federatedSearchFn != nil {
		return m.federatedSearchFn(ctx, params)
	}
	return &search.FederatedSearchResponse{}, nil
}

// newHandlerWithMocks creates a Handler with a real MCP server and the provided
// mock dependencies. Pass nil for any dependency you don't need.
func newHandlerWithMocks(opts struct {
	docSvc   *mockDocumentService
	zimRepo  *mockZimArchiveRepo
	gitRepo  *mockGitTemplateRepo
	kiwixC   *mockKiwixClient
	searcher *mockSearcher
}) *Handler {
	srv := mcp.NewServer(
		&mcp.Implementation{Name: "test", Version: "v0.0.0"},
		nil,
	)
	h := &Handler{
		server: srv,
		logger: slog.Default(),
	}
	if opts.docSvc != nil {
		h.documentService = opts.docSvc
	}
	if opts.zimRepo != nil {
		h.zimArchiveRepo = opts.zimRepo
	}
	if opts.gitRepo != nil {
		h.gitTemplateRepo = opts.gitRepo
	}
	if opts.kiwixC != nil {
		h.kiwixFactory = &mockKiwixFactory{client: opts.kiwixC}
	}
	if opts.searcher != nil {
		h.searcher = opts.searcher
	} else {
		h.searcher = &mockSearcher{} // default no-op searcher
	}
	return h
}

// ===== Handler constructor tests =====

func TestNew(t *testing.T) {
	t.Run("creates handler with minimal config", func(t *testing.T) {
		h := New(Config{
			ServerName:    "test",
			ServerVersion: "v1",
			Logger:        slog.Default(),
		})
		if h == nil {
			t.Fatal("New() returned nil")
		}
		if h.server == nil {
			t.Error("server is nil")
		}
		if h.httpHandler == nil {
			t.Error("httpHandler is nil")
		}
	})

	t.Run("conditionally registers zim tools when enabled", func(t *testing.T) {
		h := New(Config{
			ServerName:     "test",
			ServerVersion:  "v1",
			Logger:         slog.Default(),
			ZimEnabled:     true,
			ZimArchiveRepo: &mockZimArchiveRepo{},
		})
		if h == nil {
			t.Fatal("New() returned nil")
		}
	})

	t.Run("does not register zim tools when repo is nil", func(t *testing.T) {
		// Should not panic even though ZimEnabled is true but repo is nil.
		h := New(Config{
			ServerName:    "test",
			ServerVersion: "v1",
			Logger:        slog.Default(),
			ZimEnabled:    true,
		})
		if h == nil {
			t.Fatal("New() returned nil")
		}
	})

	t.Run("conditionally registers git template tools when enabled", func(t *testing.T) {
		h := New(Config{
			ServerName:          "test",
			ServerVersion:       "v1",
			Logger:              slog.Default(),
			GitTemplatesEnabled: true,
			GitTemplateRepo:     &mockGitTemplateRepo{},
		})
		if h == nil {
			t.Fatal("New() returned nil")
		}
	})
}

// ===== Document tool handler tests =====

func TestHandleReadDocument(t *testing.T) {
	ctx := ctxWithAdminUser()

	t.Run("returns document with content and tags", func(t *testing.T) {
		docSvc := &mockDocumentService{
			findByUUIDFn: func(_ context.Context, uuid string) (*model.Document, error) {
				return &model.Document{
					ID:       1,
					UUID:     uuid,
					Title:    "Test Doc",
					FileType: "markdown",
					Content:  sql.NullString{String: "Hello world\n\nSecond paragraph", Valid: true},
					IsPublic: true,
				}, nil
			},
			tagsForDocFn: func(_ context.Context, _ int64) ([]model.DocumentTag, error) {
				return []model.DocumentTag{{Tag: "go"}, {Tag: "test"}}, nil
			},
		}
		h := newHandlerWithMocks(struct {
			docSvc   *mockDocumentService
			zimRepo  *mockZimArchiveRepo
			gitRepo  *mockGitTemplateRepo
			kiwixC   *mockKiwixClient
			searcher *mockSearcher
		}{docSvc: docSvc})

		_, resp, err := h.handleReadDocument(ctx, nil, dto.ReadDocumentInput{UUID: "abc-123"})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !resp.Success {
			t.Error("expected Success=true")
		}
		if resp.Document == nil {
			t.Fatal("expected Document to be non-nil")
		}
		if resp.Document.UUID != "abc-123" {
			t.Errorf("UUID = %q, want %q", resp.Document.UUID, "abc-123")
		}
		if resp.Content != "Hello world\n\nSecond paragraph" {
			t.Errorf("Content = %q, want full content", resp.Content)
		}
		if resp.Truncated {
			t.Error("expected Truncated=false")
		}
		if len(resp.Document.Tags) != 2 {
			t.Errorf("Tags count = %d, want 2", len(resp.Document.Tags))
		}
	})

	t.Run("returns error when document not found", func(t *testing.T) {
		docSvc := &mockDocumentService{
			findByUUIDFn: func(_ context.Context, _ string) (*model.Document, error) {
				return nil, service.ErrNotFound
			},
		}
		h := newHandlerWithMocks(struct {
			docSvc   *mockDocumentService
			zimRepo  *mockZimArchiveRepo
			gitRepo  *mockGitTemplateRepo
			kiwixC   *mockKiwixClient
			searcher *mockSearcher
		}{docSvc: docSvc})

		_, _, err := h.handleReadDocument(ctx, nil, dto.ReadDocumentInput{UUID: "not-found"})
		if err == nil {
			t.Fatal("expected error for missing document")
		}
	})

	t.Run("returns error when service fails", func(t *testing.T) {
		docSvc := &mockDocumentService{
			findByUUIDFn: func(_ context.Context, _ string) (*model.Document, error) {
				return nil, errors.New("db error")
			},
		}
		h := newHandlerWithMocks(struct {
			docSvc   *mockDocumentService
			zimRepo  *mockZimArchiveRepo
			gitRepo  *mockGitTemplateRepo
			kiwixC   *mockKiwixClient
			searcher *mockSearcher
		}{docSvc: docSvc})

		_, _, err := h.handleReadDocument(ctx, nil, dto.ReadDocumentInput{UUID: "fail"})
		if err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("applies summary_only truncation", func(t *testing.T) {
		docSvc := &mockDocumentService{
			findByUUIDFn: func(_ context.Context, uuid string) (*model.Document, error) {
				return &model.Document{
					ID:       1,
					UUID:     uuid,
					Title:    "Doc",
					FileType: "markdown",
					Content:  sql.NullString{String: "Intro text.\n\n# Section\n\nBody.", Valid: true},
				}, nil
			},
			tagsForDocFn: func(_ context.Context, _ int64) ([]model.DocumentTag, error) {
				return nil, nil
			},
		}
		h := newHandlerWithMocks(struct {
			docSvc   *mockDocumentService
			zimRepo  *mockZimArchiveRepo
			gitRepo  *mockGitTemplateRepo
			kiwixC   *mockKiwixClient
			searcher *mockSearcher
		}{docSvc: docSvc})

		_, resp, err := h.handleReadDocument(ctx, nil, dto.ReadDocumentInput{
			UUID:        "trunc-1",
			SummaryOnly: true,
		})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !resp.Truncated {
			t.Error("expected Truncated=true")
		}
		if resp.Content != "Intro text." {
			t.Errorf("Content = %q, want %q", resp.Content, "Intro text.")
		}
	})

	t.Run("returns error when tags fail to load", func(t *testing.T) {
		docSvc := &mockDocumentService{
			findByUUIDFn: func(_ context.Context, uuid string) (*model.Document, error) {
				return &model.Document{ID: 1, UUID: uuid, FileType: "md"}, nil
			},
			tagsForDocFn: func(_ context.Context, _ int64) ([]model.DocumentTag, error) {
				return nil, errors.New("tags error")
			},
		}
		h := newHandlerWithMocks(struct {
			docSvc   *mockDocumentService
			zimRepo  *mockZimArchiveRepo
			gitRepo  *mockGitTemplateRepo
			kiwixC   *mockKiwixClient
			searcher *mockSearcher
		}{docSvc: docSvc})

		_, _, err := h.handleReadDocument(ctx, nil, dto.ReadDocumentInput{UUID: "x"})
		if err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("M2M token (no user) can read public document", func(t *testing.T) {
		m2mCtx := ctxWithTokenOnly() // bearer token but no user
		docSvc := &mockDocumentService{
			findByUUIDFn: func(_ context.Context, uuid string) (*model.Document, error) {
				return &model.Document{ID: 1, UUID: uuid, Title: "Public", FileType: "md", IsPublic: true}, nil
			},
			tagsForDocFn: func(_ context.Context, _ int64) ([]model.DocumentTag, error) {
				return nil, nil
			},
		}
		h := newHandlerWithMocks(struct {
			docSvc   *mockDocumentService
			zimRepo  *mockZimArchiveRepo
			gitRepo  *mockGitTemplateRepo
			kiwixC   *mockKiwixClient
			searcher *mockSearcher
		}{docSvc: docSvc})

		_, resp, err := h.handleReadDocument(m2mCtx, nil, dto.ReadDocumentInput{UUID: "pub-1"})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !resp.Success {
			t.Error("expected Success=true for public document with M2M token")
		}
	})

	t.Run("M2M token (no user) cannot read private document", func(t *testing.T) {
		m2mCtx := ctxWithTokenOnly() // bearer token but no user
		docSvc := &mockDocumentService{
			findByUUIDFn: func(_ context.Context, uuid string) (*model.Document, error) {
				return &model.Document{ID: 2, UUID: uuid, Title: "Private", FileType: "md", IsPublic: false}, nil
			},
		}
		h := newHandlerWithMocks(struct {
			docSvc   *mockDocumentService
			zimRepo  *mockZimArchiveRepo
			gitRepo  *mockGitTemplateRepo
			kiwixC   *mockKiwixClient
			searcher *mockSearcher
		}{docSvc: docSvc})

		_, _, err := h.handleReadDocument(m2mCtx, nil, dto.ReadDocumentInput{UUID: "priv-1"})
		if err == nil {
			t.Fatal("expected error for private document with M2M token")
		}
	})
}

func TestHandleCreateDocument(t *testing.T) {
	ctx := ctxWithAdminUser()

	t.Run("creates document successfully", func(t *testing.T) {
		docSvc := &mockDocumentService{
			createFn: func(_ context.Context, params service.CreateDocumentParams) (*model.Document, error) {
				return &model.Document{
					UUID:      "new-uuid",
					Title:     params.Title,
					FileType:  params.FileType,
					CreatedAt: sql.NullTime{Valid: false},
				}, nil
			},
		}
		h := newHandlerWithMocks(struct {
			docSvc   *mockDocumentService
			zimRepo  *mockZimArchiveRepo
			gitRepo  *mockGitTemplateRepo
			kiwixC   *mockKiwixClient
			searcher *mockSearcher
		}{docSvc: docSvc})

		_, resp, err := h.handleCreateDocument(ctx, nil, dto.CreateDocumentInput{
			Title:    "New Doc",
			Content:  "Content here",
			FileType: "markdown",
			Tags:     []string{"go"},
		})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !resp.Success {
			t.Error("expected Success=true")
		}
		if resp.Document == nil {
			t.Fatal("expected Document to be non-nil")
		}
		if resp.Document.UUID != "new-uuid" {
			t.Errorf("UUID = %q, want %q", resp.Document.UUID, "new-uuid")
		}
	})

	t.Run("returns error when service fails", func(t *testing.T) {
		docSvc := &mockDocumentService{
			createFn: func(_ context.Context, _ service.CreateDocumentParams) (*model.Document, error) {
				return nil, errors.New("create failed")
			},
		}
		h := newHandlerWithMocks(struct {
			docSvc   *mockDocumentService
			zimRepo  *mockZimArchiveRepo
			gitRepo  *mockGitTemplateRepo
			kiwixC   *mockKiwixClient
			searcher *mockSearcher
		}{docSvc: docSvc})

		_, _, err := h.handleCreateDocument(ctx, nil, dto.CreateDocumentInput{
			Title:    "Fail",
			Content:  "x",
			FileType: "markdown",
		})
		if err == nil {
			t.Fatal("expected error")
		}
	})
}

func TestHandleUpdateDocument(t *testing.T) {
	ctx := ctxWithAdminUser()

	t.Run("updates document successfully", func(t *testing.T) {
		docSvc := &mockDocumentService{
			updateFn: func(_ context.Context, uuid string, _ service.UpdateDocumentParams) (*model.Document, error) {
				return &model.Document{
					UUID:     uuid,
					Title:    "Updated Title",
					FileType: "markdown",
				}, nil
			},
		}
		h := newHandlerWithMocks(struct {
			docSvc   *mockDocumentService
			zimRepo  *mockZimArchiveRepo
			gitRepo  *mockGitTemplateRepo
			kiwixC   *mockKiwixClient
			searcher *mockSearcher
		}{docSvc: docSvc})

		_, resp, err := h.handleUpdateDocument(ctx, nil, dto.UpdateDocumentInput{
			UUID:  "abc-123",
			Title: "Updated Title",
		})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !resp.Success {
			t.Error("expected Success=true")
		}
		if resp.Document.Title != "Updated Title" {
			t.Errorf("Title = %q, want %q", resp.Document.Title, "Updated Title")
		}
	})

	t.Run("returns error when service fails", func(t *testing.T) {
		docSvc := &mockDocumentService{
			updateFn: func(_ context.Context, _ string, _ service.UpdateDocumentParams) (*model.Document, error) {
				return nil, errors.New("not found")
			},
		}
		h := newHandlerWithMocks(struct {
			docSvc   *mockDocumentService
			zimRepo  *mockZimArchiveRepo
			gitRepo  *mockGitTemplateRepo
			kiwixC   *mockKiwixClient
			searcher *mockSearcher
		}{docSvc: docSvc})

		_, _, err := h.handleUpdateDocument(ctx, nil, dto.UpdateDocumentInput{UUID: "x"})
		if err == nil {
			t.Fatal("expected error")
		}
	})
}

func TestHandleDeleteDocument(t *testing.T) {
	ctx := ctxWithAdminUser()

	t.Run("deletes document successfully", func(t *testing.T) {
		docSvc := &mockDocumentService{
			deleteFn: func(_ context.Context, _ string) error {
				return nil
			},
		}
		h := newHandlerWithMocks(struct {
			docSvc   *mockDocumentService
			zimRepo  *mockZimArchiveRepo
			gitRepo  *mockGitTemplateRepo
			kiwixC   *mockKiwixClient
			searcher *mockSearcher
		}{docSvc: docSvc})

		_, resp, err := h.handleDeleteDocument(ctx, nil, dto.DeleteDocumentInput{UUID: "del-uuid"})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !resp.Success {
			t.Error("expected Success=true")
		}
		if resp.UUID != "del-uuid" {
			t.Errorf("UUID = %q, want %q", resp.UUID, "del-uuid")
		}
	})

	t.Run("returns error when service fails", func(t *testing.T) {
		docSvc := &mockDocumentService{
			deleteFn: func(_ context.Context, _ string) error {
				return errors.New("forbidden")
			},
		}
		h := newHandlerWithMocks(struct {
			docSvc   *mockDocumentService
			zimRepo  *mockZimArchiveRepo
			gitRepo  *mockGitTemplateRepo
			kiwixC   *mockKiwixClient
			searcher *mockSearcher
		}{docSvc: docSvc})

		_, _, err := h.handleDeleteDocument(ctx, nil, dto.DeleteDocumentInput{UUID: "x"})
		if err == nil {
			t.Fatal("expected error")
		}
	})
}

func TestHandleSearchDocuments(t *testing.T) {
	ctx := ctxWithAdminUser()

	t.Run("returns not configured when searcher is nil", func(t *testing.T) {
		h := newHandlerWithMocks(struct {
			docSvc   *mockDocumentService
			zimRepo  *mockZimArchiveRepo
			gitRepo  *mockGitTemplateRepo
			kiwixC   *mockKiwixClient
			searcher *mockSearcher
		}{})
		h.searcher = nil // explicitly nil for this test

		_, resp, err := h.handleSearchDocuments(ctx, nil, dto.SearchDocumentsInput{Query: "test"})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if resp.Success {
			t.Error("expected Success=false")
		}
		if resp.Message != "Search service not configured" {
			t.Errorf("Message = %q", resp.Message)
		}
	})

	t.Run("returns error when searcher fails", func(t *testing.T) {
		s := &mockSearcher{
			searchFn: func(_ context.Context, _ search.SearchParams) (*search.SearchResponse, error) {
				return nil, errors.New("search error")
			},
		}
		h := newHandlerWithMocks(struct {
			docSvc   *mockDocumentService
			zimRepo  *mockZimArchiveRepo
			gitRepo  *mockGitTemplateRepo
			kiwixC   *mockKiwixClient
			searcher *mockSearcher
		}{searcher: s})

		_, _, err := h.handleSearchDocuments(ctx, nil, dto.SearchDocumentsInput{Query: "test"})
		if err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("clamps limit to defaults", func(t *testing.T) {
		var capturedLimit int64
		s := &mockSearcher{
			searchFn: func(_ context.Context, params search.SearchParams) (*search.SearchResponse, error) {
				capturedLimit = params.Limit
				return &search.SearchResponse{}, nil
			},
		}
		h := newHandlerWithMocks(struct {
			docSvc   *mockDocumentService
			zimRepo  *mockZimArchiveRepo
			gitRepo  *mockGitTemplateRepo
			kiwixC   *mockKiwixClient
			searcher *mockSearcher
		}{searcher: s})

		// Limit 0 defaults to 10
		_, _, _ = h.handleSearchDocuments(ctx, nil, dto.SearchDocumentsInput{Query: "x", Limit: 0})
		if capturedLimit != 10 {
			t.Errorf("default limit = %d, want 10", capturedLimit)
		}

		// Limit > 100 clamped to 100
		_, _, _ = h.handleSearchDocuments(ctx, nil, dto.SearchDocumentsInput{Query: "x", Limit: 200})
		if capturedLimit != 100 {
			t.Errorf("max limit = %d, want 100", capturedLimit)
		}
	})

	t.Run("builds structured filters from file_type and tags", func(t *testing.T) {
		var capturedParams search.SearchParams
		s := &mockSearcher{
			searchFn: func(_ context.Context, params search.SearchParams) (*search.SearchResponse, error) {
				capturedParams = params
				return &search.SearchResponse{}, nil
			},
		}
		h := newHandlerWithMocks(struct {
			docSvc   *mockDocumentService
			zimRepo  *mockZimArchiveRepo
			gitRepo  *mockGitTemplateRepo
			kiwixC   *mockKiwixClient
			searcher *mockSearcher
		}{searcher: s})

		_, _, _ = h.handleSearchDocuments(ctx, nil, dto.SearchDocumentsInput{
			Query:    "test",
			FileType: "markdown",
			Tags:     []string{"go", "testing"},
		})
		if capturedParams.FileType != "markdown" {
			t.Errorf("FileType = %q, want %q", capturedParams.FileType, "markdown")
		}
		if len(capturedParams.Tags) != 2 || capturedParams.Tags[0] != "go" || capturedParams.Tags[1] != "testing" {
			t.Errorf("Tags = %v, want [go testing]", capturedParams.Tags)
		}
	})
}

// ===== ZIM tool handler tests =====

func TestHandleListZimArchives(t *testing.T) {
	ctx := ctxWithAdminUser()

	t.Run("returns archives successfully", func(t *testing.T) {
		repo := &mockZimArchiveRepo{
			listFn: func(_ context.Context, _, _, _ string, _, _ int) ([]model.ZimArchive, error) {
				return []model.ZimArchive{
					{
						Name:         "devdocs_en_go",
						Title:        "Go Documentation",
						Language:     "en",
						ArticleCount: 1000,
						FileSize:     1024 * 1024,
						Description:  sql.NullString{String: "Go docs", Valid: true},
						Category:     sql.NullString{String: "devdocs", Valid: true},
					},
				}, nil
			},
		}
		h := newHandlerWithMocks(struct {
			docSvc   *mockDocumentService
			zimRepo  *mockZimArchiveRepo
			gitRepo  *mockGitTemplateRepo
			kiwixC   *mockKiwixClient
			searcher *mockSearcher
		}{zimRepo: repo})

		_, resp, err := h.handleListZimArchives(ctx, nil, dto.ListZimArchivesInput{})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !resp.Success {
			t.Error("expected Success=true")
		}
		if resp.Count != 1 {
			t.Errorf("Count = %d, want 1", resp.Count)
		}
		if resp.Archives[0].Name != "devdocs_en_go" {
			t.Errorf("Name = %q", resp.Archives[0].Name)
		}
		if resp.Archives[0].Description != "Go docs" {
			t.Errorf("Description = %q", resp.Archives[0].Description)
		}
		if resp.Archives[0].Category != "devdocs" {
			t.Errorf("Category = %q", resp.Archives[0].Category)
		}
	})

	t.Run("clamps limit", func(t *testing.T) {
		var capturedLimit int
		repo := &mockZimArchiveRepo{
			listFn: func(_ context.Context, _, _, _ string, limit, _ int) ([]model.ZimArchive, error) {
				capturedLimit = limit
				return nil, nil
			},
		}
		h := newHandlerWithMocks(struct {
			docSvc   *mockDocumentService
			zimRepo  *mockZimArchiveRepo
			gitRepo  *mockGitTemplateRepo
			kiwixC   *mockKiwixClient
			searcher *mockSearcher
		}{zimRepo: repo})

		_, _, _ = h.handleListZimArchives(ctx, nil, dto.ListZimArchivesInput{Limit: 0})
		if capturedLimit != 50 {
			t.Errorf("default limit = %d, want 50", capturedLimit)
		}

		_, _, _ = h.handleListZimArchives(ctx, nil, dto.ListZimArchivesInput{Limit: 150})
		if capturedLimit != 100 {
			t.Errorf("max limit = %d, want 100", capturedLimit)
		}
	})

	t.Run("returns error when repo fails", func(t *testing.T) {
		repo := &mockZimArchiveRepo{
			listFn: func(_ context.Context, _, _, _ string, _, _ int) ([]model.ZimArchive, error) {
				return nil, errors.New("db error")
			},
		}
		h := newHandlerWithMocks(struct {
			docSvc   *mockDocumentService
			zimRepo  *mockZimArchiveRepo
			gitRepo  *mockGitTemplateRepo
			kiwixC   *mockKiwixClient
			searcher *mockSearcher
		}{zimRepo: repo})

		_, _, err := h.handleListZimArchives(ctx, nil, dto.ListZimArchivesInput{})
		if err == nil {
			t.Fatal("expected error")
		}
	})
}

func TestHandleSearchZim(t *testing.T) {
	ctx := ctxWithAdminUser()

	t.Run("returns not configured when client is nil", func(t *testing.T) {
		h := newHandlerWithMocks(struct {
			docSvc   *mockDocumentService
			zimRepo  *mockZimArchiveRepo
			gitRepo  *mockGitTemplateRepo
			kiwixC   *mockKiwixClient
			searcher *mockSearcher
		}{})

		_, resp, err := h.handleSearchZim(ctx, nil, dto.SearchZimInput{Archive: "test", Query: "go"})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if resp.Success {
			t.Error("expected Success=false")
		}
		if resp.Message != "Kiwix service not configured" {
			t.Errorf("Message = %q", resp.Message)
		}
	})

	t.Run("returns search results", func(t *testing.T) {
		kc := &mockKiwixClient{
			searchFn: func(_ context.Context, archive, query, st string, limit int) ([]kiwix.SearchResult, error) {
				return []kiwix.SearchResult{
					{Title: "Result 1", Path: "/r1", Snippet: "Snippet 1", Score: 0.9},
				}, nil
			},
		}
		h := newHandlerWithMocks(struct {
			docSvc   *mockDocumentService
			zimRepo  *mockZimArchiveRepo
			gitRepo  *mockGitTemplateRepo
			kiwixC   *mockKiwixClient
			searcher *mockSearcher
		}{kiwixC: kc})

		_, resp, err := h.handleSearchZim(ctx, nil, dto.SearchZimInput{Archive: "devdocs", Query: "go"})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !resp.Success {
			t.Error("expected Success=true")
		}
		if resp.Count != 1 {
			t.Errorf("Count = %d, want 1", resp.Count)
		}
		if resp.SearchType != "fulltext" {
			t.Errorf("SearchType = %q, want %q", resp.SearchType, "fulltext")
		}
	})

	t.Run("defaults search type to fulltext", func(t *testing.T) {
		var capturedType string
		kc := &mockKiwixClient{
			searchFn: func(_ context.Context, _, _, st string, _ int) ([]kiwix.SearchResult, error) {
				capturedType = st
				return nil, nil
			},
		}
		h := newHandlerWithMocks(struct {
			docSvc   *mockDocumentService
			zimRepo  *mockZimArchiveRepo
			gitRepo  *mockGitTemplateRepo
			kiwixC   *mockKiwixClient
			searcher *mockSearcher
		}{kiwixC: kc})

		_, _, _ = h.handleSearchZim(ctx, nil, dto.SearchZimInput{Archive: "test", Query: "go", SearchType: ""})
		if capturedType != "fulltext" {
			t.Errorf("SearchType = %q, want %q", capturedType, "fulltext")
		}
	})

	t.Run("clamps limit", func(t *testing.T) {
		var capturedLimit int
		kc := &mockKiwixClient{
			searchFn: func(_ context.Context, _, _, _ string, limit int) ([]kiwix.SearchResult, error) {
				capturedLimit = limit
				return nil, nil
			},
		}
		h := newHandlerWithMocks(struct {
			docSvc   *mockDocumentService
			zimRepo  *mockZimArchiveRepo
			gitRepo  *mockGitTemplateRepo
			kiwixC   *mockKiwixClient
			searcher *mockSearcher
		}{kiwixC: kc})

		_, _, _ = h.handleSearchZim(ctx, nil, dto.SearchZimInput{Archive: "x", Query: "x", Limit: 0})
		if capturedLimit != 20 {
			t.Errorf("default limit = %d, want 20", capturedLimit)
		}

		_, _, _ = h.handleSearchZim(ctx, nil, dto.SearchZimInput{Archive: "x", Query: "x", Limit: 100})
		if capturedLimit != 50 {
			t.Errorf("max limit = %d, want 50", capturedLimit)
		}
	})

	t.Run("returns error when client fails", func(t *testing.T) {
		kc := &mockKiwixClient{
			searchFn: func(_ context.Context, _, _, _ string, _ int) ([]kiwix.SearchResult, error) {
				return nil, errors.New("kiwix down")
			},
		}
		h := newHandlerWithMocks(struct {
			docSvc   *mockDocumentService
			zimRepo  *mockZimArchiveRepo
			gitRepo  *mockGitTemplateRepo
			kiwixC   *mockKiwixClient
			searcher *mockSearcher
		}{kiwixC: kc})

		_, _, err := h.handleSearchZim(ctx, nil, dto.SearchZimInput{Archive: "x", Query: "x"})
		if err == nil {
			t.Fatal("expected error")
		}
	})
}

func TestHandleReadZimArticle(t *testing.T) {
	ctx := ctxWithAdminUser()

	t.Run("returns not configured when client is nil", func(t *testing.T) {
		h := newHandlerWithMocks(struct {
			docSvc   *mockDocumentService
			zimRepo  *mockZimArchiveRepo
			gitRepo  *mockGitTemplateRepo
			kiwixC   *mockKiwixClient
			searcher *mockSearcher
		}{})

		_, resp, err := h.handleReadZimArticle(ctx, nil, dto.ReadZimArticleInput{Archive: "test", Path: "/a"})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if resp.Success {
			t.Error("expected Success=false")
		}
	})

	t.Run("reads article successfully", func(t *testing.T) {
		kc := &mockKiwixClient{
			readArticleFn: func(_ context.Context, archive, path string) (*kiwix.Article, error) {
				return &kiwix.Article{
					Title:   "Go Tutorial",
					Content: "Intro.\n\n# Getting Started\n\nStep 1.",
				}, nil
			},
		}
		h := newHandlerWithMocks(struct {
			docSvc   *mockDocumentService
			zimRepo  *mockZimArchiveRepo
			gitRepo  *mockGitTemplateRepo
			kiwixC   *mockKiwixClient
			searcher *mockSearcher
		}{kiwixC: kc})

		_, resp, err := h.handleReadZimArticle(ctx, nil, dto.ReadZimArticleInput{Archive: "dev", Path: "/go"})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !resp.Success {
			t.Error("expected Success=true")
		}
		if resp.Title != "Go Tutorial" {
			t.Errorf("Title = %q", resp.Title)
		}
	})

	t.Run("applies summary_only truncation", func(t *testing.T) {
		kc := &mockKiwixClient{
			readArticleFn: func(_ context.Context, _, _ string) (*kiwix.Article, error) {
				return &kiwix.Article{
					Title:   "Art",
					Content: "Summary.\n\n# Details\n\nMore.",
				}, nil
			},
		}
		h := newHandlerWithMocks(struct {
			docSvc   *mockDocumentService
			zimRepo  *mockZimArchiveRepo
			gitRepo  *mockGitTemplateRepo
			kiwixC   *mockKiwixClient
			searcher *mockSearcher
		}{kiwixC: kc})

		_, resp, err := h.handleReadZimArticle(ctx, nil, dto.ReadZimArticleInput{
			Archive:     "x",
			Path:        "/a",
			SummaryOnly: true,
		})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !resp.Truncated {
			t.Error("expected Truncated=true")
		}
		if resp.Content != "Summary." {
			t.Errorf("Content = %q, want %q", resp.Content, "Summary.")
		}
	})

	t.Run("returns error when client fails", func(t *testing.T) {
		kc := &mockKiwixClient{
			readArticleFn: func(_ context.Context, _, _ string) (*kiwix.Article, error) {
				return nil, errors.New("not found")
			},
		}
		h := newHandlerWithMocks(struct {
			docSvc   *mockDocumentService
			zimRepo  *mockZimArchiveRepo
			gitRepo  *mockGitTemplateRepo
			kiwixC   *mockKiwixClient
			searcher *mockSearcher
		}{kiwixC: kc})

		_, _, err := h.handleReadZimArticle(ctx, nil, dto.ReadZimArticleInput{Archive: "x", Path: "/a"})
		if err == nil {
			t.Fatal("expected error")
		}
	})
}

// ===== Git template tool handler tests =====

func TestHandleListGitTemplates(t *testing.T) {
	ctx := ctxWithAdminUser()

	t.Run("returns templates successfully", func(t *testing.T) {
		repo := &mockGitTemplateRepo{
			listFn: func(_ context.Context, _ string, _, _ int) ([]model.GitTemplate, error) {
				return []model.GitTemplate{
					{
						UUID:           "t1",
						Name:           "Claude Template",
						Description:    sql.NullString{String: "CLAUDE.md setup", Valid: true},
						Category:       sql.NullString{String: "claude", Valid: true},
						Tags:           sql.NullString{String: `["ai","claude"]`, Valid: true},
						FileCount:      5,
						TotalSizeBytes: 1024,
						Status:         "synced",
					},
				}, nil
			},
		}
		h := newHandlerWithMocks(struct {
			docSvc   *mockDocumentService
			zimRepo  *mockZimArchiveRepo
			gitRepo  *mockGitTemplateRepo
			kiwixC   *mockKiwixClient
			searcher *mockSearcher
		}{gitRepo: repo})

		_, resp, err := h.handleListGitTemplates(ctx, nil, dto.ListGitTemplatesInput{})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !resp.Success {
			t.Error("expected Success=true")
		}
		if resp.Count != 1 {
			t.Errorf("Count = %d, want 1", resp.Count)
		}
		if resp.Templates[0].UUID != "t1" {
			t.Errorf("UUID = %q", resp.Templates[0].UUID)
		}
		if resp.Templates[0].Description != "CLAUDE.md setup" {
			t.Errorf("Description = %q", resp.Templates[0].Description)
		}
		if resp.Templates[0].Category != "claude" {
			t.Errorf("Category = %q", resp.Templates[0].Category)
		}
		if len(resp.Templates[0].Tags) != 2 {
			t.Errorf("Tags count = %d, want 2", len(resp.Templates[0].Tags))
		}
	})

	t.Run("clamps limit", func(t *testing.T) {
		var capturedLimit int
		repo := &mockGitTemplateRepo{
			listFn: func(_ context.Context, _ string, limit, _ int) ([]model.GitTemplate, error) {
				capturedLimit = limit
				return nil, nil
			},
		}
		h := newHandlerWithMocks(struct {
			docSvc   *mockDocumentService
			zimRepo  *mockZimArchiveRepo
			gitRepo  *mockGitTemplateRepo
			kiwixC   *mockKiwixClient
			searcher *mockSearcher
		}{gitRepo: repo})

		_, _, _ = h.handleListGitTemplates(ctx, nil, dto.ListGitTemplatesInput{Limit: 0})
		if capturedLimit != 50 {
			t.Errorf("default limit = %d, want 50", capturedLimit)
		}

		_, _, _ = h.handleListGitTemplates(ctx, nil, dto.ListGitTemplatesInput{Limit: 200})
		if capturedLimit != 100 {
			t.Errorf("max limit = %d, want 100", capturedLimit)
		}
	})

	t.Run("returns error when repo fails", func(t *testing.T) {
		repo := &mockGitTemplateRepo{
			listFn: func(_ context.Context, _ string, _, _ int) ([]model.GitTemplate, error) {
				return nil, errors.New("db error")
			},
		}
		h := newHandlerWithMocks(struct {
			docSvc   *mockDocumentService
			zimRepo  *mockZimArchiveRepo
			gitRepo  *mockGitTemplateRepo
			kiwixC   *mockKiwixClient
			searcher *mockSearcher
		}{gitRepo: repo})

		_, _, err := h.handleListGitTemplates(ctx, nil, dto.ListGitTemplatesInput{})
		if err == nil {
			t.Fatal("expected error")
		}
	})
}

func TestHandleSearchGitTemplates(t *testing.T) {
	ctx := ctxWithAdminUser()

	t.Run("returns results from searcher", func(t *testing.T) {
		s := &mockSearcher{
			searchFn: func(_ context.Context, params search.SearchParams) (*search.SearchResponse, error) {
				return &search.SearchResponse{
					Hits: []search.SearchResult{
						{UUID: "t1", Title: "Found Template", Description: "desc", Source: "git_templates"},
					},
					EstimatedTotal: 1,
				}, nil
			},
		}
		repo := &mockGitTemplateRepo{}
		h := newHandlerWithMocks(struct {
			docSvc   *mockDocumentService
			zimRepo  *mockZimArchiveRepo
			gitRepo  *mockGitTemplateRepo
			kiwixC   *mockKiwixClient
			searcher *mockSearcher
		}{gitRepo: repo, searcher: s})

		_, resp, err := h.handleSearchGitTemplates(ctx, nil, dto.SearchGitTemplatesInput{Query: "claude"})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !resp.Success {
			t.Error("expected Success=true")
		}
		if resp.Total != 1 {
			t.Errorf("Total = %d, want 1", resp.Total)
		}
	})

	t.Run("clamps limit", func(t *testing.T) {
		var capturedLimit int64
		s := &mockSearcher{
			searchFn: func(_ context.Context, params search.SearchParams) (*search.SearchResponse, error) {
				capturedLimit = params.Limit
				return &search.SearchResponse{}, nil
			},
		}
		h := newHandlerWithMocks(struct {
			docSvc   *mockDocumentService
			zimRepo  *mockZimArchiveRepo
			gitRepo  *mockGitTemplateRepo
			kiwixC   *mockKiwixClient
			searcher *mockSearcher
		}{searcher: s})

		_, _, _ = h.handleSearchGitTemplates(ctx, nil, dto.SearchGitTemplatesInput{Query: "x", Limit: 0})
		if capturedLimit != 10 {
			t.Errorf("default limit = %d, want 10", capturedLimit)
		}

		_, _, _ = h.handleSearchGitTemplates(ctx, nil, dto.SearchGitTemplatesInput{Query: "x", Limit: 100})
		if capturedLimit != 50 {
			t.Errorf("max limit = %d, want 50", capturedLimit)
		}
	})

	t.Run("returns error when search fails", func(t *testing.T) {
		s := &mockSearcher{
			searchFn: func(_ context.Context, _ search.SearchParams) (*search.SearchResponse, error) {
				return nil, errors.New("search error")
			},
		}
		h := newHandlerWithMocks(struct {
			docSvc   *mockDocumentService
			zimRepo  *mockZimArchiveRepo
			gitRepo  *mockGitTemplateRepo
			kiwixC   *mockKiwixClient
			searcher *mockSearcher
		}{searcher: s})

		_, _, err := h.handleSearchGitTemplates(ctx, nil, dto.SearchGitTemplatesInput{Query: "x"})
		if err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("returns error when meilisearch search fails", func(t *testing.T) {
		s := &mockSearcher{
			searchFn: func(_ context.Context, _ search.SearchParams) (*search.SearchResponse, error) {
				return nil, errors.New("search error")
			},
		}
		repo := &mockGitTemplateRepo{}
		h := newHandlerWithMocks(struct {
			docSvc   *mockDocumentService
			zimRepo  *mockZimArchiveRepo
			gitRepo  *mockGitTemplateRepo
			kiwixC   *mockKiwixClient
			searcher *mockSearcher
		}{gitRepo: repo, searcher: s})

		_, _, err := h.handleSearchGitTemplates(ctx, nil, dto.SearchGitTemplatesInput{Query: "x"})
		if err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("empty description and category in search results", func(t *testing.T) {
		s := &mockSearcher{
			searchFn: func(_ context.Context, _ search.SearchParams) (*search.SearchResponse, error) {
				return &search.SearchResponse{
					Hits: []search.SearchResult{
						{UUID: "t2", Title: "No Desc Template", Source: "git_templates"},
					},
					EstimatedTotal: 1,
				}, nil
			},
		}
		h := newHandlerWithMocks(struct {
			docSvc   *mockDocumentService
			zimRepo  *mockZimArchiveRepo
			gitRepo  *mockGitTemplateRepo
			kiwixC   *mockKiwixClient
			searcher *mockSearcher
		}{searcher: s})

		_, resp, err := h.handleSearchGitTemplates(ctx, nil, dto.SearchGitTemplatesInput{Query: "x"})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if resp.Total != 1 {
			t.Errorf("Total = %d, want 1", resp.Total)
		}
		if resp.Results[0].Description != "" {
			t.Errorf("Description = %q, want empty", resp.Results[0].Description)
		}
		if resp.Results[0].Category != "" {
			t.Errorf("Category = %q, want empty", resp.Results[0].Category)
		}
	})

	t.Run("meilisearch path builds results from hits", func(t *testing.T) {
		s := &mockSearcher{
			searchFn: func(_ context.Context, _ search.SearchParams) (*search.SearchResponse, error) {
				hit := makeMCPHit(map[string]any{
					"uuid":        "uuid-ms-1",
					"name":        "Meilisearch Template",
					"description": "A great template",
					"category":    "backend",
					"file_count":  float64(7),
					"status":      "synced",
				})
				return &search.SearchResponse{
					Hits:               []search.SearchResult{hit},
					EstimatedTotal: 1,
				}, nil
			},
		}
		repo := &mockGitTemplateRepo{}
		h := newHandlerWithMocks(struct {
			docSvc   *mockDocumentService
			zimRepo  *mockZimArchiveRepo
			gitRepo  *mockGitTemplateRepo
			kiwixC   *mockKiwixClient
			searcher *mockSearcher
		}{gitRepo: repo, searcher: s})

		_, resp, err := h.handleSearchGitTemplates(ctx, nil, dto.SearchGitTemplatesInput{Query: "backend"})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !resp.Success {
			t.Error("expected Success=true")
		}
		if resp.Total != 1 {
			t.Errorf("Total = %d, want 1", resp.Total)
		}
		r := resp.Results[0]
		if r.UUID != "uuid-ms-1" {
			t.Errorf("UUID = %q, want uuid-ms-1", r.UUID)
		}
		if r.Description != "A great template" {
			t.Errorf("Description = %q, want 'A great template'", r.Description)
		}
		if r.Category != "backend" {
			t.Errorf("Category = %q, want 'backend'", r.Category)
		}
		if r.FileCount != 7 {
			t.Errorf("FileCount = %d, want 7", r.FileCount)
		}
	})

	t.Run("meilisearch path clamps limit", func(t *testing.T) {
		var capturedLimit int64
		s := &mockSearcher{
			searchFn: func(_ context.Context, params search.SearchParams) (*search.SearchResponse, error) {
				capturedLimit = params.Limit
				return &search.SearchResponse{}, nil
			},
		}
		repo := &mockGitTemplateRepo{}
		h := newHandlerWithMocks(struct {
			docSvc   *mockDocumentService
			zimRepo  *mockZimArchiveRepo
			gitRepo  *mockGitTemplateRepo
			kiwixC   *mockKiwixClient
			searcher *mockSearcher
		}{gitRepo: repo, searcher: s})

		_, _, _ = h.handleSearchGitTemplates(ctx, nil, dto.SearchGitTemplatesInput{Query: "x", Limit: 0})
		if capturedLimit != 10 {
			t.Errorf("default limit = %d, want 10", capturedLimit)
		}

		_, _, _ = h.handleSearchGitTemplates(ctx, nil, dto.SearchGitTemplatesInput{Query: "x", Limit: 100})
		if capturedLimit != 50 {
			t.Errorf("max limit = %d, want 50", capturedLimit)
		}
	})

	t.Run("search path adds category filter when provided", func(t *testing.T) {
		var capturedParams search.SearchParams
		s := &mockSearcher{
			searchFn: func(_ context.Context, params search.SearchParams) (*search.SearchResponse, error) {
				capturedParams = params
				return &search.SearchResponse{}, nil
			},
		}
		repo := &mockGitTemplateRepo{}
		h := newHandlerWithMocks(struct {
			docSvc   *mockDocumentService
			zimRepo  *mockZimArchiveRepo
			gitRepo  *mockGitTemplateRepo
			kiwixC   *mockKiwixClient
			searcher *mockSearcher
		}{gitRepo: repo, searcher: s})

		_, _, _ = h.handleSearchGitTemplates(ctx, nil, dto.SearchGitTemplatesInput{Query: "x", Category: "backend"})
		if capturedParams.Category != "backend" {
			t.Errorf("Category = %q, want %q", capturedParams.Category, "backend")
		}
	})
}

func TestHandleGetTemplateStructure(t *testing.T) {
	ctx := ctxWithAdminUser()

	t.Run("returns template structure with variables", func(t *testing.T) {
		repo := &mockGitTemplateRepo{
			findByUUIDFn: func(_ context.Context, uuid string) (*model.GitTemplate, error) {
				return &model.GitTemplate{
					ID:             1,
					UUID:           uuid,
					Name:           "Test Template",
					Description:    sql.NullString{String: "A template", Valid: true},
					Category:       sql.NullString{String: "claude", Valid: true},
					FileCount:      3,
					TotalSizeBytes: 2048,
				}, nil
			},
			filesForTmplFn: func(_ context.Context, _ int64) ([]model.GitTemplateFile, error) {
				return []model.GitTemplateFile{
					{Path: "CLAUDE.md", IsEssential: true, Content: sql.NullString{String: "Project: {{project_name}}", Valid: true}},
					{Path: "README.md", IsEssential: false, Content: sql.NullString{String: "Hello", Valid: true}},
					{Path: "memory-bank/context.md", IsEssential: true, Content: sql.NullString{String: "{{author}}", Valid: true}},
				}, nil
			},
		}
		h := newHandlerWithMocks(struct {
			docSvc   *mockDocumentService
			zimRepo  *mockZimArchiveRepo
			gitRepo  *mockGitTemplateRepo
			kiwixC   *mockKiwixClient
			searcher *mockSearcher
		}{gitRepo: repo})

		_, resp, err := h.handleGetTemplateStructure(ctx, nil, dto.GetTemplateStructureInput{UUID: "t1"})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !resp.Success {
			t.Error("expected Success=true")
		}
		if resp.Template == nil {
			t.Fatal("Template is nil")
		}
		if len(resp.Template.FileTree) != 3 {
			t.Errorf("FileTree count = %d, want 3", len(resp.Template.FileTree))
		}
		if len(resp.Template.EssentialFiles) != 2 {
			t.Errorf("EssentialFiles count = %d, want 2", len(resp.Template.EssentialFiles))
		}
		if len(resp.Template.Variables) != 2 {
			t.Errorf("Variables count = %d, want 2 (project_name, author)", len(resp.Template.Variables))
		}
	})

	t.Run("returns not found when template missing", func(t *testing.T) {
		repo := &mockGitTemplateRepo{
			findByUUIDFn: func(_ context.Context, _ string) (*model.GitTemplate, error) {
				return nil, errors.New("not found")
			},
		}
		h := newHandlerWithMocks(struct {
			docSvc   *mockDocumentService
			zimRepo  *mockZimArchiveRepo
			gitRepo  *mockGitTemplateRepo
			kiwixC   *mockKiwixClient
			searcher *mockSearcher
		}{gitRepo: repo})

		_, resp, err := h.handleGetTemplateStructure(ctx, nil, dto.GetTemplateStructureInput{UUID: "missing"})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if resp.Success {
			t.Error("expected Success=false")
		}
	})

	t.Run("returns error when files query fails", func(t *testing.T) {
		repo := &mockGitTemplateRepo{
			findByUUIDFn: func(_ context.Context, _ string) (*model.GitTemplate, error) {
				return &model.GitTemplate{ID: 1, UUID: "t1", Name: "T"}, nil
			},
			filesForTmplFn: func(_ context.Context, _ int64) ([]model.GitTemplateFile, error) {
				return nil, errors.New("db error")
			},
		}
		h := newHandlerWithMocks(struct {
			docSvc   *mockDocumentService
			zimRepo  *mockZimArchiveRepo
			gitRepo  *mockGitTemplateRepo
			kiwixC   *mockKiwixClient
			searcher *mockSearcher
		}{gitRepo: repo})

		_, _, err := h.handleGetTemplateStructure(ctx, nil, dto.GetTemplateStructureInput{UUID: "t1"})
		if err == nil {
			t.Fatal("expected error")
		}
	})
}

func TestHandleGetTemplateFile(t *testing.T) {
	ctx := ctxWithAdminUser()

	t.Run("returns file with variable substitution", func(t *testing.T) {
		repo := &mockGitTemplateRepo{
			findByUUIDFn: func(_ context.Context, _ string) (*model.GitTemplate, error) {
				return &model.GitTemplate{ID: 1, UUID: "t1", Name: "T"}, nil
			},
			findFileByPathFn: func(_ context.Context, _ int64, _ string) (*model.GitTemplateFile, error) {
				return &model.GitTemplateFile{
					Path:        "CLAUDE.md",
					Filename:    "CLAUDE.md",
					SizeBytes:   100,
					IsEssential: true,
					Content:     sql.NullString{String: "Project: {{project_name}}", Valid: true},
				}, nil
			},
		}
		h := newHandlerWithMocks(struct {
			docSvc   *mockDocumentService
			zimRepo  *mockZimArchiveRepo
			gitRepo  *mockGitTemplateRepo
			kiwixC   *mockKiwixClient
			searcher *mockSearcher
		}{gitRepo: repo})

		_, resp, err := h.handleGetTemplateFile(ctx, nil, dto.GetTemplateFileInput{
			UUID:      "t1",
			Path:      "CLAUDE.md",
			Variables: `{"project_name":"DocuMCP"}`,
		})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !resp.Success {
			t.Error("expected Success=true")
		}
		if resp.Content != "Project: DocuMCP" {
			t.Errorf("Content = %q, want %q", resp.Content, "Project: DocuMCP")
		}
		if len(resp.UnresolvedVariables) != 0 {
			t.Errorf("UnresolvedVariables = %v, want empty", resp.UnresolvedVariables)
		}
	})

	t.Run("returns unresolved variables when not all provided", func(t *testing.T) {
		repo := &mockGitTemplateRepo{
			findByUUIDFn: func(_ context.Context, _ string) (*model.GitTemplate, error) {
				return &model.GitTemplate{ID: 1, UUID: "t1"}, nil
			},
			findFileByPathFn: func(_ context.Context, _ int64, _ string) (*model.GitTemplateFile, error) {
				return &model.GitTemplateFile{
					Path:     "f.md",
					Filename: "f.md",
					Content:  sql.NullString{String: "{{name}} by {{author}}", Valid: true},
				}, nil
			},
		}
		h := newHandlerWithMocks(struct {
			docSvc   *mockDocumentService
			zimRepo  *mockZimArchiveRepo
			gitRepo  *mockGitTemplateRepo
			kiwixC   *mockKiwixClient
			searcher *mockSearcher
		}{gitRepo: repo})

		_, resp, err := h.handleGetTemplateFile(ctx, nil, dto.GetTemplateFileInput{
			UUID:      "t1",
			Path:      "f.md",
			Variables: `{"name":"DocuMCP"}`,
		})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(resp.UnresolvedVariables) != 1 || resp.UnresolvedVariables[0] != "author" {
			t.Errorf("UnresolvedVariables = %v, want [author]", resp.UnresolvedVariables)
		}
	})

	t.Run("returns error for invalid variables JSON", func(t *testing.T) {
		repo := &mockGitTemplateRepo{
			findByUUIDFn: func(_ context.Context, _ string) (*model.GitTemplate, error) {
				return &model.GitTemplate{ID: 1, UUID: "t1"}, nil
			},
			findFileByPathFn: func(_ context.Context, _ int64, _ string) (*model.GitTemplateFile, error) {
				return &model.GitTemplateFile{
					Path:     "f.md",
					Filename: "f.md",
					Content:  sql.NullString{String: "text", Valid: true},
				}, nil
			},
		}
		h := newHandlerWithMocks(struct {
			docSvc   *mockDocumentService
			zimRepo  *mockZimArchiveRepo
			gitRepo  *mockGitTemplateRepo
			kiwixC   *mockKiwixClient
			searcher *mockSearcher
		}{gitRepo: repo})

		_, _, err := h.handleGetTemplateFile(ctx, nil, dto.GetTemplateFileInput{
			UUID:      "t1",
			Path:      "f.md",
			Variables: `{invalid}`,
		})
		if err == nil {
			t.Fatal("expected error for invalid JSON")
		}
	})

	t.Run("returns not found when template missing", func(t *testing.T) {
		repo := &mockGitTemplateRepo{
			findByUUIDFn: func(_ context.Context, _ string) (*model.GitTemplate, error) {
				return nil, errors.New("not found")
			},
		}
		h := newHandlerWithMocks(struct {
			docSvc   *mockDocumentService
			zimRepo  *mockZimArchiveRepo
			gitRepo  *mockGitTemplateRepo
			kiwixC   *mockKiwixClient
			searcher *mockSearcher
		}{gitRepo: repo})

		_, resp, err := h.handleGetTemplateFile(ctx, nil, dto.GetTemplateFileInput{UUID: "missing", Path: "x"})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if resp.Success {
			t.Error("expected Success=false")
		}
	})

	t.Run("returns not found when file missing", func(t *testing.T) {
		repo := &mockGitTemplateRepo{
			findByUUIDFn: func(_ context.Context, _ string) (*model.GitTemplate, error) {
				return &model.GitTemplate{ID: 1, UUID: "t1"}, nil
			},
			findFileByPathFn: func(_ context.Context, _ int64, _ string) (*model.GitTemplateFile, error) {
				return nil, errors.New("not found")
			},
		}
		h := newHandlerWithMocks(struct {
			docSvc   *mockDocumentService
			zimRepo  *mockZimArchiveRepo
			gitRepo  *mockGitTemplateRepo
			kiwixC   *mockKiwixClient
			searcher *mockSearcher
		}{gitRepo: repo})

		_, resp, err := h.handleGetTemplateFile(ctx, nil, dto.GetTemplateFileInput{UUID: "t1", Path: "missing.md"})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if resp.Success {
			t.Error("expected Success=false")
		}
	})
}

func TestHandleGetDeploymentGuide(t *testing.T) {
	ctx := ctxWithAdminUser()

	t.Run("returns deployment guide with essential files only", func(t *testing.T) {
		repo := &mockGitTemplateRepo{
			findByUUIDFn: func(_ context.Context, _ string) (*model.GitTemplate, error) {
				return &model.GitTemplate{
					ID:          1,
					UUID:        "t1",
					Name:        "My Template",
					Description: sql.NullString{String: "A template", Valid: true},
				}, nil
			},
			filesForTmplFn: func(_ context.Context, _ int64) ([]model.GitTemplateFile, error) {
				return []model.GitTemplateFile{
					{Path: "CLAUDE.md", IsEssential: true, Content: sql.NullString{String: "# {{project_name}}", Valid: true}},
					{Path: "README.md", IsEssential: false, Content: sql.NullString{String: "Readme", Valid: true}},
					{Path: "memory-bank/ctx.md", IsEssential: true, Content: sql.NullString{String: "Context for {{project_name}}", Valid: true}},
				}, nil
			},
		}
		h := newHandlerWithMocks(struct {
			docSvc   *mockDocumentService
			zimRepo  *mockZimArchiveRepo
			gitRepo  *mockGitTemplateRepo
			kiwixC   *mockKiwixClient
			searcher *mockSearcher
		}{gitRepo: repo})

		_, resp, err := h.handleGetDeploymentGuide(ctx, nil, dto.GetDeploymentGuideInput{
			UUID:      "t1",
			Variables: `{"project_name":"DocuMCP"}`,
		})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !resp.Success {
			t.Error("expected Success=true")
		}
		if resp.Guide == nil {
			t.Fatal("Guide is nil")
		}
		if len(resp.Guide.Files) != 2 {
			t.Errorf("Files count = %d, want 2 (essential only)", len(resp.Guide.Files))
		}
		if resp.Guide.Files[0].Content != "# DocuMCP" {
			t.Errorf("File[0] Content = %q", resp.Guide.Files[0].Content)
		}
		if len(resp.Guide.UnresolvedVariables) != 0 {
			t.Errorf("UnresolvedVariables = %v, want empty", resp.Guide.UnresolvedVariables)
		}
	})

	t.Run("returns not found when template missing", func(t *testing.T) {
		repo := &mockGitTemplateRepo{
			findByUUIDFn: func(_ context.Context, _ string) (*model.GitTemplate, error) {
				return nil, errors.New("not found")
			},
		}
		h := newHandlerWithMocks(struct {
			docSvc   *mockDocumentService
			zimRepo  *mockZimArchiveRepo
			gitRepo  *mockGitTemplateRepo
			kiwixC   *mockKiwixClient
			searcher *mockSearcher
		}{gitRepo: repo})

		_, resp, err := h.handleGetDeploymentGuide(ctx, nil, dto.GetDeploymentGuideInput{UUID: "missing"})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if resp.Success {
			t.Error("expected Success=false")
		}
	})

	t.Run("returns error for invalid variables JSON", func(t *testing.T) {
		repo := &mockGitTemplateRepo{
			findByUUIDFn: func(_ context.Context, _ string) (*model.GitTemplate, error) {
				return &model.GitTemplate{ID: 1, UUID: "t1", Name: "T"}, nil
			},
			filesForTmplFn: func(_ context.Context, _ int64) ([]model.GitTemplateFile, error) {
				return nil, nil
			},
		}
		h := newHandlerWithMocks(struct {
			docSvc   *mockDocumentService
			zimRepo  *mockZimArchiveRepo
			gitRepo  *mockGitTemplateRepo
			kiwixC   *mockKiwixClient
			searcher *mockSearcher
		}{gitRepo: repo})

		_, _, err := h.handleGetDeploymentGuide(ctx, nil, dto.GetDeploymentGuideInput{
			UUID:      "t1",
			Variables: `{bad}`,
		})
		if err == nil {
			t.Fatal("expected error for invalid JSON")
		}
	})
}

func TestHandleDownloadTemplate(t *testing.T) {
	ctx := ctxWithAdminUser()

	t.Run("returns zip archive by default", func(t *testing.T) {
		repo := &mockGitTemplateRepo{
			findByUUIDFn: func(_ context.Context, _ string) (*model.GitTemplate, error) {
				return &model.GitTemplate{ID: 1, UUID: "t1", Name: "T", Slug: "my-template"}, nil
			},
			filesForTmplFn: func(_ context.Context, _ int64) ([]model.GitTemplateFile, error) {
				return []model.GitTemplateFile{
					{Path: "file.md", Content: sql.NullString{String: "Hello", Valid: true}},
				}, nil
			},
		}
		h := newHandlerWithMocks(struct {
			docSvc   *mockDocumentService
			zimRepo  *mockZimArchiveRepo
			gitRepo  *mockGitTemplateRepo
			kiwixC   *mockKiwixClient
			searcher *mockSearcher
		}{gitRepo: repo})

		_, resp, err := h.handleDownloadTemplate(ctx, nil, dto.DownloadTemplateInput{UUID: "t1"})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !resp.Success {
			t.Error("expected Success=true")
		}
		if resp.Format != "zip" {
			t.Errorf("Format = %q, want %q", resp.Format, "zip")
		}
		if resp.Filename != "my-template.zip" {
			t.Errorf("Filename = %q, want %q", resp.Filename, "my-template.zip")
		}
		if resp.FileCount != 1 {
			t.Errorf("FileCount = %d, want 1", resp.FileCount)
		}
		if resp.ArchiveBase64 == "" {
			t.Error("ArchiveBase64 is empty")
		}
		if resp.SizeBytes == 0 {
			t.Error("SizeBytes is 0")
		}
	})

	t.Run("returns tar.gz archive when requested", func(t *testing.T) {
		repo := &mockGitTemplateRepo{
			findByUUIDFn: func(_ context.Context, _ string) (*model.GitTemplate, error) {
				return &model.GitTemplate{ID: 1, UUID: "t1", Name: "T", Slug: "tmpl"}, nil
			},
			filesForTmplFn: func(_ context.Context, _ int64) ([]model.GitTemplateFile, error) {
				return []model.GitTemplateFile{
					{Path: "f.md", Content: sql.NullString{String: "Hi", Valid: true}},
				}, nil
			},
		}
		h := newHandlerWithMocks(struct {
			docSvc   *mockDocumentService
			zimRepo  *mockZimArchiveRepo
			gitRepo  *mockGitTemplateRepo
			kiwixC   *mockKiwixClient
			searcher *mockSearcher
		}{gitRepo: repo})

		_, resp, err := h.handleDownloadTemplate(ctx, nil, dto.DownloadTemplateInput{UUID: "t1", Format: "tar.gz"})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if resp.Format != "tar.gz" {
			t.Errorf("Format = %q, want %q", resp.Format, "tar.gz")
		}
		if resp.Filename != "tmpl.tar.gz" {
			t.Errorf("Filename = %q, want %q", resp.Filename, "tmpl.tar.gz")
		}
	})

	t.Run("applies variable substitution", func(t *testing.T) {
		repo := &mockGitTemplateRepo{
			findByUUIDFn: func(_ context.Context, _ string) (*model.GitTemplate, error) {
				return &model.GitTemplate{ID: 1, UUID: "t1", Name: "T", Slug: "s"}, nil
			},
			filesForTmplFn: func(_ context.Context, _ int64) ([]model.GitTemplateFile, error) {
				return []model.GitTemplateFile{
					{Path: "f.md", Content: sql.NullString{String: "Hello {{name}}, by {{author}}", Valid: true}},
				}, nil
			},
		}
		h := newHandlerWithMocks(struct {
			docSvc   *mockDocumentService
			zimRepo  *mockZimArchiveRepo
			gitRepo  *mockGitTemplateRepo
			kiwixC   *mockKiwixClient
			searcher *mockSearcher
		}{gitRepo: repo})

		_, resp, err := h.handleDownloadTemplate(ctx, nil, dto.DownloadTemplateInput{
			UUID:      "t1",
			Variables: `{"name":"World"}`,
		})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(resp.UnresolvedVariables) != 1 || resp.UnresolvedVariables[0] != "author" {
			t.Errorf("UnresolvedVariables = %v, want [author]", resp.UnresolvedVariables)
		}
	})

	t.Run("returns not found when template missing", func(t *testing.T) {
		repo := &mockGitTemplateRepo{
			findByUUIDFn: func(_ context.Context, _ string) (*model.GitTemplate, error) {
				return nil, errors.New("not found")
			},
		}
		h := newHandlerWithMocks(struct {
			docSvc   *mockDocumentService
			zimRepo  *mockZimArchiveRepo
			gitRepo  *mockGitTemplateRepo
			kiwixC   *mockKiwixClient
			searcher *mockSearcher
		}{gitRepo: repo})

		_, resp, err := h.handleDownloadTemplate(ctx, nil, dto.DownloadTemplateInput{UUID: "missing"})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if resp.Success {
			t.Error("expected Success=false")
		}
	})
}

// ===== Unified search handler tests =====

func TestHandleUnifiedSearch(t *testing.T) {
	ctx := ctxWithAdminUser()

	t.Run("returns not configured when searcher is nil", func(t *testing.T) {
		h := newHandlerWithMocks(struct {
			docSvc   *mockDocumentService
			zimRepo  *mockZimArchiveRepo
			gitRepo  *mockGitTemplateRepo
			kiwixC   *mockKiwixClient
			searcher *mockSearcher
		}{})
		h.searcher = nil // explicitly nil for this test

		_, resp, err := h.handleUnifiedSearch(ctx, nil, dto.UnifiedSearchInput{Query: "test"})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if resp.Success {
			t.Error("expected Success=false")
		}
		if resp.Message != "Search service not configured" {
			t.Errorf("Message = %q", resp.Message)
		}
	})

	t.Run("returns error when federated search fails", func(t *testing.T) {
		s := &mockSearcher{
			federatedSearchFn: func(_ context.Context, _ search.FederatedSearchParams) (*search.FederatedSearchResponse, error) {
				return nil, errors.New("federation error")
			},
		}
		h := newHandlerWithMocks(struct {
			docSvc   *mockDocumentService
			zimRepo  *mockZimArchiveRepo
			gitRepo  *mockGitTemplateRepo
			kiwixC   *mockKiwixClient
			searcher *mockSearcher
		}{searcher: s})

		_, _, err := h.handleUnifiedSearch(ctx, nil, dto.UnifiedSearchInput{Query: "test"})
		if err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("clamps limit", func(t *testing.T) {
		var capturedLimit int64
		s := &mockSearcher{
			federatedSearchFn: func(_ context.Context, params search.FederatedSearchParams) (*search.FederatedSearchResponse, error) {
				capturedLimit = params.Limit
				return &search.FederatedSearchResponse{}, nil
			},
		}
		h := newHandlerWithMocks(struct {
			docSvc   *mockDocumentService
			zimRepo  *mockZimArchiveRepo
			gitRepo  *mockGitTemplateRepo
			kiwixC   *mockKiwixClient
			searcher *mockSearcher
		}{searcher: s})

		_, _, _ = h.handleUnifiedSearch(ctx, nil, dto.UnifiedSearchInput{Query: "x", Limit: 0})
		if capturedLimit != 20 {
			t.Errorf("default limit = %d, want 20", capturedLimit)
		}

		_, _, _ = h.handleUnifiedSearch(ctx, nil, dto.UnifiedSearchInput{Query: "x", Limit: 200})
		if capturedLimit != 100 {
			t.Errorf("max limit = %d, want 100", capturedLimit)
		}
	})

	t.Run("maps type names to index UIDs", func(t *testing.T) {
		var capturedIndexes []string
		s := &mockSearcher{
			federatedSearchFn: func(_ context.Context, params search.FederatedSearchParams) (*search.FederatedSearchResponse, error) {
				capturedIndexes = params.Indexes
				return &search.FederatedSearchResponse{}, nil
			},
		}
		h := newHandlerWithMocks(struct {
			docSvc   *mockDocumentService
			zimRepo  *mockZimArchiveRepo
			gitRepo  *mockGitTemplateRepo
			kiwixC   *mockKiwixClient
			searcher *mockSearcher
		}{searcher: s})

		_, _, _ = h.handleUnifiedSearch(ctx, nil, dto.UnifiedSearchInput{
			Query: "x",
			Types: []string{"document", "git_template"},
		})
		if len(capturedIndexes) != 2 {
			t.Fatalf("indexes count = %d, want 2", len(capturedIndexes))
		}
		if capturedIndexes[0] != search.IndexDocuments {
			t.Errorf("index[0] = %q, want %q", capturedIndexes[0], search.IndexDocuments)
		}
		if capturedIndexes[1] != search.IndexGitTemplates {
			t.Errorf("index[1] = %q, want %q", capturedIndexes[1], search.IndexGitTemplates)
		}
	})

	t.Run("ignores unknown type names", func(t *testing.T) {
		var capturedIndexes []string
		s := &mockSearcher{
			federatedSearchFn: func(_ context.Context, params search.FederatedSearchParams) (*search.FederatedSearchResponse, error) {
				capturedIndexes = params.Indexes
				return &search.FederatedSearchResponse{}, nil
			},
		}
		h := newHandlerWithMocks(struct {
			docSvc   *mockDocumentService
			zimRepo  *mockZimArchiveRepo
			gitRepo  *mockGitTemplateRepo
			kiwixC   *mockKiwixClient
			searcher *mockSearcher
		}{searcher: s})

		_, _, _ = h.handleUnifiedSearch(ctx, nil, dto.UnifiedSearchInput{
			Query: "x",
			Types: []string{"unknown_type"},
		})
		if len(capturedIndexes) != 0 {
			t.Errorf("indexes count = %d, want 0 for unknown type", len(capturedIndexes))
		}
	})
}

// --- Test helper ---

// makeHit builds a search.SearchResult from a map of arbitrary values.
func makeHit(m map[string]any) search.SearchResult {
	return mapToSearchResult(m)
}

// ===== Search result-building tests =====

func TestHandleSearchDocumentsResults(t *testing.T) {
	t.Parallel()
	ctx := ctxWithAdminUser()

	t.Run("returns populated results from hits", func(t *testing.T) {
		t.Parallel()
		s := &mockSearcher{
			searchFn: func(_ context.Context, _ search.SearchParams) (*search.SearchResponse, error) {
				return &search.SearchResponse{
					Hits: []search.SearchResult{
						makeHit(map[string]any{
							"uuid":        "aaa-bbb",
							"title":       "First Doc",
							"file_type":   "markdown",
							"description": "desc one",
							"word_count":  float64(500),
						}),
						makeHit(map[string]any{
							"uuid":        "ccc-ddd",
							"title":       "Second Doc",
							"file_type":   "pdf",
							"description": "desc two",
							"word_count":  float64(1200),
						}),
					},
				}, nil
			},
		}
		h := newHandlerWithMocks(struct {
			docSvc   *mockDocumentService
			zimRepo  *mockZimArchiveRepo
			gitRepo  *mockGitTemplateRepo
			kiwixC   *mockKiwixClient
			searcher *mockSearcher
		}{searcher: s})

		_, resp, err := h.handleSearchDocuments(ctx, nil, dto.SearchDocumentsInput{Query: "test"})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !resp.Success {
			t.Fatal("expected Success=true")
		}
		if resp.Count != 2 {
			t.Fatalf("Count = %d, want 2", resp.Count)
		}
		if resp.Results[0].UUID != "aaa-bbb" {
			t.Errorf("Results[0].UUID = %q, want %q", resp.Results[0].UUID, "aaa-bbb")
		}
		if resp.Results[0].Title != "First Doc" {
			t.Errorf("Results[0].Title = %q, want %q", resp.Results[0].Title, "First Doc")
		}
		if resp.Results[0].FileType != "markdown" {
			t.Errorf("Results[0].FileType = %q, want %q", resp.Results[0].FileType, "markdown")
		}
		if resp.Results[0].Description != "desc one" {
			t.Errorf("Results[0].Description = %q", resp.Results[0].Description)
		}
		if resp.Results[0].ContentLength != 500 {
			t.Errorf("Results[0].ContentLength = %v, want 500", resp.Results[0].ContentLength)
		}
		if resp.Results[1].UUID != "ccc-ddd" {
			t.Errorf("Results[1].UUID = %q, want %q", resp.Results[1].UUID, "ccc-ddd")
		}
	})

	t.Run("result includes tags from hit", func(t *testing.T) {
		t.Parallel()
		s := &mockSearcher{
			searchFn: func(_ context.Context, _ search.SearchParams) (*search.SearchResponse, error) {
				return &search.SearchResponse{
					Hits: []search.SearchResult{
						makeHit(map[string]any{
							"uuid":  "tag-doc",
							"title": "Tagged",
							"tags":  []any{"go", "test"},
						}),
					},
				}, nil
			},
		}
		h := newHandlerWithMocks(struct {
			docSvc   *mockDocumentService
			zimRepo  *mockZimArchiveRepo
			gitRepo  *mockGitTemplateRepo
			kiwixC   *mockKiwixClient
			searcher *mockSearcher
		}{searcher: s})

		_, resp, err := h.handleSearchDocuments(ctx, nil, dto.SearchDocumentsInput{Query: "tagged"})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(resp.Results) != 1 {
			t.Fatalf("Results length = %d, want 1", len(resp.Results))
		}
		if len(resp.Results[0].Tags) != 2 {
			t.Fatalf("Tags length = %d, want 2", len(resp.Results[0].Tags))
		}
		if resp.Results[0].Tags[0] != "go" || resp.Results[0].Tags[1] != "test" {
			t.Errorf("Tags = %v, want [go test]", resp.Results[0].Tags)
		}
	})

	t.Run("include_content=true populates content field", func(t *testing.T) {
		t.Parallel()
		s := &mockSearcher{
			searchFn: func(_ context.Context, _ search.SearchParams) (*search.SearchResponse, error) {
				return &search.SearchResponse{
					Hits: []search.SearchResult{
						makeHit(map[string]any{
							"uuid":    "content-doc",
							"title":   "With Content",
							"content": "full document content here",
						}),
					},
				}, nil
			},
		}
		h := newHandlerWithMocks(struct {
			docSvc   *mockDocumentService
			zimRepo  *mockZimArchiveRepo
			gitRepo  *mockGitTemplateRepo
			kiwixC   *mockKiwixClient
			searcher *mockSearcher
		}{searcher: s})

		_, resp, err := h.handleSearchDocuments(ctx, nil, dto.SearchDocumentsInput{
			Query:          "content",
			IncludeContent: true,
		})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if resp.Results[0].Content != "full document content here" {
			t.Errorf("Content = %q, want %q", resp.Results[0].Content, "full document content here")
		}
	})

	t.Run("include_content=false omits content field", func(t *testing.T) {
		t.Parallel()
		s := &mockSearcher{
			searchFn: func(_ context.Context, _ search.SearchParams) (*search.SearchResponse, error) {
				return &search.SearchResponse{
					Hits: []search.SearchResult{
						makeHit(map[string]any{
							"uuid":    "no-content-doc",
							"title":   "Without Content",
							"content": "this should be omitted",
						}),
					},
				}, nil
			},
		}
		h := newHandlerWithMocks(struct {
			docSvc   *mockDocumentService
			zimRepo  *mockZimArchiveRepo
			gitRepo  *mockGitTemplateRepo
			kiwixC   *mockKiwixClient
			searcher *mockSearcher
		}{searcher: s})

		_, resp, err := h.handleSearchDocuments(ctx, nil, dto.SearchDocumentsInput{
			Query:          "content",
			IncludeContent: false,
		})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if resp.Results[0].Content != "" {
			t.Errorf("Content = %q, want empty string", resp.Results[0].Content)
		}
	})

	t.Run("invalid file_type filter is silently ignored", func(t *testing.T) {
		t.Parallel()
		var capturedParams search.SearchParams
		s := &mockSearcher{
			searchFn: func(_ context.Context, params search.SearchParams) (*search.SearchResponse, error) {
				capturedParams = params
				return &search.SearchResponse{}, nil
			},
		}
		h := newHandlerWithMocks(struct {
			docSvc   *mockDocumentService
			zimRepo  *mockZimArchiveRepo
			gitRepo  *mockGitTemplateRepo
			kiwixC   *mockKiwixClient
			searcher *mockSearcher
		}{searcher: s})

		_, _, err := h.handleSearchDocuments(ctx, nil, dto.SearchDocumentsInput{
			Query:    "test",
			FileType: "exe",
		})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if capturedParams.FileType != "" {
			t.Errorf("FileType should be empty for invalid type, got %q", capturedParams.FileType)
		}
	})
}

func TestHandleUnifiedSearchResults(t *testing.T) {
	t.Parallel()
	ctx := ctxWithAdminUser()

	t.Run("returns populated results from hits", func(t *testing.T) {
		t.Parallel()
		s := &mockSearcher{
			federatedSearchFn: func(_ context.Context, _ search.FederatedSearchParams) (*search.FederatedSearchResponse, error) {
				return &search.FederatedSearchResponse{
					Hits: []search.SearchResult{
						makeHit(map[string]any{
							"uuid":        "fed-aaa",
							"title":       "Federated Doc",
							"description": "a federated result",
							"_federation": map[string]any{
								"indexUid": search.IndexDocuments,
							},
							"_rankingScore": 0.95,
						}),
					},
				}, nil
			},
		}
		h := newHandlerWithMocks(struct {
			docSvc   *mockDocumentService
			zimRepo  *mockZimArchiveRepo
			gitRepo  *mockGitTemplateRepo
			kiwixC   *mockKiwixClient
			searcher *mockSearcher
		}{searcher: s})

		_, resp, err := h.handleUnifiedSearch(ctx, nil, dto.UnifiedSearchInput{Query: "test"})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !resp.Success {
			t.Fatal("expected Success=true")
		}
		if resp.Total != 1 {
			t.Fatalf("Total = %d, want 1", resp.Total)
		}
		if resp.Results[0].Source != "document" {
			t.Errorf("Source = %q, want %q", resp.Results[0].Source, "document")
		}
		if resp.Results[0].UUID != "fed-aaa" {
			t.Errorf("UUID = %q, want %q", resp.Results[0].UUID, "fed-aaa")
		}
		if resp.Results[0].Title != "Federated Doc" {
			t.Errorf("Title = %q, want %q", resp.Results[0].Title, "Federated Doc")
		}
		if resp.Results[0].Description != "a federated result" {
			t.Errorf("Description = %q", resp.Results[0].Description)
		}
		if resp.Results[0].Score != 0.95 {
			t.Errorf("Score = %v, want 0.95", resp.Results[0].Score)
		}
	})

	t.Run("detects source from document federation metadata", func(t *testing.T) {
		t.Parallel()
		s := &mockSearcher{
			federatedSearchFn: func(_ context.Context, _ search.FederatedSearchParams) (*search.FederatedSearchResponse, error) {
				return &search.FederatedSearchResponse{
					Hits: []search.SearchResult{
						makeHit(map[string]any{
							"uuid":        "fallback-doc",
							"title":       "Fallback Doc",
							"file_type":   "markdown",
							"_federation": map[string]any{"indexUid": "documents"},
						}),
					},
				}, nil
			},
		}
		h := newHandlerWithMocks(struct {
			docSvc   *mockDocumentService
			zimRepo  *mockZimArchiveRepo
			gitRepo  *mockGitTemplateRepo
			kiwixC   *mockKiwixClient
			searcher *mockSearcher
		}{searcher: s})

		_, resp, err := h.handleUnifiedSearch(ctx, nil, dto.UnifiedSearchInput{Query: "test"})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if resp.Results[0].Source != "document" {
			t.Errorf("Source = %q, want %q", resp.Results[0].Source, "document")
		}
	})

	t.Run("detects source from zim_archive federation metadata", func(t *testing.T) {
		t.Parallel()
		s := &mockSearcher{
			federatedSearchFn: func(_ context.Context, _ search.FederatedSearchParams) (*search.FederatedSearchResponse, error) {
				return &search.FederatedSearchResponse{
					Hits: []search.SearchResult{
						makeHit(map[string]any{
							"uuid":          "fallback-zim",
							"title":         "Fallback ZIM",
							"article_count": float64(100),
							"_federation":   map[string]any{"indexUid": "zim_archives"},
						}),
					},
				}, nil
			},
		}
		h := newHandlerWithMocks(struct {
			docSvc   *mockDocumentService
			zimRepo  *mockZimArchiveRepo
			gitRepo  *mockGitTemplateRepo
			kiwixC   *mockKiwixClient
			searcher *mockSearcher
		}{searcher: s})

		_, resp, err := h.handleUnifiedSearch(ctx, nil, dto.UnifiedSearchInput{Query: "test"})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if resp.Results[0].Source != "zim_archive" {
			t.Errorf("Source = %q, want %q", resp.Results[0].Source, "zim_archive")
		}
	})

	t.Run("detects source from git_template federation metadata", func(t *testing.T) {
		t.Parallel()
		s := &mockSearcher{
			federatedSearchFn: func(_ context.Context, _ search.FederatedSearchParams) (*search.FederatedSearchResponse, error) {
				return &search.FederatedSearchResponse{
					Hits: []search.SearchResult{
						makeHit(map[string]any{
							"uuid":           "fallback-git",
							"title":          "Fallback Git",
							"readme_content": "# README",
							"_federation":    map[string]any{"indexUid": "git_templates"},
						}),
					},
				}, nil
			},
		}
		h := newHandlerWithMocks(struct {
			docSvc   *mockDocumentService
			zimRepo  *mockZimArchiveRepo
			gitRepo  *mockGitTemplateRepo
			kiwixC   *mockKiwixClient
			searcher *mockSearcher
		}{searcher: s})

		_, resp, err := h.handleUnifiedSearch(ctx, nil, dto.UnifiedSearchInput{Query: "test"})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if resp.Results[0].Source != "git_template" {
			t.Errorf("Source = %q, want %q", resp.Results[0].Source, "git_template")
		}
	})

	t.Run("uses title fallback from name field", func(t *testing.T) {
		t.Parallel()
		s := &mockSearcher{
			federatedSearchFn: func(_ context.Context, _ search.FederatedSearchParams) (*search.FederatedSearchResponse, error) {
				return &search.FederatedSearchResponse{
					Hits: []search.SearchResult{
						makeHit(map[string]any{
							"uuid":           "name-fallback",
							"name":           "Template Name",
							"readme_content": "readme",
						}),
					},
				}, nil
			},
		}
		h := newHandlerWithMocks(struct {
			docSvc   *mockDocumentService
			zimRepo  *mockZimArchiveRepo
			gitRepo  *mockGitTemplateRepo
			kiwixC   *mockKiwixClient
			searcher *mockSearcher
		}{searcher: s})

		_, resp, err := h.handleUnifiedSearch(ctx, nil, dto.UnifiedSearchInput{Query: "test"})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if resp.Results[0].Title != "Template Name" {
			t.Errorf("Title = %q, want %q", resp.Results[0].Title, "Template Name")
		}
	})

	t.Run("sources_searched reflects hit sources", func(t *testing.T) {
		t.Parallel()
		s := &mockSearcher{
			federatedSearchFn: func(_ context.Context, _ search.FederatedSearchParams) (*search.FederatedSearchResponse, error) {
				return &search.FederatedSearchResponse{
					Hits: []search.SearchResult{
						makeHit(map[string]any{
							"uuid":      "src-doc",
							"title":     "Doc",
							"file_type": "markdown",
						}),
						makeHit(map[string]any{
							"uuid":           "src-git",
							"title":          "Git",
							"readme_content": "readme",
						}),
					},
				}, nil
			},
		}
		h := newHandlerWithMocks(struct {
			docSvc   *mockDocumentService
			zimRepo  *mockZimArchiveRepo
			gitRepo  *mockGitTemplateRepo
			kiwixC   *mockKiwixClient
			searcher *mockSearcher
		}{searcher: s})

		_, resp, err := h.handleUnifiedSearch(ctx, nil, dto.UnifiedSearchInput{Query: "test"})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		// sources_searched reflects all queried indexes, not just those with hits.
		if len(resp.SourcesSearched) != 3 {
			t.Fatalf("SourcesSearched length = %d, want 3", len(resp.SourcesSearched))
		}
		foundDoc, foundGit := false, false
		for _, s := range resp.SourcesSearched {
			if s == "document" {
				foundDoc = true
			}
			if s == "git_template" {
				foundGit = true
			}
		}
		if !foundDoc {
			t.Error("SourcesSearched missing 'document'")
		}
		if !foundGit {
			t.Error("SourcesSearched missing 'git_template'")
		}
	})
}
