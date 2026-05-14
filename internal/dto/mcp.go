// Package dto defines data transfer objects for MCP tool inputs and outputs.
package dto

// --- Document tools ---.

// SearchDocumentsInput holds parameters for searching uploaded documents.
type SearchDocumentsInput struct {
	Query           string   `json:"query" jsonschema:"Search query text (supports keywords, phrases),required"`
	FileType        string   `json:"file_type,omitempty" jsonschema:"Filter by file type: markdown, pdf, docx, xlsx, html"`
	Tags            []string `json:"tags,omitempty" jsonschema:"Filter by document tags (AND logic)"`
	Limit           int      `json:"limit,omitempty" jsonschema:"Maximum results (default 10, max 100)"`
	IncludeSnippets bool     `json:"include_snippets,omitempty" jsonschema:"Include matched text context snippets"`
	IncludeContent  bool     `json:"include_content,omitempty" jsonschema:"Include full document content"`
}

// ReadDocumentInput holds parameters for reading a single document by UUID.
type ReadDocumentInput struct {
	UUID          string `json:"uuid" jsonschema:"UUID of the document to read,required"`
	SummaryOnly   bool   `json:"summary_only,omitempty" jsonschema:"Return only the lead section"`
	MaxParagraphs int    `json:"max_paragraphs,omitempty" jsonschema:"Limit content to first N paragraphs (1-100)"`
}

// CreateDocumentInput holds parameters for creating a new document.
type CreateDocumentInput struct {
	Title       string   `json:"title" jsonschema:"Document title (max 255 chars),required"`
	Content     string   `json:"content" jsonschema:"Full document content,required"`
	FileType    string   `json:"file_type" jsonschema:"Document file type: markdown or html,required"`
	Description string   `json:"description,omitempty" jsonschema:"Brief description or summary (max 1000 chars)"`
	IsPublic    bool     `json:"is_public,omitempty" jsonschema:"Make document publicly readable"`
	Tags        []string `json:"tags,omitempty" jsonschema:"Topic tags for categorization"`
}

// UpdateDocumentInput holds parameters for updating an existing document.
type UpdateDocumentInput struct {
	UUID        string   `json:"uuid" jsonschema:"Document UUID to update,required"`
	Title       string   `json:"title,omitempty" jsonschema:"New title (max 255 chars)"`
	Description string   `json:"description,omitempty" jsonschema:"New description (max 1000 chars)"`
	IsPublic    *bool    `json:"is_public,omitempty" jsonschema:"Update public visibility"`
	Tags        []string `json:"tags,omitempty" jsonschema:"New tags (replaces existing)"`
}

// ReplaceDocumentContentInput holds parameters for replacing the body of an
// inline (markdown / html) document. Metadata (title, description, tags,
// visibility) is preserved; only the content changes. Binary file-backed
// documents (pdf, docx, xlsx, epub) are rejected — those must be updated
// via the REST POST /api/documents/{uuid}/content endpoint.
type ReplaceDocumentContentInput struct {
	UUID    string `json:"uuid" jsonschema:"Document UUID,required"`
	Content string `json:"content" jsonschema:"New full document content (max 10 MB). Replaces the existing body entirely.,required"`
}

// DeleteDocumentInput holds parameters for deleting a document by UUID.
type DeleteDocumentInput struct {
	UUID string `json:"uuid" jsonschema:"UUID of document to delete,required"`
}

// ListDocumentsInput holds parameters for listing documents with optional filters.
type ListDocumentsInput struct {
	FileType string `json:"file_type,omitempty" jsonschema:"Filter by file type: markdown, pdf, docx, xlsx, html"`
	Status   string `json:"status,omitempty" jsonschema:"Filter by processing status: pending, indexed, failed"`
	Limit    int    `json:"limit,omitempty" jsonschema:"Maximum results (default 50, max 100)"`
	Offset   int    `json:"offset,omitempty" jsonschema:"Pagination offset (default 0)"`
}

// --- ZIM tools ---.

