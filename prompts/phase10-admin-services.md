# Phase 10: Admin UI — OAuth & External Services Pages

You are implementing DocuMCP-go. Run `/memory-bank` first to load project context.

**Depends on**: Phase 9 (shared components, data table pattern, modal pattern, toast pattern, SSE integration)

## Your Task

Implement the remaining admin pages: OAuth client management, external service management, ZIM archive browsing, Confluence space management, Git template file browser, real-time notifications, and queue management UI. This completes the Vue 3 frontend.

**Primary agent**: `typescript-writer`
**Secondary agents**: `go-writer`, `test-generator`

## Requirements Addressed

- REQ-UI-007: OAuth client list with create/edit/revoke
- REQ-UI-008: OAuth client creation with multi-field form
- REQ-UI-009: One-time client secret display with copy button
- REQ-UI-010: External service CRUD with health check and priority ordering
- REQ-UI-011: ZIM archive list with category/language filters and enable/disable toggles
- REQ-UI-012: ZIM archive article search and inline reading
- REQ-UI-013: Confluence space list with sync trigger and connection test
- REQ-UI-014: Git template file browser with recursive tree and content viewer
- REQ-UI-015: Event-driven toast notifications from SSE

## Patterns from Phase 9

Reuse these patterns established in Phase 9:

- **Data tables**: `@tanstack/vue-table` with `useVueTable`, column definitions, `FlexRender`
- **Shared components**: `DataTable`, `Pagination`, `StatusBadge`, `EmptyState`, `ConfirmDialog`, `SearchInput`
- **Modals**: Headless UI `Dialog` + `DialogPanel` + `DialogTitle`, controlled by parent
- **Toasts**: `vue-sonner` `toast()` for success/error/info
- **Stores**: Pinia composition API with `defineStore('name', () => { ... })`
- **API calls**: Use the generated API client from `@/api/generated`

## Steps

### 1. Go API Additions

**`PUT /api/admin/external-services/reorder`** — priority reordering:
```go
// In ExternalServiceHandler or a new endpoint:
// Request body: { "service_ids": [3, 1, 2] } — IDs in desired priority order
// Updates priority column for each service
```

Add to `internal/handler/api/external_service_handler.go` and register in routes.

**`POST /api/admin/git-templates/validate-url`** — SSRF validation:
```go
// Request body: { "url": "https://github.com/user/repo" }
// Response: { "valid": true } or { "valid": false, "error": "URL targets private address" }
// Uses security.ValidateExternalURL internally
```

Add to `internal/handler/api/git_template_handler.go` and register in routes.

**Route registration** — add to `internal/server/routes.go` inside the `/api/admin` group:
```go
r.Put("/external-services/reorder", deps.ExternalServiceHandler.Reorder)
r.Post("/git-templates/validate-url", deps.GitTemplateHandler.ValidateURL)
```

### 2. OAuth Client Pages

**Store — `frontend/src/stores/oauthClients.ts`**:
```ts
export const useOAuthClientsStore = defineStore('oauthClients', () => {
  // State
  const clients = ref<OAuthClient[]>([])
  const total = ref(0)
  const loading = ref(false)

  // Actions
  async function fetchClients(params?: { page?: number; per_page?: number }) { /* GET /api/external-services or /api/admin/oauth-clients */ }
  async function createClient(data: CreateOAuthClientRequest): Promise<{ client_id: string; client_secret: string }> { /* POST */ }
  async function revokeClient(id: number) { /* POST /{id}/revoke */ }

  return { clients, total, loading, fetchClients, createClient, revokeClient }
})
```

**`frontend/src/views/OAuthClientListView.vue`**:
- TanStack Table: Client Name, Client ID, Redirect URIs, Grant Types, Status, Created, Actions
- "Create Client" button → `OAuthClientCreateModal`
- Revoke button → `ConfirmDialog` → revoke → toast

**`frontend/src/components/oauth/OAuthClientCreateModal.vue`**:
- Headless UI `Dialog`
- Form fields:
  - Client name (text input)
  - Redirect URIs (dynamic list — add/remove inputs)
  - Grant types (Headless UI `Listbox` multi-select: `authorization_code`, `client_credentials`, `device_code`, `refresh_token`)
  - Token endpoint auth method (`Listbox`: `client_secret_post`, `client_secret_basic`, `none`)
  - Scope (text input, space-separated)
