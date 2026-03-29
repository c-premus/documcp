package mcphandler

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	authmiddleware "github.com/c-premus/documcp/internal/auth/middleware"
	authscope "github.com/c-premus/documcp/internal/auth/scope"
	"github.com/c-premus/documcp/internal/dto"
	"github.com/c-premus/documcp/internal/model"
	"github.com/c-premus/documcp/internal/search"
	"github.com/c-premus/documcp/internal/service"
)

// --- Response types ---

type searchDocumentsResponse struct {
	Success bool                   `json:"success"`
	Query   string                 `json:"query"`
	Count   int                    `json:"count"`
	Results []documentSearchResult `json:"results"`
	Message string                 `json:"message,omitempty"`
}

type documentSearchResult struct {
	UUID          string   `json:"uuid"`
	Title         string   `json:"title"`
	FileType      string   `json:"file_type"`
	Description   string   `json:"description,omitempty"`
	Tags          []string `json:"tags,omitempty"`
	ContentLength float64  `json:"content_length,omitempty"`
	Content       string   `json:"content,omitempty"`
}

type readDocumentResponse struct {
	Success        bool          `json:"success"`
	Document       *documentMeta `json:"document"`
	Content        string        `json:"content"`
	OriginalLength int           `json:"original_length"`
	Truncated      bool          `json:"truncated"`
}

type documentMeta struct {
	UUID        string   `json:"uuid"`
	Title       string   `json:"title"`
	Description string   `json:"description,omitempty"`
	FileType    string   `json:"file_type"`
	WordCount   int64    `json:"word_count"`
	IsPublic    bool     `json:"is_public"`
	Tags        []string `json:"tags"`
	ContentHash string   `json:"content_hash,omitempty"`
	CreatedAt   string   `json:"created_at,omitempty"`
	UpdatedAt   string   `json:"updated_at,omitempty"`
	ProcessedAt string   `json:"processed_at,omitempty"`
}

type createDocumentResponse struct {
	Success  bool         `json:"success"`
	Message  string       `json:"message"`
	Document *documentRef `json:"document"`
}

type documentRef struct {
	UUID      string `json:"uuid"`
	Title     string `json:"title"`
	FileType  string `json:"file_type"`
	CreatedAt string `json:"created_at,omitempty"`
}

type updateDocumentResponse struct {
	Success  bool         `json:"success"`
	Message  string       `json:"message"`
	Document *documentRef `json:"document"`
}

type deleteDocumentResponse struct {
	Success bool   `json:"success"`
	Message string `json:"message"`
	UUID    string `json:"uuid"`
}

// --- Tool registration ---

// registerDocumentTools registers document CRUD and search tools on the MCP server.
func (h *Handler) registerDocumentTools() {
	mcp.AddTool(h.server, &mcp.Tool{
		Name: "search_documents",
		Description: "Full-text search across documents.\n\n" +
			"**Filters:**\n" +
			"- `file_type`: markdown, pdf, docx, xlsx, html\n" +
			"- `tags`: Filter by document tags (AND logic)\n" +
			"- `include_snippets`: Show matched text context\n" +
			"- `include_content`: Include full document content in results (default false to reduce response size)\n\n" +
			"Returns UUID, title, description, file type, tags, content_length, and optional snippets/content. " +
			"Max 100 results, ranked by relevance.\n\n" +
			"**Workflow:** Use `uuid` from results with `read_document` to fetch full content.",
		Annotations: &mcp.ToolAnnotations{
			ReadOnlyHint:   true,
			IdempotentHint: true,
		},
	}, h.handleSearchDocuments)

	mcp.AddTool(h.server, &mcp.Tool{
		Name:        "read_document",
		Description: "Retrieve document content by UUID. Supports `summary_only` and `max_paragraphs` for truncation.",
		Annotations: &mcp.ToolAnnotations{
			ReadOnlyHint:   true,
			IdempotentHint: true,
		},
	}, h.handleReadDocument)

	mcp.AddTool(h.server, &mcp.Tool{
		Name:        "create_document",
		Description: "Create a new document (markdown or html). Auto-indexed for search.",
	}, h.handleCreateDocument)

	mcp.AddTool(h.server, &mcp.Tool{
		Name:        "update_document",
		Description: "Modify a document's title, description, tags, or visibility.",
	}, h.handleUpdateDocument)

	mcp.AddTool(h.server, &mcp.Tool{
		Name:        "delete_document",
		Description: "Remove a document by UUID (ownership required).",
	}, h.handleDeleteDocument)
}

// --- Tool handlers ---

