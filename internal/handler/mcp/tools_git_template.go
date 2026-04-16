package mcphandler

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	authscope "github.com/c-premus/documcp/internal/auth/scope"
	gitclient "github.com/c-premus/documcp/internal/client/git"
	"github.com/c-premus/documcp/internal/dto"
	"github.com/c-premus/documcp/internal/search"
	"github.com/c-premus/documcp/internal/service"
)

// --- Response types ---

type listGitTemplatesResponse struct {
	Success   bool              `json:"success"`
	Templates []gitTemplateItem `json:"templates"`
	Count     int               `json:"count"`
}

type gitTemplateItem struct {
	UUID         string   `json:"uuid"`
	Name         string   `json:"name"`
	Description  string   `json:"description,omitempty"`
	Category     string   `json:"category,omitempty"`
	Tags         []string `json:"tags,omitempty"`
	FileCount    int      `json:"file_count"`
	TotalSize    int64    `json:"total_size"`
	Status       string   `json:"status"`
	CreatedAt    string   `json:"created_at,omitempty"`
	UpdatedAt    string   `json:"updated_at,omitempty"`
	LastSyncedAt string   `json:"last_synced_at,omitempty"`
}

type gitTemplateSearchResult struct {
	UUID         string   `json:"uuid"`
	Name         string   `json:"name"`
	Description  string   `json:"description,omitempty"`
	Category     string   `json:"category,omitempty"`
	FileCount    int      `json:"file_count,omitempty"`
	Status       string   `json:"status,omitempty"`
	MatchedFiles []string `json:"matched_files,omitempty"`
}

type searchGitTemplatesResponse struct {
	Success bool                      `json:"success"`
	Message string                    `json:"message,omitempty"`
	Query   string                    `json:"query"`
	Results []gitTemplateSearchResult `json:"results"`
	Total   int                       `json:"total"`
}

type getTemplateStructureResponse struct {
	Success  bool                     `json:"success"`
	Template *templateStructureDetail `json:"template"`
	Message  string                   `json:"message,omitempty"`
}

type templateStructureDetail struct {
	UUID           string   `json:"uuid"`
	Name           string   `json:"name"`
	Description    string   `json:"description,omitempty"`
	Category       string   `json:"category,omitempty"`
	FileTree       []string `json:"file_tree"`
	EssentialFiles []string `json:"essential_files"`
	Variables      []string `json:"variables"`
	FileCount      int      `json:"file_count"`
	TotalSize      int64    `json:"total_size"`
}

type getTemplateFileResponse struct {
	Success             bool              `json:"success"`
	File                *templateFileInfo `json:"file,omitempty"`
	Content             string            `json:"content,omitempty"`
	UnresolvedVariables []string          `json:"unresolved_variables,omitempty"`
	Message             string            `json:"message,omitempty"`
}

type templateFileInfo struct {
	Path        string `json:"path"`
	Filename    string `json:"filename"`
	Extension   string `json:"extension,omitempty"`
	SizeBytes   int64  `json:"size_bytes"`
	IsEssential bool   `json:"is_essential"`
	ContentHash string `json:"content_hash,omitempty"`
	CreatedAt   string `json:"created_at,omitempty"`
	UpdatedAt   string `json:"updated_at,omitempty"`
}

type getDeploymentGuideResponse struct {
	Success bool             `json:"success"`
	Guide   *deploymentGuide `json:"guide,omitempty"`
	Message string           `json:"message,omitempty"`
}

type deploymentGuide struct {
	TemplateName        string           `json:"template_name"`
	Description         string           `json:"description,omitempty"`
	Steps               []string         `json:"steps"`
	Files               []deploymentFile `json:"files"`
	UnresolvedVariables []string         `json:"unresolved_variables,omitempty"`
}

type deploymentFile struct {
	Path    string `json:"path"`
	Content string `json:"content"`
}

type downloadTemplateResponse struct {
	Success             bool     `json:"success"`
	Template            string   `json:"template,omitempty"`
	Filename            string   `json:"filename,omitempty"`
	Format              string   `json:"format,omitempty"`
	SizeBytes           int      `json:"size_bytes,omitempty"`
	FileCount           int      `json:"file_count,omitempty"`
	ArchiveBase64       string   `json:"archive_base64,omitempty"`
	UnresolvedVariables []string `json:"unresolved_variables,omitempty"`
	Usage               string   `json:"usage,omitempty"`
	Message             string   `json:"message,omitempty"`
}