- On submit → POST → receive `client_id` + `client_secret` → open `SecretDisplayModal`

**`frontend/src/components/oauth/SecretDisplayModal.vue`** (REQ-UI-009):
- Headless UI `Dialog` with warning styling
- Displays `client_id` and `client_secret` in readonly inputs
- **Copy button** for each field — uses `navigator.clipboard.writeText()`
- Warning text: "This secret will not be shown again. Copy it now."
- Only close button, no cancel (secret is already created)
- `@close` emits to parent to refresh the client list

### 3. External Service Pages

**Store — `frontend/src/stores/externalServices.ts`**:
```ts
export const useExternalServicesStore = defineStore('externalServices', () => {
  const services = ref<ExternalService[]>([])
  const loading = ref(false)

  async function fetchServices() { /* GET /api/external-services */ }
  async function createService(data: CreateExternalServiceRequest) { /* POST */ }
  async function updateService(uuid: string, data: UpdateExternalServiceRequest) { /* PUT */ }
  async function deleteService(uuid: string) { /* DELETE */ }
  async function checkHealth(uuid: string) { /* POST /{uuid}/health-check */ }
  async function reorderServices(serviceIds: number[]) { /* PUT /api/admin/external-services/reorder */ }

  return { services, loading, fetchServices, createService, updateService, deleteService, checkHealth, reorderServices }
})
```

**`frontend/src/views/ExternalServiceListView.vue`**:
- TanStack Table: Name, Type (kiwix/confluence), Base URL, Health Status (colored badge), Priority, Actions
- "Add Service" button → modal with form (name, type selector, base URL, API key)
- Health check button per row → POST → update status badge
- Delete button → `ConfirmDialog`
- **Priority reordering**: drag handle or up/down arrows. On change → PUT reorder endpoint.
- Type-specific fields: Confluence shows email:token format hint for API key

### 4. ZIM Archive Pages

**Store — `frontend/src/stores/zimArchives.ts`**:
```ts
export const useZimArchivesStore = defineStore('zimArchives', () => {
  const archives = ref<ZimArchive[]>([])
  const loading = ref(false)

  async function fetchArchives(params?: { category?: string; language?: string; search?: string }) { /* GET /api/zim/archives */ }
  async function toggleEnabled(uuid: string) { /* POST /admin/zim-archives/{uuid}/toggle-enabled */ }
  async function toggleSearchable(uuid: string) { /* POST /admin/zim-archives/{uuid}/toggle-searchable */ }
  async function searchArticles(archive: string, query: string) { /* GET /api/zim/archives/{archive}/search */ }
  async function readArticle(archive: string, path: string) { /* GET /api/zim/archives/{archive}/articles/{path} */ }

  return { archives, loading, fetchArchives, toggleEnabled, toggleSearchable, searchArticles, readArticle }
})
```

**`frontend/src/views/ZimArchiveListView.vue`**:
- Toolbar: search input, category filter (`Listbox`: devdocs, wikipedia, stack_exchange, other), language filter
- TanStack Table: Name, Title, Category (badge), Language, Article Count, Size, Enabled (Switch), Searchable (Switch)
- Toggle switches use Headless UI `Switch`, call toggle endpoint on change
- Click archive name → navigate to browse view

**`frontend/src/views/ZimArchiveBrowseView.vue`** (REQ-UI-012):
- Two-pane layout: search panel (left/top) + article viewer (right/bottom)
- Search input at top → GET `/api/zim/archives/{archive}/search?q=query`
- Results list below search: title + snippet
- Click result → load article content inline → render in `prose` class
- Back button to archive list

### 5. Confluence Space Pages

**Store — `frontend/src/stores/confluenceSpaces.ts`**:
```ts
export const useConfluenceSpacesStore = defineStore('confluenceSpaces', () => {
  const spaces = ref<ConfluenceSpace[]>([])
  const loading = ref(false)

  async function fetchSpaces() { /* GET /api/confluence/spaces */ }
  async function toggleEnabled(uuid: string) { /* POST */ }
  async function toggleSearchable(uuid: string) { /* POST */ }

  return { spaces, loading, fetchSpaces, toggleEnabled, toggleSearchable }
})
```

