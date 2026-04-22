package mcphandler

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"slices"
	"testing"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestMCPContractRegistration is the regression guard for api-design C5 + C6:
// the set of tools and prompts the SDK publishes to a client must match what
// docs/contracts/mcp-contract.json claims, under every combination of the
// conditional feature flags. If a registrar forgets to wire a new tool, or
// the contract drifts from the handler, this test fails at the registration
// boundary — before a real client notices in production.
//
// The SDK's ListTools returns sorted-by-name results (see featureSet.all in
// the go-sdk), so the assertion is set-equality, not the contract's
// toolRegistrationOrder order. An additional sub-check verifies
// toolRegistrationOrder and promptRegistrationOrder are exactly the union of
// the conditional-registration buckets — catching drift between the two
// contract fields themselves.
func TestMCPContractRegistration(t *testing.T) {
	t.Parallel()

	contract := loadMCPContract(t)

	t.Run("contract toolRegistrationOrder covers all conditional buckets", func(t *testing.T) {
		want := contract.allToolNames()
		got := slices.Sorted(slices.Values(contract.ToolRegistrationOrder))
		assert.Equal(t, want, got,
			"toolRegistrationOrder must be the sorted union of always_available + zim + git_templates tools")
	})

	t.Run("contract promptRegistrationOrder covers all conditional buckets", func(t *testing.T) {
		want := contract.allPromptNames()
		got := slices.Sorted(slices.Values(contract.PromptRegistrationOrder))
		assert.Equal(t, want, got,
			"promptRegistrationOrder must be the sorted union of always_available + zim + git_templates prompts")
	})

	cases := []struct {
		name string
		zim  bool
		git  bool
	}{
		{name: "both flags off registers only always-available", zim: false, git: false},
		{name: "zim enabled adds zim tools and prompt", zim: true, git: false},
		{name: "git templates enabled adds git template tools and prompt", zim: false, git: true},
		{name: "all flags on registers the full contract set", zim: true, git: true},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			srv := newContractTestServer(t, tc.zim, tc.git)

			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			t.Cleanup(cancel)

			client := mcp.NewClient(&mcp.Implementation{Name: "contract-regression", Version: "0.0.0"}, nil)
			session, err := client.Connect(ctx, &mcp.StreamableClientTransport{
				Endpoint:             srv.URL,
				DisableStandaloneSSE: true,
			}, nil)
			require.NoError(t, err)
			t.Cleanup(func() { _ = session.Close() })

			wantTools := contract.expectedTools(tc.zim, tc.git)
			wantPrompts := contract.expectedPrompts(tc.zim, tc.git)

			listedTools, err := session.ListTools(ctx, nil)
			require.NoError(t, err)
			gotTools := make([]string, 0, len(listedTools.Tools))
			for _, tool := range listedTools.Tools {
				gotTools = append(gotTools, tool.Name)
			}
			slices.Sort(gotTools)
			assert.Equal(t, wantTools, gotTools, "tool set mismatch; missing tools or unexpected registrations")

			listedPrompts, err := session.ListPrompts(ctx, nil)
			require.NoError(t, err)
			gotPrompts := make([]string, 0, len(listedPrompts.Prompts))
			for _, p := range listedPrompts.Prompts {
				gotPrompts = append(gotPrompts, p.Name)
			}
			slices.Sort(gotPrompts)
			assert.Equal(t, wantPrompts, gotPrompts, "prompt set mismatch")
		})
	}
}

// newContractTestServer spins up a real MCP handler behind httptest with the
// given feature-flag combination. Dependencies are zero-value mocks — the
// test only calls ListTools / ListPrompts, which don't invoke any tool
// handler, so mock methods are never exercised.
func newContractTestServer(t *testing.T, zim, git bool) *httptest.Server {
	t.Helper()

	cfg := Config{
		ServerName:          "documcp-contract",
		ServerVersion:       "0.0.0-test",
		Logger:              slog.New(slog.DiscardHandler),
		DocumentService:     &mockDocumentService{},
		DocumentRepo:        &mockDocumentRepo{},
		Searcher:            &mockSearcher{},
		ZimEnabled:          zim,
		GitTemplatesEnabled: git,
	}
	if zim {
		cfg.ZimArchiveRepo = &mockZimArchiveRepo{}
	}
	if git {
		cfg.GitTemplateRepo = &mockGitTemplateRepo{}
	}

	h := New(cfg)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		h.ServeHTTP(w, r)
	}))
	t.Cleanup(srv.Close)
	return srv
}