// --- Variable substitution ---

// --- Tool registration ---

// registerGitTemplateTools registers git template tools (list, search, structure, file, deploy, download).
func (h *Handler) registerGitTemplateTools() {
	h.registerListGitTemplates()
	h.registerSearchGitTemplates()
	h.registerGetTemplateStructure()
	h.registerGetTemplateFile()
	h.registerGetDeploymentGuide()
	h.registerDownloadTemplate()
}

func (h *Handler) registerListGitTemplates() {
	mcp.AddTool(h.server, &mcp.Tool{
		Name: "list_git_templates",
		Description: "List available Git templates for project bootstrapping.\n\n" +
			"**Template Categories:**\n" +
			"- `claude`: CLAUDE.md templates with AI assistant instructions\n" +
			"- `memory-bank`: Memory Bank documentation structures\n" +
			"- `project`: Full project scaffolding templates\n\n" +
			"Returns template name, description, category, file count, and sync status.\n" +
			"Use `get_template_structure` for detailed file listing.\n\n" +
			"**Note:** Only text files are synced from repositories. Binary files (PDFs, " +
			"images, compiled artifacts) are excluded. Per-file limit: 1 MB, total: 10 MB.\n\n" +
			"**Workflow:** Use the `uuid` field with `get_template_structure` to see files " +
			"and variables, or `get_deployment_guide` to get all essential files at once.",
		Annotations: &mcp.ToolAnnotations{
			ReadOnlyHint:   true,
			IdempotentHint: true,
		},
	}, h.handleListGitTemplates)
}

func (h *Handler) registerSearchGitTemplates() {
	mcp.AddTool(h.server, &mcp.Tool{
		Name: "search_git_templates",
		Description: "Full-text search across Git template files, README content, and metadata.\n\n" +
			"Use this to find templates by:\n" +
			"- File content (searches all template files)\n" +
			"- Purpose or technology\n" +
			"- Template name, description, or filenames\n\n" +
			"Results include `matched_files` paths for direct use with `get_template_file`.\n\n" +
			"Returns matching templates with relevance ranking.\n" +
			"Use `get_template_structure` for detailed file listing of a specific template.\n\n" +
			"**Workflow:** Use `uuid` from results with `get_template_structure` to explore " +
			"files, or `get_deployment_guide` for complete bootstrap instructions.",
		Annotations: &mcp.ToolAnnotations{
			ReadOnlyHint:   true,
			IdempotentHint: true,
		},
	}, h.handleSearchGitTemplates)
}

func (h *Handler) registerGetTemplateStructure() {
	mcp.AddTool(h.server, &mcp.Tool{
		Name: "get_template_structure",
		Description: "Get the structure and metadata of a Git template.\n\n" +
			"Returns:\n" +
			"- File tree showing all template files\n" +
			"- List of essential files (CLAUDE.md, memory-bank/*, etc.)\n" +
			"- Required variables ({{project_name}}, etc.)\n" +
			"- Template manifest if available\n\n" +
			"File count reflects text files only — binary files (PDFs, images) are " +
			"excluded during sync.\n\n" +
			"Use this before `get_template_file` to understand the template contents.\n\n" +
			"**Workflow:** Use file paths with `get_template_file` to read individual files, " +
			"or `get_deployment_guide` to get all essential files with variable substitution.",
		Annotations: &mcp.ToolAnnotations{
			ReadOnlyHint:   true,
			IdempotentHint: true,
		},
	}, h.handleGetTemplateStructure)
}

func (h *Handler) registerGetTemplateFile() {
	mcp.AddTool(h.server, &mcp.Tool{
		Name: "get_template_file",
		Description: "Retrieve a specific file from a Git template.\n\n" +
			"Optionally provide variables to substitute {{placeholders}} in the content.\n\n" +
			"**Common Variables:**\n" +
			"- `project_name`: Project identifier\n" +
			"- `project_description`: Brief description\n" +
			"- `author`: Author name\n" +
			"- `date`: Current date\n\n" +
			"Returns file content with substitutions applied.",
		Annotations: &mcp.ToolAnnotations{
			ReadOnlyHint:   true,
			IdempotentHint: true,
		},
	}, h.handleGetTemplateFile)
}

