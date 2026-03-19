package server

import (
	"log/slog"
	"net"
	"net/http"
	"strings"
	"time"

	"github.com/go-chi/chi/v5/middleware"
)

// SecurityHeaders adds recommended security headers to every response.
func SecurityHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Frame-Options", "DENY")
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("X-XSS-Protection", "0")
		w.Header().Set("Referrer-Policy", "strict-origin-when-cross-origin")
		w.Header().Set("Permissions-Policy", "camera=(), microphone=(), geolocation=()")
		w.Header().Set("Content-Security-Policy", "default-src 'self'; script-src 'self'; style-src 'self' 'unsafe-inline'; img-src 'self' data:; connect-src 'self'")
		w.Header().Set("Cache-Control", "no-store")

		// HSTS: instruct browsers to only use HTTPS. Only set when the
		// request arrived over TLS (or via a trusted proxy that sets
		// X-Forwarded-Proto) to avoid breaking plain-HTTP dev setups.
		if r.TLS != nil || r.Header.Get("X-Forwarded-Proto") == "https" {
			w.Header().Set("Strict-Transport-Security", "max-age=63072000; includeSubDomains")
		}

		next.ServeHTTP(w, r)
	})
}

// blockedFiles is the set of root-level filenames that must return 404.
var blockedFiles = map[string]bool{
	"composer.json": true, "composer.lock": true,
	"package.json": true, "package-lock.json": true,
	"yarn.lock": true, ".htaccess": true, "web.config": true,
	"Makefile": true, "go.mod": true, "go.sum": true,
	"Dockerfile": true, "docker-compose.yml": true,
	".env": true, ".env.example": true,
}

// BlockSensitiveFiles returns 404 for dotfiles (except /.well-known) and
// known sensitive files (package manager locks, server config) per REQ-SEC-003.
func BlockSensitiveFiles(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		p := r.URL.Path

		// Check the first path segment for dotfiles.
		first := strings.TrimPrefix(p, "/")
		if idx := strings.IndexByte(first, '/'); idx != -1 {
			first = first[:idx]
		}
		if strings.HasPrefix(first, ".") && first != ".well-known" {
			http.NotFound(w, r)
			return
		}

		// Block known sensitive files at the root level (e.g. /composer.json).
		if blockedFiles[first] {
			http.NotFound(w, r)
			return
		}

		next.ServeHTTP(w, r)
	})
}

// MaxBodySize returns middleware that limits request body size. Requests with
// Content-Type multipart/form-data are excluded (file uploads have their own
// limits). This prevents denial of service via oversized JSON payloads.
func MaxBodySize(maxBytes int64) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ct := r.Header.Get("Content-Type")
			if !strings.HasPrefix(ct, "multipart/form-data") {
				r.Body = http.MaxBytesReader(w, r.Body, maxBytes)
			}
			next.ServeHTTP(w, r)
		})
	}
}

// RealIP returns middleware that sets r.RemoteAddr to the client's real IP
// address. When trustedProxies is non-empty, forwarded headers (X-Real-IP,
// X-Forwarded-For) are only honoured if the direct connection originates from
// a trusted network. When trustedProxies is empty, headers are ignored and
// RemoteAddr is used as-is — secure by default.
func RealIP(trustedProxies []*net.IPNet) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			r.RemoteAddr = extractIP(r, trustedProxies)
			next.ServeHTTP(w, r)
		})
	}
}

// extractIP gets the client IP. When the request originates from a trusted
// proxy (RemoteAddr falls within a trustedProxies CIDR), it checks X-Real-IP
// and X-Forwarded-For headers. Otherwise it uses RemoteAddr only, preventing
// IP spoofing via header manipulation.
func extractIP(r *http.Request, trustedProxies []*net.IPNet) string {
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		host = r.RemoteAddr
	}

	if len(trustedProxies) > 0 {
		remoteIP := net.ParseIP(host)
		if remoteIP != nil && ipInNets(remoteIP, trustedProxies) {
			if ip := r.Header.Get("X-Real-Ip"); ip != "" {
				if parsed := net.ParseIP(strings.TrimSpace(ip)); parsed != nil {
					return parsed.String()
				}
			}
			if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
				// Walk from rightmost (most recent proxy) to leftmost,
				// skipping trusted proxy IPs. The first untrusted IP is
				// the real client. Prevents spoofing via prepended headers.
				ips := strings.Split(xff, ",")
				for i := len(ips) - 1; i >= 0; i-- {
					candidate := strings.TrimSpace(ips[i])
					parsed := net.ParseIP(candidate)
					if parsed == nil {
						continue
					}
					if !ipInNets(parsed, trustedProxies) {
						return parsed.String()
					}
				}
			}
		}
	}

	// Normalize to canonical form so IPv6 variations produce the same
	// string for rate-limiter keys and log output.
	if parsed := net.ParseIP(host); parsed != nil {
		return parsed.String()
	}
	return host
}

// ipInNets returns true if ip is contained in any of the given networks.
func ipInNets(ip net.IP, nets []*net.IPNet) bool {
	for _, n := range nets {
		if n.Contains(ip) {
			return true
		}
	}
	return false
}

// TimeoutExcept returns middleware that applies chi's context-based timeout to
// all requests except those whose path starts with any of the excluded prefixes.
// This allows long-lived connections (e.g. SSE streams on /documcp) to stay open
// while enforcing timeouts on all other routes.
func TimeoutExcept(timeout time.Duration, excludedPrefixes ...string) func(http.Handler) http.Handler {
	timeoutMW := middleware.Timeout(timeout)
	return func(next http.Handler) http.Handler {
		withTimeout := timeoutMW(next)
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			for _, prefix := range excludedPrefixes {
				if strings.HasPrefix(r.URL.Path, prefix) {
					next.ServeHTTP(w, r)
					return
				}
			}
			withTimeout.ServeHTTP(w, r)
		})
	}
}

// RequestLogger returns middleware that logs each request using slog.
// It captures method, path, status code, duration, and request ID.
func RequestLogger(logger *slog.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()
			ww := middleware.NewWrapResponseWriter(w, r.ProtoMajor)

			defer func() {
				// Suppress noisy health/metrics endpoint logging.
				path := r.URL.Path
				if strings.HasPrefix(path, "/health") || path == "/metrics" {
					return
				}
				logger.Info("request completed",
					"method", r.Method,
					"path", path,
					"status", ww.Status(),
					"duration", time.Since(start),
					"request_id", middleware.GetReqID(r.Context()),
				)
			}()

			next.ServeHTTP(ww, r)
		})
	}
}
