package api

import (
	"context"
	"log/slog"
	"net/http"

	"git.999.haus/chris/DocuMCP-go/internal/queue"
)

// Interfaces accepted by DashboardHandler — defined where consumed.

type dashboardDocumentCounter interface {
	Count(ctx context.Context) (int, error)
}

type dashboardUserCounter interface {
	CountUsers(ctx context.Context) (int, error)
}

type dashboardOAuthClientCounter interface {
	CountClients(ctx context.Context) (int, error)
}

type dashboardExternalServiceCounter interface {
	Count(ctx context.Context) (int, error)
}

type dashboardZimCounter interface {
	Count(ctx context.Context) (int, error)
}

type dashboardConfluenceCounter interface {
	Count(ctx context.Context) (int, error)
}

type dashboardGitTemplateCounter interface {
	Count(ctx context.Context) (int, error)
}

// DashboardHandler serves aggregate statistics for the admin dashboard.
type DashboardHandler struct {
	docRepo         dashboardDocumentCounter
	userRepo        dashboardUserCounter
	oauthClientRepo dashboardOAuthClientCounter
	extSvcRepo      dashboardExternalServiceCounter
	zimRepo         dashboardZimCounter
	confluenceRepo  dashboardConfluenceCounter
	gitTemplateRepo dashboardGitTemplateCounter
	riverClient     *queue.RiverClient
	logger          *slog.Logger
}

// NewDashboardHandler creates a DashboardHandler. Any counter may be nil; nil
// counters produce a zero value in the response.
func NewDashboardHandler(
	docRepo dashboardDocumentCounter,
	userRepo dashboardUserCounter,
	oauthClientRepo dashboardOAuthClientCounter,
	extSvcRepo dashboardExternalServiceCounter,
	zimRepo dashboardZimCounter,
	confluenceRepo dashboardConfluenceCounter,
	gitTemplateRepo dashboardGitTemplateCounter,
	riverClient *queue.RiverClient,
	logger *slog.Logger,
) *DashboardHandler {
	return &DashboardHandler{
		docRepo:         docRepo,
		userRepo:        userRepo,
		oauthClientRepo: oauthClientRepo,
		extSvcRepo:      extSvcRepo,
		zimRepo:         zimRepo,
		confluenceRepo:  confluenceRepo,
		gitTemplateRepo: gitTemplateRepo,
		riverClient:     riverClient,
		logger:          logger,
	}
}

// Stats handles GET /api/admin/dashboard/stats.
func (h *DashboardHandler) Stats(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	docs := h.countOrZero(ctx, "documents", func(ctx context.Context) (int, error) {
		if h.docRepo == nil {
			return 0, nil
		}
		return h.docRepo.Count(ctx)
	})

	users := h.countOrZero(ctx, "users", func(ctx context.Context) (int, error) {
		if h.userRepo == nil {
			return 0, nil
		}
		return h.userRepo.CountUsers(ctx)
	})

	oauthClients := h.countOrZero(ctx, "oauth_clients", func(ctx context.Context) (int, error) {
		if h.oauthClientRepo == nil {
			return 0, nil
		}
		return h.oauthClientRepo.CountClients(ctx)
	})

	extServices := h.countOrZero(ctx, "external_services", func(ctx context.Context) (int, error) {
		if h.extSvcRepo == nil {
			return 0, nil
		}
		return h.extSvcRepo.Count(ctx)
	})

	zimArchives := h.countOrZero(ctx, "zim_archives", func(ctx context.Context) (int, error) {
		if h.zimRepo == nil {
			return 0, nil
		}
		return h.zimRepo.Count(ctx)
	})

	confluenceSpaces := h.countOrZero(ctx, "confluence_spaces", func(ctx context.Context) (int, error) {
		if h.confluenceRepo == nil {
			return 0, nil
		}
		return h.confluenceRepo.Count(ctx)
	})

	gitTemplates := h.countOrZero(ctx, "git_templates", func(ctx context.Context) (int, error) {
		if h.gitTemplateRepo == nil {
			return 0, nil
		}
		return h.gitTemplateRepo.Count(ctx)
	})

	resp := map[string]any{
		"documents":         docs,
		"users":             users,
		"oauth_clients":     oauthClients,
		"external_services": extServices,
		"zim_archives":      zimArchives,
		"confluence_spaces": confluenceSpaces,
		"git_templates":     gitTemplates,
	}

	if h.riverClient != nil {
		stats, err := h.riverClient.QueueStats(ctx)
		if err != nil {
			h.logger.Error("fetching queue stats for dashboard", "error", err)
		} else {
			resp["queue"] = map[string]int{
				"pending":   stats["available"] + stats["retryable"],
				"completed": stats["running"],
				"failed":    stats["discarded"] + stats["cancelled"],
			}
		}
	}

	jsonResponse(w, http.StatusOK, resp)
}

// countOrZero calls fn and returns the result, logging and returning 0 on error.
func (h *DashboardHandler) countOrZero(ctx context.Context, name string, fn func(context.Context) (int, error)) int {
	count, err := fn(ctx)
	if err != nil {
		h.logger.Error("counting entities for dashboard", "entity", name, "error", err)
		return 0
	}
	return count
}
