# Phase 2A: MCP Server Implementation

You are implementing DocuMCP-go, a documentation server that exposes knowledge bases through the Model Context Protocol (MCP).

## Context

Run `/memory-bank` first to load project context. Phase 1 (foundation) is complete — config, database, models, HTTP server, and DI wiring are all in place.

## Your Task

Implement the MCP server with all 18 tools and 7 prompts. This is the core feature of the application.

## Steps

### 1. Add dependencies

```bash
go get github.com/modelcontextprotocol/go-sdk
```

Read the MCP Go SDK documentation to understand the API:
- Use Context7 to look up `modelcontextprotocol/go-sdk` usage patterns
- Read `docs/contracts/mcp-contract.json` for the exact tool/prompt schemas

### 2. Create MCP handler — `internal/handler/mcp/handler.go`

- Create an MCP server using the official Go SDK
- Register it as an HTTP handler on the `/documcp` route (already stubbed in `internal/server/routes.go`)
- The MCP server should use `mcp.NewServer()` and `mcp.NewStreamableHTTPHandler()`

### 3. Implement all 18 MCP tools

Read `docs/contracts/mcp-contract.json` for exact tool names, descriptions, and input schemas.

Create tool handlers in `internal/handler/mcp/tools/`. Each tool should:
- Validate input from the MCP request
- Create a DTO
- Call the appropriate service/action/repository
- Return formatted JSON response
- Never expose internal database IDs — use UUIDs

Tools to implement (group by domain):

**Document tools** (always registered):
- `search_documents` — full-text search via Meilisearch
- `read_document` — read by UUID with optional summary_only/max_paragraphs
- `create_document` — create markdown/html documents
- `update_document` — update title, description, tags, visibility
- `delete_document` — soft delete by UUID

**ZIM tools** (conditionally registered — check if Kiwix service exists):
- `list_zim_archives` — list with optional category/language/query filters
- `search_zim` — fulltext or suggest search within an archive
- `read_zim_article` — read article with optional summary_only/max_paragraphs

**Confluence tools** (conditionally registered):
- `list_confluence_spaces` — list with optional type/query filters
- `search_confluence` — CQL or simple query search
- `read_confluence_page` — read by page_id or space_key+title

**Git template tools** (conditionally registered):
- `list_git_templates` — list with optional category filter
- `search_git_templates` — full-text search across READMEs
- `get_template_structure` — file tree and metadata
- `get_template_file` — read file with variable substitution
- `get_deployment_guide` — all essential files with instructions
- `download_template` — base64-encoded archive

**Unified search**:
- `unified_search` — federated search across all source types

### 4. Implement all 7 MCP prompts

Read `docs/contracts/mcp-contract.json` for prompt definitions.

Create prompt handlers in `internal/handler/mcp/prompts/`:
- `document_analysis`
- `search_query_builder`
- `knowledge_base_builder`
- `git_template_setup`
- `zim_research`
- `confluence_research`
- `cross_source_research`

### 5. Create repository layer

For each domain, create repositories in `internal/repository/`:
- `document_repository.go` — CRUD for documents, versions, tags
- `external_service_repository.go` — find enabled services by type
- `zim_archive_repository.go` — CRUD for ZIM archives
- `confluence_space_repository.go` — CRUD for Confluence spaces
- `git_template_repository.go` — CRUD for git templates + files
- `search_query_repository.go` — log search queries

Use sqlx with handwritten SQL. Soft deletes where applicable (`deleted_at IS NULL`).

### 6. Create service/action layer

For document operations, create services in `internal/service/`:
- `document_service.go` — orchestrates document CRUD, delegates to repo + search

For simpler operations, create actions in `internal/action/`:
- Actions for ZIM, Confluence, Git template operations

### 7. Wire into the app

Update `internal/app/app.go` to instantiate repositories, services, and the MCP handler.
Update `internal/server/routes.go` to mount the MCP handler.

### 8. Write tests

Write table-driven tests for:
- Each MCP tool handler (mock the repository/service layer)
- Repository methods (these will be integration tests later, for now test SQL building)
- Service orchestration logic

### 9. Verify

```bash
go build ./...
go test ./...
golangci-lint run
```

## Rules

- Follow patterns from CLAUDE.md (constructor injection, context-first, error wrapping)
- Accept interfaces, return structs
- Use `slog.Logger` for structured logging
- Match the MCP contract exactly — tool names, parameter names, response shapes
- Conditional tool registration: check if external service is configured before registering ZIM/Confluence/Git tools
- Commit after each major milestone (repository layer, tool handlers, prompts) using `/commit`
