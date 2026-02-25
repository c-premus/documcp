// Package admin provides HTTP handlers for the templ+htmx admin UI.
package admin

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"strconv"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	authmiddleware "git.999.haus/chris/DocuMCP-go/internal/auth/middleware"
	"git.999.haus/chris/DocuMCP-go/internal/model"
	"git.999.haus/chris/DocuMCP-go/internal/repository"
	"git.999.haus/chris/DocuMCP-go/internal/service"
	templates "git.999.haus/chris/DocuMCP-go/web/templates"
)

// Handler serves templ+htmx admin pages.
type Handler struct {
	documentRepo        *repository.DocumentRepository
	oauthRepo           *repository.OAuthRepository
	externalServiceRepo *repository.ExternalServiceRepository
	zimArchiveRepo      *repository.ZimArchiveRepository
	confluenceSpaceRepo *repository.ConfluenceSpaceRepository
	gitTemplateRepo     *repository.GitTemplateRepository
	documentPipeline    *service.DocumentPipeline
	externalServiceSvc  *service.ExternalServiceService
	logger              *slog.Logger
}

// NewHandler creates a new admin Handler.
func NewHandler(
	documentRepo *repository.DocumentRepository,
	oauthRepo *repository.OAuthRepository,
	externalServiceRepo *repository.ExternalServiceRepository,
	zimArchiveRepo *repository.ZimArchiveRepository,
	confluenceSpaceRepo *repository.ConfluenceSpaceRepository,
	gitTemplateRepo *repository.GitTemplateRepository,
	documentPipeline *service.DocumentPipeline,
	externalServiceSvc *service.ExternalServiceService,
	logger *slog.Logger,
) *Handler {
	return &Handler{
		documentRepo:        documentRepo,
		oauthRepo:           oauthRepo,
		externalServiceRepo: externalServiceRepo,
		zimArchiveRepo:      zimArchiveRepo,
		confluenceSpaceRepo: confluenceSpaceRepo,
		gitTemplateRepo:     gitTemplateRepo,
		documentPipeline:    documentPipeline,
		externalServiceSvc:  externalServiceSvc,
		logger:              logger,
	}
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

// pageData builds a PageData with user from context and navigation items.
func (h *Handler) pageData(r *http.Request, title, activePage string) templates.PageData {
	user, _ := authmiddleware.UserFromContext(r.Context())

	nav := []templates.NavItem{
		{Label: "Dashboard", URL: "/admin", Icon: "home", Active: activePage == "dashboard", Visible: true},
		{Label: "Documents", URL: "/admin/documents", Icon: "file-text", Active: activePage == "documents", Visible: true},
		{Label: "Users", URL: "/admin/users", Icon: "users", Active: activePage == "users", Visible: true},
		{Label: "OAuth Clients", URL: "/admin/oauth-clients", Icon: "key", Active: activePage == "oauth-clients", Visible: true},
		{Label: "External Services", URL: "/admin/external-services", Icon: "server", Active: activePage == "external-services", Visible: true},
		{Label: "ZIM Archives", URL: "/admin/zim-archives", Icon: "archive", Active: activePage == "zim-archives", Visible: true},
		{Label: "Confluence Spaces", URL: "/admin/confluence-spaces", Icon: "book-open", Active: activePage == "confluence-spaces", Visible: true},
		{Label: "Git Templates", URL: "/admin/git-templates", Icon: "git-branch", Active: activePage == "git-templates", Visible: true},
	}

	return templates.PageData{
		Title:      title,
		User:       user,
		Nav:        nav,
		ActivePage: activePage,
	}
}

// paginationData calculates pagination from total items.
func paginationData(total, perPage, page int, baseURL string) templates.PaginationData {
	totalPages := total / perPage
	if total%perPage != 0 {
		totalPages++
	}
	if totalPages < 1 {
		totalPages = 1
	}
	return templates.PaginationData{
		CurrentPage: page,
		TotalPages:  totalPages,
		TotalItems:  total,
		PerPage:     perPage,
		BaseURL:     baseURL,
	}
}

// queryParam returns a query parameter value or the default.
func queryParam(r *http.Request, name, defaultVal string) string {
	v := r.URL.Query().Get(name)
	if v == "" {
		return defaultVal
	}
	return v
}

// intParam parses an integer from a query parameter, returning the default on failure.
func intParam(r *http.Request, name string, defaultVal int) int {
	v := r.URL.Query().Get(name)
	if v == "" {
		return defaultVal
	}
	n, err := strconv.Atoi(v)
	if err != nil {
		return defaultVal
	}
	return n
}

// isHTMX returns true if the request was made by htmx.
func isHTMX(r *http.Request) bool {
	return r.Header.Get("HX-Request") == "true"
}

// renderError logs the error and writes an HTTP 500 response.
func (h *Handler) renderError(w http.ResponseWriter, r *http.Request, msg string, err error) {
	h.logger.Error(msg, "error", err, "path", r.URL.Path)
	http.Error(w, "Internal server error", http.StatusInternalServerError)
}

// ---------------------------------------------------------------------------
// 1. Login
// ---------------------------------------------------------------------------

// Login renders the login page.
func (h *Handler) Login(w http.ResponseWriter, r *http.Request) {
	if err := templates.LoginPage().Render(r.Context(), w); err != nil {
		h.renderError(w, r, "rendering login page", err)
	}
}

// ---------------------------------------------------------------------------
// 2. Dashboard
// ---------------------------------------------------------------------------

// Dashboard renders the admin dashboard with entity counts.
func (h *Handler) Dashboard(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	docCount, err := h.documentRepo.Count(ctx)
	if err != nil {
		h.renderError(w, r, "counting documents", err)
		return
	}

	userCount, err := h.oauthRepo.CountUsers(ctx)
	if err != nil {
		h.renderError(w, r, "counting users", err)
		return
	}

	clientCount, err := h.oauthRepo.CountClients(ctx)
	if err != nil {
		h.renderError(w, r, "counting oauth clients", err)
		return
	}

	zimCount, err := h.zimArchiveRepo.Count(ctx)
	if err != nil {
		h.renderError(w, r, "counting zim archives", err)
		return
	}

	confluenceCount, err := h.confluenceSpaceRepo.Count(ctx)
	if err != nil {
		h.renderError(w, r, "counting confluence spaces", err)
		return
	}

	gitCount, err := h.gitTemplateRepo.Count(ctx)
	if err != nil {
		h.renderError(w, r, "counting git templates", err)
		return
	}

	stats := templates.DashboardStats{
		Documents:        docCount,
		Users:            userCount,
		OAuthClients:     clientCount,
		ZimArchives:      zimCount,
		ConfluenceSpaces: confluenceCount,
		GitTemplates:     gitCount,
		HasZim:           zimCount > 0,
		HasConfluence:    confluenceCount > 0,
		HasGitTemplates:  gitCount > 0,
	}

	data := h.pageData(r, "Dashboard", "dashboard")
	if err := templates.DashboardPage(data, stats).Render(ctx, w); err != nil {
		h.renderError(w, r, "rendering dashboard", err)
	}
}

// ---------------------------------------------------------------------------
// 3-6. Documents
// ---------------------------------------------------------------------------

// DocumentList renders the document list page or htmx partial.
func (h *Handler) DocumentList(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	query := queryParam(r, "q", "")
	fileType := queryParam(r, "file_type", "")
	status := queryParam(r, "status", "")
	page := intParam(r, "page", 1)
	perPage := 20

	result, err := h.documentRepo.List(ctx, repository.DocumentListParams{
		Query:    query,
		FileType: fileType,
		Status:   status,
		Limit:    perPage,
		Offset:   (page - 1) * perPage,
	})
	if err != nil {
		h.renderError(w, r, "listing documents", err)
		return
	}

	pagination := paginationData(result.Total, perPage, page, "/admin/documents")

	if isHTMX(r) {
		if err := templates.DocumentListTable(result.Documents, pagination).Render(ctx, w); err != nil {
			h.renderError(w, r, "rendering document list table", err)
		}
		return
	}

	data := h.pageData(r, "Documents", "documents")
	if err := templates.DocumentListPage(data, result.Documents, pagination, query, fileType, status).Render(ctx, w); err != nil {
		h.renderError(w, r, "rendering document list page", err)
	}
}

// DocumentDetail renders the document detail page.
func (h *Handler) DocumentDetail(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	docUUID := chi.URLParam(r, "uuid")

	doc, err := h.documentRepo.FindByUUID(ctx, docUUID)
	if err != nil {
		h.renderError(w, r, "finding document", err)
		return
	}

	tags, err := h.documentRepo.TagsForDocument(ctx, doc.ID)
	if err != nil {
		h.renderError(w, r, "loading document tags", err)
		return
	}

	data := h.pageData(r, doc.Title, "documents")
	if err := templates.DocumentDetailPage(data, *doc, tags).Render(ctx, w); err != nil {
		h.renderError(w, r, "rendering document detail", err)
	}
}

// DocumentUpload handles multipart file upload for documents.
func (h *Handler) DocumentUpload(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	if err := r.ParseMultipartForm(32 << 20); err != nil {
		http.Error(w, "Failed to parse form", http.StatusBadRequest)
		return
	}

	file, header, err := r.FormFile("file")
	if err != nil {
		http.Error(w, "File is required", http.StatusBadRequest)
		return
	}
	defer file.Close() //nolint:errcheck

	title := r.FormValue("title")
	if title == "" {
		title = header.Filename
	}
	description := r.FormValue("description")
	isPublic := r.FormValue("is_public") == "on" || r.FormValue("is_public") == "true"

	var userID *int64
	if user, ok := authmiddleware.UserFromContext(ctx); ok {
		userID = &user.ID
	}

	// Save to a temporary file so the pipeline can process it.
	tmpFile, err := os.CreateTemp("", "upload-*-"+header.Filename)
	if err != nil {
		h.renderError(w, r, "creating temp file", err)
		return
	}
	defer os.Remove(tmpFile.Name()) //nolint:errcheck
	defer tmpFile.Close()          //nolint:errcheck

	if _, err := io.Copy(tmpFile, file); err != nil {
		h.renderError(w, r, "writing temp file", err)
		return
	}

	// Seek back to beginning for the pipeline reader.
	if _, err := tmpFile.Seek(0, io.SeekStart); err != nil {
		h.renderError(w, r, "seeking temp file", err)
		return
	}

	doc, err := h.documentPipeline.Upload(ctx, service.UploadDocumentParams{
		Title:       title,
		Description: description,
		FileName:    header.Filename,
		FileSize:    header.Size,
		Reader:      tmpFile,
		IsPublic:    isPublic,
		UserID:      userID,
	})
	if err != nil {
		h.renderError(w, r, "uploading document", err)
		return
	}

	http.Redirect(w, r, "/admin/documents/"+doc.UUID, http.StatusSeeOther)
}

// DocumentDelete soft-deletes a document and returns an empty response.
func (h *Handler) DocumentDelete(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	docUUID := chi.URLParam(r, "uuid")

	doc, err := h.documentRepo.FindByUUID(ctx, docUUID)
	if err != nil {
		h.renderError(w, r, "finding document for deletion", err)
		return
	}

	if err := h.documentRepo.SoftDelete(ctx, doc.ID); err != nil {
		h.renderError(w, r, "deleting document", err)
		return
	}

	w.WriteHeader(http.StatusOK)
}

// ---------------------------------------------------------------------------
// 7-9. Users
// ---------------------------------------------------------------------------

// UserList renders the user list page or htmx partial.
func (h *Handler) UserList(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	query := queryParam(r, "q", "")
	page := intParam(r, "page", 1)
	perPage := 20

	users, total, err := h.oauthRepo.ListUsers(ctx, query, perPage, (page-1)*perPage)
	if err != nil {
		h.renderError(w, r, "listing users", err)
		return
	}

	pagination := paginationData(total, perPage, page, "/admin/users")

	if isHTMX(r) {
		if err := templates.UserListTable(users, pagination).Render(ctx, w); err != nil {
			h.renderError(w, r, "rendering user list table", err)
		}
		return
	}

	data := h.pageData(r, "Users", "users")
	if err := templates.UserListPage(data, users, pagination, query).Render(ctx, w); err != nil {
		h.renderError(w, r, "rendering user list page", err)
	}
}

// UserDetail renders the user detail page.
func (h *Handler) UserDetail(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		http.Error(w, "Invalid user ID", http.StatusBadRequest)
		return
	}

	user, err := h.oauthRepo.FindUserByID(ctx, id)
	if err != nil {
		h.renderError(w, r, "finding user", err)
		return
	}

	data := h.pageData(r, user.Name, "users")
	if err := templates.UserDetailPage(data, *user).Render(ctx, w); err != nil {
		h.renderError(w, r, "rendering user detail", err)
	}
}

