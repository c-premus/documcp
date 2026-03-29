package api

import (
	"archive/tar"
	"archive/zip"
	"bytes"
	"compress/gzip"
	"context"
	"database/sql"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"net/url"
	"path/filepath"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/riverqueue/river"
	"github.com/riverqueue/river/rivertype"

	gitclient "github.com/c-premus/documcp/internal/client/git"
	"github.com/c-premus/documcp/internal/model"
	"github.com/c-premus/documcp/internal/queue"
	"github.com/c-premus/documcp/internal/security"
	"github.com/c-premus/documcp/internal/stringutil"
)

// gitTemplateRepo defines the methods used by GitTemplateHandler -- defined where consumed.
type gitTemplateRepo interface {
	List(ctx context.Context, category string, limit, offset int) ([]model.GitTemplate, error)
	CountFiltered(ctx context.Context, category string) (int, error)
	Search(ctx context.Context, query, category string, limit int) ([]model.GitTemplate, error)
	FindByUUID(ctx context.Context, uuid string) (*model.GitTemplate, error)
	Create(ctx context.Context, tmpl *model.GitTemplate) error
	Update(ctx context.Context, tmpl *model.GitTemplate) error
	SoftDelete(ctx context.Context, id int64) error
	FilesForTemplate(ctx context.Context, templateID int64) ([]model.GitTemplateFile, error)
	FindFileByPath(ctx context.Context, templateID int64, path string) (*model.GitTemplateFile, error)
}

// gitTemplateJobInserter enqueues background jobs. Defined where consumed.
type gitTemplateJobInserter interface {
	Insert(ctx context.Context, args river.JobArgs, opts *river.InsertOpts) (*rivertype.JobInsertResult, error)
}

// GitTemplateHandler handles REST API endpoints for git templates.
type GitTemplateHandler struct {
	repo     gitTemplateRepo
	inserter gitTemplateJobInserter
	logger   *slog.Logger
}

// NewGitTemplateHandler creates a new GitTemplateHandler.
func NewGitTemplateHandler(
	repo gitTemplateRepo,
	inserter gitTemplateJobInserter,
	logger *slog.Logger,
) *GitTemplateHandler {
	return &GitTemplateHandler{
		repo:     repo,
		inserter: inserter,
		logger:   logger,
	}
}

// gitTemplateResponse is the JSON representation of a git template.
type gitTemplateResponse struct {
	UUID           string   `json:"uuid"`
	Name           string   `json:"name"`
	Slug           string   `json:"slug"`
	Description    string   `json:"description,omitempty"`
	RepositoryURL  string   `json:"repository_url"`
	Branch         string   `json:"branch"`
	Category       string   `json:"category,omitempty"`
	Tags           []string `json:"tags"`
	IsPublic       bool     `json:"is_public"`
	Status         string   `json:"status"`
	ErrorMessage   string   `json:"error_message,omitempty"`
	FileCount      int      `json:"file_count"`
	TotalSizeBytes int64    `json:"total_size_bytes"`
	LastSyncedAt   string   `json:"last_synced_at,omitempty"`
	LastCommitSHA  string   `json:"last_commit_sha,omitempty"`
	CreatedAt      string   `json:"created_at,omitempty"`
	UpdatedAt      string   `json:"updated_at,omitempty"`
}

// gitTemplateFileResponse is the JSON representation of a template file entry in the file tree.
type gitTemplateFileResponse struct {
	Path        string `json:"path"`
	Filename    string `json:"filename"`
	Extension   string `json:"extension,omitempty"`
	SizeBytes   int64  `json:"size_bytes"`
	IsEssential bool   `json:"is_essential"`
	ContentHash string `json:"content_hash,omitempty"`
}