func (h *Handler) registerGetDeploymentGuide() {
	mcp.AddTool(h.server, &mcp.Tool{
		Name: "get_deployment_guide",
		Description: "Generate a deployment guide with all essential files for a Git template.\n\n" +
			"This tool provides:\n" +
			"- Step-by-step deployment instructions\n" +
			"- All essential file contents (CLAUDE.md, memory-bank/*, etc.)\n" +
			"- Variable substitution applied to content\n" +
			"- List of any unresolved variables\n\n" +
			"Use this to bootstrap a new project with the template structure.\n" +
			"The AI agent can then write these files to the target project.",
		Annotations: &mcp.ToolAnnotations{
			ReadOnlyHint:   true,
			IdempotentHint: true,
		},
	}, h.handleGetDeploymentGuide)
}

func (h *Handler) registerDownloadTemplate() {
	mcp.AddTool(h.server, &mcp.Tool{
		Name: "download_template",
		Description: "Download a complete Git template as a base64-encoded archive (zip or tar.gz).\n\n" +
			"Returns the entire template in a single response, reducing deployment from " +
			"~45 API calls to 1. Contains text files only — binary files (PDFs, images) " +
			"are excluded during sync.\n\n" +
			"**Formats:** zip (default), tar.gz\n\n" +
			"Optionally apply variable substitutions ({{KEY}} placeholders) to all files. " +
			"Decode the base64 `archive_base64` field and save as the indicated filename.",
		Annotations: &mcp.ToolAnnotations{
			ReadOnlyHint:   true,
			IdempotentHint: true,
		},
	}, h.handleDownloadTemplate)
}

// --- Tool handlers ---

func (h *Handler) handleListGitTemplates(
	ctx context.Context,
	_ *mcp.CallToolRequest,
	input dto.ListGitTemplatesInput,
) (*mcp.CallToolResult, listGitTemplatesResponse, error) {
	if err := requireMCPScope(ctx, authscope.MCPRead); err != nil {
		return nil, listGitTemplatesResponse{}, errors.New("mcp:read scope required")
	}
	limit := clampPagination(input.Limit, 50, 100)

	templates, err := h.gitTemplateRepo.List(ctx, input.Category, limit, 0)
	if err != nil {
		return nil, listGitTemplatesResponse{}, fmt.Errorf("listing git templates: %w", err)
	}

	items := make([]gitTemplateItem, 0, len(templates))
	for i := range templates {
		gt := &templates[i]
		item := gitTemplateItem{
			UUID:      gt.UUID,
			Name:      gt.Name,
			FileCount: gt.FileCount,
			TotalSize: gt.TotalSizeBytes,
			Status:    string(gt.Status),
		}
		if gt.Description.Valid {
			item.Description = gt.Description.String
		}
		if gt.Category.Valid {
			item.Category = gt.Category.String
		}

		tags, tagErr := gt.ParseTags()
		if tagErr != nil {
			h.logger.WarnContext(ctx, "failed to parse tags for template",
				"uuid", gt.UUID, "error", tagErr)
		}
		item.Tags = tags

		if gt.CreatedAt.Valid {
			item.CreatedAt = gt.CreatedAt.Time.Format(time.RFC3339)
		}
		if gt.UpdatedAt.Valid {
			item.UpdatedAt = gt.UpdatedAt.Time.Format(time.RFC3339)
		}
		if gt.LastSyncedAt.Valid {
			item.LastSyncedAt = gt.LastSyncedAt.Time.Format(time.RFC3339)
		}

		items = append(items, item)
	}

	return nil, listGitTemplatesResponse{
		Success:   true,
		Templates: items,
		Count:     len(items),
	}, nil
}

