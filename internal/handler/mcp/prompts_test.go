package mcphandler

import (
	"context"
	"strings"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// --- defaultArg ---

func TestDefaultArg(t *testing.T) {
	tests := []struct {
		name     string
		args     map[string]string
		key      string
		fallback string
		want     string
	}{
		{
			name:     "key present returns value",
			args:     map[string]string{"task": "compare"},
			key:      "task",
			fallback: "summarize",
			want:     "compare",
		},
		{
			name:     "key empty returns fallback",
			args:     map[string]string{"task": ""},
			key:      "task",
			fallback: "summarize",
			want:     "summarize",
		},
		{
			name:     "key absent returns fallback",
			args:     map[string]string{},
			key:      "task",
			fallback: "summarize",
			want:     "summarize",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := defaultArg(tt.args, tt.key, tt.fallback)
			if got != tt.want {
				t.Errorf("defaultArg() = %q, want %q", got, tt.want)
			}
		})
	}
}

// --- truncate ---

func TestTruncate(t *testing.T) {
	tests := []struct {
		name   string
		s      string
		maxLen int
		want   string
	}{
		{
			name:   "short string unchanged",
			s:      "hello",
			maxLen: 10,
			want:   "hello",
		},
		{
			name:   "long string truncated with ellipsis",
			s:      "this is a very long string that exceeds the limit",
			maxLen: 10,
			want:   "this is a ...",
		},
		{
			name:   "exact length unchanged",
			s:      "exactly10!",
			maxLen: 10,
			want:   "exactly10!",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := truncate(tt.s, tt.maxLen)
			if got != tt.want {
				t.Errorf("truncate(%q, %d) = %q, want %q", tt.s, tt.maxLen, got, tt.want)
			}
		})
	}
}

// --- documentTaskGuidance ---

func TestDocumentTaskGuidance(t *testing.T) {
	tests := []struct {
		name string
		task string
	}{
		{"summarize", "summarize"},
		{"compare", "compare"},
		{"extract", "extract"},
		{"assess", "assess"},
		{"unknown defaults to summarize", "unknown_task"},
	}

	// Capture the default (summarize) guidance for comparison with unknown.
	defaultGuidance := documentTaskGuidance("summarize")

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := documentTaskGuidance(tt.task)
			if got == "" {
				t.Errorf("documentTaskGuidance(%q) returned empty string", tt.task)
			}
			if tt.task == "unknown_task" && got != defaultGuidance {
				t.Errorf("documentTaskGuidance(%q) = %q, want default %q", tt.task, got, defaultGuidance)
			}
		})
	}
}

// --- documentFocusGuidance ---

func TestDocumentFocusGuidance(t *testing.T) {
	tests := []struct {
		name  string
		focus string
	}{
		{"overview", "overview"},
		{"technical", "technical"},
		{"business", "business"},
		{"actionable", "actionable"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := documentFocusGuidance(tt.focus)
			if got == "" {
				t.Errorf("documentFocusGuidance(%q) returned empty string", tt.focus)
			}
		})
	}
}

// --- documentLengthGuidance ---

func TestDocumentLengthGuidance(t *testing.T) {
	tests := []struct {
		name   string
		length string
	}{
		{"brief", "brief"},
		{"detailed", "detailed"},
		{"comprehensive", "comprehensive"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := documentLengthGuidance(tt.length)
			if got == "" {
				t.Errorf("documentLengthGuidance(%q) returned empty string", tt.length)
			}
		})
	}
}

// --- contentTypeTemplate ---

func TestContentTypeTemplate(t *testing.T) {
	tests := []struct {
		name        string
		contentType string
	}{
		{"guide (default)", "guide"},
		{"runbook", "runbook"},
		{"reference", "reference"},
		{"tutorial", "tutorial"},
		{"adr", "adr"},
		{"unknown defaults to guide", "nonexistent"},
	}

	defaultTemplate := contentTypeTemplate("guide")

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := contentTypeTemplate(tt.contentType)
			if got == "" {
				t.Errorf("contentTypeTemplate(%q) returned empty string", tt.contentType)
			}
			if tt.contentType == "nonexistent" && got != defaultTemplate {
				t.Errorf("contentTypeTemplate(%q) = %q, want default %q", tt.contentType, got, defaultTemplate)
			}
		})
	}
}

