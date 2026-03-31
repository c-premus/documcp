// Package server manages the HTTP server lifecycle and middleware.
package server

import (
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"runtime/debug"
	"strings"
	"time"

	"github.com/go-chi/chi/v5/middleware"
)

// SecurityHeaders returns middleware that adds recommended security headers to
// every response. The hstsMaxAge parameter controls the HSTS max-age directive
// in seconds; set to 0 to disable HSTS entirely.
func SecurityHeaders(hstsMaxAge int) func(http.Handler) http.Handler {
	hstsValue := fmt.Sprintf("max-age=%d; includeSubDomains", hstsMaxAge)
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("X-Frame-Options", "DENY")
			w.Header().Set("X-Content-Type-Options", "nosniff")
			w.Header().Set("X-XSS-Protection", "0")
			w.Header().Set("Referrer-Policy", "strict-origin-when-cross-origin")
			w.Header().Set("Permissions-Policy", "camera=(), microphone=(), geolocation=()")
			w.Header().Set("Content-Security-Policy", "default-src 'self'; script-src 'self'; style-src 'self' 'unsafe-inline'; img-src 'self' data:; connect-src 'self'; form-action 'self'; frame-ancestors 'none'")
			w.Header().Set("Cache-Control", "no-store")

			// HSTS: instruct browsers to only use HTTPS. Only set when the
			// request arrived over TLS (or via a trusted proxy that sets
			// X-Forwarded-Proto) to avoid breaking plain-HTTP dev setups.
			if hstsMaxAge > 0 && (r.TLS != nil || r.Header.Get("X-Forwarded-Proto") == "https") {
				w.Header().Set("Strict-Transport-Security", hstsValue)
			}

			next.ServeHTTP(w, r)
		})
	}
}

// SafeRecoverer recovers from panics and returns a generic 500 response without
// leaking stack traces or internal details to the client.
func SafeRecoverer(logger *slog.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			defer func() {
				if rvr := recover(); rvr != nil {
					reqID := middleware.GetReqID(r.Context())
					logger.Error("panic recovered",
						"error", rvr,
						"stack", string(debug.Stack()),
						"method", r.Method,
						"path", r.URL.Path,
						"request_id", reqID,
					)
					if r.Header.Get("Connection") == "Upgrade" {
						return
					}
					http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
				}
			}()
			next.ServeHTTP(w, r)
		})
	}
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
// address. When trustedProxies is non-empty, forwarded headers (X-Forwarded-For,
// X-Real-IP) are only honored if the direct connection originates from a
// trusted network. When trustedProxies is empty, headers are ignored and
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
// proxy (RemoteAddr falls within a trustedProxies CIDR), it checks
// X-Forwarded-For and X-Real-IP headers. Otherwise it uses RemoteAddr only,
// preventing IP spoofing via header manipulation.
//
// X-Forwarded-For is checked first because reverse proxies like Traefik
// resolve the trust chain and place the real client IP there. X-Real-IP
// is a fallback — some proxies (e.g. Traefik) set it to the direct peer
// (a proxy IP), not the end client. In both cases, IPs that fall within
// trusted proxy CIDRs are skipped to avoid returning a proxy address.
func extractIP(r *http.Request, trustedProxies []*net.IPNet) string {
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		host = r.RemoteAddr
	}

	if len(trustedProxies) > 0 {
		remoteIP := net.ParseIP(host)
		if remoteIP != nil && ipInNets(remoteIP, trustedProxies) {
			// X-Forwarded-For: walk right-to-left, skip trusted proxies.
			// The first untrusted IP is the real client. This prevents
			// spoofing via attacker-prepended entries at the front.
			if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
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
			// X-Real-IP fallback: only use if the value is not a
			// trusted proxy. Some reverse proxies set X-Real-IP to
			// the direct peer rather than the resolved client IP.
			if ip := r.Header.Get("X-Real-Ip"); ip != "" {
				if parsed := net.ParseIP(strings.TrimSpace(ip)); parsed != nil {
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
					"client_ip", r.RemoteAddr,
					"request_id", middleware.GetReqID(r.Context()),
				)
			}()

			next.ServeHTTP(ww, r)
		})
	}
}
