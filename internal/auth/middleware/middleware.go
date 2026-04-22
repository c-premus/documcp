// Package authmiddleware provides HTTP middleware for authentication and authorization.
package authmiddleware

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/gorilla/sessions"

	"github.com/c-premus/documcp/internal/auth/oauth"
	"github.com/c-premus/documcp/internal/model"
	"github.com/c-premus/documcp/internal/observability"
)

// LoginAtSessionKey is the session-values key under which the OIDC callback
// writes the login timestamp (unix seconds). Exported so the OIDC handler and
// tests can reference a single source of truth.
const LoginAtSessionKey = "login_at"

type contextKey string

const (
	// UserContextKey is the context key for the authenticated user.
	UserContextKey contextKey = "user"
	// AccessTokenContextKey is the context key for the validated access token.
	AccessTokenContextKey contextKey = "access_token"
)

// Sentinel errors for auth helpers.
var (
	errNotBearer    = errors.New("not a bearer token")
	errInvalidToken = errors.New("invalid or expired token")
	errNoSession    = errors.New("no valid session")
	errSessionStale = errors.New("session exceeded absolute lifetime")
)

// bearerResult holds the outcome of bearer token authentication.
type bearerResult struct {
	token *model.OAuthAccessToken
	user  *model.User
}

// authenticateBearerToken extracts and validates a bearer token from the
// Authorization header value. On success it triggers a debounced background
// last_used_at update via the oauth service and optionally loads the
// associated user.
func authenticateBearerToken(ctx context.Context, authHeader string, oauthService *oauth.Service, logger *slog.Logger) (*bearerResult, error) {
	bearerToken := strings.TrimPrefix(authHeader, "Bearer ")
	if bearerToken == authHeader {
		return nil, errNotBearer
	}

	token, err := oauthService.ValidateAccessToken(ctx, bearerToken)
	if err != nil {
		return nil, errInvalidToken
	}

	oauthService.TouchClientLastUsedAsync(token.ClientID, logger)

	result := &bearerResult{token: token}
	if token.UserID.Valid {
		user, err := oauthService.FindUserByID(ctx, token.UserID.Int64)
		if err != nil {
			logger.Warn("loading user for bearer token", "user_id", token.UserID.Int64, "error", err)
		} else {
			result.user = user
		}
	}
	return result, nil
}

// loadSessionUser retrieves the authenticated user from the session cookie.
// When absoluteMaxAge > 0, sessions without a login_at anchor or whose anchor
// has aged beyond absoluteMaxAge return errSessionStale so callers clear the
// cookie and force a fresh OIDC flow.
func loadSessionUser(ctx context.Context, r *http.Request, store sessions.Store, oauthService *oauth.Service, absoluteMaxAge time.Duration, logger *slog.Logger) (*model.User, *sessions.Session, error) {
	session, err := store.Get(r, "documcp_session")
	if err != nil {
		logger.Warn("session decode error", "error", err)
	}

	userID, ok := session.Values["user_id"].(int64)
	if !ok || userID == 0 {
		return nil, session, errNoSession
	}

	if absoluteMaxAge > 0 {
		loginAt, hasAnchor := session.Values[LoginAtSessionKey].(int64)
		if !hasAnchor || time.Since(time.Unix(loginAt, 0)) > absoluteMaxAge {
			return nil, session, errSessionStale
		}
	}

	user, err := oauthService.FindUserByID(ctx, userID)
	if err != nil {
		return nil, session, fmt.Errorf("loading session user %d: %w", userID, err)
	}

	return user, session, nil
}

// setBearerContext sets the access token and optional user in the request context.
func setBearerContext(r *http.Request, result *bearerResult) *http.Request {
	ctx := context.WithValue(r.Context(), AccessTokenContextKey, result.token)
	if result.user != nil {
		ctx = context.WithValue(ctx, UserContextKey, result.user)
		observability.SetUser(ctx, result.user.ID, result.user.Email)
	}
	return r.WithContext(ctx)
}

// UserFromContext returns the authenticated user from the request context.
func UserFromContext(ctx context.Context) (*model.User, bool) {
	user, ok := ctx.Value(UserContextKey).(*model.User)
	return user, ok
}

// BearerToken validates an OAuth 2.1 bearer token from the Authorization header.
// On success, it sets the access token and user in the request context.
// When no Authorization header is present, it is bearer-only and rejects the request.
// Use BearerOrSession to allow session cookie fallback for SPA admin routes.
func BearerToken(oauthService *oauth.Service, logger *slog.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			authHeader := r.Header.Get("Authorization")
			if authHeader == "" {
				logger.Warn("auth failed: missing bearer token",
					"client_ip", r.RemoteAddr,
					"path", r.URL.Path,
					"method", r.Method,
				)
				w.Header().Set("WWW-Authenticate", `Bearer`)
				jsonError(w, http.StatusUnauthorized, "Bearer token required")
				return
			}

			result, err := authenticateBearerToken(r.Context(), authHeader, oauthService, logger)
			if err != nil {
				if errors.Is(err, errInvalidToken) {
					logger.Warn("auth failed: invalid bearer token",
						"client_ip", r.RemoteAddr,
						"path", r.URL.Path,
						"method", r.Method,
					)
					w.Header().Set("WWW-Authenticate", `Bearer error="invalid_token"`)
					jsonError(w, http.StatusUnauthorized, "Invalid or expired token")
				} else {
					logger.Warn("auth failed: malformed bearer token",
						"client_ip", r.RemoteAddr,
						"path", r.URL.Path,
						"method", r.Method,
					)
					w.Header().Set("WWW-Authenticate", `Bearer`)
					jsonError(w, http.StatusUnauthorized, "Bearer token required")
				}
				return
			}

			next.ServeHTTP(w, setBearerContext(r, result))
		})
	}
}

