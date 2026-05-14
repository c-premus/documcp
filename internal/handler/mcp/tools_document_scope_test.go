package mcphandler

import (
	"context"
	"database/sql"
	"errors"
	"strings"
	"testing"

	authmiddleware "github.com/c-premus/documcp/internal/auth/middleware"
	"github.com/c-premus/documcp/internal/dto"
	"github.com/c-premus/documcp/internal/model"
	"github.com/c-premus/documcp/internal/service"
)

// TestMCPWriteTools_RequireMCPWriteScope asserts every write-capable document
// tool handler rejects a token that lacks mcp:write, even when the token has
// mcp:access and mcp:read. This is a regression guard: the per-tool
// requireMCPScope calls in tools_document.go are the only thing gating write
// access once a bearer token passes the route-level mcp:access check.
//
// If a new write tool is registered in registerDocumentTools, ADD IT TO THE
// TABLE below. The comment at the end of registerDocumentTools points here.
func TestMCPWriteTools_RequireMCPWriteScope(t *testing.T) {
	readOnlyToken := &model.OAuthAccessToken{
		Scope: sql.NullString{String: "mcp:access mcp:read", Valid: true},
	}
	ctx := context.WithValue(context.Background(), authmiddleware.AccessTokenContextKey, readOnlyToken)
	// Attach a non-admin user so we exercise the non-M2M code path. Ownership
	// checks on update/delete won't run — scope rejection happens first.
	ctx = context.WithValue(ctx, authmiddleware.UserContextKey, &model.User{ID: 1, IsAdmin: false})

	h := &Handler{
		documentService: &mockDocumentService{},
		documentRepo:    &mockDocumentRepo{},
	}

	cases := []struct {
		name string
		call func(context.Context) error
	}{
		{
			name: "create_document",
			call: func(ctx context.Context) error {
				_, _, err := h.handleCreateDocument(ctx, nil, dto.CreateDocumentInput{
					Title:    "t",
					Content:  "c",
					FileType: "markdown",
				})
				return err
			},
		},
		{
			name: "update_document",
			call: func(ctx context.Context) error {
				_, _, err := h.handleUpdateDocument(ctx, nil, dto.UpdateDocumentInput{
					UUID:  "aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa",
					Title: "t",
				})
				return err
			},
		},
		{
			name: "delete_document",
			call: func(ctx context.Context) error {
				_, _, err := h.handleDeleteDocument(ctx, nil, dto.DeleteDocumentInput{
					UUID: "aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa",
				})
				return err
			},
		},
		{
			name: "replace_document_content",
			call: func(ctx context.Context) error {
				_, _, err := h.handleReplaceDocumentContent(ctx, nil, dto.ReplaceDocumentContentInput{
					UUID:    "aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa",
					Content: "new body",
				})
				return err
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := tc.call(ctx)
			if err == nil {
				t.Fatal("expected scope-required error, got nil — did the handler lose its requireMCPScope check?")
			}
			if !strings.Contains(strings.ToLower(err.Error()), "scope") {
				t.Errorf("error %q does not mention scope — the handler may be rejecting for a different reason", err.Error())
			}
		})
	}
}

// TestMCPWriteTools_AcceptMCPWriteScope is the positive counterpart: the scope
// check must NOT reject a token carrying mcp:write. We stub the service just
// enough to let each handler complete without a nil dereference; all we care
// about is that the returned error is not errInsufficientScope.
func TestMCPWriteTools_AcceptMCPWriteScope(t *testing.T) {
	writeToken := &model.OAuthAccessToken{
		Scope: sql.NullString{String: "mcp:access mcp:read mcp:write", Valid: true},
	}
	ctx := context.WithValue(context.Background(), authmiddleware.AccessTokenContextKey, writeToken)
	ctx = context.WithValue(ctx, authmiddleware.UserContextKey, &model.User{ID: 1, IsAdmin: true})

	stubDoc := &model.Document{UUID: "aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa", Title: "t", FileType: "markdown"}
	svc := &mockDocumentService{
		createFn: func(_ context.Context, _ service.CreateDocumentParams) (*model.Document, error) {
			return stubDoc, nil
		},
		updateFn: func(_ context.Context, _ string, _ service.UpdateDocumentParams) (*model.Document, error) {
			return stubDoc, nil
		},
		replaceInlineFn: func(_ context.Context, _ string, _ service.ReplaceInlineContentParams) (*model.Document, error) {
			return stubDoc, nil
		},
		// checkDocumentOwnership consults FindByUUID even for admins.
		findByUUIDFn: func(_ context.Context, _ string) (*model.Document, error) {
			return stubDoc, nil
		},
		deleteFn: func(_ context.Context, _ string) error { return nil },
	}
	h := &Handler{documentService: svc, documentRepo: &mockDocumentRepo{}}

	t.Run("create_document", func(t *testing.T) {
		_, _, err := h.handleCreateDocument(ctx, nil, dto.CreateDocumentInput{
			Title: "t", Content: "c", FileType: "markdown",
		})
		if err != nil && errors.Is(err, errInsufficientScope) {
			t.Errorf("rejected a mcp:write token: %v", err)
		}
	})
	t.Run("update_document", func(t *testing.T) {
		_, _, err := h.handleUpdateDocument(ctx, nil, dto.UpdateDocumentInput{
			UUID: "aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa", Title: "t",
		})
		if err != nil && errors.Is(err, errInsufficientScope) {
			t.Errorf("rejected a mcp:write token: %v", err)
		}
	})
	t.Run("delete_document", func(t *testing.T) {
		_, _, err := h.handleDeleteDocument(ctx, nil, dto.DeleteDocumentInput{
			UUID: "aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa",
		})
		if err != nil && errors.Is(err, errInsufficientScope) {
			t.Errorf("rejected a mcp:write token: %v", err)
		}
	})
	t.Run("replace_document_content", func(t *testing.T) {
		_, _, err := h.handleReplaceDocumentContent(ctx, nil, dto.ReplaceDocumentContentInput{
			UUID:    "aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa",
			Content: "new body",
		})
		if err != nil && errors.Is(err, errInsufficientScope) {
			t.Errorf("rejected a mcp:write token: %v", err)
		}
	})
}