// List handles GET /api/git-templates -- list git templates with optional filters.
func (h *GitTemplateHandler) List(w http.ResponseWriter, r *http.Request) {
	category := r.URL.Query().Get("category")

	perPage, offset := parsePaginationParam(r, "per_page", 50, 100)

	total, err := h.repo.CountFiltered(r.Context(), category)
	if err != nil {
		h.logger.Error("counting git templates", "error", err)
		errorResponse(w, http.StatusInternalServerError, "failed to count git templates")
		return
	}

	templates, err := h.repo.List(r.Context(), category, perPage, offset)
	if err != nil {
		h.logger.Error("listing git templates", "error", err)
		errorResponse(w, http.StatusInternalServerError, "failed to list git templates")
		return
	}

	items := make([]gitTemplateResponse, 0, len(templates))
	for i := range templates {
		items = append(items, toGitTemplateResponse(&templates[i]))
	}

	jsonResponse(w, http.StatusOK, listResponse(items, total, perPage, offset))
}

// Search handles GET /api/git-templates/search -- search templates by query and category.
func (h *GitTemplateHandler) Search(w http.ResponseWriter, r *http.Request) {
	query := r.URL.Query().Get("q")
	category := r.URL.Query().Get("category")

	limit, _ := parsePagination(r, 50, 100)

	templates, err := h.repo.Search(r.Context(), query, category, limit)
	if err != nil {
		h.logger.Error("searching git templates", "query", query, "error", err)
		errorResponse(w, http.StatusInternalServerError, "failed to search git templates")
		return
	}

	items := make([]gitTemplateResponse, 0, len(templates))
	for i := range templates {
		items = append(items, toGitTemplateResponse(&templates[i]))
	}

	jsonResponse(w, http.StatusOK, map[string]any{
		"data": items,
		"meta": map[string]any{
			"query": query,
			"total": len(items),
		},
	})
}

// Show handles GET /api/git-templates/{uuid} -- get a single git template.
func (h *GitTemplateHandler) Show(w http.ResponseWriter, r *http.Request) {
	tmplUUID := chi.URLParam(r, "uuid")

	tmpl, err := h.repo.FindByUUID(r.Context(), tmplUUID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			errorResponse(w, http.StatusNotFound, "git template not found")
			return
		}
		h.logger.Error("finding git template", "uuid", tmplUUID, "error", err)
		errorResponse(w, http.StatusInternalServerError, "failed to find git template")
		return
	}

	jsonResponse(w, http.StatusOK, map[string]any{
		"data": toGitTemplateResponse(tmpl),
	})
}

// Create handles POST /api/git-templates -- create a new git template.
func (h *GitTemplateHandler) Create(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Name          string   `json:"name"`
		RepositoryURL string   `json:"repository_url"`
		Description   string   `json:"description"`
		Branch        string   `json:"branch"`
		GitToken      string   `json:"git_token"`
		Category      string   `json:"category"`
		Tags          []string `json:"tags"`
		IsPublic      bool     `json:"is_public"`
	}

	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		errorResponse(w, http.StatusBadRequest, "invalid JSON body")
		return
	}

	if body.Name == "" {
		errorResponse(w, http.StatusBadRequest, "name is required")
		return
	}
	if body.RepositoryURL == "" {
		errorResponse(w, http.StatusBadRequest, "repository_url is required")
		return
	}
	if err := security.ValidateExternalURL(body.RepositoryURL, true); err != nil {
		h.logger.Warn("SSRF validation rejected URL", "url", body.RepositoryURL, "error", err)
		errorResponse(w, http.StatusBadRequest, "Invalid repository URL")
		return
	}

	branch := body.Branch
	if branch == "" {
		branch = "main"
	}

	tmpl := &model.GitTemplate{
		UUID:          uuid.New().String(),
		Name:          body.Name,
		Slug:          stringutil.Slugify(body.Name),
		RepositoryURL: body.RepositoryURL,
		Branch:        branch,
		IsPublic:      body.IsPublic,
		IsEnabled:     true,
		Status:        "pending",
	}

	if body.Description != "" {
		tmpl.Description = sql.NullString{String: body.Description, Valid: true}
	}
	if body.GitToken != "" {
		tmpl.GitToken = sql.NullString{String: body.GitToken, Valid: true}
	}
	if body.Category != "" {
		tmpl.Category = sql.NullString{String: body.Category, Valid: true}
	}
	if len(body.Tags) > 0 {
		tagsJSON, err := json.Marshal(body.Tags)
		if err != nil {
			h.logger.Error("marshaling tags", "error", err)
			errorResponse(w, http.StatusInternalServerError, "failed to process tags")
			return
		}
		tmpl.Tags = sql.NullString{String: string(tagsJSON), Valid: true}
	}

	if err := h.repo.Create(r.Context(), tmpl); err != nil {
		h.logger.Error("creating git template", "error", err)
		errorResponse(w, http.StatusInternalServerError, "failed to create git template")
		return
	}

	if h.inserter != nil {
		if _, err := h.inserter.Insert(r.Context(), queue.SyncGitTemplatesArgs{}, nil); err != nil {
			h.logger.Warn("failed to enqueue git template sync after create", "error", err)
		}
	}

	jsonResponse(w, http.StatusCreated, map[string]any{
		"data":    toGitTemplateResponse(tmpl),
		"message": "Git template created and queued for sync.",
	})
}

