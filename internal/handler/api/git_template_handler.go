package api

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"net/url"

	"github.com/go-chi/chi/v5"

	gitclient "github.com/c-premus/documcp/internal/client/git"
	"github.com/c-premus/documcp/internal/model"
	"github.com/c-premus/documcp/internal/service"
)

// gitTemplateSearchRepo defines the search method the handler still needs directly.
type gitTemplateSearchRepo interface {
	Search(ctx context.Context, query, category string, limit int) ([]model.GitTemplate, error)
}

// GitTemplateHandler handles REST API endpoints for git templates.
type GitTemplateHandler struct {
	service *service.GitTemplateService
	repo    gitTemplateSearchRepo
	logger  *slog.Logger
}

// NewGitTemplateHandler creates a new GitTemplateHandler.
func NewGitTemplateHandler(
	svc *service.GitTemplateService,
	repo gitTemplateSearchRepo,
	logger *slog.Logger,
) *GitTemplateHandler {
	return &GitTemplateHandler{
		service: svc,
		repo:    repo,
		logger:  logger,
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

	templates, total, err := h.service.List(r.Context(), category, perPage, offset)
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

	tmpl, err := h.service.FindByUUID(r.Context(), tmplUUID)
	if err != nil {
		h.handleServiceError(w, err, "finding git template", "uuid", tmplUUID)
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

	input := service.CreateGitTemplateInput{
		Name:          body.Name,
		Description:   body.Description,
		RepositoryURL: body.RepositoryURL,
		Branch:        body.Branch,
		GitToken:      body.GitToken,
		Category:      body.Category,
		Tags:          body.Tags,
		IsPublic:      body.IsPublic,
	}

	tmpl, err := h.service.Create(r.Context(), input)
	if err != nil {
		h.handleServiceError(w, err, "creating git template")
		return
	}

	jsonResponse(w, http.StatusCreated, map[string]any{
		"data":    toGitTemplateResponse(tmpl),
		"message": "Git template created and queued for sync.",
	})
}

// Update handles PUT /api/git-templates/{uuid} -- partial update of a git template.
func (h *GitTemplateHandler) Update(w http.ResponseWriter, r *http.Request) {
	tmplUUID := chi.URLParam(r, "uuid")

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

	input := service.UpdateGitTemplateInput{}
	if body.Name != "" {
		input.Name = &body.Name
	}
	if body.RepositoryURL != "" {
		input.RepositoryURL = &body.RepositoryURL
	}
	if body.Description != "" {
		input.Description = &body.Description
	}
	if body.Branch != "" {
		input.Branch = &body.Branch
	}
	if body.GitToken != "" {
		input.GitToken = &body.GitToken
	}
	if body.Category != "" {
		input.Category = &body.Category
	}
	if body.Tags != nil {
		input.Tags = &body.Tags
	}
	if body.IsPublic != nil {
		input.IsPublic = body.IsPublic
	}

	tmpl, err := h.service.Update(r.Context(), tmplUUID, input)
	if err != nil {
		h.handleServiceError(w, err, "updating git template", "uuid", tmplUUID)
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

	if err := h.service.Delete(r.Context(), tmplUUID); err != nil {
		h.handleServiceError(w, err, "deleting git template", "uuid", tmplUUID)
		return
	}

	jsonResponse(w, http.StatusOK, map[string]any{
		"message": "Git template deleted successfully.",
	})
}

// Sync handles POST /api/git-templates/{uuid}/sync -- trigger a template sync.
func (h *GitTemplateHandler) Sync(w http.ResponseWriter, r *http.Request) {
	tmplUUID := chi.URLParam(r, "uuid")

	if err := h.service.EnqueueSync(r.Context(), tmplUUID); err != nil {
		h.handleServiceError(w, err, "enqueuing git template sync", "uuid", tmplUUID)
		return
	}

	jsonResponse(w, http.StatusAccepted, map[string]any{
		"message": "Sync queued",
	})
}

// Structure handles GET /api/git-templates/{uuid}/structure -- return the file tree.
func (h *GitTemplateHandler) Structure(w http.ResponseWriter, r *http.Request) {
	tmplUUID := chi.URLParam(r, "uuid")

	structure, err := h.service.Structure(r.Context(), tmplUUID)
	if err != nil {
		h.handleServiceError(w, err, "getting template structure", "uuid", tmplUUID)
		return
	}

	fileItems := make([]gitTemplateFileResponse, 0, len(structure.Files))
	for i := range structure.Files {
		f := &structure.Files[i]
		fileItems = append(fileItems, gitTemplateFileResponse{
			Path:        f.Path,
			Filename:    f.Filename,
			SizeBytes:   f.SizeBytes,
			IsEssential: f.IsEssential,
			Extension:   nullStringValue(f.Extension),
			ContentHash: nullStringValue(f.ContentHash),
		})
	}

	jsonResponse(w, http.StatusOK, map[string]any{
		"data": map[string]any{
			"uuid":            structure.Template.UUID,
			"name":            structure.Template.Name,
			"file_tree":       structure.FileTree,
			"essential_files": structure.EssentialFiles,
			"variables":       structure.Variables,
			"files":           fileItems,
			"file_count":      structure.FileCount,
			"total_size":      structure.TotalSize,
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

	// Parse optional variables from query parameter.
	variablesJSON := r.URL.Query().Get("variables")
	var variables map[string]string

	if variablesJSON != "" {
		var parseErr error
		variables, parseErr = gitclient.ParseVariablesJSON(variablesJSON)
		if parseErr != nil {
			errorResponse(w, http.StatusBadRequest, "invalid variables JSON")
			return
		}
	}

	result, err := h.service.File(r.Context(), tmplUUID, filePath, variables)
	if err != nil {
		h.handleServiceError(w, err, "reading template file", "uuid", tmplUUID, "path", filePath)
		return
	}

	resp := map[string]any{
		"data": map[string]any{
			"path":         result.File.Path,
			"filename":     result.File.Filename,
			"size_bytes":   result.File.SizeBytes,
			"is_essential": result.File.IsEssential,
			"content":      result.Content,
		},
	}
	if len(result.Unresolved) > 0 {
		resp["unresolved_variables"] = result.Unresolved
	}

	jsonResponse(w, http.StatusOK, resp)
}

// DeploymentGuide handles GET /api/git-templates/{uuid}/deployment-guide -- generate a deployment guide.
func (h *GitTemplateHandler) DeploymentGuide(w http.ResponseWriter, r *http.Request) {
	tmplUUID := chi.URLParam(r, "uuid")

	// Parse optional variables from query parameter.
	variablesJSON := r.URL.Query().Get("variables")
	variables, _ := gitclient.ParseVariablesJSON(variablesJSON)

	guide, err := h.service.DeploymentGuide(r.Context(), tmplUUID, variables)
	if err != nil {
		h.handleServiceError(w, err, "generating deployment guide", "uuid", tmplUUID)
		return
	}

	type deploymentFileResp struct {
		Path    string `json:"path"`
		Content string `json:"content"`
	}

	files := make([]deploymentFileResp, 0, len(guide.Files))
	for _, f := range guide.Files {
		files = append(files, deploymentFileResp{
			Path:    f.Path,
			Content: f.Content,
		})
	}

	jsonResponse(w, http.StatusOK, map[string]any{
		"data": map[string]any{
			"template_name":        guide.Template.Name,
			"description":          nullStringValue(guide.Template.Description),
			"steps":                guide.Steps,
			"files":                files,
			"unresolved_variables": guide.Unresolved,
		},
	})
}

// Download handles POST /api/git-templates/{uuid}/download -- download template as an archive.
func (h *GitTemplateHandler) Download(w http.ResponseWriter, r *http.Request) {
	tmplUUID := chi.URLParam(r, "uuid")

	var body struct {
		Format    string            `json:"format"`
		Variables map[string]string `json:"variables"`
	}

	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
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

	result, err := h.service.BuildArchive(r.Context(), tmplUUID, format, body.Variables)
	if err != nil {
		h.handleServiceError(w, err, "building template archive", "uuid", tmplUUID)
		return
	}

	jsonResponse(w, http.StatusOK, map[string]any{
		"data": map[string]any{
			"template":             tmplUUID,
			"filename":             result.Filename,
			"format":               result.Format,
			"size_bytes":           len(result.Data),
			"file_count":           result.FileCount,
			"archive_base64":       string(result.Data),
			"unresolved_variables": result.Unresolved,
		},
	})
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

	if err := h.service.ValidateRepositoryURL(body.URL); err != nil {
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
		Status:         string(gt.Status),
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

// handleServiceError maps service-layer errors to HTTP responses.
func (h *GitTemplateHandler) handleServiceError(w http.ResponseWriter, err error, msg string, keyvals ...any) {
	switch {
	case errors.Is(err, service.ErrGitTemplateNotFound):
		errorResponse(w, http.StatusNotFound, "git template not found")
	case errors.Is(err, service.ErrEncryptionDisabled):
		errorResponse(w, http.StatusUnprocessableEntity, "git tokens require encryption; set ENCRYPTION_KEY")
	case errors.Is(err, service.ErrInvalidURL):
		errorResponse(w, http.StatusBadRequest, "Invalid repository URL")
	default:
		args := append([]any{"error", err}, keyvals...)
		h.logger.Error(msg, args...)
		errorResponse(w, http.StatusInternalServerError, "failed: "+msg)
	}
}