// UserToggleAdmin toggles a user's admin status and redirects back.
func (h *Handler) UserToggleAdmin(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		http.Error(w, "Invalid user ID", http.StatusBadRequest)
		return
	}

	if err := h.oauthRepo.ToggleAdmin(ctx, id); err != nil {
		h.renderError(w, r, "toggling admin", err)
		return
	}

	http.Redirect(w, r, fmt.Sprintf("/admin/users/%d", id), http.StatusSeeOther)
}

// ---------------------------------------------------------------------------
// 10-13. OAuth Clients
// ---------------------------------------------------------------------------

// OAuthClientList renders the OAuth client list page or htmx partial.
func (h *Handler) OAuthClientList(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	query := queryParam(r, "q", "")
	page := intParam(r, "page", 1)
	perPage := 20

	clients, total, err := h.oauthRepo.ListClients(ctx, query, perPage, (page-1)*perPage)
	if err != nil {
		h.renderError(w, r, "listing oauth clients", err)
		return
	}

	pagination := paginationData(total, perPage, page, "/admin/oauth-clients")

	if isHTMX(r) {
		if err := templates.OAuthClientListTable(clients, pagination).Render(ctx, w); err != nil {
			h.renderError(w, r, "rendering oauth client list table", err)
		}
		return
	}

	data := h.pageData(r, "OAuth Clients", "oauth-clients")
	if err := templates.OAuthClientListPage(data, clients, pagination, query).Render(ctx, w); err != nil {
		h.renderError(w, r, "rendering oauth client list page", err)
	}
}

