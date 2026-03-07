package api

import (
	"database/sql"
	"errors"
	"log/slog"
	"net/http"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"

	"git.999.haus/chris/DocuMCP-go/internal/client/confluence"
	"git.999.haus/chris/DocuMCP-go/internal/model"
	"git.999.haus/chris/DocuMCP-go/internal/repository"
)

// ConfluenceHandler handles REST API endpoints for Confluence spaces and pages.
type ConfluenceHandler struct {
	repo             *repository.ConfluenceSpaceRepository
	confluenceClient *confluence.Client // can be nil if not configured
	logger           *slog.Logger
}

// NewConfluenceHandler creates a new ConfluenceHandler.
func NewConfluenceHandler(
	repo *repository.ConfluenceSpaceRepository,
	confluenceClient *confluence.Client,
	logger *slog.Logger,
) *ConfluenceHandler {
	return &ConfluenceHandler{
		repo:             repo,
		confluenceClient: confluenceClient,
		logger:           logger,
	}
}

// confluenceSpaceResponse is the JSON representation of a Confluence space.
type confluenceSpaceResponse struct {
	UUID         string `json:"uuid"`
	ConfluenceID string `json:"confluence_id"`
	Key          string `json:"key"`
	Name         string `json:"name"`
	Description  string `json:"description,omitempty"`
	Type         string `json:"type"`
	Status       string `json:"status"`
	HomepageID   string `json:"homepage_id,omitempty"`
	IconURL      string `json:"icon_url,omitempty"`
	IsSearchable bool   `json:"is_searchable"`
	LastSyncedAt string `json:"last_synced_at,omitempty"`
	CreatedAt    string `json:"created_at,omitempty"`
	UpdatedAt    string `json:"updated_at,omitempty"`
}

// ListSpaces handles GET /api/confluence/spaces -- list Confluence spaces with optional filters.
func (h *ConfluenceHandler) ListSpaces(w http.ResponseWriter, r *http.Request) {
	query := r.URL.Query().Get("query")
	spaceType := r.URL.Query().Get("type")

	perPage, _ := strconv.Atoi(r.URL.Query().Get("per_page"))
	if perPage <= 0 {
		perPage = 50
	}

	spaces, err := h.repo.List(r.Context(), spaceType, query, perPage)
	if err != nil {
		h.logger.Error("listing confluence spaces", "error", err)
		errorResponse(w, http.StatusInternalServerError, "failed to list Confluence spaces")
		return
	}

	items := make([]confluenceSpaceResponse, 0, len(spaces))
	for i := range spaces {
		items = append(items, toConfluenceSpaceResponse(&spaces[i]))
	}

	jsonResponse(w, http.StatusOK, map[string]any{
		"data": items,
		"meta": map[string]any{
			"total": len(items),
		},
	})
}

// ShowSpace handles GET /api/confluence/spaces/{key} -- get a single Confluence space.
func (h *ConfluenceHandler) ShowSpace(w http.ResponseWriter, r *http.Request) {
	spaceKey := chi.URLParam(r, "key")

	space, err := h.repo.FindByKey(r.Context(), spaceKey)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			errorResponse(w, http.StatusNotFound, "Confluence space not found")
			return
		}
		h.logger.Error("finding confluence space", "key", spaceKey, "error", err)
		errorResponse(w, http.StatusInternalServerError, "failed to find Confluence space")
		return
	}

	jsonResponse(w, http.StatusOK, map[string]any{
		"data": toConfluenceSpaceResponse(space),
	})
}

// SearchPages handles GET /api/confluence/pages -- search Confluence pages via the API client.
func (h *ConfluenceHandler) SearchPages(w http.ResponseWriter, r *http.Request) {
	if h.confluenceClient == nil {
		errorResponse(w, http.StatusServiceUnavailable, "Confluence integration not configured")
		return
	}

	query := r.URL.Query().Get("query")
	space := r.URL.Query().Get("space")

	if query == "" {
		errorResponse(w, http.StatusBadRequest, "'query' parameter is required")
		return
	}

	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	if limit <= 0 {
		limit = 25
	}

	start, _ := strconv.Atoi(r.URL.Query().Get("start"))

	result, err := h.confluenceClient.SearchPages(r.Context(), confluence.SearchPagesParams{
		Query: query,
		Space: space,
		Limit: limit,
		Start: start,
	})
	if err != nil {
		h.logger.Error("searching confluence pages", "error", err)
		errorResponse(w, http.StatusInternalServerError, "page search failed")
		return
	}

	jsonResponse(w, http.StatusOK, map[string]any{
		"data": result.Pages,
		"meta": map[string]any{
			"total":    result.Total,
			"start":    result.Start,
			"limit":    result.Limit,
			"has_more": result.HasMore,
			"cql":      result.CQL,
		},
	})
}

// ReadPage handles GET /api/confluence/pages/{pageId} -- read a full Confluence page.
func (h *ConfluenceHandler) ReadPage(w http.ResponseWriter, r *http.Request) {
	if h.confluenceClient == nil {
		errorResponse(w, http.StatusServiceUnavailable, "Confluence integration not configured")
		return
	}

	pageID := chi.URLParam(r, "pageId")

	page, err := h.confluenceClient.ReadPage(r.Context(), pageID)
	if err != nil {
		h.logger.Error("reading confluence page", "page_id", pageID, "error", err)
		errorResponse(w, http.StatusInternalServerError, "failed to read page")
		return
	}

	jsonResponse(w, http.StatusOK, map[string]any{
		"data": page,
	})
}

// toConfluenceSpaceResponse converts a ConfluenceSpace model to its JSON response DTO.
func toConfluenceSpaceResponse(cs *model.ConfluenceSpace) confluenceSpaceResponse {
	resp := confluenceSpaceResponse{
		UUID:         cs.UUID,
		ConfluenceID: cs.ConfluenceID,
		Key:          cs.Key,
		Name:         cs.Name,
		Type:         cs.Type,
		Status:       cs.Status,
		IsSearchable: cs.IsSearchable,
	}

	if cs.Description.Valid {
		resp.Description = cs.Description.String
	}
	if cs.HomepageID.Valid {
		resp.HomepageID = cs.HomepageID.String
	}
	if cs.IconURL.Valid {
		resp.IconURL = cs.IconURL.String
	}
	if cs.LastSyncedAt.Valid {
		resp.LastSyncedAt = cs.LastSyncedAt.Time.Format(time.RFC3339)
	}
	if cs.CreatedAt.Valid {
		resp.CreatedAt = cs.CreatedAt.Time.Format(time.RFC3339)
	}
	if cs.UpdatedAt.Valid {
		resp.UpdatedAt = cs.UpdatedAt.Time.Format(time.RFC3339)
	}

	return resp
}