// --- scopeGuidance ---

func TestScopeGuidance(t *testing.T) {
	tests := []struct {
		name  string
		scope string
		want  string // substring to verify correct branch
	}{
		{"single", "single", "single, self-contained"},
		{"multi", "multi", "series of related"},
		{"default returns single", "other", "single, self-contained"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := scopeGuidance(tt.scope)
			if got == "" {
				t.Errorf("scopeGuidance(%q) returned empty string", tt.scope)
			}
			if !strings.Contains(got, tt.want) {
				t.Errorf("scopeGuidance(%q) = %q, want substring %q", tt.scope, got, tt.want)
			}
		})
	}
}

// --- registerPrompts ---

func TestRegisterPrompts(t *testing.T) {
	t.Run("all features enabled does not panic", func(t *testing.T) {
		h := newTestHandler()
		// Should not panic.
		h.registerPrompts(true, true)
	})

	t.Run("all features disabled does not panic", func(t *testing.T) {
		h := newTestHandler()
		// Should not panic.
		h.registerPrompts(false, false)
	})
}

// --- Prompt handler tests ---

func TestHandleDocumentAnalysis(t *testing.T) {
	h := newTestHandler()

	tests := []struct {
		name string
		args map[string]string
	}{
		{
			name: "with all arguments",
			args: map[string]string{
				"document_ids": "abc-123,def-456",
				"task":         "compare",
				"focus":        "technical",
				"length":       "comprehensive",
			},
		},
		{
			name: "with defaults",
			args: map[string]string{
				"document_ids": "abc-123",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := makePromptRequest("document_analysis", tt.args)
			result, err := h.handleDocumentAnalysis(context.Background(), req)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			assertPromptResult(t, result)
			assertMessageRole(t, result.Messages[0], "assistant")
			assertMessageRole(t, result.Messages[1], "user")
			assertMessageContains(t, result.Messages[1], tt.args["document_ids"])
		})
	}
}

func TestHandleSearchQueryBuilder(t *testing.T) {
	h := newTestHandler()

	tests := []struct {
		name string
		args map[string]string
	}{
		{
			name: "with all arguments",
			args: map[string]string{
				"goal":       "find deployment docs",
				"context":    "kubernetes project",
				"file_types": "markdown",
			},
		},
		{
			name: "with goal only",
			args: map[string]string{
				"goal": "find API reference",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := makePromptRequest("search_query_builder", tt.args)
			result, err := h.handleSearchQueryBuilder(context.Background(), req)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			assertPromptResult(t, result)
			assertMessageRole(t, result.Messages[0], "assistant")
			assertMessageRole(t, result.Messages[1], "user")
			assertMessageContains(t, result.Messages[1], tt.args["goal"])
		})
	}
}

func TestHandleKnowledgeBaseBuilder(t *testing.T) {
	h := newTestHandler()

	tests := []struct {
		name string
		args map[string]string
	}{
		{
			name: "with all arguments",
			args: map[string]string{
				"goal":         "document onboarding process",
				"content_type": "runbook",
				"scope":        "multi",
			},
		},
		{
			name: "with goal only",
			args: map[string]string{
				"goal": "create architecture docs",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := makePromptRequest("knowledge_base_builder", tt.args)
			result, err := h.handleKnowledgeBaseBuilder(context.Background(), req)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			assertPromptResult(t, result)
			assertMessageRole(t, result.Messages[0], "assistant")
			assertMessageRole(t, result.Messages[1], "user")
			assertMessageContains(t, result.Messages[1], tt.args["goal"])
		})
	}
}

func TestHandleGitTemplateSetup(t *testing.T) {
	h := newTestHandler()

	tests := []struct {
		name string
		args map[string]string
	}{
		{
			name: "with all arguments",
			args: map[string]string{
				"intent":   "new Go service",
				"category": "service",
				"depth":    "configure",
			},
		},
		{
			name: "with intent only",
			args: map[string]string{
				"intent": "set up CLAUDE.md",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := makePromptRequest("git_template_setup", tt.args)
			result, err := h.handleGitTemplateSetup(context.Background(), req)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			assertPromptResult(t, result)
			assertMessageRole(t, result.Messages[0], "assistant")
			assertMessageRole(t, result.Messages[1], "user")
			assertMessageContains(t, result.Messages[1], tt.args["intent"])
		})
	}
}