// OAuthClientDetail renders the OAuth client detail page.
func (h *Handler) OAuthClientDetail(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		http.Error(w, "Invalid client ID", http.StatusBadRequest)
		return
	}

	client, err := h.oauthRepo.FindClientByID(ctx, id)
	if err != nil {
		h.renderError(w, r, "finding oauth client", err)
		return
	}

	data := h.pageData(r, client.ClientName, "oauth-clients")
	if err := templates.OAuthClientDetailPage(data, *client).Render(ctx, w); err != nil {
		h.renderError(w, r, "rendering oauth client detail", err)
	}
}

// OAuthClientCreate creates a new OAuth client and renders the secret modal.
func (h *Handler) OAuthClientCreate(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	if err := r.ParseForm(); err != nil {
		http.Error(w, "Failed to parse form", http.StatusBadRequest)
		return
	}

	clientName := r.FormValue("client_name")
	redirectURIs := r.FormValue("redirect_uris")
	grantTypes := r.FormValue("grant_types")
	scope := r.FormValue("scope")

	if clientName == "" {
		http.Error(w, "Client name is required", http.StatusBadRequest)
		return
	}

	// Generate client credentials.
	clientID := uuid.New().String()

	rawSecret := make([]byte, 32)
	if _, err := rand.Read(rawSecret); err != nil {
		h.renderError(w, r, "generating client secret", err)
		return
	}
	plainSecret := hex.EncodeToString(rawSecret)

	hash := sha256.Sum256([]byte(plainSecret))
	hashedSecret := hex.EncodeToString(hash[:])

	// Default grant types and redirect URIs if not provided.
	if grantTypes == "" {
		grantTypes = `["authorization_code"]`
	}
	if redirectURIs == "" {
		redirectURIs = `[]`
	}

	client := &model.OAuthClient{
		ClientID:                clientID,
		ClientSecret:            sql.NullString{String: hashedSecret, Valid: true},
		ClientName:              clientName,
		RedirectURIs:            redirectURIs,
		GrantTypes:              grantTypes,
		ResponseTypes:           `["code"]`,
		TokenEndpointAuthMethod: "client_secret_basic",
		Scope:                   sql.NullString{String: scope, Valid: scope != ""},
		IsActive:                true,
	}

	if err := h.oauthRepo.CreateClient(ctx, client); err != nil {
		h.renderError(w, r, "creating oauth client", err)
		return
	}

	if err := templates.OAuthClientSecretModal(clientName, clientID, plainSecret).Render(ctx, w); err != nil {
		h.renderError(w, r, "rendering oauth client secret modal", err)
	}
}