// BearerOrSession tries bearer token auth first, then falls back to session
// cookie auth. This allows the same API routes to serve both MCP/API clients
// (bearer token) and the admin SPA (session cookie).
//
// absoluteMaxAge caps session lifetime regardless of activity (security M1).
// When > 0, sessions without a login_at anchor or whose anchor is older than
// absoluteMaxAge are rejected as stale and the cookie is cleared.
func BearerOrSession(oauthService *oauth.Service, store sessions.Store, logger *slog.Logger, absoluteMaxAge time.Duration) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// If Authorization header is present, use bearer token auth.
			if authHeader := r.Header.Get("Authorization"); authHeader != "" {
				result, err := authenticateBearerToken(r.Context(), authHeader, oauthService, logger)
				if err != nil {
					if errors.Is(err, errInvalidToken) {
						logger.Warn("auth failed: invalid bearer token",
							"client_ip", r.RemoteAddr,
							"path", r.URL.Path,
							"method", r.Method,
						)
						w.Header().Set("WWW-Authenticate", `Bearer error="invalid_token"`)
						jsonError(w, http.StatusUnauthorized, "Invalid or expired token")
					} else {
						logger.Warn("auth failed: malformed bearer token",
							"client_ip", r.RemoteAddr,
							"path", r.URL.Path,
							"method", r.Method,
						)
						w.Header().Set("WWW-Authenticate", `Bearer`)
						jsonError(w, http.StatusUnauthorized, "Bearer token required")
					}
					return
				}

				next.ServeHTTP(w, setBearerContext(r, result))
				return
			}

			// No Authorization header — try session cookie.
			user, session, err := loadSessionUser(r.Context(), r, store, oauthService, absoluteMaxAge, logger)
			if err != nil {
				if errors.Is(err, errSessionStale) {
					logger.Info("auth failed: session exceeded absolute lifetime",
						"client_ip", r.RemoteAddr,
						"path", r.URL.Path,
						"method", r.Method,
					)
					clearSessionCookie(w, r, session)
				} else {
					logger.Warn("auth failed: invalid session",
						"client_ip", r.RemoteAddr,
						"path", r.URL.Path,
						"method", r.Method,
					)
				}
				jsonError(w, http.StatusUnauthorized, "Authentication required")
				return
			}

			ctx := context.WithValue(r.Context(), UserContextKey, user)
			observability.SetUser(ctx, user.ID, user.Email)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// clearSessionCookie writes an immediately-expiring cookie to remove a stale
// session so the browser stops sending it.
func clearSessionCookie(w http.ResponseWriter, r *http.Request, session *sessions.Session) {
	if session == nil {
		return
	}
	session.Options.MaxAge = -1
	_ = session.Save(r, w)
}

// BearerTokenWithAudience validates a bearer token like BearerToken and
// additionally requires that the token's RFC 8707 resource binding matches
// expectedResource. Tokens minted before the audience-binding migration
// (Resource is NULL) are rejected.
func BearerTokenWithAudience(oauthService *oauth.Service, logger *slog.Logger, expectedResource string) func(http.Handler) http.Handler {
	inner := BearerToken(oauthService, logger)
	return func(next http.Handler) http.Handler {
		return inner(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			token, ok := r.Context().Value(AccessTokenContextKey).(*model.OAuthAccessToken)
			if !ok {
				// BearerToken should always set the token; defensive guard.
				w.Header().Set("WWW-Authenticate", `Bearer error="invalid_token"`)
				jsonError(w, http.StatusUnauthorized, "Invalid or expired token")
				return
			}
			if !audienceMatches(token, expectedResource) {
				logger.Warn("auth failed: token audience mismatch",
					"client_ip", r.RemoteAddr,
					"path", r.URL.Path,
					"expected", expectedResource,
					"token_resource", nullStringOrEmpty(token.Resource),
				)
				w.Header().Set("WWW-Authenticate", `Bearer error="invalid_token", error_description="audience mismatch"`)
				jsonError(w, http.StatusUnauthorized, "Token not valid for this resource")
				return
			}
			next.ServeHTTP(w, r)
		}))
	}
}