// mcpContract mirrors the subset of docs/contracts/mcp-contract.json this
// regression test reads. Fields are PascalCase to match Go conventions; the
// json tags bind to the contract's snake_case keys.
type mcpContract struct {
	ToolRegistrationOrder   []string                `json:"toolRegistrationOrder"`
	PromptRegistrationOrder []string                `json:"promptRegistrationOrder"`
	ConditionalRegistration conditionalRegistration `json:"conditionalRegistrationSummary"`
}

type conditionalRegistration struct {
	AlwaysAvailable      []string         `json:"always_available"`
	RequiresKiwixService featurePartition `json:"requires_kiwix_service"`
	RequiresGitTemplates featurePartition `json:"requires_git_templates_enabled"`
}

type featurePartition struct {
	Tools   []string `json:"tools"`
	Prompts []string `json:"prompts"`
}

// loadMCPContract reads the hand-maintained contract JSON. The path is
// relative to this test file (internal/handler/mcp/) → three dirs up to the
// repo root, then docs/contracts/.
func loadMCPContract(t *testing.T) mcpContract {
	t.Helper()
	raw, err := os.ReadFile("../../../docs/contracts/mcp-contract.json")
	require.NoError(t, err, "mcp-contract.json should be readable from the test dir")

	var c mcpContract
	require.NoError(t, json.Unmarshal(raw, &c), "contract JSON should parse into mcpContract")

	require.NotEmpty(t, c.ToolRegistrationOrder, "contract should list registered tools")
	require.NotEmpty(t, c.PromptRegistrationOrder, "contract should list registered prompts")
	return c
}

// always_available mixes tools and prompts in one array; partition them by
// cross-referencing the per-type registration orders.
func (c mcpContract) alwaysTools() []string {
	toolSet := make(map[string]struct{}, len(c.ToolRegistrationOrder))
	for _, n := range c.ToolRegistrationOrder {
		toolSet[n] = struct{}{}
	}
	out := make([]string, 0)
	for _, n := range c.ConditionalRegistration.AlwaysAvailable {
		if _, ok := toolSet[n]; ok {
			out = append(out, n)
		}
	}
	return out
}

func (c mcpContract) alwaysPrompts() []string {
	promptSet := make(map[string]struct{}, len(c.PromptRegistrationOrder))
	for _, n := range c.PromptRegistrationOrder {
		promptSet[n] = struct{}{}
	}
	out := make([]string, 0)
	for _, n := range c.ConditionalRegistration.AlwaysAvailable {
		if _, ok := promptSet[n]; ok {
			out = append(out, n)
		}
	}
	return out
}

// expectedTools returns the sorted tool-name set the handler should publish
// for the given feature-flag combination.
func (c mcpContract) expectedTools(zim, git bool) []string {
	out := append([]string{}, c.alwaysTools()...)
	if zim {
		out = append(out, c.ConditionalRegistration.RequiresKiwixService.Tools...)
	}
	if git {
		out = append(out, c.ConditionalRegistration.RequiresGitTemplates.Tools...)
	}
	slices.Sort(out)
	return out
}

func (c mcpContract) expectedPrompts(zim, git bool) []string {
	out := append([]string{}, c.alwaysPrompts()...)
	if zim {
		out = append(out, c.ConditionalRegistration.RequiresKiwixService.Prompts...)
	}
	if git {
		out = append(out, c.ConditionalRegistration.RequiresGitTemplates.Prompts...)
	}
	slices.Sort(out)
	return out
}

func (c mcpContract) allToolNames() []string { return c.expectedTools(true, true) }

func (c mcpContract) allPromptNames() []string { return c.expectedPrompts(true, true) }
