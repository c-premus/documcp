// Package authmiddleware provides HTTP middleware for authentication and authorization.
package authmiddleware

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/url"
	"strings"

	"github.com/gorilla/sessions"

	"git.999.haus/chris/DocuMCP-go/internal/auth/oauth"
	"git.999.haus/chris/DocuMCP-go/internal/model"
)

type contextKey string

const (
	// UserContextKey is the context key for the authenticated user.
	UserContextKey contextKey = "user"
	// AccessTokenContextKey is the context key for the validated access token.
	AccessTokenContextKey contextKey = "access_token"
)

// UserFromContext returns the authenticated user from the request context.
func UserFromContext(ctx context.Context) (*model.User, bool) {
	user, ok := ctx.Value(UserContextKey).(*model.User)
	return user, ok
}

// BearerToken validates an OAuth 2.1 bearer token from the Authorization header.
// On success, it sets the access token and user in the request context.
// When no Authorization header is present, it is bearer-only and rejects the request.
// Use BearerOrSession to allow session cookie fallback for SPA admin routes.
func BearerToken(oauthService *oauth.Service) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			authHeader := r.Header.Get("Authorization")
			if authHeader == "" {
				w.Header().Set("WWW-Authenticate", `Bearer`)
				jsonError(w, http.StatusUnauthorized, "Bearer token required")
				return
			}

			bearerToken := strings.TrimPrefix(authHeader, "Bearer ")
			if bearerToken == authHeader {
				w.Header().Set("WWW-Authenticate", `Bearer`)
				jsonError(w, http.StatusUnauthorized, "Bearer token required")
				return
			}

			token, err := oauthService.ValidateAccessToken(r.Context(), bearerToken)
			if err != nil {
				w.Header().Set("WWW-Authenticate", `Bearer error="invalid_token"`)
				jsonError(w, http.StatusUnauthorized, "Invalid or expired token")
				return
			}

			go func(id int64) {
				touchCtx, cancel := context.WithTimeout(context.Background(), oauthService.ClientTouchTimeout())
				defer cancel()
				if err := oauthService.TouchClientLastUsed(touchCtx, id); err != nil {
					slog.Warn("updating oauth client last_used_at", "client_id", id, "error", err)
				}
			}(token.ClientID)

			ctx := context.WithValue(r.Context(), AccessTokenContextKey, token)

			// Optionally load user
			if token.UserID.Valid {
				user, err := oauthService.FindUserByID(r.Context(), token.UserID.Int64)
				if err != nil {
					slog.Warn("loading user for bearer token", "user_id", token.UserID.Int64, "error", err)
				} else {
					ctx = context.WithValue(ctx, UserContextKey, user)
				}
			}

			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// BearerOrSession tries bearer token auth first, then falls back to session
// cookie auth. This allows the same API routes to serve both MCP/API clients
// (bearer token) and the admin SPA (session cookie).
func BearerOrSession(oauthService *oauth.Service, store sessions.Store) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// If Authorization header is present, use bearer token auth.
			if authHeader := r.Header.Get("Authorization"); authHeader != "" {
				bearerToken := strings.TrimPrefix(authHeader, "Bearer ")
				if bearerToken == authHeader {
					w.Header().Set("WWW-Authenticate", `Bearer`)
					jsonError(w, http.StatusUnauthorized, "Bearer token required")
					return
				}

				token, err := oauthService.ValidateAccessToken(r.Context(), bearerToken)
				if err != nil {
					w.Header().Set("WWW-Authenticate", `Bearer error="invalid_token"`)
					jsonError(w, http.StatusUnauthorized, "Invalid or expired token")
					return
				}

				go func(id int64) {
					touchCtx, cancel := context.WithTimeout(context.Background(), oauthService.ClientTouchTimeout())
					defer cancel()
					if err := oauthService.TouchClientLastUsed(touchCtx, id); err != nil {
						slog.Warn("updating oauth client last_used_at", "client_id", id, "error", err)
					}
				}(token.ClientID)

				ctx := context.WithValue(r.Context(), AccessTokenContextKey, token)
				if token.UserID.Valid {
					user, err := oauthService.FindUserByID(r.Context(), token.UserID.Int64)
					if err != nil {
						slog.Warn("loading user for bearer token", "user_id", token.UserID.Int64, "error", err)
					} else {
						ctx = context.WithValue(ctx, UserContextKey, user)
					}
				}

				next.ServeHTTP(w, r.WithContext(ctx))
				return
			}

			// No Authorization header — try session cookie.
			session, _ := store.Get(r, "documcp_session")
			userID, ok := session.Values["user_id"].(int64)
			if !ok || userID == 0 {
				jsonError(w, http.StatusUnauthorized, "Authentication required")
				return
			}

			user, err := oauthService.FindUserByID(r.Context(), userID)
			if err != nil {
				jsonError(w, http.StatusUnauthorized, "Authentication required")
				return
			}

			ctx := context.WithValue(r.Context(), UserContextKey, user)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// SessionAuth validates an admin session cookie.
// On success, it sets the user in the request context.
func SessionAuth(store sessions.Store, oauthService *oauth.Service) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			session, _ := store.Get(r, "documcp_session")
			userID, ok := session.Values["user_id"].(int64)
			if !ok || userID == 0 {
				http.Redirect(w, r, "/auth/login?redirect="+url.QueryEscape(r.URL.RequestURI()), http.StatusFound)
				return
			}

			user, err := oauthService.FindUserByID(r.Context(), userID)
			if err != nil {
				// Session invalid — clear it
				session.Options.MaxAge = -1
				_ = session.Save(r, w)
				http.Redirect(w, r, "/auth/login?redirect="+url.QueryEscape(r.URL.RequestURI()), http.StatusFound)
				return
			}

			ctx := context.WithValue(r.Context(), UserContextKey, user)
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
// Session-authenticated users (no access token) are allowed through, as scope enforcement
// only applies to OAuth token-based access.
// Must be used after BearerToken or BearerOrSession middleware.
func RequireScope(scope string) func(http.Handler) http.Handler {
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
