package server

import (
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"

	"git.999.haus/chris/DocuMCP-go/internal/handler"
)

// RegisterRoutes configures all middleware and route groups on the server.
// The version string is embedded in the health response.
func (s *Server) RegisterRoutes(version string) {
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
	health := handler.NewHealthHandler(version)
	r.Method(http.MethodGet, "/health", health)

	// MCP endpoint (TODO)
	r.Route("/documcp", func(r chi.Router) {
		// TODO: register MCP handlers
	})

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