// OAuthClientRevoke deactivates an OAuth client and returns an empty response.
func (h *Handler) OAuthClientRevoke(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		http.Error(w, "Invalid client ID", http.StatusBadRequest)
		return
	}

	if err := h.oauthRepo.DeactivateClient(ctx, id); err != nil {
		h.renderError(w, r, "revoking oauth client", err)
		return
	}

	w.WriteHeader(http.StatusOK)
}

// ---------------------------------------------------------------------------
// 14-18. External Services
// ---------------------------------------------------------------------------

// ExternalServiceList renders the external service list page or htmx partial.
func (h *Handler) ExternalServiceList(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	query := queryParam(r, "q", "")
	page := intParam(r, "page", 1)
	perPage := 20

	services, total, err := h.externalServiceRepo.List(ctx, "", "", perPage, (page-1)*perPage)
	if err != nil {
		h.renderError(w, r, "listing external services", err)
		return
	}

	// Filter by query client-side if provided, since the repo doesn't have a query param.
	if query != "" {
		filtered := make([]model.ExternalService, 0, len(services))
		q := strings.ToLower(query)
		for _, svc := range services {
			if strings.Contains(strings.ToLower(svc.Name), q) || strings.Contains(strings.ToLower(svc.Slug), q) {
				filtered = append(filtered, svc)
			}
		}
		services = filtered
	}

	pagination := paginationData(total, perPage, page, "/admin/external-services")

	if isHTMX(r) {
		if err := templates.ExternalServiceListTable(services, pagination).Render(ctx, w); err != nil {
			h.renderError(w, r, "rendering external service list table", err)
		}
		return
	}

	data := h.pageData(r, "External Services", "external-services")
	if err := templates.ExternalServiceListPage(data, services, pagination, query).Render(ctx, w); err != nil {
		h.renderError(w, r, "rendering external service list page", err)
	}
}