// BearerOrSessionWithAudience layers an audience check on top of
// BearerOrSession. The audience check applies only when the request is
// authenticated by bearer token; session-cookie requests bypass the check
// because sessions are not bound to a specific resource.
func BearerOrSessionWithAudience(oauthService *oauth.Service, store sessions.Store, logger *slog.Logger, expectedResource string, absoluteMaxAge time.Duration) func(http.Handler) http.Handler {
	inner := BearerOrSession(oauthService, store, logger, absoluteMaxAge)
	return func(next http.Handler) http.Handler {
		return inner(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// If a token is present in context, the request was authenticated
			// via bearer; enforce audience. Otherwise it's a session — pass.
			token, ok := r.Context().Value(AccessTokenContextKey).(*model.OAuthAccessToken)
			if ok && !audienceMatches(token, expectedResource) {
				logger.Warn("auth failed: token audience mismatch",
					"client_ip", r.RemoteAddr,
					"path", r.URL.Path,
					"expected", expectedResource,
					"token_resource", nullStringOrEmpty(token.Resource),
				)
				w.Header().Set("WWW-Authenticate", `Bearer error="invalid_token", error_description="audience mismatch"`)
				jsonError(w, http.StatusUnauthorized, "Token not valid for this resource")
				return
			}
			next.ServeHTTP(w, r)
		}))
	}
}

// audienceMatches returns true when the token's bound resource exactly equals
// the expected resource. NULL/empty resource never matches.
func audienceMatches(token *model.OAuthAccessToken, expected string) bool {
	if !token.Resource.Valid || token.Resource.String == "" {
		return false
	}
	return token.Resource.String == expected
}

func nullStringOrEmpty(s sql.NullString) string {
	if !s.Valid {
		return ""
	}
	return s.String
}

// SessionAuth validates an admin session cookie.
// On success, it sets the user in the request context.
//
// absoluteMaxAge caps session lifetime regardless of activity (security M1).
// Stale sessions redirect to /auth/login after clearing the cookie.
func SessionAuth(store sessions.Store, oauthService *oauth.Service, logger *slog.Logger, absoluteMaxAge time.Duration) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			user, session, err := loadSessionUser(r.Context(), r, store, oauthService, absoluteMaxAge, logger)
			if err != nil {
				if !errors.Is(err, errNoSession) {
					// User lookup failed or session stale — clear the cookie.
					clearSessionCookie(w, r, session)
				}
				http.Redirect(w, r, "/auth/login?redirect="+url.QueryEscape(r.URL.RequestURI()), http.StatusFound)
				return
			}

			ctx := context.WithValue(r.Context(), UserContextKey, user)
			observability.SetUser(ctx, user.ID, user.Email)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// RequireAdmin checks that the authenticated user is an admin.
// Must be used after SessionAuth or BearerToken middleware.
func RequireAdmin(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		user, ok := UserFromContext(r.Context())
		if !ok || !user.IsAdmin {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusForbidden)
			_ = json.NewEncoder(w).Encode(map[string]any{
				"error":   "Forbidden",
				"message": "Admin privileges required.",
			})
			return
		}
		next.ServeHTTP(w, r)
	})
}

// RequireScope returns middleware that checks the authenticated token has the required scope.
// Scopes are space-delimited per RFC 6749. Tokens with no scope or empty scope are rejected.
// Must be used after BearerToken or BearerOrSession middleware.
//
// SECURITY CONTRACT: Session-authenticated users (no access token) are allowed through
// because session cookies do not carry OAuth scopes. For these users, data-level access
// control MUST be enforced at the handler level:
//   - Document handlers filter by ownership (OwnerOrPublic) for non-admin users
//   - Write routes are additionally gated by RequireAdmin middleware
//   - External sources (ZIM, git templates) are accessible to all authenticated users
//
// Any new endpoint serving user-scoped data MUST enforce ownership for non-admin users.
func RequireScope(scope string, logger *slog.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			token, ok := r.Context().Value(AccessTokenContextKey).(*model.OAuthAccessToken)
			if !ok || token == nil {
				// No access token — check if this is a session-authenticated user.
				if _, hasUser := UserFromContext(r.Context()); hasUser {
					next.ServeHTTP(w, r)
					return
				}
				jsonError(w, http.StatusUnauthorized, "Authentication required")
				return
			}

			tokenScope := ""
			if token.Scope.Valid {
				tokenScope = token.Scope.String
			}

			// Check if the required scope is present (space-delimited).
			for s := range strings.SplitSeq(tokenScope, " ") {
				if s == scope {
					next.ServeHTTP(w, r)
					return
				}
			}

			logger.Warn("auth failed: insufficient scope",
				"client_ip", r.RemoteAddr,
				"path", r.URL.Path,
				"method", r.Method,
				"required_scope", scope,
			)
			w.Header().Set("WWW-Authenticate", `Bearer error="insufficient_scope"`)
			jsonError(w, http.StatusForbidden, "Insufficient scope")
		})
	}
}

func jsonError(w http.ResponseWriter, status int, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(map[string]any{
		"error":   http.StatusText(status),
		"message": message,
	})
}
