package mcphandler

import (
	"log/slog"
	"net/http"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"git.999.haus/chris/DocuMCP-go/internal/repository"
	"git.999.haus/chris/DocuMCP-go/internal/service"
)

// serverInstructions is the MCP server instructions describing all available
// tools and prompts. It is sent to clients during initialization.
const serverInstructions = `Documentation knowledge base with full-text search.

**Unified Search**
- ` + "`unified_search`" + ` - Search across ALL sources (documents, templates, archives, wikis) in one request. Use for discovery; type-specific tools for deep search.

**Documents**
- ` + "`search_documents`" + ` - Full-text search with filters (file type, tags). Returns metadata and snippets.
- ` + "`read_document`" + ` - Retrieve document content by UUID. Supports ` + "`summary_only`" + ` and ` + "`max_paragraphs`" + `.
- ` + "`create_document`" + ` - Create documents (markdown, pdf, docx, xlsx, html). Auto-indexed for search.
- ` + "`update_document`" + ` - Modify title, description, tags, or visibility.
- ` + "`delete_document`" + ` - Remove documents (ownership required).

**ZIM Archives** (offline documentation: DevDocs, Wikipedia, Stack Exchange)
- ` + "`list_zim_archives`" + ` - List available archives with category/language filters.
- ` + "`search_zim`" + ` - Search within an archive. ` + "`suggest`" + ` matches titles, ` + "`fulltext`" + ` searches content.
- ` + "`read_zim_article`" + ` - Retrieve article content. Supports ` + "`summary_only`" + ` and ` + "`max_paragraphs`" + `.

**Confluence**
- ` + "`list_confluence_spaces`" + ` - List spaces (global or personal). Returns space keys for filtering.
- ` + "`search_confluence`" + ` - Search pages via CQL or simple query. Supports space filtering.
- ` + "`read_confluence_page`" + ` - Retrieve page content as markdown by ID or space+title.

**Git Templates** (project bootstrapping: CLAUDE.md, Memory Bank)
- ` + "`list_git_templates`" + ` - List available templates with category filter.
- ` + "`search_git_templates`" + ` - Full-text search across README files and metadata.
- ` + "`get_template_structure`" + ` - View folder tree, essential files, and required variables.
- ` + "`get_template_file`" + ` - Retrieve file content with optional variable substitution.
- ` + "`get_deployment_guide`" + ` - Get deployment instructions with all essential files.
- ` + "`download_template`" + ` - Download a complete template as a base64-encoded archive.

**Availability**: ZIM tools require Kiwix service configuration. Confluence tools require Confluence service configuration. Git Template tools are enabled by default. Document tools are always available.

**Access Control**: Document modifications require ownership. Public documents are readable by all.`

// Handler holds the MCP server and all dependencies needed for tool/prompt registration.
type Handler struct {
	server      *mcp.Server
	httpHandler http.Handler
	logger      *slog.Logger

	// Dependencies for tools
	documentService     *service.DocumentService
	documentRepo        *repository.DocumentRepository
	externalServiceRepo *repository.ExternalServiceRepository
	zimArchiveRepo      *repository.ZimArchiveRepository
	confluenceSpaceRepo *repository.ConfluenceSpaceRepository
	gitTemplateRepo     *repository.GitTemplateRepository
	searchQueryRepo     *repository.SearchQueryRepository
}

// Config holds all optional dependencies for the MCP handler.
// Nil fields mean those tool groups will not be registered.
type Config struct {
	ServerName    string
	ServerVersion string
	Logger        *slog.Logger

	// Always required
	DocumentService *service.DocumentService
	DocumentRepo    *repository.DocumentRepository
	SearchQueryRepo *repository.SearchQueryRepository

	// Conditionally registered
	ExternalServiceRepo *repository.ExternalServiceRepository
	ZimArchiveRepo      *repository.ZimArchiveRepository
	ConfluenceSpaceRepo *repository.ConfluenceSpaceRepository
	GitTemplateRepo     *repository.GitTemplateRepository

	// Feature flags
	ZimEnabled          bool
	ConfluenceEnabled   bool
	GitTemplatesEnabled bool
}

// New creates and configures the MCP handler with all tools and prompts.
func New(cfg Config) *Handler {
	mcpServer := mcp.NewServer(
		&mcp.Implementation{
			Name:    cfg.ServerName,
			Version: cfg.ServerVersion,
		},
		&mcp.ServerOptions{
			Instructions: serverInstructions,
		},
	)

	h := &Handler{
		server:              mcpServer,
		logger:              cfg.Logger,
		documentService:     cfg.DocumentService,
		documentRepo:        cfg.DocumentRepo,
		externalServiceRepo: cfg.ExternalServiceRepo,
		zimArchiveRepo:      cfg.ZimArchiveRepo,
		confluenceSpaceRepo: cfg.ConfluenceSpaceRepo,
		gitTemplateRepo:     cfg.GitTemplateRepo,
		searchQueryRepo:     cfg.SearchQueryRepo,
	}

	// Register tools
	h.registerDocumentTools()
	h.registerUnifiedSearchTool()

	if cfg.ZimEnabled && cfg.ZimArchiveRepo != nil {
		h.registerZimTools()
	}

	if cfg.ConfluenceEnabled && cfg.ConfluenceSpaceRepo != nil {
		h.registerConfluenceTools()
	}

	if cfg.GitTemplatesEnabled && cfg.GitTemplateRepo != nil {
		h.registerGitTemplateTools()
	}

	// Register prompts
	h.registerPrompts(cfg.ZimEnabled, cfg.ConfluenceEnabled, cfg.GitTemplatesEnabled)

	// Create HTTP handler using the streamable HTTP transport
	h.httpHandler = mcp.NewStreamableHTTPHandler(func(_ *http.Request) *mcp.Server {
		return mcpServer
	}, nil)

	return h
}

// ServeHTTP implements http.Handler.
func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	h.httpHandler.ServeHTTP(w, r)
}