// ExternalServiceDetail renders the external service detail page.
func (h *Handler) ExternalServiceDetail(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	svcUUID := chi.URLParam(r, "uuid")

	svc, err := h.externalServiceRepo.FindByUUID(ctx, svcUUID)
	if err != nil {
		h.renderError(w, r, "finding external service", err)
		return
	}

	data := h.pageData(r, svc.Name, "external-services")
	if err := templates.ExternalServiceDetailPage(data, *svc).Render(ctx, w); err != nil {
		h.renderError(w, r, "rendering external service detail", err)
	}
}

// ExternalServiceCreate creates a new external service and redirects.
func (h *Handler) ExternalServiceCreate(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	if err := r.ParseForm(); err != nil {
		http.Error(w, "Failed to parse form", http.StatusBadRequest)
		return
	}

	name := r.FormValue("name")
	svcType := r.FormValue("type")
	baseURL := r.FormValue("base_url")
	apiKey := r.FormValue("api_key")

	if name == "" || svcType == "" || baseURL == "" {
		http.Error(w, "Name, type, and base URL are required", http.StatusBadRequest)
		return
	}

	priority := 0
	if v := r.FormValue("priority"); v != "" {
		if p, err := strconv.Atoi(v); err == nil {
			priority = p
		}
	}

	svc := &model.ExternalService{
		UUID:      uuid.New().String(),
		Name:      name,
		Slug:      slugify(name),
		Type:      svcType,
		BaseURL:   baseURL,
		Priority:  priority,
		Status:    "unknown",
		IsEnabled: true,
	}

	if apiKey != "" {
		svc.APIKey = sql.NullString{String: apiKey, Valid: true}
	}

	if err := h.externalServiceRepo.Create(ctx, svc); err != nil {
		h.renderError(w, r, "creating external service", err)
		return
	}

	http.Redirect(w, r, "/admin/external-services/"+svc.UUID, http.StatusSeeOther)
}

