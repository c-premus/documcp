// Package mcphandler provides the MCP protocol handler and tool implementations.
package mcphandler

import (
	"context"
	"errors"
	"slices"
	"strings"

	authmiddleware "github.com/c-premus/documcp/internal/auth/middleware"
	authscope "github.com/c-premus/documcp/internal/auth/scope"
	"github.com/c-premus/documcp/internal/model"
)

// errInsufficientScope is returned when a tool requires a scope the token lacks.
var errInsufficientScope = errors.New("insufficient scope")

// requireMCPScope checks that the access token in the context has the given scope.
// MCP is bearer-only — fail closed if no token is present.
func requireMCPScope(ctx context.Context, scope string) error {
	token, ok := ctx.Value(authmiddleware.AccessTokenContextKey).(*model.OAuthAccessToken)
	if !ok || token == nil {
		// No token in context — MCP requires bearer auth, fail closed.
		return errInsufficientScope
	}
	tokenScope := ""
	if token.Scope.Valid {
		tokenScope = token.Scope.String
	}
	if slices.Contains(authscope.ParseScopes(tokenScope), scope) {
		return nil
	}
	return errInsufficientScope
}

// validFileTypes is the whitelist of allowed document file types.
var validFileTypes = map[string]bool{
	"markdown": true,
	"pdf":      true,
	"docx":     true,
	"xlsx":     true,
	"html":     true,
}

// isValidFileType returns true if the file type is in the whitelist.
func isValidFileType(ft string) bool {
	return validFileTypes[strings.ToLower(ft)]
}

