package mcphandler

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/c-premus/documcp/internal/client/kiwix"
	"github.com/c-premus/documcp/internal/model"
	"github.com/c-premus/documcp/internal/repository"
	"github.com/c-premus/documcp/internal/search"
	"github.com/c-premus/documcp/internal/service"
)

// --- Interfaces defined where consumed ---

// documentServicer abstracts the document service methods used by MCP tool handlers.
type documentServicer interface {
	FindByUUID(ctx context.Context, uuid string) (*model.Document, error)
	TagsForDocument(ctx context.Context, documentID int64) ([]model.DocumentTag, error)
	Create(ctx context.Context, params service.CreateDocumentParams) (*model.Document, error)
	Update(ctx context.Context, uuid string, params service.UpdateDocumentParams) (*model.Document, error)
	Delete(ctx context.Context, uuid string) error
}

// zimArchiveLister abstracts the ZIM archive repository methods.
type zimArchiveLister interface {
	List(ctx context.Context, category, language, query string, limit, offset int) ([]model.ZimArchive, error)
	ListSearchable(ctx context.Context) ([]model.ZimArchive, error)
}

// gitTemplateStore abstracts the git template repository methods.
type gitTemplateStore interface {
	List(ctx context.Context, category string, limit, offset int) ([]model.GitTemplate, error)
	Search(ctx context.Context, query, category string, limit int) ([]model.GitTemplate, error)
	FindByUUID(ctx context.Context, uuid string) (*model.GitTemplate, error)
	FilesForTemplate(ctx context.Context, templateID int64) ([]model.GitTemplateFile, error)
	FindFileByPath(ctx context.Context, templateID int64, path string) (*model.GitTemplateFile, error)
}

// kiwixSearcher abstracts the Kiwix client methods.
type kiwixSearcher interface {
	Search(ctx context.Context, archiveName, query, searchType string, limit int) ([]kiwix.SearchResult, error)
	ReadArticle(ctx context.Context, archiveName, articlePath string) (*kiwix.Article, error)
	FetchCatalog(ctx context.Context) ([]kiwix.CatalogEntry, error)
	HasFulltextIndex(ctx context.Context, archiveName string) bool
}

// kiwixClientFactory creates or returns a cached Kiwix client on demand.
// Get returns a kiwixSearcher (satisfied by *kiwix.Client and test mocks).
type kiwixClientFactory interface {
	Get(ctx context.Context) (kiwixSearcher, error)
}

// documentLister abstracts the document repository methods used by list_documents.
type documentLister interface {
	List(ctx context.Context, params repository.DocumentListParams) (*repository.DocumentListResult, error)
	TagsForDocuments(ctx context.Context, documentIDs []int64) (map[int64][]model.DocumentTag, error)
}

// contentSearcher abstracts the search.Searcher methods.
type contentSearcher interface {
	Search(ctx context.Context, params search.SearchParams) (*search.SearchResponse, error)
	FederatedSearch(ctx context.Context, params search.FederatedSearchParams) (*search.FederatedSearchResponse, error)
	SearchGitTemplateFiles(ctx context.Context, query string, limit int64) ([]search.FileSearchResult, error)
}