func (h *Handler) handleSearchDocuments(
	ctx context.Context,
	_ *mcp.CallToolRequest,
	input dto.SearchDocumentsInput,
) (*mcp.CallToolResult, searchDocumentsResponse, error) {
	if err := requireMCPScope(ctx, authscope.MCPRead); err != nil {
		return nil, searchDocumentsResponse{}, errors.New("mcp:read scope required for document search")
	}
	if h.searcher == nil {
		return nil, searchDocumentsResponse{
			Success: false,
			Query:   input.Query,
			Count:   0,
			Results: []documentSearchResult{},
			Message: "Search service not configured",
		}, nil
	}

	limit := int64(input.Limit)
	if limit <= 0 {
		limit = 10
	}
	if limit > 100 {
		limit = 100
	}

	// Build structured search params.
	params := search.SearchParams{
		Query:    input.Query,
		IndexUID: search.IndexDocuments,
		Limit:    limit,
	}
	if input.FileType != "" && isValidFileType(input.FileType) {
		params.FileType = input.FileType
	}
	if len(input.Tags) > 0 {
		params.Tags = input.Tags
	}

	// Restrict document visibility based on authentication context.
	// M2M tokens (no user) see only public documents; non-admin users see own + public.
	user, _ := authmiddleware.UserFromContext(ctx)
	switch {
	case user == nil:
		pub := true
		params.IsPublic = &pub
	case user.IsAdmin:
		params.IsAdmin = true
	default:
		params.UserID = &user.ID
	}

	resp, err := h.searcher.Search(ctx, params)
	if err != nil {
		return nil, searchDocumentsResponse{}, fmt.Errorf("searching documents: %w", err)
	}

	results := make([]documentSearchResult, 0, len(resp.Hits))
	for _, sr := range resp.Hits {
		result := documentSearchResult{
			UUID:          sr.UUID,
			Title:         sr.Title,
			Description:   sr.Description,
			FileType:      search.ExtraString(sr.Extra, "file_type"),
			Tags:          search.ExtraStringSlice(sr.Extra, "tags"),
			ContentLength: search.ExtraFloat64(sr.Extra, "word_count"),
		}
		if input.IncludeContent {
			result.Content = search.ExtraString(sr.Extra, "content")
		}
		results = append(results, result)
	}

	return nil, searchDocumentsResponse{
		Success: true,
		Query:   input.Query,
		Count:   len(results),
		Results: results,
	}, nil
}

func (h *Handler) handleReadDocument(
	ctx context.Context,
	_ *mcp.CallToolRequest,
	input dto.ReadDocumentInput,
) (*mcp.CallToolResult, readDocumentResponse, error) {
	if err := requireMCPScope(ctx, authscope.MCPRead); err != nil {
		return nil, readDocumentResponse{}, errors.New("mcp:read scope required for reading documents")
	}
	doc, err := h.documentService.FindByUUID(ctx, input.UUID)
	if err != nil {
		return nil, readDocumentResponse{}, fmt.Errorf("finding document: %w", err)
	}

	// Restrict document visibility based on authentication context.
	// M2M tokens (no user) see only public documents; non-admin users see own + public.
	user, _ := authmiddleware.UserFromContext(ctx)
	if user == nil {
		if !doc.IsPublic {
			return nil, readDocumentResponse{}, errors.New("document not found")
		}
	} else if !user.IsAdmin {
		if !doc.IsPublic && (!doc.UserID.Valid || doc.UserID.Int64 != user.ID) {
			return nil, readDocumentResponse{}, errors.New("document not found")
		}
	}

	tags, err := h.documentService.TagsForDocument(ctx, doc.ID)
	if err != nil {
		return nil, readDocumentResponse{}, fmt.Errorf("loading tags: %w", err)
	}

	content := doc.Content.String
	originalLength := len(content)
	content, truncated := truncateContent(content, input.SummaryOnly, input.MaxParagraphs)

	return nil, readDocumentResponse{
		Success:        true,
		Document:       buildDocumentMeta(doc, tags),
		Content:        content,
		OriginalLength: originalLength,
		Truncated:      truncated,
	}, nil
}

func (h *Handler) handleCreateDocument(
	ctx context.Context,
	_ *mcp.CallToolRequest,
	input dto.CreateDocumentInput,
) (*mcp.CallToolResult, createDocumentResponse, error) {
	if err := requireMCPScope(ctx, authscope.MCPWrite); err != nil {
		return nil, createDocumentResponse{}, errors.New("mcp:write scope required for document creation")
	}
	// Set the owner from the authenticated user context.
	var userID *int64
	if user, _ := authmiddleware.UserFromContext(ctx); user != nil {
		userID = &user.ID
	}

	doc, err := h.documentService.Create(ctx, service.CreateDocumentParams{
		Title:       input.Title,
		Content:     input.Content,
		FileType:    input.FileType,
		Description: input.Description,
		IsPublic:    input.IsPublic,
		Tags:        input.Tags,
		UserID:      userID,
	})
	if err != nil {
		return nil, createDocumentResponse{}, fmt.Errorf("creating document: %w", err)
	}

	return nil, createDocumentResponse{
		Success: true,
		Message: fmt.Sprintf("Document %q created successfully.", doc.Title),
		Document: &documentRef{
			UUID:      doc.UUID,
			Title:     doc.Title,
			FileType:  doc.FileType,
			CreatedAt: formatNullTime(doc.CreatedAt),
		},
	}, nil
}

