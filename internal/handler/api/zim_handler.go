package api

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log/slog"
	"net/http"

	"github.com/go-chi/chi/v5"

	"git.999.haus/chris/DocuMCP-go/internal/client/kiwix"
	"git.999.haus/chris/DocuMCP-go/internal/model"
)

// zimArchiveRepo defines the methods used by ZimHandler -- defined where consumed.
type zimArchiveRepo interface {
	List(ctx context.Context, category, language, query string, limit, offset int) ([]model.ZimArchive, error)
	CountFiltered(ctx context.Context, category, language, query string) (int, error)
	FindByName(ctx context.Context, name string) (*model.ZimArchive, error)
}

// kiwixSearcher defines the methods used by ZimHandler from the kiwix client -- defined where consumed.
type kiwixSearcher interface {
	Search(ctx context.Context, archiveName, query, searchType string, limit int) ([]kiwix.SearchResult, error)
	ReadArticle(ctx context.Context, archiveName, articlePath string) (*kiwix.Article, error)
}

// kiwixClientFactory creates or returns a cached Kiwix client on demand.
type kiwixClientFactory interface {
	Get(ctx context.Context) (kiwixSearcher, error)
}

// ZimHandler handles REST API endpoints for ZIM archives.
type ZimHandler struct {
	repo         zimArchiveRepo
	kiwixFactory kiwixClientFactory // lazy-init; nil if not configured
	logger       *slog.Logger
}

// NewZimHandler creates a new ZimHandler.
func NewZimHandler(
	repo zimArchiveRepo,
	kiwixFactory kiwixClientFactory,
	logger *slog.Logger,
) *ZimHandler {
	return &ZimHandler{
		repo:         repo,
		kiwixFactory: kiwixFactory,
		logger:       logger,
	}
}

// KiwixFactoryAdapter wraps *kiwix.ClientFactory to satisfy kiwixClientFactory.
type KiwixFactoryAdapter struct {
	Factory *kiwix.ClientFactory
}

// Get returns the kiwix client as a kiwixSearcher.
func (a *KiwixFactoryAdapter) Get(ctx context.Context) (kiwixSearcher, error) {
	return a.Factory.Get(ctx)
}

// getKiwixClient returns a Kiwix client from the factory.
func (h *ZimHandler) getKiwixClient(ctx context.Context) (kiwixSearcher, error) {
	if h.kiwixFactory == nil {
		return nil, errors.New("kiwix not configured")
	}
	return h.kiwixFactory.Get(ctx)
}

// zimArchiveResponse is the JSON representation of a ZIM archive.
type zimArchiveResponse struct {
	UUID          string   `json:"uuid"`
	Name          string   `json:"name"`
	Title         string   `json:"title"`
	Description   string   `json:"description,omitempty"`
	Language      string   `json:"language"`
	Category      string   `json:"category,omitempty"`
	Creator       string   `json:"creator,omitempty"`
	Publisher     string   `json:"publisher,omitempty"`
	ArticleCount  int64    `json:"article_count"`
	MediaCount    int64    `json:"media_count"`
	FileSize      int64    `json:"file_size"`
	FileSizeHuman string   `json:"file_size_human"`
	Tags          []string `json:"tags"`
	LastSyncedAt  string   `json:"last_synced_at,omitempty"`
}

// zimSearchResultResponse is the JSON representation of a ZIM search result.
type zimSearchResultResponse struct {
	Title   string  `json:"title"`
	Path    string  `json:"path"`
	Snippet string  `json:"snippet,omitempty"`
	Score   float64 `json:"score,omitempty"`
}

// zimArticleResponse is the JSON representation of a ZIM article.
type zimArticleResponse struct {
	ArchiveName string `json:"archive_name"`
	Path        string `json:"path"`
	Title       string `json:"title"`
	Content     string `json:"content"`
	MIMEType    string `json:"mime_type"`
}

// List handles GET /api/zim/archives -- list ZIM archives with optional filters.
func (h *ZimHandler) List(w http.ResponseWriter, r *http.Request) {
	query := r.URL.Query().Get("query")
	category := r.URL.Query().Get("category")
	language := r.URL.Query().Get("language")

	perPage, offset := parsePaginationParam(r, "per_page", 50, 100)

	total, err := h.repo.CountFiltered(r.Context(), category, language, query)
	if err != nil {
		h.logger.Error("counting zim archives", "error", err)
		errorResponse(w, http.StatusInternalServerError, "failed to count ZIM archives")
		return
	}

	archives, err := h.repo.List(r.Context(), category, language, query, perPage, offset)
	if err != nil {
		h.logger.Error("listing zim archives", "error", err)
		errorResponse(w, http.StatusInternalServerError, "failed to list ZIM archives")
		return
	}

	items := make([]zimArchiveResponse, 0, len(archives))
	for i := range archives {
		items = append(items, toZimArchiveResponse(&archives[i]))
	}

	jsonResponse(w, http.StatusOK, listResponse(items, total, perPage, offset))
}

// Show handles GET /api/zim/archives/{archive} -- get a single ZIM archive by name.
func (h *ZimHandler) Show(w http.ResponseWriter, r *http.Request) {
	archiveName := chi.URLParam(r, "archive")

	archive, err := h.repo.FindByName(r.Context(), archiveName)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			errorResponse(w, http.StatusNotFound, "ZIM archive not found")
			return
		}
		h.logger.Error("finding zim archive", "name", archiveName, "error", err)
		errorResponse(w, http.StatusInternalServerError, "failed to find ZIM archive")
		return
	}

	jsonResponse(w, http.StatusOK, map[string]any{
		"data": toZimArchiveResponse(archive),
	})
}

