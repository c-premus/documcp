---
title: I Rewrote My MCP Documentation Server in Go and Dropped 3 Containers
published: false
description: How a 4-container PHP stack became a single Go binary — and what I learned running it in production.
tags: go, mcp, opensource, devops
cover_image:
---

```
Before (PHP/Laravel):
  4 containers (app, queue worker, websocket, monitoring)
  + Redis (queues, cache, sessions)
  + Meilisearch (full-text search)
  + 9 Airflow DAGs (scheduled jobs)
  = ~7.4GB memory limits across 7 moving parts

After (Go):
  1 container
  + PostgreSQL (already existed)
  = 22MB RSS, 1 moving part
```

|            | PHP (Laravel)           | Go                 |
|------------|-------------------------|--------------------|
| Containers | 4 + Redis + Meilisearch | 1                  |
| Image size | ~500 MB                 | 48 MB              |
| Memory     | ~7.4 GB (limits)        | 22 MB (RSS)        |
| Search     | Meilisearch             | PostgreSQL FTS     |
| Job queue  | Horizon + Redis         | River (PostgreSQL) |
| Scheduler  | 9 Airflow DAGs          | Built-in           |

That's the before and after for [DocuMCP](https://github.com/c-premus/documcp), an MCP documentation server I run in production. It serves documents, git project templates, and offline ZIM archives (DevDocs, Wikipedia, Stack Exchange via [Kiwix](https://www.kiwix.org/)) to AI agents through the [Model Context Protocol](https://modelcontextprotocol.io/).

The PHP version worked. But my git history had commits like "Increase Horizon memory limits to prevent OOM crashes" and "Increase documcp-app memory for Vite builds." The feature set had stabilized — I wasn't adding new capabilities, I was tuning infrastructure. Four containers and three external services to serve documents was architecture that had outgrown its justification.

## How It Works

DocuMCP exposes three types of content to MCP clients:

- **Documents** — upload PDF, DOCX, XLSX, HTML, or Markdown. Content is extracted, full-text indexed, and searchable by AI agents.
- **Git templates** — syncs project template repositories on a schedule. Agents can search across files, read specific files, or download entire templates.
- **ZIM archives** — integrates with [Kiwix](https://www.kiwix.org/) to serve offline documentation. DevDocs, Wikipedia, Stack Exchange — whatever ZIM archives you point it at.

There are 15 MCP tools and 6 prompts. OAuth 2.1 with PKCE handles auth (authorization code, device flow, dynamic client registration). A Vue 3 admin panel is embedded in the binary for managing everything.

The entire stack is one Go binary plus PostgreSQL. PostgreSQL handles the database, full-text search (`tsvector`/`tsquery` + `pg_trgm`), and the job queue ([River](https://riverqueue.com/)). The binary includes the HTTP server, queue workers, and a scheduler for periodic jobs — git syncs, archive discovery, token cleanup.

### Search

Full-text search uses PostgreSQL native FTS. Each searchable table has a `tsvector` column — ZIM archives and git templates use `GENERATED ALWAYS` stored columns; the documents table uses a trigger because tags live in a join table and need to be included in the vector.

`pg_trgm` provides fuzzy matching. Synonym expansion (e.g., "js" matches "javascript") happens in Go code rather than a PostgreSQL thesaurus dictionary — the binary runs in a distroless container with no filesystem access to write thesaurus config files.

For a corpus of a few hundred documents plus archive metadata, this runs at 2ms p50. I gave up Meilisearch's typo tolerance and BM25 ranking, which was the right tradeoff for this use case — queries here are precise terms like "OAuth" or "deployment guide," not fuzzy user-generated searches.

The `unified_search` MCP tool fans out concurrently: PostgreSQL FTS across documents, git templates, and ZIM metadata, plus parallel Kiwix article searches across all configured archives. Results merge with synthetic scoring so agents get a single ranked response.

## Production Lessons

**No git binary in distroless.** The image is a static Go binary on `gcr.io/distroless/static:nonroot`. No shell, no package manager, 48MB total. On day one I discovered that `exec.Command("git", "clone", ...)` doesn't work when there's no `git`. Replaced all git operations with [go-git](https://github.com/go-git/go-git) (pure Go) — no subprocess spawning, no PATH dependency.

**Volume ownership mismatch.** Distroless runs as UID 65534 (`nonroot`). The storage volume still had ownership from the old PHP container (UID 1000). The Go process couldn't even `stat` the directory — `permission denied` on a path that clearly existed. Fix was `chown -R 65534:65534` on the volume data. If you're migrating to a different base image, check your volume UIDs first.

**Shared schema, clean cutover.** Both versions use the same PostgreSQL schema. The migration was a container swap — stop the old containers, start the new one. Rollback plan: `docker compose up` with the old image.

## Numbers

From Prometheus and OpenTelemetry, under real MCP traffic:

| Metric | Value |
|--------|-------|
| Container RSS (idle / under load) | 22 MB / 33 MB |
| Go heap | 10 MB |
| Image size | 48 MB |
| Document search (tsvector) p50 / p95 | 3.2 ms / 4.4 ms |
| Document read p50 / p95 | 1 ms / 1.9 ms |
| Git template structure p50 / p95 | 16 ms / 61 ms |
| DB connections | 4 |
| Goroutines | 67 |

---

DocuMCP is open source and running in production, serving documents and templates to Claude via MCP. [Source on GitHub.](https://github.com/c-premus/documcp)

If you're building MCP servers and need OAuth 2.1 in front of them, [mcp-gate](https://github.com/c-premus/mcp-gate) is a companion proxy I built for that.

What's the last dependency you dropped that turned out to be unnecessary?

---

[GitHub](https://github.com/c-premus/documcp) | [Docker Hub](https://hub.docker.com/r/cpremus/documcp)