**`frontend/src/views/ConfluenceSpaceListView.vue`**:
- TanStack Table: Name, Key, Type (global/personal), Description, Enabled (Switch), Searchable (Switch)
- Toggle switches for enabled/searchable
- Connection test button (calls external service health check for confluence type)

### 6. Git Template Pages

**Store — `frontend/src/stores/gitTemplates.ts`**:
```ts
export const useGitTemplatesStore = defineStore('gitTemplates', () => {
  const templates = ref<GitTemplate[]>([])
  const currentTemplate = ref<GitTemplate | null>(null)
  const files = ref<TemplateFile[]>([])
  const loading = ref(false)

  async function fetchTemplates(params?: { search?: string }) { /* GET /api/git-templates */ }
  async function createTemplate(data: CreateGitTemplateRequest) { /* POST */ }
  async function deleteTemplate(uuid: string) { /* DELETE */ }
  async function syncTemplate(uuid: string) { /* POST /{uuid}/sync */ }
  async function fetchStructure(uuid: string) { /* GET /{uuid}/structure */ }
  async function readFile(uuid: string, path: string) { /* GET /{uuid}/files/{path} */ }
  async function validateURL(url: string): Promise<{ valid: boolean; error?: string }> { /* POST /api/admin/git-templates/validate-url */ }

  return { templates, currentTemplate, files, loading, fetchTemplates, createTemplate, deleteTemplate, syncTemplate, fetchStructure, readFile, validateURL }
})
```

**`frontend/src/views/GitTemplateListView.vue`**:
- TanStack Table: Name, Category (badge), Repository URL, Branch, Last Sync, File Count, Actions
- "Add Template" button → modal:
  - Repository URL input with SSRF validation (calls `validateURL` on blur, shows error/success inline)
  - Name, branch, category (`Listbox`: claude, memory-bank, project), description
- Sync button per row → POST sync → toast
- Delete button → `ConfirmDialog`
- Click template → navigate to file browser

**`frontend/src/views/GitTemplateFilesView.vue`** (REQ-UI-014):
- Two-pane layout: file tree (left sidebar) + file content viewer (right main)
- File tree: recursive `TreeNode.vue` component
- On mount: fetch structure → build tree
- Click file → fetch content → display in viewer
- Viewer: syntax highlighting for known file types (or just monospace `<pre>` for now)
- Back button to template list

**`frontend/src/components/shared/TreeNode.vue`** — recursive file tree:
```vue
<script setup lang="ts">
import { ref } from 'vue'
import { FolderIcon, FolderOpenIcon, DocumentIcon } from '@heroicons/vue/24/outline'

export interface TreeItem {
  name: string
  path: string
  type: 'file' | 'directory'
  children?: TreeItem[]
}

const props = defineProps<{
  item: TreeItem
  depth?: number
}>()

defineEmits<{
  select: [path: string]
}>()

const expanded = ref(props.item.type === 'directory' && (props.depth ?? 0) < 2)
</script>

<template>
  <div>
    <button
      class="flex items-center w-full px-2 py-1 text-sm text-left hover:bg-gray-100 rounded"
      :style="{ paddingLeft: `${(depth ?? 0) * 16 + 8}px` }"
      @click="item.type === 'directory' ? (expanded = !expanded) : $emit('select', item.path)"
    >
      <component
        :is="item.type === 'directory' ? (expanded ? FolderOpenIcon : FolderIcon) : DocumentIcon"
        class="h-4 w-4 mr-2 flex-shrink-0"
        :class="item.type === 'directory' ? 'text-blue-500' : 'text-gray-400'"
      />
      <span class="truncate">{{ item.name }}</span>
    </button>
    <div v-if="expanded && item.children">
      <TreeNode
        v-for="child in item.children"
        :key="child.path"
        :item="child"
        :depth="(depth ?? 0) + 1"
        @select="$emit('select', $event)"
      />
    </div>
  </div>
</template>
```

### 7. Notifications — `frontend/src/components/layout/AppNotifications.vue`

Update the layout notification component (REQ-UI-015):

```vue
<script setup lang="ts">
import { Toaster } from 'vue-sonner'
</script>

<template>
  <Toaster
    position="top-right"
    :toastOptions="{
      style: { fontFamily: 'inherit' },
      className: 'text-sm',
    }"
    richColors
    closeButton
  />
</template>
```

