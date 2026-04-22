package api //nolint:revive // package name matches REST API handler directory convention

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"github.com/google/uuid"

	authmiddleware "github.com/c-premus/documcp/internal/auth/middleware"
	"github.com/c-premus/documcp/internal/model"
	"github.com/c-premus/documcp/internal/queue"
)

// SessionValidator re-checks that the credentials backing a long-lived stream
// are still valid. Called periodically from SSE handlers so tokens revoked
// mid-stream, users deleted mid-stream, or admins demoted mid-stream drop
// the connection instead of receiving events until the client disconnects.
type SessionValidator interface {
	FindUserByID(ctx context.Context, id int64) (*model.User, error)
	FindAccessTokenByID(ctx context.Context, id int64) (*model.OAuthAccessToken, error)
}

// SSEHandler streams real-time queue events via Server-Sent Events.
type SSEHandler struct {
	eventBus          queue.EventSubscriber
	heartbeatInterval time.Duration
	validator         SessionValidator
	logger            *slog.Logger
}

// NewSSEHandler creates an SSEHandler. validator may be nil to disable
// mid-stream credential re-validation (tests). logger may be nil.
func NewSSEHandler(eventBus queue.EventSubscriber, heartbeatInterval time.Duration, validator SessionValidator, logger *slog.Logger) *SSEHandler {
	if logger == nil {
		logger = slog.New(slog.DiscardHandler)
	}
	return &SSEHandler{
		eventBus:          eventBus,
		heartbeatInterval: heartbeatInterval,
		validator:         validator,
		logger:            logger,
	}
}

// errStreamCredentialsInvalid is returned by revalidate when the stream must
// be dropped.
var errStreamCredentialsInvalid = errors.New("stream credentials no longer valid")

// revalidate re-checks user and token while the stream is open. Returns an
// error wrapped in errStreamCredentialsInvalid when the stream should end.
// With a nil validator (tests) this is a no-op so tests don't need a DB.
//
// requireAdmin triggers a drop when the user's admin bit flipped off — only
// meaningful for the admin /events/stream. For the per-user stream we re-fetch
// the user but don't drop on admin changes; instead the caller updates its
// local filter state so non-admin events flow at the new privilege level.
func (h *SSEHandler) revalidate(ctx context.Context, userID, tokenID int64, requireAdmin bool) (*model.User, error) {
	if h.validator == nil {
		return nil, nil
	}

	user, err := h.validator.FindUserByID(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("%w: user lookup: %w", errStreamCredentialsInvalid, err)
	}
	if user == nil {
		return nil, fmt.Errorf("%w: user %d not found", errStreamCredentialsInvalid, userID)
	}
	if requireAdmin && !user.IsAdmin {
		return nil, fmt.Errorf("%w: user %d no longer admin", errStreamCredentialsInvalid, userID)
	}

	if tokenID > 0 {
		token, err := h.validator.FindAccessTokenByID(ctx, tokenID)
		if err != nil {
			return nil, fmt.Errorf("%w: token lookup: %w", errStreamCredentialsInvalid, err)
		}
		if token == nil {
			return nil, fmt.Errorf("%w: token %d not found", errStreamCredentialsInvalid, tokenID)
		}
		if token.Revoked {
			return nil, fmt.Errorf("%w: token %d revoked", errStreamCredentialsInvalid, tokenID)
		}
		if !token.ExpiresAt.IsZero() && time.Now().After(token.ExpiresAt) {
			return nil, fmt.Errorf("%w: token %d expired", errStreamCredentialsInvalid, tokenID)
		}
	}
	return user, nil
}

