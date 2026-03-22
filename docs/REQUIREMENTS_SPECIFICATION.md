# DocuMCP Requirements Specification

**Version**: 1.0
**Date**: 2026-02-24
**Status**: Reference (derived from production v1.17.3)

> **Note (March 2026):** Confluence integration (REQ-MCP-TOOL-010–012, REQ-MCP-PROMPT-006, `confluence_spaces` index) was removed from the Go rewrite. These requirements are retained for historical reference only and are not implemented in DocuMCP-go.
**Purpose**: Complete functional and non-functional requirements for DocuMCP, suitable for reimplementation in any language/framework.

---

## Table of Contents

1. [System Overview](#1-system-overview)
2. [Authentication & Authorization](#2-authentication--authorization)
3. [MCP Protocol Server](#3-mcp-protocol-server)
4. [Document Management](#4-document-management)
5. [Search System](#5-search-system)
6. [External Service Integrations](#6-external-service-integrations)
7. [Admin Web Interface](#7-admin-web-interface)
8. [REST API](#8-rest-api)
9. [Background Processing](#9-background-processing)
10. [Observability](#10-observability)
11. [Security Requirements](#11-security-requirements)
12. [Non-Functional Requirements](#12-non-functional-requirements)
13. [Database Schema](#13-database-schema)
14. [Configuration](#14-configuration)

---

## 1. System Overview

### 1.1 Purpose

DocuMCP is a documentation server that exposes knowledge bases through the Model Context Protocol (MCP), enabling AI agents (Claude, GPT, etc.) to search, read, and manage documentation. It also provides a REST API and web-based admin panel.

### 1.2 Core Capabilities

| Capability | Description |
|-----------|-------------|
| MCP Server | JSON-RPC 2.0 endpoint with 18 tools and 7 prompts |
| Document Management | Upload, extract, index, search PDF/DOCX/XLSX/HTML/Markdown |
| OAuth 2.1 Authorization Server | PKCE, device authorization, dynamic client registration |
| OIDC Authentication | User login via external identity provider |
| Full-Text Search | Federated multi-index search via Meilisearch |
| External Integrations | ZIM archives (Kiwix), Confluence, Git template repositories |
| Admin Panel | Web UI for document, user, OAuth client, and service management |
| Observability | OpenTelemetry tracing, Prometheus metrics, structured logging |

### 1.3 User Personas

| Persona | Interaction |
|---------|-------------|
| AI Agent | MCP endpoint via OAuth 2.1 bearer tokens |
| Documentation Admin | Web UI via OIDC login |
| DevOps Engineer | Health checks, metrics, Grafana dashboards |
| CLI Tool User | Device Authorization Grant (RFC 8628) |
| Claude.ai Connected Service | OAuth 2.1 with RFC 9728 discovery |

---

## 2. Authentication & Authorization

### 2.1 OIDC User Authentication

**REQ-AUTH-001**: The system MUST authenticate users via an external OpenID Connect (OIDC) provider.

**REQ-AUTH-002**: OIDC auto-discovery MUST be supported (fetching `/.well-known/openid-configuration` from the provider).

**REQ-AUTH-003**: Manual endpoint configuration MUST be supported as an alternative to auto-discovery.

**REQ-AUTH-004**: The system MUST support group-based admin authorization via a configurable OIDC claim (e.g., `groups`).

**REQ-AUTH-005**: Users MUST be auto-provisioned on first OIDC login (create if not exists, update on subsequent logins).

**REQ-AUTH-006**: OIDC token validation MUST verify: `iss`, `exp`, `aud`, `iat`, `nbf`, `sub` claims with 5-minute clock skew tolerance.

**REQ-AUTH-007**: Optional session-based login (email/password) MUST be supported for environments without OIDC.

### 2.2 OAuth 2.1 Authorization Server

#### 2.2.1 Authorization Server Metadata (RFC 8414)

**REQ-OAUTH-001**: The system MUST expose `GET /.well-known/oauth-authorization-server` returning server metadata including all supported endpoints, grant types, response types, and scopes.

#### 2.2.2 Protected Resource Metadata (RFC 9728)

**REQ-OAUTH-002**: The system MUST expose `GET /.well-known/oauth-protected-resource/{path?}` identifying the protected resource and its authorization server.

**REQ-OAUTH-003**: The `resource` field MUST be path-aware (e.g., `https://domain.com/documcp` for the MCP endpoint).

**REQ-OAUTH-004**: 401 responses from the MCP endpoint MUST include a `WWW-Authenticate` header referencing the resource metadata URL.

#### 2.2.3 Dynamic Client Registration (RFC 7591)

**REQ-OAUTH-005**: The system MUST support `POST /oauth/register` for dynamic client registration.

**REQ-OAUTH-006**: Registration MUST accept: `client_name` (required), `redirect_uris` (required, array), `grant_types`, `response_types`, `token_endpoint_auth_method`, `scope`, `software_id`, `software_version`.

**REQ-OAUTH-007**: Registration MUST be optionally gated behind authentication (configurable).

**REQ-OAUTH-008**: Public clients (`token_endpoint_auth_method=none`) MUST NOT receive a `client_secret`.

**REQ-OAUTH-009**: Confidential clients MUST receive a one-time `client_secret` (64 random characters, bcrypt-hashed for storage).

#### 2.2.4 Authorization Code Grant with PKCE

**REQ-OAUTH-010**: The system MUST implement the authorization code grant flow with consent screen.

**REQ-OAUTH-011**: Public clients MUST use PKCE with S256 method (no `plain` method allowed).

**REQ-OAUTH-012**: PKCE verification MUST use constant-time comparison (`hash_equals`).

**REQ-OAUTH-013**: Authorization codes MUST expire after 10 minutes (configurable).

**REQ-OAUTH-014**: Authorization codes MUST be single-use (revoked after exchange).

**REQ-OAUTH-015**: Redirect URI validation MUST require exact match, with RFC 8252 localhost port flexibility for native apps.

**REQ-OAUTH-016**: The consent screen MUST include a nonce to prevent stale-tab / TOCTOU attacks (10-minute expiry).

#### 2.2.5 Token Endpoint

**REQ-OAUTH-017**: `POST /oauth/token` MUST support grant types: `authorization_code`, `refresh_token`, `urn:ietf:params:oauth:grant-type:device_code`.

**REQ-OAUTH-018**: Access tokens MUST expire after 1 hour (configurable).

**REQ-OAUTH-019**: Refresh tokens MUST expire after 30 days (configurable).

**REQ-OAUTH-020**: Token format MUST be `{token_id}|{64_char_random}` for efficient lookup.

**REQ-OAUTH-021**: Tokens MUST be stored as SHA-256 hashes (not bcrypt, for performance).

**REQ-OAUTH-022**: Token lookup MUST be ID-based (parse ID from token, fetch by ID, verify hash) for O(1) performance.

**REQ-OAUTH-023**: Refresh token rotation: using a refresh token MUST revoke the old access+refresh tokens and issue new ones.

#### 2.2.6 Token Revocation (RFC 7009)

**REQ-OAUTH-024**: `POST /oauth/revoke` MUST revoke access or refresh tokens.

**REQ-OAUTH-025**: Revocation MUST always return 200 (never reveal token existence).

**REQ-OAUTH-026**: Revoking an access token MUST also revoke its associated refresh token.

#### 2.2.7 Device Authorization Grant (RFC 8628)

**REQ-OAUTH-027**: `POST /oauth/device/code` MUST generate a `device_code` and `user_code`.

**REQ-OAUTH-028**: User codes MUST use Base-20 charset (BCDFGHJKLMNPQRSTVWXZ) in XXXX-XXXX format (no vowels, no confusable characters).

**REQ-OAUTH-029**: Device codes MUST be hashed (SHA-256). User codes MUST be stored in plaintext for lookup.

**REQ-OAUTH-030**: Device code lifetime MUST be 15 minutes (configurable).

**REQ-OAUTH-031**: The system MUST return `authorization_pending` while the user hasn't approved.

**REQ-OAUTH-032**: The system MUST return `slow_down` and increase the polling interval by 5 seconds (capped at 300s) when the device polls too fast.

**REQ-OAUTH-033**: Device code status transitions: `pending` -> `authorized`/`denied` -> `exchanged` (one-time use).

**REQ-OAUTH-034**: A web-based verification page MUST be provided at the `verification_uri` for user code entry and consent.

### 2.3 Scopes

**REQ-OAUTH-035**: The system MUST support scopes: `mcp:access` (default), `mcp:read`, `mcp:write`.

**REQ-OAUTH-036**: Scopes MUST be advertised in authorization server metadata.

### 2.4 Rate Limiting

| Endpoint | Rate |
|----------|------|
| Token endpoint | 30/min + 100/hour per IP |
| Client registration | 10/hour + 50/day per IP |
| Authorization | 30/min per IP |
| Device code request | 30/min per IP |
| Device verification | 5/min + 30/hour per IP |
| MCP endpoint | 60/min per user or IP |
| API read | 60/min per user or IP |
| API write | 30/min per user or IP |
| Search | 120/min per user or IP |

### 2.5 MCP Endpoint Authentication

**REQ-AUTH-008**: The MCP endpoint MUST accept Bearer tokens in the `Authorization` header.

**REQ-AUTH-009**: The system MUST first attempt OAuth 2.1 access token validation (local DB lookup), then fall back to OIDC token validation (external provider).

**REQ-AUTH-010**: Authentication failures MUST be tracked in metrics.

---

## 3. MCP Protocol Server

### 3.1 Protocol

**REQ-MCP-001**: The system MUST implement MCP protocol version `2024-11-05` via JSON-RPC 2.0 over HTTP POST.

**REQ-MCP-002**: The MCP endpoint path MUST be configurable (default: `/documcp`).

**REQ-MCP-003**: Pagination MUST support up to 100 items per page (all tools visible in one page).

### 3.2 Tools (18 total)

All tool responses MUST include `"success": true` on success. Error responses MUST use generic messages (never expose internal error details). All JSON encoding MUST use flags: `JSON_THROW_ON_ERROR | JSON_UNESCAPED_SLASHES | JSON_UNESCAPED_UNICODE` (no pretty-print).

#### 3.2.1 Unified Search (1 tool)

**REQ-MCP-TOOL-001**: `unified_search` - Federated search across all content sources.
- Parameters: `query` (required, string), `types` (optional, array of: document/zim_archive/confluence_space/git_template), `limit` (optional, 1-100, default 10), `offset` (optional, default 0)
- Behavior: Executes Meilisearch `multiSearch()` with `MultiSearchFederation` across all available indexes. Applies per-source access control filters. Returns normalized results with source type identification.

#### 3.2.2 Document Tools (5 tools)

**REQ-MCP-TOOL-002**: `search_documents` - Full-text document search.
- Parameters: `query` (required), `file_type` (optional, enum), `tags` (optional, string[]), `limit` (optional, 1-100), `include_snippets` (optional, bool), `include_content` (optional, bool)
- Returns: Array of results with uuid, title, description, file_type, tags, content_length, optional snippet/content.

**REQ-MCP-TOOL-003**: `read_document` - Read document by UUID.
- Parameters: `uuid` (required), `summary_only` (optional, bool), `max_paragraphs` (optional, 1-100)
- Returns: Document metadata + content. Supports smart truncation at paragraph boundaries.
- Access control: Public documents readable by anyone; private documents require ownership or admin.

**REQ-MCP-TOOL-004**: `create_document` - Create document from content.
- Parameters: `title` (required), `content` (required), `file_type` (required, markdown|html), `description` (optional), `is_public` (optional), `tags` (optional)
- Requires authentication.

**REQ-MCP-TOOL-005**: `update_document` - Update document metadata.
- Parameters: `uuid` (required), `title` (optional), `description` (optional), `is_public` (optional), `tags` (optional, replaces existing)
- Requires authentication and ownership/admin.

**REQ-MCP-TOOL-006**: `delete_document` - Soft-delete document.
- Parameters: `uuid` (required)
- Requires authentication and ownership/admin.

#### 3.2.3 ZIM Archive Tools (3 tools)

These tools MUST only register when ZIM integration is enabled AND the Kiwix service is available.

**REQ-MCP-TOOL-007**: `list_zim_archives` - List available ZIM archives.
- Parameters: `query` (optional), `category` (optional, enum: devdocs/wikipedia/stack_exchange/other), `language` (optional), `limit` (optional, 1-100)

**REQ-MCP-TOOL-008**: `search_zim` - Search within a ZIM archive.
- Parameters: `archive` (required, archive name), `query` (required), `search_type` (optional, suggest|fulltext), `limit` (optional, 1-50)

**REQ-MCP-TOOL-009**: `read_zim_article` - Read article from ZIM archive.
- Parameters: `archive` (required), `path` (required), `summary_only` (optional), `max_paragraphs` (optional)
- Content truncation: Smart paragraph-boundary truncation, lead section extraction for summary.

#### 3.2.4 Confluence Tools (3 tools)

These tools MUST only register when Confluence integration is enabled AND the service is available.

**REQ-MCP-TOOL-010**: `list_confluence_spaces` - List Confluence spaces.
- Parameters: `query` (optional), `type` (optional, global|personal|knowledge_base), `limit` (optional, 1-100)

**REQ-MCP-TOOL-011**: `search_confluence` - Search Confluence pages.
- Parameters: `cql` (optional, advanced CQL query), `query` (optional, simple text), `space` (optional), `limit` (optional, 1-50)
- Requires either `cql` or `query`.

**REQ-MCP-TOOL-012**: `read_confluence_page` - Read page content.
- Parameters: `page_id` (optional), `space_key` + `title` (alternative), `summary_only` (optional), `max_paragraphs` (optional)
- Content: Confluence storage format converted to Markdown.

#### 3.2.5 Git Template Tools (6 tools)

These tools MUST only register when Git Templates feature is enabled.

**REQ-MCP-TOOL-013**: `list_git_templates` - List templates.
- Parameters: `category` (optional, claude|memory-bank|project), `limit` (optional, 1-100)

**REQ-MCP-TOOL-014**: `search_git_templates` - Search templates.
- Parameters: `query` (required), `category` (optional), `limit` (optional, 1-50)

**REQ-MCP-TOOL-015**: `get_template_structure` - View file tree.
- Parameters: `uuid` (required)
- Returns: ASCII tree, essential files list, required variables, file count.

**REQ-MCP-TOOL-016**: `get_template_file` - Read file with variable substitution.
- Parameters: `uuid` (required), `path` (required), `variables` (optional, JSON string of key-value pairs)
- Variable substitution: Replace `{{variable_name}}` placeholders. Report unresolved variables.

**REQ-MCP-TOOL-017**: `get_deployment_guide` - Generate deployment instructions.
- Parameters: `uuid` (required), `variables` (optional, JSON string)

**REQ-MCP-TOOL-018**: `download_template` - Download as archive.
- Parameters: `uuid` (required), `format` (optional, zip|tar.gz), `variables` (optional, JSON string)
- Returns: Base64-encoded archive in JSON response. Requires authentication.

### 3.3 Tool Annotations

Read-only tools MUST be annotated as `readOnly` and `idempotent`. Write tools (create, update, delete, download) have no read-only annotation.

### 3.4 Conditional Registration

Tools and prompts for external services MUST implement a `shouldRegister()` check:
- ZIM/Confluence tools: Verify config flag AND runtime service availability
- Git Template tools: Verify config flag
- Document tools and unified search: Always register

### 3.5 Prompts (7 total)

All prompts MUST return multi-message format: an assistant message (tool reference, workflow guidance) and a user message (specific intent/parameters).

**REQ-MCP-PROMPT-001**: `document_analysis` - Summarize, compare, extract, or assess documents.
- Parameters: `document_ids` (required, UUID[]), `task` (summarize|compare|extract|assess), `focus` (technical|business|overview|actionable), `length` (brief|detailed|comprehensive)

**REQ-MCP-PROMPT-002**: `search_query_builder` - Help construct effective search queries.
- Parameters: `goal` (required), `context` (optional), `file_types` (optional)

**REQ-MCP-PROMPT-003**: `knowledge_base_builder` - Guide document creation.
- Parameters: `goal` (required), `content_type` (guide|reference|runbook|notes), `scope` (single|collection|organize)

**REQ-MCP-PROMPT-004**: `git_template_setup` - Guide template discovery and deployment.
- Parameters: `intent` (required), `category` (optional), `depth` (browse|preview|deploy)
- Conditional: Only when Git Templates enabled.

**REQ-MCP-PROMPT-005**: `zim_research` - Guide ZIM-based research.
- Parameters: `topic` (required), `depth` (quick|standard|deep), `preferred_sources` (optional)
- Conditional: Only when ZIM enabled and service available.

**REQ-MCP-PROMPT-006**: `confluence_research` - Guide Confluence research.
- Parameters: `topic` (required), `space` (optional), `depth` (quick|standard|deep)
- Conditional: Only when Confluence enabled and service available.

**REQ-MCP-PROMPT-007**: `cross_source_research` - Guide multi-source research.
- Parameters: `topic` (required), `sources` (optional, array), `depth` (quick|standard|deep)
- Always registered.

---

## 4. Document Management

### 4.1 Supported Formats

| Format | MIME Types | Extractor |
|--------|-----------|-----------|
| PDF | `application/pdf` | pdftohtml/pdftotext (poppler-utils) |
| DOCX | `application/vnd.openxmlformats-officedocument.wordprocessingml.document` | Go DOCX library (excelize) |
| XLSX | `application/vnd.openxmlformats-officedocument.spreadsheetml.sheet` | Library-based, exports as CSV per sheet |
| HTML | `text/html`, `application/xhtml+xml` | HTML-to-Markdown converter |
| Markdown | `text/markdown`, `text/plain`, `text/x-markdown` | Pass-through (no conversion) |

**REQ-DOC-001**: Maximum file upload size: 50 MB (configurable).

### 4.2 Processing Pipeline

**REQ-DOC-002**: Document upload MUST follow this pipeline:
1. Validate file (extension, MIME type, size)
2. Store file to filesystem
3. Create database record with status `processing`
4. Dispatch background job for text extraction
5. Extract text content to Markdown format
6. Extract metadata (title, author, page count, images, etc.)
7. Compute word count and SHA-256 content hash
8. Update status to `extracted`
9. Dispatch background job for Meilisearch indexing
10. Index in Meilisearch
11. Update status to `indexed`

**REQ-DOC-003**: Status transitions: `processing` -> `extracted` -> `indexed`. Error states: `failed` (extraction), `index_failed` (indexing).

**REQ-DOC-004**: Processing jobs MUST retry 3 times with exponential backoff (60s, 120s, 300s).

**REQ-DOC-005**: PDF extraction timeout: 120 seconds. Known-unrecoverable errors (secured PDFs) MUST NOT retry.

### 4.3 Document Model

**REQ-DOC-006**: Documents MUST support soft deletes with configurable retention (default 30 days).

**REQ-DOC-007**: Documents MUST have a UUID for external identification (never expose internal IDs).

**REQ-DOC-008**: Documents MUST support: title, description, file_type, file_path, file_size, mime_type, content (extracted text), content_hash, metadata (JSON), word_count, user_id (owner), is_public, status, error_message, tags (many-to-many).

### 4.4 Access Control

**REQ-DOC-009**: Admin users can access all documents.

**REQ-DOC-010**: Document owners can CRUD their own documents.

**REQ-DOC-011**: Public documents (`is_public=true`) are readable by any authenticated user.

**REQ-DOC-012**: Soft-deleted documents MUST be invisible except to restore/force-delete operations.

### 4.5 Content Analysis

**REQ-DOC-013**: The system MUST provide metadata analysis for uploaded files: auto-extract title (from metadata, first H1, or filename), description (from meta tags or first paragraph), and suggested tags (from headings).

---

## 5. Search System

### 5.1 Meilisearch Integration

**REQ-SEARCH-001**: The system MUST use Meilisearch for full-text search across 4 indexes: documents, zim_archives, confluence_spaces, git_templates.

**REQ-SEARCH-002**: Each index MUST have configured: searchable attributes, filterable attributes, sortable attributes, ranking rules, typo tolerance.

**REQ-SEARCH-003**: Soft-deleted records MUST be tracked in Meilisearch via `__soft_deleted` filter attribute.

### 5.2 Federated Search

**REQ-SEARCH-004**: The unified search MUST use Meilisearch `multiSearch()` with federation to search all indexes in a single HTTP request.

**REQ-SEARCH-005**: Per-source access control filters MUST be applied: documents and git_templates enforce `__soft_deleted=0` + user/public visibility; ZIM and Confluence have no user-level access control.

**REQ-SEARCH-006**: Results MUST be normalized with source type identification via the `_federation.indexUid` field.

### 5.3 Index Configuration

**Documents index**:
- Searchable: title, description, content, tags
- Filterable: uuid, file_type, status, user_id, is_public, tags, created_at, updated_at, __soft_deleted
- Sortable: created_at, updated_at, word_count
- Stop words: the, a, an, and, or, but
- Synonyms: php/hypertext-preprocessor, js/javascript/ecmascript, ts/typescript

**ZIM archives index**:
- Searchable: title, name, description, creator, tags
- Filterable: uuid, language, category, creator, tags, article_count
- Sortable: title, article_count

**Confluence spaces index**:
- Searchable: name, key, description
- Filterable: uuid, key, type, status, external_service_id, is_enabled, __soft_deleted
- Sortable: name, key

**Git templates index**:
- Searchable: name, description, readme_content, category, tags
- Filterable: uuid, slug, category, user_id, is_public, status, __soft_deleted
- Sortable: name, created_at

---

## 6. External Service Integrations

### 6.1 Service Management

**REQ-EXT-001**: External services MUST be configurable via both environment variables (priority) and database records.

**REQ-EXT-002**: Each service record MUST track: type, name, base_url, api_key (encrypted), config (encrypted JSON), priority, status (healthy/unhealthy/unknown), health metrics (latency, error count, consecutive failures, last error).

**REQ-EXT-003**: Health checks MUST be supported for all service types with configurable timeouts (default 5 seconds).

### 6.2 ZIM Archives (Kiwix)

**REQ-ZIM-001**: The system MUST integrate with Kiwix-Serve via its HTTP API for ZIM archive access.

**REQ-ZIM-002**: Catalog sync MUST fetch the OPDS catalog and create/update/disable local archive records.

**REQ-ZIM-003**: Article search MUST support both suggest (autocomplete) and fulltext modes.

**REQ-ZIM-004**: Article content MUST be converted from HTML to plain text with: script/style removal, HTML entity decoding, LaTeX/MathML conversion to readable text.

**REQ-ZIM-005**: Caching: metadata 1 hour, content responses as configured. Cache corruption detection with automatic recovery.

**REQ-ZIM-006**: Path traversal prevention: block `..`, null bytes, hidden files, leading slashes.

### 6.3 Confluence

**REQ-CONF-001**: The system MUST integrate with Confluence REST API v1 with Basic authentication (email + API token).

**REQ-CONF-002**: Space sync MUST fetch all spaces and create/update/disable local records.

**REQ-CONF-003**: Page search MUST support both CQL (advanced) and simple text queries.

**REQ-CONF-004**: Page content MUST be converted from Confluence storage format (XHTML) to Markdown: handle macros (info/note/warning/tip panels -> blockquotes, code blocks, expand, status), tables, headings, links, lists.

**REQ-CONF-005**: Caching: metadata 1 hour, content 10 minutes, search results 5 minutes.

### 6.4 Git Templates

**REQ-GIT-001**: The system MUST clone Git repositories and extract template files for browsing and download.

**REQ-GIT-002**: Clone/pull MUST use `--depth 1 --single-branch` for efficiency.

**REQ-GIT-003**: Credentials MUST be passed via environment variable mechanism (e.g., `GIT_ASKPASS`), never in CLI arguments or config files. Temporary credential scripts MUST be deleted in finally blocks.

**REQ-GIT-004**: SSRF protection: block localhost, loopback, and private IP ranges. Re-validate URL at clone time (TOCTOU prevention). Support configurable allowed-hosts whitelist.

**REQ-GIT-005**: File extraction MUST: skip symlinks, skip `.git` directory, enforce per-file size limits (1 MB), enforce total size limits (10 MB), detect binary files, canonicalize paths.

**REQ-GIT-006**: Essential file detection via glob patterns: CLAUDE.md, memory-bank/*.md, template.json, README.md, .claude/**/\*.

**REQ-GIT-007**: Variable substitution: detect `{{variable_name}}` placeholders, validate key format, strip control characters from values.

**REQ-GIT-008**: Archive generation: support zip and tar.gz formats, base64-encode for JSON response, apply variable substitution, prevent zip-slip via path sanitization.

**REQ-GIT-009**: Templates MUST support soft deletes and categories (claude, memory-bank, project).

**REQ-GIT-010**: Error messages from git operations MUST be sanitized (remove URLs, tokens, base64 credentials).

---

## 7. Admin Web Interface

### 7.1 Dashboard

**REQ-UI-001**: Dashboard MUST show resource count cards: Documents, Users, OAuth Clients (always visible). ZIM Archives, Confluence Spaces, Git Templates (conditionally visible based on service availability).

### 7.2 Document Management

**REQ-UI-002**: Document list with: full-text search, file type filter, status filter, sortable columns (title, file_type, status, created_at, word_count), pagination, upload modal.

**REQ-UI-003**: File upload with AI-powered metadata analysis: auto-suggest title, description, tags from file content.

**REQ-UI-004**: Document viewer: render Markdown (GFM), sanitized HTML, or plain text based on type.

**REQ-UI-005**: Soft-deleted documents page: search, restore, individual purge, bulk purge by age.

### 7.3 User Management

**REQ-UI-006**: User list with: search, create/edit/delete modals, sortable columns, document/query counts. Prevent self-deletion.

### 7.4 OAuth Client Management

**REQ-UI-007**: OAuth client list with: search, create modal (name, redirect URIs, auth method, grant types, scope), edit modal (name, redirect URIs, grant types, scope, active toggle), delete confirmation.

**REQ-UI-008**: Auth method MUST NOT be editable after creation.

**REQ-UI-009**: Client secret MUST be displayed exactly once after creation in a dedicated modal.

**REQ-UI-010**: "Last used" column derived from access token creation timestamps (no schema change needed).

### 7.5 External Service Management

**REQ-UI-011**: Service manager with: CRUD for services (Kiwix, Confluence types), health check execution, enable/disable toggle, priority ordering. Environment-managed services shown as read-only.

### 7.6 ZIM Archive Management

**REQ-UI-012**: ZIM archive manager: list with search/category/language/service filters, OPDS catalog sync, enable/searchable toggles, connection testing, archive browsing (search + article reading within archive).

### 7.7 Confluence Space Management

**REQ-UI-013**: Confluence space manager: list with search/type/service filters, space sync, enable/searchable toggles, connection testing.

### 7.8 Git Template Management

**REQ-UI-014**: Git template manager: list with search/category/status filters, register new templates (repository URL with SSRF validation, branch, credentials, category), edit, sync trigger, enable/public toggles, file browser.

### 7.9 Notifications

**REQ-UI-015**: Notifications MUST use event dispatch (not session flash) for compatibility with AJAX-driven UI frameworks.

---

## 8. REST API

### 8.1 Authentication

**REQ-API-001**: API endpoints MUST support Bearer token authentication (Bearer tokens (OAuth 2.1)).

**REQ-API-002**: Public endpoints: `GET /api/health`, `GET /api/search`, `GET /api/search/popular`.

### 8.2 Document Endpoints

| Method | Path | Description |
|--------|------|-------------|
| GET | `/api/documents` | List with pagination and file_type filter |
| POST | `/api/documents` | Create (file upload) |
| GET | `/api/documents/{uuid}` | Show |
| PUT | `/api/documents/{uuid}` | Update metadata |
| DELETE | `/api/documents/{uuid}` | Soft delete |
| GET | `/api/documents/{uuid}/download` | Download original file |
| POST | `/api/documents/analyze` | Analyze file for metadata suggestions |

### 8.3 Search Endpoints

| Method | Path | Description |
|--------|------|-------------|
| GET | `/api/search` | Full-text search with facets |
| GET | `/api/search/popular` | Popular search queries |
| GET | `/api/search/autocomplete` | Title suggestions |
| POST | `/api/search/unified` | Cross-source federated search |

### 8.4 ZIM Endpoints

| Method | Path | Description |
|--------|------|-------------|
| GET | `/api/zim/archives` | List archives |
| GET | `/api/zim/archives/{archive}` | Show archive |
| GET | `/api/zim/archives/{archive}/search` | Search articles |
| GET | `/api/zim/archives/{archive}/suggest` | Suggest articles |
| GET | `/api/zim/archives/{archive}/articles/{path}` | Read article |

### 8.5 Confluence Endpoints

| Method | Path | Description |
|--------|------|-------------|
| GET | `/api/confluence/spaces` | List spaces |
| GET | `/api/confluence/spaces/{key}` | Show space |
| GET | `/api/confluence/pages` | Search pages (CQL or text) |
| GET | `/api/confluence/pages/{pageId}` | Read page |

### 8.6 Git Template Endpoints

| Method | Path | Description |
|--------|------|-------------|
| GET | `/api/git-templates` | List |
| GET | `/api/git-templates/search` | Search |
| POST | `/api/git-templates` | Register |
| GET | `/api/git-templates/{uuid}` | Show |
| PUT | `/api/git-templates/{uuid}` | Update |
| DELETE | `/api/git-templates/{uuid}` | Delete |
| POST | `/api/git-templates/{uuid}/sync` | Trigger sync |
| GET | `/api/git-templates/{uuid}/structure` | File tree |
| GET | `/api/git-templates/{uuid}/files/{path}` | Read file |
| GET | `/api/git-templates/{uuid}/deployment-guide` | Deployment guide |
| POST | `/api/git-templates/{uuid}/download` | Download archive |

### 8.7 Health Endpoints

| Method | Path | Auth | Description |
|--------|------|------|-------------|
| GET | `/api/health` | None | Basic health (for load balancers) |
| GET | `/api/health/deep` | Internal token | Database, Redis, Meilisearch, filesystem, queue |

### 8.8 Response Format

**REQ-API-003**: API resources MUST include HATEOAS `_links` with rel, href, method.

**REQ-API-004**: All responses MUST use UUIDs for resource identification (never internal IDs).

**REQ-API-005**: Error responses MUST use consistent format: `{message, status, errors?, meta: {request_id}}`.

**REQ-API-006**: OAuth error responses MUST follow RFC 6749 format: `{error, error_description}`.

---

## 9. Background Processing

### 9.1 Queue System

**REQ-QUEUE-001**: The system MUST support Redis-backed queues with priority levels: `high`, `default`, `low`.

**REQ-QUEUE-002**: Queue supervision (Horizon or equivalent) with auto-scaling (1-20 processes).

**REQ-QUEUE-003**: Failed jobs MUST be stored for inspection and retry.

### 9.2 Jobs

| Job | Queue | Timeout | Retries | Backoff |
|-----|-------|---------|---------|---------|
| ProcessDocumentUpload | high | 300s | 3 | 60s, 120s, 300s |
| IndexDocumentInMeilisearch | default | 300s | 3 | 60s, 120s, 300s |
| ReindexAllDocuments | low | 3600s | 1 | - |

### 9.3 Scheduled Tasks

| Task | Schedule | Description |
|------|----------|-------------|
| OAuth token pruning | Daily 2 AM | Remove expired tokens/codes |
| Orphaned file cleanup | Weekly Sunday 2 AM | Remove files without DB records |
| Soft-delete cleanup | Weekly Sunday 3 AM | Permanently delete 30+ day old soft-deleted docs |
| Search index verification | Daily 4 AM | Verify and repair Meilisearch index integrity |
| ZIM sync | Daily 5 AM | Sync OPDS catalog |
| Confluence sync | Daily 5:30 AM | Sync spaces |
| Git template sync | Daily 6 AM | Sync all templates |
| ZIM cleanup | Daily 6 AM | Clean orphaned ZIM data |
| Service health check | Daily 3 AM | Check all external services |

### 9.4 Events

| Event | Channel | Purpose |
|-------|---------|---------|
| DocumentProcessed | Private user.{userId} | Status updates during processing |
| DocumentIndexed | Private user.{userId} | Indexing completion |
| DocumentProcessingFailed | Private user.{userId} | Processing failure notification |

---

## 10. Observability

### 10.1 OpenTelemetry Tracing

**REQ-OBS-001**: Distributed tracing via OpenTelemetry with OTLP export.

**REQ-OBS-002**: Auto-instrumentation for: HTTP server, HTTP client, database queries, Redis, queue jobs, cache operations, events, views/templates, search operations.

**REQ-OBS-003**: Console command instrumentation MUST be opt-in (explicit command list).

**REQ-OBS-004**: Custom span naming: `action.{namespace}.{operation}`, `service.{name}.{method}`, `middleware.{name}`, `extractor.{type}`, `job.{name}.{step}`.

**REQ-OBS-005**: Sensitive data MUST be redacted from span attributes (tokens, codes, credentials).

**REQ-OBS-006**: Trace context propagation via W3C Trace Context (`traceparent` header).

### 10.2 Prometheus Metrics

**REQ-OBS-007**: `/metrics` endpoint in Prometheus text format.

**REQ-OBS-008**: Metrics: `documents_uploaded_total`, `documents_deleted_total`, `http_requests_total`, `auth_failures_total`, `document_search_duration_seconds`, `document_search_results`, `http_request_duration_seconds`.

### 10.3 Health Checks

**REQ-OBS-009**: Shallow health (`/api/health`): 200 OK with timestamp and environment. Must be public for load balancers.

**REQ-OBS-010**: Deep health (`/api/health/deep`): Check database, Redis, Meilisearch, filesystem, queue. Return 200 (healthy) or 503 (degraded). Optionally token-protected.

### 10.4 Logging

**REQ-OBS-011**: Structured logging with trace ID correlation.

**REQ-OBS-012**: Log levels: configurable, default debug in development.

---

## 11. Security Requirements

### 11.1 Transport & Headers

**REQ-SEC-001**: HSTS header in production (max-age=31536000, includeSubDomains, preload).

**REQ-SEC-002**: Security headers: X-Content-Type-Options (nosniff), X-Frame-Options (SAMEORIGIN), X-XSS-Protection, Referrer-Policy, Permissions-Policy, Content-Security-Policy.

**REQ-SEC-003**: Hidden/sensitive files MUST be blocked at the application layer (return 404, not 403): dotfiles (except .well-known), composer.json/lock, package.json/lock, server config files.

### 11.2 Credential Security

**REQ-SEC-004**: Passwords and client secrets: bcrypt hashed.

**REQ-SEC-005**: Machine-generated tokens (access, refresh, auth codes, device codes): SHA-256 hashed.

**REQ-SEC-006**: Sensitive fields in database MUST use application-level encryption (api_key, git_token, config).

**REQ-SEC-007**: Tokens MUST never appear in logs, error messages, or CLI arguments.

### 11.3 Input Validation

**REQ-SEC-008**: All API endpoints MUST use dedicated request validation (not inline).

**REQ-SEC-009**: Path traversal prevention for file access (ZIM articles, git template files).

**REQ-SEC-010**: SSRF protection for URL inputs (git repository URLs): block private IPs, validate at clone time.

**REQ-SEC-011**: Filter injection prevention in Meilisearch queries (escape user input).

**REQ-SEC-012**: XXE hardening for Office document parsing.

### 11.4 Authorization

**REQ-SEC-013**: Two-layer authorization: middleware for route-level access, policies for resource-level access.

**REQ-SEC-014**: Admin flag (`is_admin`) MUST NOT be mass-assignable.

---

## 12. Non-Functional Requirements

### 12.1 Performance

**REQ-NFR-001**: Search response time: < 100ms.

**REQ-NFR-002**: Health check (shallow): < 5ms.

**REQ-NFR-003**: Health check (deep): < 50ms.

**REQ-NFR-004**: Token verification: ~1ms (SHA-256).

### 12.2 Reliability

**REQ-NFR-005**: Background jobs MUST retry with exponential backoff.

**REQ-NFR-006**: External service clients MUST implement caching with graceful degradation.

**REQ-NFR-007**: Cache corruption MUST be detected and auto-recovered.

### 12.3 Scalability

**REQ-NFR-008**: HTTP server MUST support multiple worker processes (e.g., 8 workers).

**REQ-NFR-009**: Queue workers MUST auto-scale (1-20 processes).

**REQ-NFR-010**: Meilisearch pagination limit: 10,000 total hits.

### 12.4 Compatibility

**REQ-NFR-011**: Database queries MUST be compatible with PostgreSQL (primary), SQLite (testing).

**REQ-NFR-012**: The system MUST run behind reverse proxies (Traefik) with configurable trusted proxy ranges.

---

## 13. Database Schema

### 13.1 Tables

| Table | Purpose | UUID | Soft Delete |
|-------|---------|------|-------------|
| users | User accounts | No | No |
| documents | Uploaded documents | Yes | Yes |
| document_versions | Document version history | No | No |
| document_tags | Document tag associations | No | No |
| search_queries | Search query tracking | No | No |
| oauth_clients | OAuth 2.1 clients | No (uses client_id) | No |
| oauth_access_tokens | Bearer tokens | No | No (revoked flag) |
| oauth_refresh_tokens | Refresh tokens | No | No (revoked flag) |
| oauth_authorization_codes | Auth codes | No | No (revoked flag) |
| oauth_device_codes | Device auth codes | No | No (status enum) |
| external_services | Service configuration | Yes | No |
| zim_archives | ZIM archive metadata | Yes | No |
| confluence_spaces | Confluence space metadata | Yes | No |
| git_templates | Git template metadata | Yes | Yes |
| git_template_files | Template file content | Yes | No |

### 13.2 Key Relationships

```
users -< documents -< document_versions
users -< documents -< document_tags
users -< search_queries
users -< oauth_clients -< oauth_access_tokens -> oauth_refresh_tokens
users -< oauth_clients -< oauth_authorization_codes
users -< oauth_clients -< oauth_device_codes
users -< git_templates -< git_template_files
external_services -< zim_archives
external_services -< confluence_spaces
```

### 13.3 Constraints

- All primary keys: auto-incrementing BIGINT
- UUIDs: CHAR(36), UNIQUE indexed
- Foreign keys with CASCADE or SET NULL as appropriate
- Composite indexes for common query patterns
- No database-specific types (no JSON columns, no unsigned integers)

---

## 14. Configuration

### 14.1 Required Environment Variables

| Variable | Purpose |
|----------|---------|
| `APP_KEY` | Application encryption key |
| `DB_CONNECTION`, `DB_HOST`, `DB_DATABASE`, `DB_USERNAME`, `DB_PASSWORD` | Database |
| `MEILISEARCH_HOST`, `MEILISEARCH_KEY` | Search engine |
| `REDIS_HOST` | Cache, queue, sessions |
| `OIDC_PROVIDER_URL`, `OIDC_CLIENT_ID`, `OIDC_CLIENT_SECRET` | Identity provider |

### 14.2 Optional Environment Variables

| Variable | Default | Purpose |
|----------|---------|---------|
| `DOCUMCP_ENDPOINT` | `/documcp` | MCP endpoint path |
| `DOCUMCP_NAME` | `DocuMCP` | Server display name |
| `DOCUMCP_MAX_FILE_SIZE` | `52428800` | Max upload (bytes) |
| `OAUTH_ACCESS_TOKEN_LIFETIME` | `3600` | Access token TTL (seconds) |
| `OAUTH_REFRESH_TOKEN_LIFETIME` | `2592000` | Refresh token TTL (seconds) |
| `OAUTH_PKCE_REQUIRED` | `true` | PKCE mandatory for public clients |
| `OAUTH_CLIENT_REGISTRATION_AUTH_REQUIRED` | `true` | Gate registration |
| `OIDC_ADMIN_GROUP` | - | OIDC group for admin role |
| `INTERNAL_API_TOKEN` | - | Protect /metrics and /health/deep |
| `OTEL_ENABLED` | `false` | Enable OpenTelemetry |
| `OTEL_EXPORTER_OTLP_ENDPOINT` | - | OTLP collector endpoint |

---

*End of Requirements Specification*
