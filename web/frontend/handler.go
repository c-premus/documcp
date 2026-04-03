// Package frontend serves the embedded Vue SPA static assets.
package frontend

import (
	"embed"
	"io/fs"
	"net/http"
	"strings"
)

//go:embed all:dist
var distFS embed.FS

// FaviconHandler returns an http.Handler that serves favicon.ico from the
// embedded dist assets. Intended to be mounted at the root ("/favicon.ico")
// so that external clients (e.g. Claude.ai) can fetch it without auth.
func FaviconHandler() http.Handler {
	dist, err := fs.Sub(distFS, "dist")
	if err != nil {
		panic("frontend dist not embedded: " + err.Error())
	}

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		r.URL.Path = "/favicon.ico"
		http.FileServer(http.FS(dist)).ServeHTTP(w, r)
	})
}

// Handler returns an http.Handler that serves the embedded Vue SPA.
// Unknown paths fall back to index.html for client-side routing.
func Handler() http.Handler {
	dist, err := fs.Sub(distFS, "dist")
	if err != nil {
		panic("frontend dist not embedded: " + err.Error())
	}
	fileServer := http.FileServer(http.FS(dist))

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		path := strings.TrimPrefix(r.URL.Path, "/")
		if path == "" {
			path = "index.html"
		}

		if f, err := dist.Open(path); err == nil {
			_ = f.Close()
			fileServer.ServeHTTP(w, r)
			return
		}

		// SPA fallback: serve index.html for client-side routing.
		r.URL.Path = "/"
		fileServer.ServeHTTP(w, r)
	})
}