// faviconDataURI is the favicon SVG encoded as a data URI for MCP clients that
// support inline icons (avoids external fetch). Included alongside URL-based
// icons so clients can choose whichever format they prefer.
const faviconDataURI = "data:image/svg+xml;base64,PHN2ZyB4bWxucz0iaHR0cDovL3d3dy53My5vcmcvMjAwMC9zdmciIHZpZXdCb3g9IjAgMCA2NCA2NCIgd2lkdGg9IjY0IiBoZWlnaHQ9IjY0Ij48bWV0YWRhdGE+PHJkZjpSREYgeG1sbnM6cmRmPSJodHRwOi8vd3d3LnczLm9yZy8xOTk5LzAyLzIyLXJkZi1zeW50YXgtbnMjIiB4bWxuczpkYz0iaHR0cDovL3B1cmwub3JnL2RjL2VsZW1lbnRzLzEuMS8iPjxyZGY6RGVzY3JpcHRpb24+PGRjOmNyZWF0b3I+UmVhbEZhdmljb25HZW5lcmF0b3I8L2RjOmNyZWF0b3I+PGRjOnNvdXJjZT5odHRwczovL3JlYWxmYXZpY29uZ2VuZXJhdG9yLm5ldDwvZGM6c291cmNlPjwvcmRmOkRlc2NyaXB0aW9uPjwvcmRmOlJERj48L21ldGFkYXRhPjxwYXRoIGQ9Ik0xMiA2IEwzOCA2IEw1MCAxOCBMNTAgNTYgTDEyIDU2IFoiIGZpbGw9IiNmOGZhZmMiIHN0cm9rZT0iIzY0NzQ4YiIgc3Ryb2tlLXdpZHRoPSIxLjUiPjwvcGF0aD48cGF0aCBkPSJNMzggNiBMMzggMTggTDUwIDE4IiBmaWxsPSIjZTJlOGYwIiBzdHJva2U9IiM2NDc0OGIiIHN0cm9rZS13aWR0aD0iMS41Ij48L3BhdGg+PGxpbmUgeDE9IjE4IiB5MT0iMjYiIHgyPSI0NCIgeTI9IjI2IiBzdHJva2U9IiM2NDc0OGIiIHN0cm9rZS13aWR0aD0iMiIgc3Ryb2tlLWxpbmVjYXA9InJvdW5kIj48L2xpbmU+PGxpbmUgeDE9IjE4IiB5MT0iMzQiIHgyPSIzOCIgeTI9IjM0IiBzdHJva2U9IiM2NDc0OGIiIHN0cm9rZS13aWR0aD0iMiIgc3Ryb2tlLWxpbmVjYXA9InJvdW5kIj48L2xpbmU+PGxpbmUgeDE9IjE4IiB5MT0iNDIiIHgyPSI0MiIgeTI9IjQyIiBzdHJva2U9IiM2NDc0OGIiIHN0cm9rZS13aWR0aD0iMiIgc3Ryb2tlLWxpbmVjYXA9InJvdW5kIj48L2xpbmU+PGNpcmNsZSBjeD0iNTAiIGN5PSI1MCIgcj0iMTAiIGZpbGw9IiMzYjgyZjYiPjwvY2lyY2xlPjxjaXJjbGUgY3g9IjUwIiBjeT0iNTAiIHI9IjQiIGZpbGw9IiNmZmZmZmYiPjwvY2lyY2xlPjxwYXRoIGQ9Ik01NiA0NCBMNjAgNDAiIHN0cm9rZT0iIzNiODJmNiIgc3Ryb2tlLXdpZHRoPSIyIiBzdHJva2UtbGluZWNhcD0icm91bmQiPjwvcGF0aD48cGF0aCBkPSJNNTggNTAgTDYyIDUwIiBzdHJva2U9IiMzYjgyZjYiIHN0cm9rZS13aWR0aD0iMiIgc3Ryb2tlLWxpbmVjYXA9InJvdW5kIj48L3BhdGg+PHBhdGggZD0iTTU2IDU2IEw2MCA2MCIgc3Ryb2tlPSIjM2I4MmY2IiBzdHJva2Utd2lkdGg9IjIiIHN0cm9rZS1saW5lY2FwPSJyb3VuZCI+PC9wYXRoPjwvc3ZnPg=="