// Search handles GET /api/zim/archives/{archive}/search -- full-text search within an archive.
func (h *ZimHandler) Search(w http.ResponseWriter, r *http.Request) {
	kiwixClient, clientErr := h.getKiwixClient(r.Context())
	if clientErr != nil {
		errorResponse(w, http.StatusServiceUnavailable, "Kiwix integration not configured")
		return
	}


	archiveName := chi.URLParam(r, "archive")

	query := r.URL.Query().Get("q")
	if query == "" {
		errorResponse(w, http.StatusBadRequest, "query parameter 'q' is required")
		return
	}

	limit, _ := parsePagination(r, 10, 100)

	results, err := kiwixClient.Search(r.Context(), archiveName, query, "fulltext", limit)
	if err != nil {
		h.logger.Error("searching zim archive", "archive", archiveName, "query", query, "error", err)
		errorResponse(w, http.StatusInternalServerError, "search failed")
		return
	}

	items := make([]zimSearchResultResponse, 0, len(results))
	for _, res := range results {
		items = append(items, zimSearchResultResponse{
			Title:   res.Title,
			Path:    res.Path,
			Snippet: res.Snippet,
			Score:   res.Score,
		})
	}

	jsonResponse(w, http.StatusOK, map[string]any{
		"data": items,
		"meta": map[string]any{
			"archive": archiveName,
			"query":   query,
			"total":   len(items),
		},
	})
}

// Suggest handles GET /api/zim/archives/{archive}/suggest -- autocomplete suggestions.
func (h *ZimHandler) Suggest(w http.ResponseWriter, r *http.Request) {
	kiwixClient, clientErr := h.getKiwixClient(r.Context())
	if clientErr != nil {
		errorResponse(w, http.StatusServiceUnavailable, "Kiwix integration not configured")
		return
	}


	archiveName := chi.URLParam(r, "archive")

	query := r.URL.Query().Get("q")
	if query == "" || len(query) < 2 {
		errorResponse(w, http.StatusBadRequest, "query parameter 'q' is required and must be at least 2 characters")
		return
	}

	limit, _ := parsePagination(r, 10, 50)

	results, err := kiwixClient.Search(r.Context(), archiveName, query, "suggest", limit)
	if err != nil {
		h.logger.Error("suggesting zim archive", "archive", archiveName, "query", query, "error", err)
		errorResponse(w, http.StatusInternalServerError, "suggest failed")
		return
	}

	items := make([]zimSearchResultResponse, 0, len(results))
	for _, res := range results {
		items = append(items, zimSearchResultResponse{
			Title:   res.Title,
			Path:    res.Path,
			Snippet: res.Snippet,
			Score:   res.Score,
		})
	}

	jsonResponse(w, http.StatusOK, map[string]any{
		"data": items,
		"meta": map[string]any{
			"archive": archiveName,
			"query":   query,
			"total":   len(items),
		},
	})
}

// ReadArticle handles GET /api/zim/archives/{archive}/articles/* -- read a single article.
func (h *ZimHandler) ReadArticle(w http.ResponseWriter, r *http.Request) {
	kiwixClient, clientErr := h.getKiwixClient(r.Context())
	if clientErr != nil {
		errorResponse(w, http.StatusServiceUnavailable, "Kiwix integration not configured")
		return
	}


	archiveName := chi.URLParam(r, "archive")
	articlePath := chi.URLParam(r, "*")

	if articlePath == "" {
		errorResponse(w, http.StatusBadRequest, "article path is required")
		return
	}

	article, err := kiwixClient.ReadArticle(r.Context(), archiveName, articlePath)
	if err != nil {
		h.logger.Error("reading zim article", "archive", archiveName, "path", articlePath, "error", err)
		errorResponse(w, http.StatusInternalServerError, "failed to read article")
		return
	}

	jsonResponse(w, http.StatusOK, map[string]any{
		"data": zimArticleResponse{
			ArchiveName: archiveName,
			Path:        articlePath,
			Title:       article.Title,
			Content:     article.Content,
			MIMEType:    article.MIMEType,
		},
	})
}

// toZimArchiveResponse converts a ZimArchive model to its JSON response DTO.
func toZimArchiveResponse(za *model.ZimArchive) zimArchiveResponse {
	tags, _ := za.ParseTags()
	if tags == nil {
		tags = []string{}
	}

	resp := zimArchiveResponse{
		UUID:          za.UUID,
		Name:          za.Name,
		Title:         za.Title,
		Language:      za.Language,
		ArticleCount:  za.ArticleCount,
		MediaCount:    za.MediaCount,
		FileSize:      za.FileSize,
		FileSizeHuman: humanFileSize(za.FileSize),
		Tags:          tags,
	}

	resp.Description = nullStringValue(za.Description)
	resp.Category = nullStringValue(za.Category)
	resp.Creator = nullStringValue(za.Creator)
	resp.Publisher = nullStringValue(za.Publisher)
	resp.LastSyncedAt = nullTimeToString(za.LastSyncedAt)

	return resp
}

// humanFileSize converts a byte count to a human-readable string (e.g. "1.2 GB", "456 MB").
func humanFileSize(bytes int64) string {
	const (
		kb = 1024
		mb = kb * 1024
		gb = mb * 1024
		tb = gb * 1024
	)

	switch {
	case bytes >= tb:
		return fmt.Sprintf("%.1f TB", float64(bytes)/float64(tb))
	case bytes >= gb:
		return fmt.Sprintf("%.1f GB", float64(bytes)/float64(gb))
	case bytes >= mb:
		return fmt.Sprintf("%.1f MB", float64(bytes)/float64(mb))
	case bytes >= kb:
		return fmt.Sprintf("%.1f KB", float64(bytes)/float64(kb))
	default:
		return fmt.Sprintf("%d B", bytes)
	}
}
