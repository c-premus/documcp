package observability

import (
	"crypto/subtle"
	"encoding/json"
	"net/http"
	"strings"
)

// InternalTokenAuth returns a middleware that requires a bearer token matching
// token. Used to gate operational endpoints (/metrics on serve-mode and
// worker-mode) behind INTERNAL_API_TOKEN. The constant-time compare prevents
// timing-side-channel guessing of the token. Empty token panics — callers
// must check for an unset token at registration time and either skip the
// middleware or warn-log; this function is for the gated path only.
func InternalTokenAuth(token string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			auth := r.Header.Get("Authorization")
			if !strings.HasPrefix(auth, "Bearer ") {
				writeJSONError(w, http.StatusUnauthorized, "Bearer token required")
				return
			}
			provided := strings.TrimPrefix(auth, "Bearer ")
			if subtle.ConstantTimeCompare([]byte(provided), []byte(token)) != 1 {
				writeJSONError(w, http.StatusUnauthorized, "Invalid token")
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

func writeJSONError(w http.ResponseWriter, status int, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(map[string]string{
		"error":   http.StatusText(status),
		"message": message,
	})
}