// Update handles PUT /api/git-templates/{uuid} -- partial update of a git template.
func (h *GitTemplateHandler) Update(w http.ResponseWriter, r *http.Request) {
	tmplUUID := chi.URLParam(r, "uuid")

	tmpl, err := h.repo.FindByUUID(r.Context(), tmplUUID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			errorResponse(w, http.StatusNotFound, "git template not found")
			return
		}
		h.logger.Error("finding git template for update", "uuid", tmplUUID, "error", err)
		errorResponse(w, http.StatusInternalServerError, "failed to find git template")
		return
	}

	var body struct {
		Name          string   `json:"name"`
		RepositoryURL string   `json:"repository_url"`
		Description   string   `json:"description"`
		Branch        string   `json:"branch"`
		GitToken      string   `json:"git_token"`
		Category      string   `json:"category"`
		Tags          []string `json:"tags"`
		IsPublic      *bool    `json:"is_public"`
	}

	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		errorResponse(w, http.StatusBadRequest, "invalid JSON body")
		return
	}

	if body.Name != "" {
		tmpl.Name = body.Name
		tmpl.Slug = stringutil.Slugify(body.Name)
	}
	if body.RepositoryURL != "" {
		if err := security.ValidateExternalURL(body.RepositoryURL, true); err != nil {
			h.logger.Warn("SSRF validation rejected URL", "url", body.RepositoryURL, "error", err)
			errorResponse(w, http.StatusBadRequest, "Invalid repository URL")
			return
		}
		tmpl.RepositoryURL = body.RepositoryURL
	}
	if body.Description != "" {
		tmpl.Description = sql.NullString{String: body.Description, Valid: true}
	}
	if body.Branch != "" {
		tmpl.Branch = body.Branch
	}
	if body.GitToken != "" {
		tmpl.GitToken = sql.NullString{String: body.GitToken, Valid: true}
	}
	if body.Category != "" {
		tmpl.Category = sql.NullString{String: body.Category, Valid: true}
	}
	if body.Tags != nil {
		tagsJSON, jsonErr := json.Marshal(body.Tags)
		if jsonErr != nil {
			h.logger.Error("marshaling tags for update", "error", jsonErr)
			errorResponse(w, http.StatusInternalServerError, "failed to process tags")
			return
		}
		tmpl.Tags = sql.NullString{String: string(tagsJSON), Valid: true}
	}
	if body.IsPublic != nil {
		tmpl.IsPublic = *body.IsPublic
	}

	if err := h.repo.Update(r.Context(), tmpl); err != nil {
		h.logger.Error("updating git template", "uuid", tmplUUID, "error", err)
		errorResponse(w, http.StatusInternalServerError, "failed to update git template")
		return
	}

	jsonResponse(w, http.StatusOK, map[string]any{
		"data":    toGitTemplateResponse(tmpl),
		"message": "Git template updated successfully.",
	})
}

