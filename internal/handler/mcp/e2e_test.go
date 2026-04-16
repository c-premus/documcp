package mcphandler

import (
	"context"
	"database/sql"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"slices"
	"testing"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	authmiddleware "github.com/c-premus/documcp/internal/auth/middleware"
	"github.com/c-premus/documcp/internal/model"
	"github.com/c-premus/documcp/internal/service"
)

// TestMCPProtocolEndToEnd drives the full Streamable HTTP transport using the
// official MCP Go SDK client — the one component the 1397-subtest unit layer
// never exercises in aggregate. It covers the closure gap docs/audit/tests.md
// finding 1 calls out: JSON-RPC framing, tool-schema publication, argument
// marshaling and validation through the SDK, and response decoding on the
// client side.
//
// The service layer is still mocked (it has its own tests); the service mocks
// record the calls the tool handlers make so the test can assert the wire
// hand-off, not just the shape of the response payload.
//
// Scope enforcement is driven through the real requireMCPScope path — a tiny
// HTTP middleware on the front of the MCP handler injects a bearer-token-shaped
// access token into the request context before the SDK dispatches to the tool
// handler. This makes the test a regression guard for the token→scope→tool
// contract that production relies on.
func TestMCPProtocolEndToEnd(t *testing.T) {
	t.Parallel()

	const testUUID = "11111111-2222-3333-4444-555555555555"
	created := time.Date(2026, 4, 16, 12, 0, 0, 0, time.UTC)

	// Service mock: returns deterministic payloads and records the args passed
	// to the tool handlers so we can assert both the RPC round-trip AND the
	// argument marshaling the SDK does on the wire.
	var createdParams service.CreateDocumentParams
	var readUUID, deletedUUID string
	docSvc := &mockDocumentService{
		createFn: func(_ context.Context, p service.CreateDocumentParams) (*model.Document, error) {
			createdParams = p
			return &model.Document{
				ID:        42,
				UUID:      testUUID,
				Title:     p.Title,
				FileType:  p.FileType,
				CreatedAt: sql.NullTime{Time: created, Valid: true},
			}, nil
		},
		findByUUIDFn: func(_ context.Context, uuid string) (*model.Document, error) {
			readUUID = uuid
			if uuid != testUUID {
				return nil, service.ErrNotFound
			}
			return &model.Document{
				ID:       42,
				UUID:     testUUID,
				Title:    "E2E Title",
				FileType: "markdown",
				IsPublic: true,
				Content:  sql.NullString{String: "# Hello\n\nE2E content.", Valid: true},
			}, nil
		},
		deleteFn: func(_ context.Context, uuid string) error {
			deletedUUID = uuid
			return nil
		},
	}

	srv := newE2ETestServer(t, docSvc)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	t.Cleanup(cancel)

	client := mcp.NewClient(&mcp.Implementation{Name: "e2e-test-client", Version: "0.0.0"}, nil)
	transport := &mcp.StreamableClientTransport{
		Endpoint:             srv.URL,
		DisableStandaloneSSE: true, // server runs Stateless; no standalone GET stream
	}
	session, err := client.Connect(ctx, transport, nil)
	require.NoError(t, err, "MCP client should initialize against a real Streamable HTTP server")
	t.Cleanup(func() { _ = session.Close() })

	t.Run("initialize round-trip carries server identity", func(t *testing.T) {
		res := session.InitializeResult()
		require.NotNil(t, res, "InitializeResult should be populated after Connect")
		assert.Equal(t, "documcp-e2e", res.ServerInfo.Name)
		assert.NotEmpty(t, res.ProtocolVersion)
	})

	t.Run("tools/list publishes the full document toolset", func(t *testing.T) {
		listed, err := session.ListTools(ctx, nil)
		require.NoError(t, err)

		names := make([]string, 0, len(listed.Tools))
		for _, tool := range listed.Tools {
			names = append(names, tool.Name)
		}
		for _, want := range []string{
			"list_documents",
			"search_documents",
			"read_document",
			"create_document",
			"update_document",
			"delete_document",
			"unified_search",
		} {
			assert.Truef(t, slices.Contains(names, want),
				"tool %q should be published; got %v", want, names)
		}
	})

	t.Run("tools/call create_document: args marshal end-to-end, response decodes as structured", func(t *testing.T) {
		args := map[string]any{
			"title":     "E2E Title",
			"content":   "# Hello\n\nE2E content.",
			"file_type": "markdown",
			"is_public": true,
			"tags":      []string{"e2e", "mcp"},
		}
		result, err := session.CallTool(ctx, &mcp.CallToolParams{
			Name:      "create_document",
			Arguments: args,
		})
		require.NoError(t, err)
		require.False(t, result.IsError, "create_document should succeed; content=%v", result.Content)
		require.NotNil(t, result.StructuredContent, "structured content should be populated by ToolHandlerFor")

		// Service layer received the args the SDK marshaled.
		assert.Equal(t, "E2E Title", createdParams.Title)
		assert.Equal(t, "markdown", createdParams.FileType)
		assert.True(t, createdParams.IsPublic)
		assert.Equal(t, []string{"e2e", "mcp"}, createdParams.Tags)

		// The ClientSession exposes StructuredContent as `any`; re-marshal +
		// unmarshal is the stable way to peek into the server's response shape
		// through the wire.
		raw, err := json.Marshal(result.StructuredContent)
		require.NoError(t, err)
		var envelope struct {
			Success  bool `json:"success"`
			Document struct {
				UUID     string `json:"uuid"`
				Title    string `json:"title"`
				FileType string `json:"file_type"`
			} `json:"document"`
		}
		require.NoError(t, json.Unmarshal(raw, &envelope))
		assert.True(t, envelope.Success)
		assert.Equal(t, testUUID, envelope.Document.UUID)
		assert.Equal(t, "E2E Title", envelope.Document.Title)
		assert.Equal(t, "markdown", envelope.Document.FileType)
	})

	t.Run("tools/call read_document: typed UUID reaches the service layer", func(t *testing.T) {
		result, err := session.CallTool(ctx, &mcp.CallToolParams{
			Name:      "read_document",
			Arguments: map[string]any{"uuid": testUUID},
		})
		require.NoError(t, err)
		require.False(t, result.IsError, "read_document should succeed; content=%v", result.Content)
		assert.Equal(t, testUUID, readUUID)
	})

	t.Run("tools/call with invalid args: server returns an error the client can observe", func(t *testing.T) {
		// file_type must be markdown or html; handler-side validation rejects
		// "pdf" (PDFs go through upload, not create_document).
		result, err := session.CallTool(ctx, &mcp.CallToolParams{
			Name: "create_document",
			Arguments: map[string]any{
				"title":     "Bad",
				"content":   "x",
				"file_type": "pdf",
			},
		})
		require.NoError(t, err, "transport error should not fire; IsError carries the tool-level failure")
		assert.True(t, result.IsError, "tool-level validation failure should surface as IsError=true")
	})

	t.Run("tools/call delete_document: success path confirms scope check + dispatch", func(t *testing.T) {
		result, err := session.CallTool(ctx, &mcp.CallToolParams{
			Name:      "delete_document",
			Arguments: map[string]any{"uuid": testUUID},
		})
		require.NoError(t, err)
		require.False(t, result.IsError, "delete_document should succeed; content=%v", result.Content)
		assert.Equal(t, testUUID, deletedUUID)
	})

	t.Run("missing scope on the context: mcp:write tools fail closed", func(t *testing.T) {
		// Point a fresh client at a server whose injected token has no write
		// scope. The SDK transport-level call still succeeds; the tool handler
		// returns IsError=true via requireMCPScope — the real-world shape when
		// a token is missing mcp:write.
		readOnlySrv := newE2ETestServerWithScope(t, docSvc, "mcp:access mcp:read")
		ctx2, cancel2 := context.WithTimeout(context.Background(), 5*time.Second)
		t.Cleanup(cancel2)

		c := mcp.NewClient(&mcp.Implementation{Name: "e2e-readonly", Version: "0.0.0"}, nil)
		s, err := c.Connect(ctx2, &mcp.StreamableClientTransport{
			Endpoint:             readOnlySrv.URL,
			DisableStandaloneSSE: true,
		}, nil)
		require.NoError(t, err)
		t.Cleanup(func() { _ = s.Close() })

		result, err := s.CallTool(ctx2, &mcp.CallToolParams{
			Name: "create_document",
			Arguments: map[string]any{
				"title":     "Denied",
				"content":   "x",
				"file_type": "markdown",
			},
		})
		require.NoError(t, err)
		assert.True(t, result.IsError, "missing mcp:write must surface as tool-level error")
	})

	t.Run("tools/call unknown tool: server errors, transport survives", func(t *testing.T) {
		_, err := session.CallTool(ctx, &mcp.CallToolParams{
			Name:      "not_a_real_tool",
			Arguments: map[string]any{},
		})
		require.Error(t, err, "unknown tool should yield a JSON-RPC error")

		// Transport must still be healthy after the error — use it again.
		listed, err := session.ListTools(ctx, nil)
		require.NoError(t, err, "transport should survive a tool-not-found error")
		assert.NotEmpty(t, listed.Tools)
	})
}