// ExternalServiceHealthCheck triggers a health check and redirects to the detail page.
func (h *Handler) ExternalServiceHealthCheck(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	svcUUID := chi.URLParam(r, "uuid")

	svc, err := h.externalServiceRepo.FindByUUID(ctx, svcUUID)
	if err != nil {
		h.renderError(w, r, "finding external service for health check", err)
		return
	}

	checker := &simpleHealthChecker{baseURL: svc.BaseURL}
	if _, err := h.externalServiceSvc.CheckHealth(ctx, svcUUID, checker); err != nil {
		h.logger.Warn("health check failed", "uuid", svcUUID, "error", err)
	}

	http.Redirect(w, r, "/admin/external-services/"+svcUUID, http.StatusSeeOther)
}

// simpleHealthChecker performs a basic HTTP GET health check.
type simpleHealthChecker struct {
	baseURL string
}

// Health performs a GET request to the base URL to check service availability.
func (c *simpleHealthChecker) Health(ctx context.Context) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.baseURL, nil)
	if err != nil {
		return fmt.Errorf("creating health check request: %w", err)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return fmt.Errorf("health check request failed: %w", err)
	}
	defer resp.Body.Close() //nolint:errcheck

	if resp.StatusCode >= 400 {
		return fmt.Errorf("health check returned status %d", resp.StatusCode)
	}
	return nil
}

// ExternalServiceDelete deletes an external service and returns an empty response.
func (h *Handler) ExternalServiceDelete(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	svcUUID := chi.URLParam(r, "uuid")

	svc, err := h.externalServiceRepo.FindByUUID(ctx, svcUUID)
	if err != nil {
		h.renderError(w, r, "finding external service for deletion", err)
		return
	}

	if err := h.externalServiceRepo.Delete(ctx, svc.ID); err != nil {
		h.renderError(w, r, "deleting external service", err)
		return
	}

	w.WriteHeader(http.StatusOK)
}

// ---------------------------------------------------------------------------
// 19-21. ZIM Archives
// ---------------------------------------------------------------------------

// ZimArchiveList renders the ZIM archive list page or htmx partial.
func (h *Handler) ZimArchiveList(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	query := queryParam(r, "q", "")
	page := intParam(r, "page", 1)
	perPage := 20

	archives, err := h.zimArchiveRepo.ListAll(ctx, query, 0)
	if err != nil {
		h.renderError(w, r, "listing zim archives", err)
		return
	}

	total := len(archives)

	// Apply manual pagination.
	start := min((page-1)*perPage, total)
	end := min(start+perPage, total)
	paged := archives[start:end]

	pagination := paginationData(total, perPage, page, "/admin/zim-archives")

	if isHTMX(r) {
		if err := templates.ZimArchiveListTable(paged, pagination).Render(ctx, w); err != nil {
			h.renderError(w, r, "rendering zim archive list table", err)
		}
		return
	}

	data := h.pageData(r, "ZIM Archives", "zim-archives")
	if err := templates.ZimArchiveListPage(data, paged, pagination, query).Render(ctx, w); err != nil {
		h.renderError(w, r, "rendering zim archive list page", err)
	}
}

// ZimArchiveToggleEnabled toggles the enabled status of a ZIM archive.
func (h *Handler) ZimArchiveToggleEnabled(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	archiveUUID := chi.URLParam(r, "uuid")

	archive, err := h.zimArchiveRepo.FindByUUID(ctx, archiveUUID)
	if err != nil {
		h.renderError(w, r, "finding zim archive", err)
		return
	}

	if err := h.zimArchiveRepo.ToggleEnabled(ctx, archive.ID); err != nil {
		h.renderError(w, r, "toggling zim archive enabled", err)
		return
	}

	http.Redirect(w, r, "/admin/zim-archives", http.StatusSeeOther)
}