// Delete handles DELETE /api/git-templates/{uuid} -- soft delete a git template.
func (h *GitTemplateHandler) Delete(w http.ResponseWriter, r *http.Request) {
	tmplUUID := chi.URLParam(r, "uuid")

	tmpl, err := h.repo.FindByUUID(r.Context(), tmplUUID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			errorResponse(w, http.StatusNotFound, "git template not found")
			return
		}
		h.logger.Error("finding git template for delete", "uuid", tmplUUID, "error", err)
		errorResponse(w, http.StatusInternalServerError, "failed to find git template")
		return
	}

	if err := h.repo.SoftDelete(r.Context(), tmpl.ID); err != nil {
		h.logger.Error("deleting git template", "uuid", tmplUUID, "error", err)
		errorResponse(w, http.StatusInternalServerError, "failed to delete git template")
		return
	}

	jsonResponse(w, http.StatusOK, map[string]any{
		"message": "Git template deleted successfully.",
	})
}

// Sync handles POST /api/git-templates/{uuid}/sync -- trigger a template sync.
func (h *GitTemplateHandler) Sync(w http.ResponseWriter, r *http.Request) {
	tmplUUID := chi.URLParam(r, "uuid")

	_, err := h.repo.FindByUUID(r.Context(), tmplUUID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			errorResponse(w, http.StatusNotFound, "git template not found")
			return
		}
		h.logger.Error("finding git template for sync", "uuid", tmplUUID, "error", err)
		errorResponse(w, http.StatusInternalServerError, "failed to find git template")
		return
	}

	if h.inserter == nil {
		errorResponse(w, http.StatusServiceUnavailable, "job queue not available")
		return
	}
	if _, err := h.inserter.Insert(r.Context(), queue.SyncGitTemplatesArgs{}, nil); err != nil {
		h.logger.Error("failed to enqueue git template sync", "uuid", tmplUUID, "error", err)
		errorResponse(w, http.StatusInternalServerError, "failed to enqueue sync job")
		return
	}
	jsonResponse(w, http.StatusAccepted, map[string]any{
		"message": "Sync queued",
	})
}

// Structure handles GET /api/git-templates/{uuid}/structure -- return the file tree.
func (h *GitTemplateHandler) Structure(w http.ResponseWriter, r *http.Request) {
	tmplUUID := chi.URLParam(r, "uuid")

	tmpl, err := h.repo.FindByUUID(r.Context(), tmplUUID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			errorResponse(w, http.StatusNotFound, "git template not found")
			return
		}
		h.logger.Error("finding git template for structure", "uuid", tmplUUID, "error", err)
		errorResponse(w, http.StatusInternalServerError, "failed to find git template")
		return
	}

	files, err := h.repo.FilesForTemplate(r.Context(), tmpl.ID)
	if err != nil {
		h.logger.Error("listing template files", "uuid", tmplUUID, "error", err)
		errorResponse(w, http.StatusInternalServerError, "failed to list template files")
		return
	}

	fileTree := make([]string, 0, len(files))
	essentialFiles := make([]string, 0)
	variableSet := make(map[string]bool)

	fileItems := make([]gitTemplateFileResponse, 0, len(files))
	for i := range files {
		f := &files[i]
		fileTree = append(fileTree, f.Path)

		if f.IsEssential {
			essentialFiles = append(essentialFiles, f.Path)
		}

		// Extract {{variables}} from file content.
		if f.Content.Valid {
			matches := gitclient.VariablePattern.FindAllStringSubmatch(f.Content.String, -1)
			for _, match := range matches {
				variableSet[match[1]] = true
			}
		}

		item := gitTemplateFileResponse{
			Path:        f.Path,
			Filename:    f.Filename,
			SizeBytes:   f.SizeBytes,
			IsEssential: f.IsEssential,
			Extension:   nullStringValue(f.Extension),
			ContentHash: nullStringValue(f.ContentHash),
		}
		fileItems = append(fileItems, item)
	}

	variables := make([]string, 0, len(variableSet))
	for v := range variableSet {
		variables = append(variables, v)
	}

	jsonResponse(w, http.StatusOK, map[string]any{
		"data": map[string]any{
			"uuid":            tmpl.UUID,
			"name":            tmpl.Name,
			"file_tree":       fileTree,
			"essential_files": essentialFiles,
			"variables":       variables,
			"files":           fileItems,
			"file_count":      tmpl.FileCount,
			"total_size":      tmpl.TotalSizeBytes,
		},
	})
}