// newE2ETestServer spins up the real mcphandler.Handler behind an httptest
// server, with a tiny bearer-token-injection middleware so tool handlers see
// the token shape the production BearerToken middleware would hand them.
func newE2ETestServer(t *testing.T, docSvc documentServicer) *httptest.Server {
	t.Helper()
	return newE2ETestServerWithScope(t, docSvc, "mcp:access mcp:read mcp:write")
}

func newE2ETestServerWithScope(t *testing.T, docSvc documentServicer, scope string) *httptest.Server {
	t.Helper()

	h := New(Config{
		ServerName:      "documcp-e2e",
		ServerVersion:   "0.0.0-test",
		Logger:          slog.New(slog.DiscardHandler),
		DocumentService: docSvc,
		DocumentRepo:    &mockDocumentRepo{},
		Searcher:        &mockSearcher{},
	})

	token := &model.OAuthAccessToken{Scope: sql.NullString{String: scope, Valid: true}}
	user := &model.User{ID: 7, IsAdmin: true}
	wrapped := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		ctx = context.WithValue(ctx, authmiddleware.AccessTokenContextKey, token)
		ctx = context.WithValue(ctx, authmiddleware.UserContextKey, user)
		h.ServeHTTP(w, r.WithContext(ctx))
	})

	srv := httptest.NewServer(wrapped)
	t.Cleanup(srv.Close)
	return srv
}

// Compile-time assertions: the mocks we reuse here satisfy the e2e contracts.
var (
	_ documentServicer = (*mockDocumentService)(nil)
	_ documentLister   = (*mockDocumentRepo)(nil)
	_ contentSearcher  = (*mockSearcher)(nil)
)
