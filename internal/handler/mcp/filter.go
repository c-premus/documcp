// Package mcphandler provides the MCP protocol handler and tool implementations.
package mcphandler

import (
	"context"
	"errors"
	"slices"
	"strings"

	authmiddleware "git.999.haus/chris/DocuMCP-go/internal/auth/middleware"
	authscope "git.999.haus/chris/DocuMCP-go/internal/auth/scope"
	"git.999.haus/chris/DocuMCP-go/internal/model"
)

// errInsufficientScope is returned when a tool requires a scope the token lacks.
var errInsufficientScope = errors.New("insufficient scope")

// requireMCPScope checks that the access token in the context has the given scope.
// Returns nil if the scope is present or if the user is session-authenticated (no token).
func requireMCPScope(ctx context.Context, scope string) error {
	token, ok := ctx.Value(authmiddleware.AccessTokenContextKey).(*model.OAuthAccessToken)
	if !ok || token == nil {
		// No token in context — should not happen for MCP (bearer-only), but be safe.
		return nil
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

// sanitizeFilterValue escapes a string for safe use in a Meilisearch filter value.
// It escapes backslashes and double quotes which are meaningful in filter syntax.
func sanitizeFilterValue(s string) string {
	s = strings.ReplaceAll(s, `\`, `\\`)
	s = strings.ReplaceAll(s, `"`, `\"`)
	return s
}
