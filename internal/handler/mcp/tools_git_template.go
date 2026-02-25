package mcphandler

import (
	"archive/tar"
	"archive/zip"
	"bytes"
	"compress/gzip"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"git.999.haus/chris/DocuMCP-go/internal/dto"
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

type searchGitTemplatesResponse struct {
	Success bool   `json:"success"`
	Message string `json:"message,omitempty"`
	Query   string `json:"query"`
	Results []any  `json:"results"`
	Total   int    `json:"total"`
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

// archiveEntry holds a single file's path and content for archive creation.
type archiveEntry struct {
	path    string
	content string
}

// --- Variable substitution ---

var variablePattern = regexp.MustCompile(`\{\{(\w+)\}\}`)

// substituteVariables replaces {{key}} placeholders in content using the
// provided variable map. It returns the substituted content and a list of
// any unresolved variable names.
func substituteVariables(content string, variables map[string]string) (string, []string) {
	var unresolved []string
	result := content

	matches := variablePattern.FindAllStringSubmatch(content, -1)
	seen := make(map[string]bool)
	for _, match := range matches {
		key := match[1]
		if val, ok := variables[key]; ok {
			result = strings.ReplaceAll(result, "{{"+key+"}}", val)
		} else if !seen[key] {
			unresolved = append(unresolved, key)
			seen[key] = true
		}
	}
	return result, unresolved
}

// parseVariablesJSON decodes a JSON string into a map of variable substitutions.
// Returns an empty map if the input is empty.
func parseVariablesJSON(raw string) (map[string]string, error) {
	if raw == "" {
		return map[string]string{}, nil
	}
	var vars map[string]string
	if err := json.Unmarshal([]byte(raw), &vars); err != nil {
		return nil, fmt.Errorf("decoding variables JSON: %w", err)
	}
	return vars, nil
}

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
		Description: "Full-text search across Git template README files and metadata.\n\n" +
			"Use this to find templates by:\n" +
			"- Content (searches README.md)\n" +
			"- Purpose or technology\n" +
			"- Template name or description\n\n" +
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
			"~45 API calls to 1.\n\n" +
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
	limit := input.Limit
	if limit <= 0 {
		limit = 50
	}
	if limit > 100 {
		limit = 100
	}

	templates, err := h.gitTemplateRepo.List(ctx, input.Category, limit)
	if err != nil {
		return nil, listGitTemplatesResponse{}, fmt.Errorf("listing git templates: %w", err)
	}

	items := make([]gitTemplateItem, 0, len(templates))
	for _, gt := range templates {
		item := gitTemplateItem{
			UUID:      gt.UUID,
			Name:      gt.Name,
			FileCount: gt.FileCount,
			TotalSize: gt.TotalSizeBytes,
			Status:    gt.Status,
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
	_ context.Context,
	_ *mcp.CallToolRequest,
	input dto.SearchGitTemplatesInput,
) (*mcp.CallToolResult, searchGitTemplatesResponse, error) {
	return nil, searchGitTemplatesResponse{
		Success: false,
		Message: "Meilisearch search not yet implemented",
		Query:   input.Query,
		Results: []any{},
		Total:   0,
	}, nil
}

func (h *Handler) handleGetTemplateStructure(
	ctx context.Context,
	_ *mcp.CallToolRequest,
	input dto.GetTemplateStructureInput,
) (*mcp.CallToolResult, getTemplateStructureResponse, error) {
	tmpl, err := h.gitTemplateRepo.FindByUUID(ctx, input.UUID)
	if err != nil {
		return nil, getTemplateStructureResponse{
			Success: false,
			Message: fmt.Sprintf("Template %s not found", input.UUID),
		}, nil
	}

	files, err := h.gitTemplateRepo.FilesForTemplate(ctx, tmpl.ID)
	if err != nil {
		return nil, getTemplateStructureResponse{}, fmt.Errorf("listing template files: %w", err)
	}

	fileTree := make([]string, 0, len(files))
	essentialFiles := make([]string, 0)
	variableSet := make(map[string]bool)

	for _, f := range files {
		fileTree = append(fileTree, f.Path)

		if f.IsEssential {
			essentialFiles = append(essentialFiles, f.Path)
		}

		// Extract {{variables}} from file content.
		if f.Content.Valid {
			matches := variablePattern.FindAllStringSubmatch(f.Content.String, -1)
			for _, match := range matches {
				variableSet[match[1]] = true
			}
		}
	}

	variables := make([]string, 0, len(variableSet))
	for v := range variableSet {
		variables = append(variables, v)
	}

	detail := &templateStructureDetail{
		UUID:           tmpl.UUID,
		Name:           tmpl.Name,
		FileTree:       fileTree,
		EssentialFiles: essentialFiles,
		Variables:      variables,
		FileCount:      tmpl.FileCount,
		TotalSize:      tmpl.TotalSizeBytes,
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
	tmpl, err := h.gitTemplateRepo.FindByUUID(ctx, input.UUID)
	if err != nil {
		return nil, getTemplateFileResponse{
			Success: false,
			Message: fmt.Sprintf("Template %s not found", input.UUID),
		}, nil
	}

	file, err := h.gitTemplateRepo.FindFileByPath(ctx, tmpl.ID, input.Path)
	if err != nil {
		return nil, getTemplateFileResponse{
			Success: false,
			Message: fmt.Sprintf("File %q not found in template %s", input.Path, input.UUID),
		}, nil
	}

	variables, err := parseVariablesJSON(input.Variables)
	if err != nil {
		return nil, getTemplateFileResponse{}, fmt.Errorf("parsing variables: %w", err)
	}

	content := file.Content.String
	var unresolved []string
	if len(variables) > 0 {
		content, unresolved = substituteVariables(content, variables)
	}

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
		Content:             content,
		UnresolvedVariables: unresolved,
	}, nil
}

func (h *Handler) handleGetDeploymentGuide(
	ctx context.Context,
	_ *mcp.CallToolRequest,
	input dto.GetDeploymentGuideInput,
) (*mcp.CallToolResult, getDeploymentGuideResponse, error) {
	tmpl, err := h.gitTemplateRepo.FindByUUID(ctx, input.UUID)
	if err != nil {
		return nil, getDeploymentGuideResponse{
			Success: false,
			Message: fmt.Sprintf("Template %s not found", input.UUID),
		}, nil
	}

	files, err := h.gitTemplateRepo.FilesForTemplate(ctx, tmpl.ID)
	if err != nil {
		return nil, getDeploymentGuideResponse{}, fmt.Errorf("listing template files: %w", err)
	}

	variables, err := parseVariablesJSON(input.Variables)
	if err != nil {
		return nil, getDeploymentGuideResponse{}, fmt.Errorf("parsing variables: %w", err)
	}

	allUnresolved := make(map[string]bool)
	deploymentFiles := make([]deploymentFile, 0)

	for _, f := range files {
		if !f.IsEssential {
			continue
		}
		content := f.Content.String
		if len(variables) > 0 {
			var unresolved []string
			content, unresolved = substituteVariables(content, variables)
			for _, u := range unresolved {
				allUnresolved[u] = true
			}
		}
		deploymentFiles = append(deploymentFiles, deploymentFile{
			Path:    f.Path,
			Content: content,
		})
	}

	unresolvedList := make([]string, 0, len(allUnresolved))
	for v := range allUnresolved {
		unresolvedList = append(unresolvedList, v)
	}

	description := ""
	if tmpl.Description.Valid {
		description = tmpl.Description.String
	}

	guide := &deploymentGuide{
		TemplateName:        tmpl.Name,
		Description:         description,
		Steps:               []string{"Create the following files in your project directory."},
		Files:               deploymentFiles,
		UnresolvedVariables: unresolvedList,
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
	tmpl, err := h.gitTemplateRepo.FindByUUID(ctx, input.UUID)
	if err != nil {
		return nil, downloadTemplateResponse{
			Success: false,
			Message: fmt.Sprintf("Template %s not found", input.UUID),
		}, nil
	}

	files, err := h.gitTemplateRepo.FilesForTemplate(ctx, tmpl.ID)
	if err != nil {
		return nil, downloadTemplateResponse{}, fmt.Errorf("listing template files: %w", err)
	}

	variables, err := parseVariablesJSON(input.Variables)
	if err != nil {
		return nil, downloadTemplateResponse{}, fmt.Errorf("parsing variables: %w", err)
	}

	format := input.Format
	if format == "" {
		format = "zip"
	}

	allUnresolved := make(map[string]bool)

	// Prepare file contents with optional variable substitution.
	entries := make([]archiveEntry, 0, len(files))
	for _, f := range files {
		content := f.Content.String
		if len(variables) > 0 {
			var unresolved []string
			content, unresolved = substituteVariables(content, variables)
			for _, u := range unresolved {
				allUnresolved[u] = true
			}
		}
		entries = append(entries, archiveEntry{path: f.Path, content: content})
	}

	var buf bytes.Buffer
	var filename string

	switch format {
	case "tar.gz":
		if err := buildTarGz(&buf, entries); err != nil {
			return nil, downloadTemplateResponse{}, fmt.Errorf("creating tar.gz archive: %w", err)
		}
		filename = tmpl.Slug + ".tar.gz"
	default:
		format = "zip"
		if err := buildZip(&buf, entries); err != nil {
			return nil, downloadTemplateResponse{}, fmt.Errorf("creating zip archive: %w", err)
		}
		filename = tmpl.Slug + ".zip"
	}

	encoded := base64.StdEncoding.EncodeToString(buf.Bytes())

	unresolvedList := make([]string, 0, len(allUnresolved))
	for v := range allUnresolved {
		unresolvedList = append(unresolvedList, v)
	}

	return nil, downloadTemplateResponse{
		Success:             true,
		Template:            tmpl.Name,
		Filename:            filename,
		Format:              format,
		SizeBytes:           buf.Len(),
		FileCount:           len(entries),
		ArchiveBase64:       encoded,
		UnresolvedVariables: unresolvedList,
		Usage:               fmt.Sprintf("Decode the base64 archive_base64 field and save as %s", filename),
	}, nil
}

// --- Archive helpers ---

// buildZip writes a zip archive containing the given entries to w.
func buildZip(w *bytes.Buffer, entries []archiveEntry) error {
	zw := zip.NewWriter(w)
	for _, e := range entries {
		fw, err := zw.Create(e.path)
		if err != nil {
			return fmt.Errorf("creating zip entry %q: %w", e.path, err)
		}
		if _, err := fw.Write([]byte(e.content)); err != nil {
			return fmt.Errorf("writing zip entry %q: %w", e.path, err)
		}
	}
	return zw.Close()
}

// buildTarGz writes a gzip-compressed tar archive containing the given entries to w.
func buildTarGz(w *bytes.Buffer, entries []archiveEntry) error {
	gw := gzip.NewWriter(w)
	tw := tar.NewWriter(gw)

	for _, e := range entries {
		hdr := &tar.Header{
			Name: e.path,
			Mode: 0644,
			Size: int64(len(e.content)),
		}
		if err := tw.WriteHeader(hdr); err != nil {
			return fmt.Errorf("writing tar header for %q: %w", e.path, err)
		}
		if _, err := tw.Write([]byte(e.content)); err != nil {
			return fmt.Errorf("writing tar entry %q: %w", e.path, err)
		}
	}

	if err := tw.Close(); err != nil {
		return fmt.Errorf("closing tar writer: %w", err)
	}
	return gw.Close()
}