func TestHandleZimResearch(t *testing.T) {
	h := newTestHandler()

	tests := []struct {
		name string
		args map[string]string
	}{
		{
			name: "with all arguments",
			args: map[string]string{
				"topic":             "Go concurrency patterns",
				"depth":             "deep",
				"preferred_sources": "devdocs,wikipedia",
			},
		},
		{
			name: "with topic only",
			args: map[string]string{
				"topic": "REST API design",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := makePromptRequest("zim_research", tt.args)
			result, err := h.handleZimResearch(context.Background(), req)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			assertPromptResult(t, result)
			assertMessageRole(t, result.Messages[0], "assistant")
			assertMessageRole(t, result.Messages[1], "user")
			assertMessageContains(t, result.Messages[1], tt.args["topic"])
		})
	}
}

func TestHandleCrossSourceResearch(t *testing.T) {
	h := newTestHandler()

	tests := []struct {
		name string
		args map[string]string
	}{
		{
			name: "with all arguments",
			args: map[string]string{
				"topic":   "observability best practices",
				"sources": "documents,zim,git",
				"depth":   "deep",
			},
		},
		{
			name: "with topic only defaults sources to all available",
			args: map[string]string{
				"topic": "database migration",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := makePromptRequest("cross_source_research", tt.args)
			result, err := h.handleCrossSourceResearch(context.Background(), req)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			assertPromptResult(t, result)
			assertMessageRole(t, result.Messages[0], "assistant")
			assertMessageRole(t, result.Messages[1], "user")
			assertMessageContains(t, result.Messages[1], tt.args["topic"])
		})
	}
}

// --- Depth guidance helpers ---

func TestZimDepthGuidance(t *testing.T) {
	for _, depth := range []string{"quick", "standard", "deep"} {
		t.Run(depth, func(t *testing.T) {
			got := zimDepthGuidance(depth)
			if got == "" {
				t.Errorf("zimDepthGuidance(%q) returned empty string", depth)
			}
		})
	}
}

func TestCrossSourceDepthGuidance(t *testing.T) {
	for _, depth := range []string{"quick", "standard", "deep"} {
		t.Run(depth, func(t *testing.T) {
			got := crossSourceDepthGuidance(depth)
			if got == "" {
				t.Errorf("crossSourceDepthGuidance(%q) returned empty string", depth)
			}
		})
	}
}

// --- Test helpers ---

// newTestHandler creates a minimal Handler with a real MCP server suitable for
// prompt handler tests. No service or repository dependencies are required
// because prompt handlers do not access them.
func newTestHandler() *Handler {
	srv := mcp.NewServer(
		&mcp.Implementation{Name: "test-server", Version: "v0.0.0"},
		nil,
	)
	return &Handler{server: srv}
}

// makePromptRequest builds a *mcp.GetPromptRequest with the given name and
// arguments, suitable for passing to prompt handler methods.
func makePromptRequest(name string, args map[string]string) *mcp.GetPromptRequest {
	return &mcp.GetPromptRequest{
		Params: &mcp.GetPromptParams{
			Name:      name,
			Arguments: args,
		},
	}
}

// assertPromptResult verifies that the result is non-nil and contains the
// expected number of messages.
func assertPromptResult(t *testing.T, result *mcp.GetPromptResult) {
	t.Helper()
	if result == nil {
		t.Fatal("result is nil")
	}
	if len(result.Messages) != 2 {
		t.Fatalf("got %d messages, want 2", len(result.Messages))
	}
}

// assertMessageRole verifies that a PromptMessage has the expected role.
func assertMessageRole(t *testing.T, msg *mcp.PromptMessage, wantRole mcp.Role) {
	t.Helper()
	if msg.Role != wantRole {
		t.Errorf("message role = %q, want %q", msg.Role, wantRole)
	}
}

// assertMessageContains verifies that the message's TextContent contains the
// given substring.
func assertMessageContains(t *testing.T, msg *mcp.PromptMessage, substr string) {
	t.Helper()
	tc, ok := msg.Content.(*mcp.TextContent)
	if !ok {
		t.Fatalf("message content is %T, want *mcp.TextContent", msg.Content)
	}
	if !strings.Contains(tc.Text, substr) {
		t.Errorf("message text does not contain %q:\n%s", substr, tc.Text)
	}
}
