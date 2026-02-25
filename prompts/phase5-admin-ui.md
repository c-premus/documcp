# Phase 5: Admin Web UI

You are implementing DocuMCP-go. Run `/memory-bank` first to load project context.

## Your Task

Implement the admin web UI using templ + htmx + Tailwind CSS.

## Steps

### 1. Add dependencies

```bash
go install github.com/a-h/templ/cmd/templ@latest
go get github.com/a-h/templ
```

Add Tailwind CSS via CDN (or local build).

### 2. Layout — `web/templates/`

- Base layout with navigation, flash messages, user menu
- Login page (OIDC redirect)
- Dashboard with statistics

### 3. Admin Pages

Each page uses htmx for partial updates (search, pagination, modals):

- **Documents** — list with search/filter, upload modal, detail view, delete confirmation
- **Users** — list, detail, admin toggle
- **OAuth Clients** — list, create modal (show one-time secret), detail, revoke
- **External Services** — list, create/edit, health check trigger, delete
- **ZIM Archives** — list, sync trigger, enable/disable
- **Confluence Spaces** — list, sync trigger, enable/disable
- **Git Templates** — list, create, sync trigger, detail with file browser

### 4. htmx Patterns

- `hx-get` / `hx-post` for partial page updates
- `hx-trigger="keyup changed delay:300ms"` for search debouncing
- `hx-target` / `hx-swap` for targeted updates
- Toast notifications via response headers
- Modal dialogs via htmx

### 5. Wire and test

- Mount admin routes behind SessionAuth + RequireAdmin middleware
- Compile templ templates
- Test that pages render without errors

```bash
templ generate
go build ./...
go test ./...
golangci-lint run
```

Commit using `/commit`.