SSE events already trigger toasts via `useDocumentEvents` from Phase 9. Extend to handle scheduler events:

In `AppLayout.vue` or a global composable:
```ts
on('job.completed', (event) => {
  const messages: Record<string, string> = {
    'sync_kiwix': 'Kiwix sync completed',
    'sync_confluence': 'Confluence sync completed',
    'sync_git_templates': 'Git template sync completed',
    'cleanup_oauth_tokens': 'OAuth token cleanup completed',
    'health_check_services': 'Service health checks completed',
  }
  const msg = messages[event.job_kind]
  if (msg) toast.info(msg)
})
```

### 8. Queue Management — `frontend/src/views/QueueView.vue`

**Store — `frontend/src/stores/queue.ts`**:
```ts
export const useQueueStore = defineStore('queue', () => {
  const stats = ref<QueueStats | null>(null)
  const failedJobs = ref<FailedJob[]>([])
  const loading = ref(false)

  async function fetchStats() { /* GET /api/admin/queue/stats */ }
  async function fetchFailedJobs(params?: { page?: number }) { /* GET /api/admin/queue/failed */ }
  async function retryJob(id: number) { /* POST /api/admin/queue/failed/{id}/retry */ }
  async function deleteJob(id: number) { /* DELETE /api/admin/queue/failed/{id} */ }

  return { stats, failedJobs, loading, fetchStats, fetchFailedJobs, retryJob, deleteJob }
})
```

**`QueueView.vue`**:
- Stats cards at top: total dispatched, completed, failed, queue depth per queue (high/default/low)
- Tabs (Headless UI `TabGroup`): "Stats" | "Failed Jobs"
- Failed jobs tab: TanStack Table with Job Kind, Queue, Attempt, Error (truncated), Failed At, Actions (retry, delete)
- Retry → POST → toast → refresh
- Delete → `ConfirmDialog` → DELETE → toast → refresh
- Auto-refresh stats every 30 seconds (or on SSE event)

### 9. Update Navigation — `frontend/src/components/layout/AppSidebar.vue`

Ensure sidebar includes all sections:

```
Dashboard
---
Documents
  Document List
  Trash
---
Content Sources
  ZIM Archives
  Confluence Spaces
  Git Templates
---
Administration (admin only)
  Users
  OAuth Clients
  External Services
  Queue
```

Use `@heroicons/vue` icons for each item. Highlight active route. Collapse sections on mobile.

### 10. Tests

**Vue component tests**:
- `OAuthClientListView.vue` — renders table, create button opens modal
- `OAuthClientCreateModal.vue` — form validation, submission
- `SecretDisplayModal.vue` — displays secret, copy button works
- `ExternalServiceListView.vue` — renders, health check trigger
- `ZimArchiveListView.vue` — renders with filters, toggle switches
- `ZimArchiveBrowseView.vue` — search, article display
- `GitTemplateListView.vue` — renders, URL validation feedback
- `GitTemplateFilesView.vue` — tree rendering, file selection
- `TreeNode.vue` — expand/collapse, file selection emit
- `QueueView.vue` — stats display, failed job table

**Store tests**:
- `oauthClients.ts` — CRUD operations with mocked fetch
- `externalServices.ts` — CRUD + reorder + health check
- `zimArchives.ts` — fetch + toggle + search
- `gitTemplates.ts` — CRUD + structure + file reading + URL validation
- `queue.ts` — stats + failed jobs + retry + delete

### 11. Verification

```bash
# Frontend
cd frontend
npx vitest run
npx vue-tsc --noEmit
npm run build

# Go
cd /workspaces/DocuMCP-go
go build ./...
go test -race ./...
golangci-lint run
```

## Commit Checkpoints

1. **OAuth pages**: oauthClients store, OAuthClientListView, OAuthClientCreateModal, SecretDisplayModal
2. **External service pages**: externalServices store, ExternalServiceListView, reorder endpoint
3. **ZIM + browsing**: zimArchives store, ZimArchiveListView, ZimArchiveBrowseView
4. **Confluence + Git templates**: confluenceSpaces store, gitTemplates store, list views, file browser, TreeNode, validate-url endpoint
5. **Notifications + Queue + tests**: AppNotifications, QueueView, queue store, all component and store tests

Use `/commit` after each checkpoint.