// Stream handles GET /api/admin/events/stream — unfiltered SSE for admin clients.
func (h *SSEHandler) Stream(w http.ResponseWriter, r *http.Request) {
	user, ok := authmiddleware.UserFromContext(r.Context())
	if !ok || user == nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "streaming not supported", http.StatusInternalServerError)
		return
	}

	// Disable the server's write deadline for this connection so the SSE stream
	// can stay open indefinitely. Without this, http.Server.WriteTimeout kills
	// the connection after the configured duration (default 10 s).
	rc := http.NewResponseController(w)
	_ = rc.SetWriteDeadline(time.Time{})

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("X-Accel-Buffering", "no")

	subID := uuid.New().String()
	events := h.eventBus.Subscribe(subID)
	if events == nil {
		http.Error(w, "too many concurrent connections", http.StatusServiceUnavailable)
		return
	}
	defer h.eventBus.Unsubscribe(subID)

	// Flush headers immediately so the browser's EventSource fires onopen now,
	// not 30 s later when the first heartbeat is sent. Without this flush the
	// HTTP 200 response is not committed until the first body write, causing
	// the browser to see a "pending" request and delaying connection-pool slots.
	// retry: 5000 tells EventSource to wait 5 s before reconnecting on error.
	_, _ = fmt.Fprint(w, "retry: 5000\n\n")
	flusher.Flush()

	heartbeat := time.NewTicker(h.heartbeatInterval)
	defer heartbeat.Stop()

	tokenID := accessTokenIDFromContext(r.Context())

	for {
		select {
		case <-r.Context().Done():
			return
		case <-heartbeat.C:
			if _, err := h.revalidate(r.Context(), user.ID, tokenID, true); err != nil {
				h.logger.Info("sse admin stream dropped on revalidation",
					"user_id", user.ID,
					"error", err,
				)
				return
			}
			_, _ = fmt.Fprint(w, ": heartbeat\n\n")
			flusher.Flush()
		case event, ok := <-events:
			if !ok {
				return
			}
			data, err := json.Marshal(event)
			if err != nil {
				continue
			}
			_, _ = fmt.Fprintf(w, "data: %s\n\n", data)
			flusher.Flush()
		}
	}
}

// UserStream handles GET /api/events/stream — SSE with per-user filtering.
// Admins see all events; non-admins see only events where UserID matches.
func (h *SSEHandler) UserStream(w http.ResponseWriter, r *http.Request) {
	user, ok := authmiddleware.UserFromContext(r.Context())
	if !ok || user == nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "streaming not supported", http.StatusInternalServerError)
		return
	}

	rc := http.NewResponseController(w)
	_ = rc.SetWriteDeadline(time.Time{})

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("X-Accel-Buffering", "no")

	subID := uuid.New().String()
	events := h.eventBus.Subscribe(subID)
	if events == nil {
		http.Error(w, "too many concurrent connections", http.StatusServiceUnavailable)
		return
	}
	defer h.eventBus.Unsubscribe(subID)

	_, _ = fmt.Fprint(w, "retry: 5000\n\n")
	flusher.Flush()

	heartbeat := time.NewTicker(h.heartbeatInterval)
	defer heartbeat.Stop()

	isAdmin := user.IsAdmin
	tokenID := accessTokenIDFromContext(r.Context())

	for {
		select {
		case <-r.Context().Done():
			return
		case <-heartbeat.C:
			fresh, err := h.revalidate(r.Context(), user.ID, tokenID, false)
			if err != nil {
				h.logger.Info("sse user stream dropped on revalidation",
					"user_id", user.ID,
					"error", err,
				)
				return
			}
			if fresh != nil {
				isAdmin = fresh.IsAdmin
			}
			_, _ = fmt.Fprint(w, ": heartbeat\n\n")
			flusher.Flush()
		case event, ok := <-events:
			if !ok {
				return
			}
			// Non-admins only see events addressed to them.
			if !isAdmin && event.UserID != user.ID {
				continue
			}
			data, err := json.Marshal(event)
			if err != nil {
				continue
			}
			_, _ = fmt.Fprintf(w, "data: %s\n\n", data)
			flusher.Flush()
		}
	}
}

// accessTokenIDFromContext returns the bearer token's primary-key id from the
// request context, or 0 when the request was authenticated via session cookie.
func accessTokenIDFromContext(ctx context.Context) int64 {
	token, ok := ctx.Value(authmiddleware.AccessTokenContextKey).(*model.OAuthAccessToken)
	if !ok || token == nil {
		return 0
	}
	return token.ID
}