// ZimArchiveToggleSearchable toggles the searchable status of a ZIM archive.
func (h *Handler) ZimArchiveToggleSearchable(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	archiveUUID := chi.URLParam(r, "uuid")

	archive, err := h.zimArchiveRepo.FindByUUID(ctx, archiveUUID)
	if err != nil {
		h.renderError(w, r, "finding zim archive", err)
		return
	}

	if err := h.zimArchiveRepo.ToggleSearchable(ctx, archive.ID); err != nil {
		h.renderError(w, r, "toggling zim archive searchable", err)
		return
	}

	http.Redirect(w, r, "/admin/zim-archives", http.StatusSeeOther)
}

// ---------------------------------------------------------------------------
// 22-24. Confluence Spaces
// ---------------------------------------------------------------------------

// ConfluenceSpaceList renders the Confluence space list page or htmx partial.
func (h *Handler) ConfluenceSpaceList(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	query := queryParam(r, "q", "")
	page := intParam(r, "page", 1)
	perPage := 20

	spaces, err := h.confluenceSpaceRepo.ListAll(ctx, query, 0)
	if err != nil {
		h.renderError(w, r, "listing confluence spaces", err)
		return
	}

	total := len(spaces)

	// Apply manual pagination.
	start := min((page-1)*perPage, total)
	end := min(start+perPage, total)
	paged := spaces[start:end]

	pagination := paginationData(total, perPage, page, "/admin/confluence-spaces")

	if isHTMX(r) {
		if err := templates.ConfluenceSpaceListTable(paged, pagination).Render(ctx, w); err != nil {
			h.renderError(w, r, "rendering confluence space list table", err)
		}
		return
	}

	data := h.pageData(r, "Confluence Spaces", "confluence-spaces")
	if err := templates.ConfluenceSpaceListPage(data, paged, pagination, query).Render(ctx, w); err != nil {
		h.renderError(w, r, "rendering confluence space list page", err)
	}
}

// ConfluenceSpaceToggleEnabled toggles the enabled status of a Confluence space.
func (h *Handler) ConfluenceSpaceToggleEnabled(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	spaceUUID := chi.URLParam(r, "uuid")

	space, err := h.confluenceSpaceRepo.FindByUUID(ctx, spaceUUID)
	if err != nil {
		h.renderError(w, r, "finding confluence space", err)
		return
	}

	if err := h.confluenceSpaceRepo.ToggleEnabled(ctx, space.ID); err != nil {
		h.renderError(w, r, "toggling confluence space enabled", err)
		return
	}

	http.Redirect(w, r, "/admin/confluence-spaces", http.StatusSeeOther)
}

// ConfluenceSpaceToggleSearchable toggles the searchable status of a Confluence space.
func (h *Handler) ConfluenceSpaceToggleSearchable(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	spaceUUID := chi.URLParam(r, "uuid")

	space, err := h.confluenceSpaceRepo.FindByUUID(ctx, spaceUUID)
	if err != nil {
		h.renderError(w, r, "finding confluence space", err)
		return
	}

	if err := h.confluenceSpaceRepo.ToggleSearchable(ctx, space.ID); err != nil {
		h.renderError(w, r, "toggling confluence space searchable", err)
		return
	}

	http.Redirect(w, r, "/admin/confluence-spaces", http.StatusSeeOther)
}

// ---------------------------------------------------------------------------
// 25-28. Git Templates
// ---------------------------------------------------------------------------