func (h *Handler) handleSearchGitTemplates(
	ctx context.Context,
	_ *mcp.CallToolRequest,
	input dto.SearchGitTemplatesInput,
) (*mcp.CallToolResult, searchGitTemplatesResponse, error) {
	if err := requireMCPScope(ctx, authscope.MCPRead); err != nil {
		return nil, searchGitTemplatesResponse{}, errors.New("mcp:read scope required")
	}
	if len(input.Query) > search.MaxQueryLength {
		return nil, searchGitTemplatesResponse{}, fmt.Errorf("query must be at most %d characters", search.MaxQueryLength)
	}

	limit := int64(clampPagination(input.Limit, 10, 50))

	if h.searcher == nil {
		return nil, searchGitTemplatesResponse{Message: "Search service not configured"}, nil
	}

	params := search.SearchParams{
		Query:    input.Query,
		IndexUID: search.IndexGitTemplates,
		Limit:    limit,
	}
	if input.Category != "" {
		params.Category = input.Category
	}

	resp, err := h.searcher.Search(ctx, params)
	if err != nil {
		return nil, searchGitTemplatesResponse{}, fmt.Errorf("searching git templates: %w", err)
	}

	// Build template-level results.
	resultMap := make(map[string]*gitTemplateSearchResult, len(resp.Hits))
	results := make([]gitTemplateSearchResult, 0, len(resp.Hits))
	for _, sr := range resp.Hits {
		r := gitTemplateSearchResult{
			UUID:        sr.UUID,
			Name:        sr.Title,
			Description: sr.Description,
			Category:    search.ExtraString(sr.Extra, "category"),
			FileCount:   search.ExtraInt(sr.Extra, "file_count"),
			Status:      search.ExtraString(sr.Extra, "status"),
		}
		results = append(results, r)
		resultMap[sr.UUID] = &results[len(results)-1]
	}

	// Also search file-level content for matched_files.
	fileResults, fileErr := h.searcher.SearchGitTemplateFiles(ctx, input.Query, limit)
	if fileErr == nil {
		for _, fr := range fileResults {
			if existing, ok := resultMap[fr.TemplateUUID]; ok {
				existing.MatchedFiles = append(existing.MatchedFiles, fr.FilePath)
			} else {
				// File matched but template-level didn't — add as new result.
				r := gitTemplateSearchResult{
					UUID:         fr.TemplateUUID,
					Name:         fr.TemplateName,
					MatchedFiles: []string{fr.FilePath},
				}
				results = append(results, r)
				resultMap[fr.TemplateUUID] = &results[len(results)-1]
			}
		}
	}

	return nil, searchGitTemplatesResponse{
		Success: true,
		Query:   input.Query,
		Results: results,
		Total:   len(results),
	}, nil
}

func (h *Handler) handleGetTemplateStructure(
	ctx context.Context,
	_ *mcp.CallToolRequest,
	input dto.GetTemplateStructureInput,
) (*mcp.CallToolResult, getTemplateStructureResponse, error) {
	if err := requireMCPScope(ctx, authscope.MCPRead); err != nil {
		return nil, getTemplateStructureResponse{}, errors.New("mcp:read scope required")
	}

	structure, err := h.gitTemplateService.Structure(ctx, input.UUID)
	if err != nil {
		if errors.Is(err, service.ErrGitTemplateNotFound) {
			return nil, getTemplateStructureResponse{
				Success: false,
				Message: "Template " + input.UUID + " not found",
			}, nil
		}
		return nil, getTemplateStructureResponse{}, fmt.Errorf("getting template structure: %w", err)
	}

	tmpl := structure.Template
	detail := &templateStructureDetail{
		UUID:           tmpl.UUID,
		Name:           tmpl.Name,
		FileTree:       structure.FileTree,
		EssentialFiles: structure.EssentialFiles,
		Variables:      structure.Variables,
		FileCount:      structure.FileCount,
		TotalSize:      structure.TotalSize,
	}
	if tmpl.Description.Valid {
		detail.Description = tmpl.Description.String
	}
	if tmpl.Category.Valid {
		detail.Category = tmpl.Category.String
	}

	return nil, getTemplateStructureResponse{
		Success:  true,
		Template: detail,
	}, nil
}