// ListZimArchivesInput holds parameters for listing available ZIM archives.
type ListZimArchivesInput struct {
	Query    string `json:"query,omitempty" jsonschema:"Search query to filter archives by title or description"`
	Category string `json:"category,omitempty" jsonschema:"Filter by category: devdocs, wikipedia, stack_exchange, other"`
	Language string `json:"language,omitempty" jsonschema:"Filter by language code (e.g. en, de, fr)"`
	Limit    int    `json:"limit,omitempty" jsonschema:"Max results (default 50, max 100)"`
}

// SearchZimInput holds parameters for searching within a ZIM archive.
type SearchZimInput struct {
	Archive    string `json:"archive" jsonschema:"Archive name to search (e.g. devdocs_en_laravel),required"`
	Query      string `json:"query" jsonschema:"Search query,required"`
	SearchType string `json:"search_type,omitempty" jsonschema:"Search type: fulltext (default) or suggest"`
	Limit      int    `json:"limit,omitempty" jsonschema:"Max results (default 20, max 50)"`
}

// ReadZimArticleInput holds parameters for reading an article from a ZIM archive.
type ReadZimArticleInput struct {
	Archive       string `json:"archive" jsonschema:"Archive name,required"`
	Path          string `json:"path" jsonschema:"Article path within the archive,required"`
	SummaryOnly   bool   `json:"summary_only,omitempty" jsonschema:"Return only the lead section"`
	MaxParagraphs int    `json:"max_paragraphs,omitempty" jsonschema:"Limit to first N paragraphs (1-100)"`
}

// --- Git Template tools ---.

// ListGitTemplatesInput holds parameters for listing available git templates.
type ListGitTemplatesInput struct {
	Category string `json:"category,omitempty" jsonschema:"Filter by category: claude, memory-bank, project"`
	Limit    int    `json:"limit,omitempty" jsonschema:"Max results (default 50, max 100)"`
}

// SearchGitTemplatesInput holds parameters for searching git templates.
type SearchGitTemplatesInput struct {
	Query    string `json:"query" jsonschema:"Search query text,required"`
	Category string `json:"category,omitempty" jsonschema:"Filter by category"`
	Limit    int    `json:"limit,omitempty" jsonschema:"Max results (default 10, max 50)"`
}

// GetTemplateStructureInput holds parameters for retrieving a template's file structure.
type GetTemplateStructureInput struct {
	UUID string `json:"uuid" jsonschema:"Template UUID,required"`
}

// GetTemplateFileInput holds parameters for reading a single file from a template.
type GetTemplateFileInput struct {
	UUID      string            `json:"uuid" jsonschema:"Template UUID,required"`
	Path      string            `json:"path" jsonschema:"File path within the template,required"`
	Variables map[string]string `json:"variables,omitempty" jsonschema:"Variables for {{placeholder}} substitution (keys are placeholder names, values are substitutions)"`
}

// GetDeploymentGuideInput holds parameters for retrieving a template's deployment guide.
type GetDeploymentGuideInput struct {
	UUID      string            `json:"uuid" jsonschema:"Template UUID,required"`
	Variables map[string]string `json:"variables,omitempty" jsonschema:"Variables for {{placeholder}} substitution"`
}

// DownloadTemplateInput holds parameters for downloading a template archive.
type DownloadTemplateInput struct {
	UUID      string            `json:"uuid" jsonschema:"Template UUID,required"`
	Format    string            `json:"format,omitempty" jsonschema:"Archive format: zip (default) or tar.gz"`
	Variables map[string]string `json:"variables,omitempty" jsonschema:"Variables for {{placeholder}} substitution"`
}

// --- Unified Search ---.

// UnifiedSearchInput holds parameters for searching across all content sources.
//
// unified_search is a discovery tool. It returns a single page of top-ranked
// results merged across sources; pagination is not supported because the
// Kiwix fan-out cannot honor an offset. Callers that need to walk the full
// result set should use the type-specific tools (search_documents, search_zim,
// search_git_templates).
type UnifiedSearchInput struct {
	Query string   `json:"query" jsonschema:"Search text (1-255 characters),required"`
	Types []string `json:"types,omitempty" jsonschema:"Filter to specific content types: document, git_template, zim_archive, zim_article"`
	Limit int      `json:"limit,omitempty" jsonschema:"Max results across all sources (default 20, max 100)"`
}