// ReadFile handles GET /api/git-templates/{uuid}/files/* -- read a single template file.
func (h *GitTemplateHandler) ReadFile(w http.ResponseWriter, r *http.Request) {
	tmplUUID := chi.URLParam(r, "uuid")
	filePath := chi.URLParam(r, "*")

	if filePath == "" {
		errorResponse(w, http.StatusBadRequest, "file path is required")
		return
	}

	// Decode percent-encoded path segments (e.g. %2F -> /).
	if decoded, err := url.PathUnescape(filePath); err == nil {
		filePath = decoded
	}

	tmpl, err := h.repo.FindByUUID(r.Context(), tmplUUID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			errorResponse(w, http.StatusNotFound, "git template not found")
			return
		}
		h.logger.Error("finding git template for file read", "uuid", tmplUUID, "error", err)
		errorResponse(w, http.StatusInternalServerError, "failed to find git template")
		return
	}

	file, err := h.repo.FindFileByPath(r.Context(), tmpl.ID, filePath)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			errorResponse(w, http.StatusNotFound, fmt.Sprintf("file %q not found in template", filePath))
			return
		}
		h.logger.Error("finding template file", "uuid", tmplUUID, "path", filePath, "error", err)
		errorResponse(w, http.StatusInternalServerError, "failed to find template file")
		return
	}

	// Parse optional variables from query parameter.
	variablesJSON := r.URL.Query().Get("variables")
	content := file.Content.String
	var unresolved []string

	if variablesJSON != "" {
		vars, parseErr := parseTemplateVariablesJSON(variablesJSON)
		if parseErr != nil {
			errorResponse(w, http.StatusBadRequest, "invalid variables JSON")
			return
		}
		if len(vars) > 0 {
			content, unresolved = substituteTemplateVariables(content, vars)
		}
	}

	resp := map[string]any{
		"data": map[string]any{
			"path":         file.Path,
			"filename":     file.Filename,
			"size_bytes":   file.SizeBytes,
			"is_essential": file.IsEssential,
			"content":      content,
		},
	}
	if len(unresolved) > 0 {
		resp["unresolved_variables"] = unresolved
	}

	jsonResponse(w, http.StatusOK, resp)
}

// DeploymentGuide handles GET /api/git-templates/{uuid}/deployment-guide -- generate a deployment guide.
func (h *GitTemplateHandler) DeploymentGuide(w http.ResponseWriter, r *http.Request) {
	tmplUUID := chi.URLParam(r, "uuid")

	tmpl, err := h.repo.FindByUUID(r.Context(), tmplUUID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			errorResponse(w, http.StatusNotFound, "git template not found")
			return
		}
		h.logger.Error("finding git template for deployment guide", "uuid", tmplUUID, "error", err)
		errorResponse(w, http.StatusInternalServerError, "failed to find git template")
		return
	}

	files, err := h.repo.FilesForTemplate(r.Context(), tmpl.ID)
	if err != nil {
		h.logger.Error("listing template files for deployment guide", "uuid", tmplUUID, "error", err)
		errorResponse(w, http.StatusInternalServerError, "failed to list template files")
		return
	}

	// Parse optional variables from query parameter.
	variablesJSON := r.URL.Query().Get("variables")
	variables, _ := parseTemplateVariablesJSON(variablesJSON)

	allUnresolved := make(map[string]bool)

	type deploymentFileResp struct {
		Path    string `json:"path"`
		Content string `json:"content"`
	}

	deploymentFiles := make([]deploymentFileResp, 0)
	for i := range files {
		if !files[i].IsEssential {
			continue
		}
		content := files[i].Content.String
		if len(variables) > 0 {
			var unresolved []string
			content, unresolved = substituteTemplateVariables(content, variables)
			for _, u := range unresolved {
				allUnresolved[u] = true
			}
		}
		deploymentFiles = append(deploymentFiles, deploymentFileResp{
			Path:    files[i].Path,
			Content: content,
		})
	}

	unresolvedList := make([]string, 0, len(allUnresolved))
	for v := range allUnresolved {
		unresolvedList = append(unresolvedList, v)
	}

	description := nullStringValue(tmpl.Description)

	jsonResponse(w, http.StatusOK, map[string]any{
		"data": map[string]any{
			"template_name":        tmpl.Name,
			"description":          description,
			"steps":                []string{"Create the following files in your project directory."},
			"files":                deploymentFiles,
			"unresolved_variables": unresolvedList,
		},
	})
}