// serverInstructions is the MCP server instructions describing all available
// tools and prompts. It is sent to clients during initialization.
const serverInstructions = `Documentation knowledge base with full-text search.

**Unified Search**
- ` + "`unified_search`" + ` - Search across ALL sources in one request: documents, git templates, ZIM archive metadata, AND ZIM article content (via Kiwix fan-out). Use for discovery; type-specific tools for deep search.

**Documents**
- ` + "`list_documents`" + ` - List all accessible documents with optional filters (file type, status). Paginated.
- ` + "`search_documents`" + ` - Full-text search with filters (file type, tags). Returns metadata and snippets.
- ` + "`read_document`" + ` - Retrieve document content by UUID. Supports ` + "`summary_only`" + ` and ` + "`max_paragraphs`" + `.
- ` + "`create_document`" + ` - Create documents (markdown, pdf, docx, xlsx, html, epub). Auto-indexed for search.
- ` + "`update_document`" + ` - Modify title, description, tags, or visibility.
- ` + "`delete_document`" + ` - Remove documents (ownership required).

**ZIM Archives** (offline documentation: DevDocs, Wikipedia, Stack Exchange)
- ` + "`list_zim_archives`" + ` - List available archives with category/language filters.
- ` + "`search_zim`" + ` - Deep search within a specific archive. ` + "`suggest`" + ` matches titles, ` + "`fulltext`" + ` searches content.
- ` + "`read_zim_article`" + ` - Retrieve article content. Supports ` + "`summary_only`" + ` and ` + "`max_paragraphs`" + `.

**Git Templates** (project bootstrapping: CLAUDE.md, Memory Bank)
- ` + "`list_git_templates`" + ` - List available templates with category filter.
- ` + "`search_git_templates`" + ` - Full-text search across template files, README content, and metadata. Returns ` + "`matched_files`" + ` paths for direct use with ` + "`get_template_file`" + `.
- ` + "`get_template_structure`" + ` - View folder tree, essential files, and required variables.
- ` + "`get_template_file`" + ` - Retrieve file content with optional variable substitution.
- ` + "`get_deployment_guide`" + ` - Get deployment instructions with all essential files.
- ` + "`download_template`" + ` - Download a complete template as a base64-encoded archive.

**Availability**: ZIM tools require Kiwix service configuration. Git Template tools are enabled by default. Document tools are always available.

**Access Control**: Document modifications require ownership. Public documents are readable by all.`

// Handler holds the MCP server and all dependencies needed for tool/prompt registration.
type Handler struct {
	server      *mcp.Server
	httpHandler http.Handler
	logger      *slog.Logger

	// Dependencies for tools (interface-typed for testability)
	documentService     documentServicer
	documentRepo        documentLister
	externalServiceRepo *repository.ExternalServiceRepository
	zimArchiveRepo      zimArchiveLister
	gitTemplateRepo     gitTemplateStore

	// Services
	gitTemplateService *service.GitTemplateService

	// External service clients
	kiwixFactory kiwixClientFactory // lazy-init; nil means ZIM tools not enabled
	searcher     contentSearcher

	// Federated search config
	federatedSearchTimeout   time.Duration
	federatedMaxArchives     int
	federatedPerArchiveLimit int
}

// Config holds all optional dependencies for the MCP handler.
// Nil fields mean those tool groups will not be registered.
type Config struct {
	ServerName    string
	ServerVersion string
	AppURL        string // Base URL for icon references (e.g. "https://docs.example.com")
	Logger        *slog.Logger

	// Always required
	DocumentService documentServicer
	DocumentRepo    documentLister

	// Conditionally registered
	ExternalServiceRepo *repository.ExternalServiceRepository
	ZimArchiveRepo      zimArchiveLister
	GitTemplateRepo     gitTemplateStore
	GitTemplateService  *service.GitTemplateService

	// External service clients
	KiwixFactory *kiwix.ClientFactory // lazy-init Kiwix client (nil = ZIM tools disabled)
	Searcher     contentSearcher

	// Federated search (Kiwix fan-out during unified_search)
	FederatedSearchTimeout   time.Duration
	FederatedMaxArchives     int
	FederatedPerArchiveLimit int

	// Feature flags
	ZimEnabled          bool
	GitTemplatesEnabled bool
}