func (h *Handler) handleUpdateDocument(
	ctx context.Context,
	_ *mcp.CallToolRequest,
	input dto.UpdateDocumentInput,
) (*mcp.CallToolResult, updateDocumentResponse, error) {
	if err := requireMCPScope(ctx, authscope.MCPWrite); err != nil {
		return nil, updateDocumentResponse{}, errors.New("mcp:write scope required for document updates")
	}
	// Non-admin users can only update their own documents.
	if err := h.checkDocumentOwnership(ctx, input.UUID); err != nil {
		return nil, updateDocumentResponse{}, err
	}

	doc, err := h.documentService.Update(ctx, input.UUID, service.UpdateDocumentParams{
		Title:       input.Title,
		Description: input.Description,
		IsPublic:    input.IsPublic,
		Tags:        input.Tags,
	})
	if err != nil {
		return nil, updateDocumentResponse{}, fmt.Errorf("updating document: %w", err)
	}

	return nil, updateDocumentResponse{
		Success: true,
		Message: fmt.Sprintf("Document %q updated successfully.", doc.Title),
		Document: &documentRef{
			UUID:      doc.UUID,
			Title:     doc.Title,
			FileType:  doc.FileType,
			CreatedAt: formatNullTime(doc.CreatedAt),
		},
	}, nil
}

func (h *Handler) handleDeleteDocument(
	ctx context.Context,
	_ *mcp.CallToolRequest,
	input dto.DeleteDocumentInput,
) (*mcp.CallToolResult, deleteDocumentResponse, error) {
	if err := requireMCPScope(ctx, authscope.MCPWrite); err != nil {
		return nil, deleteDocumentResponse{}, errors.New("mcp:write scope required for document deletion")
	}
	// Non-admin users can only delete their own documents.
	if err := h.checkDocumentOwnership(ctx, input.UUID); err != nil {
		return nil, deleteDocumentResponse{}, err
	}

	if err := h.documentService.Delete(ctx, input.UUID); err != nil {
		return nil, deleteDocumentResponse{}, fmt.Errorf("deleting document: %w", err)
	}

	return nil, deleteDocumentResponse{
		Success: true,
		Message: "Document deleted successfully.",
		UUID:    input.UUID,
	}, nil
}

// --- Helpers ---

// checkDocumentOwnership verifies that the current user owns the document or is an admin.
// Returns an error if the user is not authorized.
func (h *Handler) checkDocumentOwnership(ctx context.Context, uuid string) error {
	doc, err := h.documentService.FindByUUID(ctx, uuid)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) || errors.Is(err, service.ErrNotFound) {
			return errors.New("document not found")
		}
		return fmt.Errorf("finding document: %w", err)
	}

	user, _ := authmiddleware.UserFromContext(ctx)
	if user == nil {
		// M2M tokens (no user context) cannot modify documents.
		return errors.New("document not found")
	} else if !user.IsAdmin {
		if !doc.UserID.Valid || doc.UserID.Int64 != user.ID {
			return errors.New("document not found")
		}
	}

	return nil
}

// buildDocumentMeta converts a model.Document and its tags into a documentMeta response.
func buildDocumentMeta(doc *model.Document, tags []model.DocumentTag) *documentMeta {
	return &documentMeta{
		UUID:        doc.UUID,
		Title:       doc.Title,
		Description: doc.Description.String,
		FileType:    doc.FileType,
		WordCount:   doc.WordCount.Int64,
		IsPublic:    doc.IsPublic,
		Tags:        tagNames(tags),
		ContentHash: doc.ContentHash.String,
		CreatedAt:   formatNullTime(doc.CreatedAt),
		UpdatedAt:   formatNullTime(doc.UpdatedAt),
		ProcessedAt: formatNullTime(doc.ProcessedAt),
	}
}

// tagNames extracts tag name strings from a slice of DocumentTag models.
func tagNames(tags []model.DocumentTag) []string {
	names := make([]string, len(tags))
	for i, t := range tags {
		names[i] = t.Tag
	}
	return names
}

// truncateContent applies summary_only or max_paragraphs truncation to content.
// It returns the (possibly truncated) content and whether truncation occurred.
func truncateContent(content string, summaryOnly bool, maxParagraphs int) (string, bool) {
	if content == "" {
		return content, false
	}

	if summaryOnly {
		// Return content before the first heading (# or ##).
		lines := strings.Split(content, "\n")
		var result []string
		for _, line := range lines {
			trimmed := strings.TrimSpace(line)
			if strings.HasPrefix(trimmed, "#") && len(result) > 0 {
				break
			}
			result = append(result, line)
		}
		truncated := strings.Join(result, "\n")
		return strings.TrimSpace(truncated), len(truncated) < len(content)
	}

	if maxParagraphs > 0 {
		paragraphs := strings.Split(content, "\n\n")
		if maxParagraphs < len(paragraphs) {
			truncated := strings.Join(paragraphs[:maxParagraphs], "\n\n")
			return truncated, true
		}
	}

	return content, false
}

// formatNullTime formats a sql.NullTime as RFC3339.
// Returns an empty string when the time is not valid (NULL).
func formatNullTime(t sql.NullTime) string {
	if !t.Valid {
		return ""
	}
	return t.Time.Format(time.RFC3339)
}