// Download handles POST /api/git-templates/{uuid}/download -- download template as an archive.
func (h *GitTemplateHandler) Download(w http.ResponseWriter, r *http.Request) {
	tmplUUID := chi.URLParam(r, "uuid")

	tmpl, err := h.repo.FindByUUID(r.Context(), tmplUUID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			errorResponse(w, http.StatusNotFound, "git template not found")
			return
		}
		h.logger.Error("finding git template for download", "uuid", tmplUUID, "error", err)
		errorResponse(w, http.StatusInternalServerError, "failed to find git template")
		return
	}

	var body struct {
		Format    string            `json:"format"`
		Variables map[string]string `json:"variables"`
	}

	if err = json.NewDecoder(r.Body).Decode(&body); err != nil {
		errorResponse(w, http.StatusBadRequest, "invalid JSON body")
		return
	}

	format := body.Format
	if format == "" {
		format = "zip"
	}
	if format != "zip" && format != "tar.gz" {
		errorResponse(w, http.StatusBadRequest, "format must be 'zip' or 'tar.gz'")
		return
	}

	files, err := h.repo.FilesForTemplate(r.Context(), tmpl.ID)
	if err != nil {
		h.logger.Error("listing template files for download", "uuid", tmplUUID, "error", err)
		errorResponse(w, http.StatusInternalServerError, "failed to list template files")
		return
	}

	allUnresolved := make(map[string]bool)

	entries := make([]templateArchiveEntry, 0, len(files))
	for i := range files {
		content := files[i].Content.String
		if len(body.Variables) > 0 {
			var unresolved []string
			content, unresolved = substituteTemplateVariables(content, body.Variables)
			for _, u := range unresolved {
				allUnresolved[u] = true
			}
		}
		entries = append(entries, templateArchiveEntry{path: files[i].Path, content: content})
	}

	var buf bytes.Buffer
	var filename string

	switch format {
	case "tar.gz":
		if err := buildTemplateArchiveTarGz(&buf, entries); err != nil {
			h.logger.Error("creating tar.gz archive", "error", err)
			errorResponse(w, http.StatusInternalServerError, "failed to create archive")
			return
		}
		filename = tmpl.Slug + ".tar.gz"
	default:
		if err := buildTemplateArchiveZip(&buf, entries); err != nil {
			h.logger.Error("creating zip archive", "error", err)
			errorResponse(w, http.StatusInternalServerError, "failed to create archive")
			return
		}
		filename = tmpl.Slug + ".zip"
	}

	encoded := base64.StdEncoding.EncodeToString(buf.Bytes())

	unresolvedList := make([]string, 0, len(allUnresolved))
	for v := range allUnresolved {
		unresolvedList = append(unresolvedList, v)
	}

	jsonResponse(w, http.StatusOK, map[string]any{
		"data": map[string]any{
			"template":             tmpl.Name,
			"filename":             filename,
			"format":               format,
			"size_bytes":           buf.Len(),
			"file_count":           len(entries),
			"archive_base64":       encoded,
			"unresolved_variables": unresolvedList,
		},
	})
}

// toGitTemplateResponse converts a GitTemplate model to its JSON response DTO.
func toGitTemplateResponse(gt *model.GitTemplate) gitTemplateResponse {
	tags, _ := gt.ParseTags()
	if tags == nil {
		tags = []string{}
	}

	resp := gitTemplateResponse{
		UUID:           gt.UUID,
		Name:           gt.Name,
		Slug:           gt.Slug,
		RepositoryURL:  gt.RepositoryURL,
		Branch:         gt.Branch,
		IsPublic:       gt.IsPublic,
		Status:         gt.Status,
		Tags:           tags,
		FileCount:      gt.FileCount,
		TotalSizeBytes: gt.TotalSizeBytes,
	}

	resp.Description = nullStringValue(gt.Description)
	resp.Category = nullStringValue(gt.Category)
	resp.ErrorMessage = nullStringValue(gt.ErrorMessage)
	resp.LastSyncedAt = nullTimeToString(gt.LastSyncedAt)
	resp.LastCommitSHA = nullStringValue(gt.LastCommitSHA)
	resp.CreatedAt = nullTimeToString(gt.CreatedAt)
	resp.UpdatedAt = nullTimeToString(gt.UpdatedAt)

	return resp
}