func (h *Handler) handleGetTemplateFile(
	ctx context.Context,
	_ *mcp.CallToolRequest,
	input dto.GetTemplateFileInput,
) (*mcp.CallToolResult, getTemplateFileResponse, error) {
	if err := requireMCPScope(ctx, authscope.MCPRead); err != nil {
		return nil, getTemplateFileResponse{}, errors.New("mcp:read scope required")
	}

	if err := gitclient.ValidateVariables(input.Variables); err != nil {
		return nil, getTemplateFileResponse{}, fmt.Errorf("validating variables: %w", err)
	}

	result, err := h.gitTemplateService.File(ctx, input.UUID, input.Path, input.Variables)
	if err != nil {
		if errors.Is(err, service.ErrGitTemplateNotFound) {
			return nil, getTemplateFileResponse{
				Success: false,
				Message: fmt.Sprintf("File %q not found in template %s", input.Path, input.UUID),
			}, nil
		}
		return nil, getTemplateFileResponse{}, fmt.Errorf("getting template file: %w", err)
	}

	file := result.File
	info := &templateFileInfo{
		Path:        file.Path,
		Filename:    file.Filename,
		SizeBytes:   file.SizeBytes,
		IsEssential: file.IsEssential,
	}
	if file.Extension.Valid {
		info.Extension = file.Extension.String
	}
	if file.ContentHash.Valid {
		info.ContentHash = file.ContentHash.String
	}
	if file.CreatedAt.Valid {
		info.CreatedAt = file.CreatedAt.Time.Format(time.RFC3339)
	}
	if file.UpdatedAt.Valid {
		info.UpdatedAt = file.UpdatedAt.Time.Format(time.RFC3339)
	}

	return nil, getTemplateFileResponse{
		Success:             true,
		File:                info,
		Content:             result.Content,
		UnresolvedVariables: result.Unresolved,
	}, nil
}

func (h *Handler) handleGetDeploymentGuide(
	ctx context.Context,
	_ *mcp.CallToolRequest,
	input dto.GetDeploymentGuideInput,
) (*mcp.CallToolResult, getDeploymentGuideResponse, error) {
	if err := requireMCPScope(ctx, authscope.MCPRead); err != nil {
		return nil, getDeploymentGuideResponse{}, errors.New("mcp:read scope required")
	}

	if err := gitclient.ValidateVariables(input.Variables); err != nil {
		return nil, getDeploymentGuideResponse{}, fmt.Errorf("validating variables: %w", err)
	}

	result, err := h.gitTemplateService.DeploymentGuide(ctx, input.UUID, input.Variables)
	if err != nil {
		if errors.Is(err, service.ErrGitTemplateNotFound) {
			return nil, getDeploymentGuideResponse{
				Success: false,
				Message: "Template " + input.UUID + " not found",
			}, nil
		}
		return nil, getDeploymentGuideResponse{}, fmt.Errorf("getting deployment guide: %w", err)
	}

	tmpl := result.Template
	files := make([]deploymentFile, 0, len(result.Files))
	for _, f := range result.Files {
		files = append(files, deploymentFile{
			Path:    f.Path,
			Content: f.Content,
		})
	}

	description := ""
	if tmpl.Description.Valid {
		description = tmpl.Description.String
	}

	guide := &deploymentGuide{
		TemplateName:        tmpl.Name,
		Description:         description,
		Steps:               result.Steps,
		Files:               files,
		UnresolvedVariables: result.Unresolved,
	}

	return nil, getDeploymentGuideResponse{
		Success: true,
		Guide:   guide,
	}, nil
}

func (h *Handler) handleDownloadTemplate(
	ctx context.Context,
	_ *mcp.CallToolRequest,
	input dto.DownloadTemplateInput,
) (*mcp.CallToolResult, downloadTemplateResponse, error) {
	if err := requireMCPScope(ctx, authscope.MCPRead); err != nil {
		return nil, downloadTemplateResponse{}, errors.New("mcp:read scope required")
	}

	if err := gitclient.ValidateVariables(input.Variables); err != nil {
		return nil, downloadTemplateResponse{}, fmt.Errorf("validating variables: %w", err)
	}

	result, err := h.gitTemplateService.BuildArchive(ctx, input.UUID, input.Format, input.Variables)
	if err != nil {
		if errors.Is(err, service.ErrGitTemplateNotFound) {
			return nil, downloadTemplateResponse{
				Success: false,
				Message: "Template " + input.UUID + " not found",
			}, nil
		}
		return nil, downloadTemplateResponse{}, fmt.Errorf("building template archive: %w", err)
	}

	return nil, downloadTemplateResponse{
		Success:             true,
		Template:            input.UUID,
		Filename:            result.Filename,
		Format:              result.Format,
		SizeBytes:           len(result.Data),
		FileCount:           result.FileCount,
		ArchiveBase64:       string(result.Data),
		UnresolvedVariables: result.Unresolved,
		Usage:               "Decode the base64 archive_base64 field and save as " + result.Filename,
	}, nil
}

// --- Archive helpers ---