// GitTemplateList renders the git template list page or htmx partial.
func (h *Handler) GitTemplateList(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	query := queryParam(r, "q", "")
	page := intParam(r, "page", 1)
	perPage := 20

	tmpls, err := h.gitTemplateRepo.ListAll(ctx, query, 0)
	if err != nil {
		h.renderError(w, r, "listing git templates", err)
		return
	}

	total := len(tmpls)

	// Apply manual pagination.
	start := min((page-1)*perPage, total)
	end := min(start+perPage, total)
	paged := tmpls[start:end]

	pagination := paginationData(total, perPage, page, "/admin/git-templates")

	if isHTMX(r) {
		if err := templates.GitTemplateListTable(paged, pagination).Render(ctx, w); err != nil {
			h.renderError(w, r, "rendering git template list table", err)
		}
		return
	}

	data := h.pageData(r, "Git Templates", "git-templates")
	if err := templates.GitTemplateListPage(data, paged, pagination, query).Render(ctx, w); err != nil {
		h.renderError(w, r, "rendering git template list page", err)
	}
}

// GitTemplateDetail renders the git template detail page with its files.
func (h *Handler) GitTemplateDetail(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	tmplUUID := chi.URLParam(r, "uuid")

	tmpl, err := h.gitTemplateRepo.FindByUUID(ctx, tmplUUID)
	if err != nil {
		h.renderError(w, r, "finding git template", err)
		return
	}

	files, err := h.gitTemplateRepo.FilesForTemplate(ctx, tmpl.ID)
	if err != nil {
		h.renderError(w, r, "loading git template files", err)
		return
	}

	data := h.pageData(r, tmpl.Name, "git-templates")
	if err := templates.GitTemplateDetailPage(data, *tmpl, files).Render(ctx, w); err != nil {
		h.renderError(w, r, "rendering git template detail", err)
	}
}

// GitTemplateCreate creates a new git template and redirects to its detail page.
func (h *Handler) GitTemplateCreate(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	if err := r.ParseForm(); err != nil {
		http.Error(w, "Failed to parse form", http.StatusBadRequest)
		return
	}

	name := r.FormValue("name")
	repoURL := r.FormValue("repository_url")
	branch := r.FormValue("branch")
	description := r.FormValue("description")
	category := r.FormValue("category")

	if name == "" || repoURL == "" {
		http.Error(w, "Name and repository URL are required", http.StatusBadRequest)
		return
	}

	if branch == "" {
		branch = "main"
	}

	var userID sql.NullInt64
	if user, ok := authmiddleware.UserFromContext(ctx); ok {
		userID = sql.NullInt64{Int64: user.ID, Valid: true}
	}

	tmpl := &model.GitTemplate{
		UUID:          uuid.New().String(),
		Name:          name,
		Slug:          slugify(name),
		RepositoryURL: repoURL,
		Branch:        branch,
		Description:   sql.NullString{String: description, Valid: description != ""},
		Category:      sql.NullString{String: category, Valid: category != ""},
		UserID:        userID,
		IsPublic:      true,
		IsEnabled:     true,
		Status:        "pending",
	}

	if err := h.gitTemplateRepo.Create(ctx, tmpl); err != nil {
		h.renderError(w, r, "creating git template", err)
		return
	}

	http.Redirect(w, r, "/admin/git-templates/"+tmpl.UUID, http.StatusSeeOther)
}

// GitTemplateDelete soft-deletes a git template and returns an empty response.
func (h *Handler) GitTemplateDelete(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	tmplUUID := chi.URLParam(r, "uuid")

	tmpl, err := h.gitTemplateRepo.FindByUUID(ctx, tmplUUID)
	if err != nil {
		h.renderError(w, r, "finding git template for deletion", err)
		return
	}

	if err := h.gitTemplateRepo.SoftDelete(ctx, tmpl.ID); err != nil {
		h.renderError(w, r, "deleting git template", err)
		return
	}

	w.WriteHeader(http.StatusOK)
}

// ---------------------------------------------------------------------------
// Utility
// ---------------------------------------------------------------------------

// slugify converts a name to a URL-friendly slug.
func slugify(name string) string {
	s := strings.ToLower(strings.TrimSpace(name))
	s = strings.Map(func(r rune) rune {
		if r >= 'a' && r <= 'z' || r >= '0' && r <= '9' {
			return r
		}
		if r == ' ' || r == '-' || r == '_' {
			return '-'
		}
		return -1
	}, s)

	for strings.Contains(s, "--") {
		s = strings.ReplaceAll(s, "--", "-")
	}
	s = strings.Trim(s, "-")

	return s
}
