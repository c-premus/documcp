// Package frontend serves the embedded Vue SPA static assets.
package frontend

import (
	"embed"
	"io/fs"
	"net/http"
	stdpath "path"
	"strings"
)

//go:embed all:dist
var distFS embed.FS

// rootAssetAllowlist is the set of static files served at the root without
// authentication. Browsers and external clients (e.g. Claude.ai) expect these
// at well-known paths outside the SPA mount. openapi.yaml is the canonical
// public REST contract and is kept unauthenticated so consumers can fetch it
// without credentials.
var rootAssetAllowlist = map[string]bool{
	"/favicon.ico":                  true,
	"/favicon.svg":                  true,
	"/favicon-96x96.png":            true,
	"/apple-touch-icon.png":         true,
	"/site.webmanifest":             true,
	"/web-app-manifest-192x192.png": true,
	"/web-app-manifest-512x512.png": true,
	"/openapi.yaml":                 true,
}

// rootAssetContentTypes overrides Content-Type for extensions not reliably
// present in all system MIME databases (e.g. CI containers).
var rootAssetContentTypes = map[string]string{
	".webmanifest": "application/manifest+json",
	".yaml":        "application/yaml",
}

// RootAssetHandler returns an http.Handler that serves whitelisted static
// files from the embedded dist assets. Mount individual routes at the root
// (e.g. "/favicon.ico", "/apple-touch-icon.png") so external clients can
// fetch them without auth.
func RootAssetHandler() http.Handler {
	dist, err := fs.Sub(distFS, "dist")
	if err != nil {
		panic("frontend dist not embedded: " + err.Error())
	}
	fileServer := http.FileServer(http.FS(dist))

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !rootAssetAllowlist[r.URL.Path] {
			http.NotFound(w, r)
			return
		}
		if ct, ok := rootAssetContentTypes[stdpath.Ext(r.URL.Path)]; ok {
			w.Header().Set("Content-Type", ct)
		}
		fileServer.ServeHTTP(w, r)
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
