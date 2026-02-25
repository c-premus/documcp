package server

import (
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"

	"git.999.haus/chris/DocuMCP-go/internal/handler"
)

// Deps holds handler dependencies injected from the app layer.
type Deps struct {
	Version    string
	MCPHandler http.Handler // nil if MCP is not configured
}

// RegisterRoutes configures all middleware and route groups on the server.
func (s *Server) RegisterRoutes(deps Deps) {
	r := s.router

	// Built-in chi middleware
	r.Use(middleware.RequestID)
	r.Use(middleware.RealIP)
	r.Use(middleware.Recoverer)
	r.Use(middleware.Timeout(60 * time.Second))

	// Application middleware
	r.Use(SecurityHeaders)
	r.Use(RequestLogger(s.logger))

	// Health check
	health := handler.NewHealthHandler(deps.Version)
	r.Method(http.MethodGet, "/health", health)

	// MCP endpoint
	if deps.MCPHandler != nil {
		r.Route("/documcp", func(r chi.Router) {
			r.Handle("/*", deps.MCPHandler)
			r.Handle("/", deps.MCPHandler)
		})
		s.logger.Info("MCP endpoint registered", "path", "/documcp")
	}

	// OAuth endpoints (TODO)
	r.Route("/oauth", func(r chi.Router) {
		// TODO: register OAuth handlers
	})

	// REST API (TODO)
	r.Route("/api", func(r chi.Router) {
		// TODO: register API handlers
	})

	// Admin UI (TODO)
	r.Route("/admin", func(r chi.Router) {
		// TODO: register admin handlers
	})
}
