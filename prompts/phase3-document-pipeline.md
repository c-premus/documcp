# Phase 3: Document Processing Pipeline + Search

You are implementing DocuMCP-go. Run `/memory-bank` first to load project context.

## Your Task

Implement document upload, content extraction, Meilisearch indexing, and federated search.

## Reference Documents

- `docs/REQUIREMENTS_SPECIFICATION.md` — document processing requirements
- `docs/contracts/openapi.yaml` — document API endpoints
- `docs/contracts/database-schema.sql` — documents, document_versions, document_tags tables

## Steps

### 1. Add dependencies

```bash
go get github.com/meilisearch/meilisearch-go
go get github.com/qax-os/excelize/v2
go get github.com/JohannesKaufmann/html-to-markdown/v2
go get github.com/microcosm-cc/bluemonday
```

Also ensure `poppler-utils` is available in the devcontainer (add to postCreateCommand).

### 2. Content Extractors — `internal/extractor/`

Create an Extractor interface and implementations:

```go
type Extractor interface {
    Extract(ctx context.Context, filePath string) (*ExtractedContent, error)
    Supports(mimeType string) bool
}

type ExtractedContent struct {
    Content  string
    Metadata map[string]any
    WordCount int
}
```

Implementations:
- `pdf/extractor.go` — shell out to `pdftotext` (poppler-utils), no CGO
- `docx/extractor.go` — DOCX is ZIP+XML, extract text from `word/document.xml`
- `xlsx/extractor.go` — use excelize to read all sheets
- `html/extractor.go` — use html-to-markdown, sanitize with bluemonday
- `markdown/extractor.go` — pass-through (already markdown)

### 3. Meilisearch Integration — `internal/search/`

- `client.go` — initialize meilisearch-go client, configure indexes
- `indexer.go` — index/update/delete documents in Meilisearch
- `searcher.go` — search single index, federated multi-index search

Configure 4 indexes matching the PHP version:
- `documents` — title, description, content, tags, file_type
- `zim_archives` — name, title, description, category, language
- `confluence_spaces` — key, name, description, type
- `git_templates` — name, description, readme_content, category, tags

### 4. Document Service — `internal/service/document_service.go`

Orchestrate the full pipeline:
- Upload: validate file, store to disk, create DB record (status: uploaded)
- Process: extract content, update DB (status: extracted), index in Meilisearch (status: indexed)
- Handle failures: update status to failed/index_failed with error_message
- Content hash: SHA-256 of extracted content for dedup detection

### 5. Background Processing — `internal/queue/`

For now, use a simple in-process goroutine worker pool:
- Document extraction job
- Meilisearch indexing job
- Jobs are dispatched after upload, processed asynchronously

### 6. Document API Handlers — `internal/handler/api/document_handler.go`

REST API endpoints from `docs/contracts/openapi.yaml`:
- `POST /api/documents` — upload document (multipart form)
- `GET /api/documents` — list with pagination, filtering
- `GET /api/documents/{uuid}` — get by UUID
- `PUT /api/documents/{uuid}` — update metadata
- `DELETE /api/documents/{uuid}` — soft delete
- `GET /api/search` — federated search

### 7. Wire and test

- Update devcontainer to include `poppler-utils`
- Update app wiring
- Write tests for each extractor (use test fixtures from `docs/contracts/test-fixtures/` if available)
- Write tests for search indexing/querying (mock Meilisearch client)
- Write tests for document service pipeline

```bash
go build ./...
go test ./...
golangci-lint run
```

## Rules

- Follow patterns from CLAUDE.md
- No CGO — shell out to poppler-utils for PDF
- Match the existing Meilisearch index configuration from the PHP version
- Commit after each major milestone using `/commit`