// substituteTemplateVariables replaces {{key}} placeholders in content with values
// from the provided map. Returns the substituted content and a list of unresolved
// variable names.
func substituteTemplateVariables(content string, variables map[string]string) (result string, missingVars []string) {
	result = content

	matches := gitclient.VariablePattern.FindAllStringSubmatch(content, -1)
	seen := make(map[string]bool)
	for _, match := range matches {
		key := match[1]
		if val, ok := variables[key]; ok {
			result = strings.ReplaceAll(result, "{{"+key+"}}", val)
		} else if !seen[key] {
			missingVars = append(missingVars, key)
			seen[key] = true
		}
	}
	return result, missingVars
}

// parseTemplateVariablesJSON decodes a JSON string into a map of variable substitutions.
// Returns an empty map if the input is empty.
func parseTemplateVariablesJSON(raw string) (map[string]string, error) {
	if raw == "" {
		return map[string]string{}, nil
	}
	var vars map[string]string
	if err := json.Unmarshal([]byte(raw), &vars); err != nil {
		return nil, fmt.Errorf("decoding variables JSON: %w", err)
	}
	return vars, nil
}


// templateArchiveEntry holds a single file's path and content for archive creation.
type templateArchiveEntry struct {
	path    string
	content string
}

// buildTemplateArchiveZip writes a zip archive containing the given entries to w.
func buildTemplateArchiveZip(w *bytes.Buffer, entries []templateArchiveEntry) error {
	zw := zip.NewWriter(w)
	for _, e := range entries {
		clean := filepath.ToSlash(filepath.Clean(e.path))
		if clean == ".." || strings.HasPrefix(clean, "../") || filepath.IsAbs(e.path) {
			continue // skip paths that would escape the archive root
		}
		fw, err := zw.Create(clean)
		if err != nil {
			return fmt.Errorf("creating zip entry %q: %w", e.path, err)
		}
		if _, err := fw.Write([]byte(e.content)); err != nil {
			return fmt.Errorf("writing zip entry %q: %w", e.path, err)
		}
	}
	if err := zw.Close(); err != nil {
		return fmt.Errorf("closing zip writer: %w", err)
	}
	return nil
}

// buildTemplateArchiveTarGz writes a gzip-compressed tar archive to w.
func buildTemplateArchiveTarGz(w *bytes.Buffer, entries []templateArchiveEntry) error {
	gw := gzip.NewWriter(w)
	tw := tar.NewWriter(gw)

	for _, e := range entries {
		clean := filepath.ToSlash(filepath.Clean(e.path))
		if clean == ".." || strings.HasPrefix(clean, "../") || filepath.IsAbs(e.path) {
			continue // skip paths that would escape the archive root
		}
		hdr := &tar.Header{
			Name: clean,
			Mode: 0o600,
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
	if err := gw.Close(); err != nil {
		return fmt.Errorf("closing gzip writer: %w", err)
	}
	return nil
}

// ValidateURL handles POST /api/admin/git-templates/validate-url -- SSRF validation.
func (h *GitTemplateHandler) ValidateURL(w http.ResponseWriter, r *http.Request) {
	var body struct {
		URL string `json:"url"`
	}

	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		errorResponse(w, http.StatusBadRequest, "invalid JSON body")
		return
	}

	if body.URL == "" {
		errorResponse(w, http.StatusBadRequest, "url is required")
		return
	}

	if err := security.ValidateExternalURL(body.URL, true); err != nil {
		h.logger.Warn("SSRF validation rejected URL", "url", body.URL, "error", err)
		jsonResponse(w, http.StatusOK, map[string]any{
			"valid": false,
			"error": "URL is not allowed",
		})
		return
	}

	jsonResponse(w, http.StatusOK, map[string]any{
		"valid": true,
	})
}
