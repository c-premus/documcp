// Package authmiddleware provides HTTP middleware for authentication and authorization.
package authmiddleware

import (
	"context"
	"encoding/json"
	"net/http"
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

// AccessTokenFromContext returns the validated access token from the request context.
func AccessTokenFromContext(ctx context.Context) (*model.OAuthAccessToken, bool) {
	token, ok := ctx.Value(AccessTokenContextKey).(*model.OAuthAccessToken)
	return token, ok
}

// BearerToken validates an OAuth 2.1 bearer token from the Authorization header.
// On success, it sets the access token and user in the request context.
func BearerToken(oauthService *oauth.Service) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			authHeader := r.Header.Get("Authorization")
			if authHeader == "" {
				jsonError(w, http.StatusUnauthorized, "Bearer token required")
				return
			}

			bearerToken := strings.TrimPrefix(authHeader, "Bearer ")
			if bearerToken == authHeader {
				jsonError(w, http.StatusUnauthorized, "Bearer token required")
				return
			}

			token, err := oauthService.ValidateAccessToken(r.Context(), bearerToken)
			if err != nil {
				jsonError(w, http.StatusUnauthorized, "Invalid or expired token")
				return
			}

			ctx := context.WithValue(r.Context(), AccessTokenContextKey, token)

			// Optionally load user
			if token.UserID.Valid {
				user, err := oauthService.FindUserByID(r.Context(), token.UserID.Int64)
				if err == nil {
					ctx = context.WithValue(ctx, UserContextKey, user)
				}
			}

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
				http.Redirect(w, r, "/auth/login?redirect="+r.URL.RequestURI(), http.StatusFound)
				return
			}

			user, err := oauthService.FindUserByID(r.Context(), userID)
			if err != nil {
				// Session invalid — clear it
				session.Options.MaxAge = -1
				_ = session.Save(r, w)
				http.Redirect(w, r, "/auth/login?redirect="+r.URL.RequestURI(), http.StatusFound)
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
			if wantsJSON(r) {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusForbidden)
				_ = json.NewEncoder(w).Encode(map[string]any{
					"error": map[string]any{
						"message": "Access denied. Admin privileges required.",
						"code":    403,
					},
				})
			} else {
				http.Error(w, "Access denied. Admin privileges required.", http.StatusForbidden)
			}
			return
		}
		next.ServeHTTP(w, r)
	})
}

func jsonError(w http.ResponseWriter, status int, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(map[string]string{"error": message})
}

func wantsJSON(r *http.Request) bool {
	accept := r.Header.Get("Accept")
	return strings.Contains(accept, "application/json")
}