// New creates and configures the MCP handler with all tools and prompts.
func New(cfg Config) *Handler {
	impl := &mcp.Implementation{
		Name:    cfg.ServerName,
		Title:   cfg.ServerName,
		Version: cfg.ServerVersion,
	}
	if cfg.AppURL != "" {
		impl.WebsiteURL = cfg.AppURL
		impl.Icons = []mcp.Icon{
			{
				Source:   cfg.AppURL + "/favicon.svg",
				MIMEType: "image/svg+xml",
				Sizes:    []string{"any"},
			},
			{
				Source:   cfg.AppURL + "/favicon-96x96.png",
				MIMEType: "image/png",
				Sizes:    []string{"96x96"},
			},
			{
				Source:   faviconDataURI,
				MIMEType: "image/svg+xml",
				Sizes:    []string{"any"},
			},
		}
	}

	mcpServer := mcp.NewServer(
		impl,
		&mcp.ServerOptions{
			Instructions: serverInstructions,
		},
	)

	var kf kiwixClientFactory
	if cfg.KiwixFactory != nil {
		kf = &kiwixFactoryAdapter{factory: cfg.KiwixFactory}
	}

	// Apply federated search defaults.
	fedTimeout := cfg.FederatedSearchTimeout
	if fedTimeout == 0 {
		fedTimeout = 5 * time.Second
	}
	fedMaxArchives := cfg.FederatedMaxArchives
	if fedMaxArchives == 0 {
		fedMaxArchives = 10
	}
	fedPerArchive := cfg.FederatedPerArchiveLimit
	if fedPerArchive == 0 {
		fedPerArchive = 3
	}

	h := &Handler{
		server:              mcpServer,
		logger:              cfg.Logger,
		documentService:     cfg.DocumentService,
		documentRepo:        cfg.DocumentRepo,
		externalServiceRepo: cfg.ExternalServiceRepo,
		zimArchiveRepo:      cfg.ZimArchiveRepo,
		gitTemplateRepo:     cfg.GitTemplateRepo,
		gitTemplateService:  cfg.GitTemplateService,
		kiwixFactory:        kf,
		searcher:            cfg.Searcher,

		federatedSearchTimeout:   fedTimeout,
		federatedMaxArchives:     fedMaxArchives,
		federatedPerArchiveLimit: fedPerArchive,
	}

	// Register tools
	h.registerDocumentTools()
	h.registerUnifiedSearchTool()

	if cfg.ZimEnabled && cfg.ZimArchiveRepo != nil {
		h.registerZimTools()
	}

	if cfg.GitTemplatesEnabled && cfg.GitTemplateRepo != nil {
		h.registerGitTemplateTools()
	}

	// Register prompts
	h.registerPrompts(cfg.ZimEnabled, cfg.GitTemplatesEnabled)

	// Create HTTP handler using the Streamable HTTP transport (protocol 2025-03-26).
	// The SDK v1.4.1 requires Accept: application/json, text/event-stream on all
	// requests. We wrap with a middleware that adds text/event-stream when missing
	// so Claude.ai clients that only send Accept: application/json still work.
	streamableHandler := mcp.NewStreamableHTTPHandler(func(_ *http.Request) *mcp.Server {
		return mcpServer
	}, &mcp.StreamableHTTPOptions{
		Stateless: false,
	})
	h.httpHandler = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		accept := r.Header.Get("Accept")
		if accept != "" && !strings.Contains(accept, "text/event-stream") && !strings.Contains(accept, "*/*") {
			r.Header.Set("Accept", accept+", text/event-stream")
		}
		streamableHandler.ServeHTTP(w, r)
	})

	return h
}

// Close terminates all active MCP sessions so the HTTP server can shut down
// without waiting for long-lived SSE streams to drain.
func (h *Handler) Close() {
	var n int
	for sess := range h.server.Sessions() {
		if err := sess.Close(); err != nil && !errors.Is(err, context.Canceled) {
			h.logger.Warn("closing MCP session", "session_id", sess.ID(), "error", err)
		}
		n++
	}
	if n > 0 {
		h.logger.Info("closed MCP sessions", "count", n)
	}
}

// ActiveSessionCount returns the number of MCP sessions currently held by
// this replica's in-memory session store. Used as the source for the
// documcp_mcp_active_sessions gauge so operators can detect hot-spotting
// across replicas behind a sticky-session load balancer.
func (h *Handler) ActiveSessionCount() int {
	var n int
	for range h.server.Sessions() {
		n++
	}
	return n
}

// kiwixFactoryAdapter wraps *kiwix.ClientFactory to satisfy kiwixClientFactory.
type kiwixFactoryAdapter struct {
	factory *kiwix.ClientFactory
}

// Get returns a Kiwix client from the underlying factory.
func (a *kiwixFactoryAdapter) Get(ctx context.Context) (kiwixSearcher, error) {
	return a.factory.Get(ctx)
}

// getKiwixClient returns a Kiwix client from the factory, or an error if
// no kiwix service is configured.
func (h *Handler) getKiwixClient(ctx context.Context) (kiwixSearcher, error) {
	if h.kiwixFactory == nil {
		return nil, errors.New("kiwix not enabled")
	}
	return h.kiwixFactory.Get(ctx)
}

// ServeHTTP implements http.Handler.
func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	h.httpHandler.ServeHTTP(w, r)
}
