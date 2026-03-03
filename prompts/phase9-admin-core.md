# Phase 9: Admin UI — Core Pages

You are implementing DocuMCP-go. Run `/memory-bank` first to load project context.

**Depends on**: Phase 8 (Vue 3 scaffold, router, stores, layout, SSE composable)

## Your Task

Implement the core admin pages: Dashboard, Document Management (list, upload with AI analysis, detail viewer, soft-delete/restore/purge), User Management, and SSE-driven event integration. This phase fills in the placeholder views created in Phase 8 and adds shared UI components.

**Primary agent**: `typescript-writer`
**Secondary agents**: `go-writer`, `test-generator`

## Requirements Addressed

- REQ-UI-001: Dashboard with system statistics
- REQ-UI-002: Document list with search, filters, sorting, pagination
- REQ-UI-003: Document upload with AI analysis before save
- REQ-UI-004: Document content viewer (Markdown, HTML, plain text rendering)
- REQ-UI-005: Soft-delete, restore, and permanent purge
- REQ-UI-006: User management (list, admin toggle, create/edit/delete)

## Architecture Patterns

All views follow this structure:

1. **View** (`.vue` in `src/views/`) — page-level component, fetches data on mount, uses Pinia store
2. **Store** (`.ts` in `src/stores/`) — Pinia composition store, API calls, state management
3. **Shared components** (`src/components/shared/`) — reusable across pages
4. **Feature components** (`src/components/{feature}/`) — feature-specific (upload modal, content viewer)

**Data table pattern** (used on every list page):
- `@tanstack/vue-table` with `useVueTable` + `getCoreRowModel` + `getSortedRowModel` + `getFilteredRowModel`
- Column definitions with typed accessors
- `FlexRender` for cell rendering
- Server-side pagination via API query params (`?page=1&per_page=25`)
- Toolbar with search input, filter dropdowns, action buttons

**Modal pattern**:
- `@headlessui/vue` `Dialog` + `DialogPanel` + `DialogTitle`
- Controlled by parent via `v-model:open` or `:open` + `@close`
- Form submission emits result to parent

**Toast pattern**:
- `vue-sonner` `toast()` for success/error/info notifications
- SSE events trigger toasts automatically via `useDocumentEvents` composable

## Steps

### 1. Go API Additions

Before building the frontend, add missing API endpoints.

**`internal/handler/api/user_handler.go`** — complete the stubs:
```go
// Ensure these methods exist and work:
// GET    /api/admin/users          — List (paginated, searchable)
// GET    /api/admin/users/{id}     — Show
// POST   /api/admin/users          — Create
// PUT    /api/admin/users/{id}     — Update
// DELETE /api/admin/users/{id}     — Delete
// POST   /api/admin/users/{id}/toggle-admin — ToggleAdmin
```

The existing `UserHandler` in `internal/handler/api/user_handler.go` has stubs — verify they return proper JSON with `response.JSON(w, status, data)` matching the response envelope pattern in `internal/handler/api/response.go`.

**`internal/handler/api/dashboard_handler.go`** — new file:
```go
// GET /api/admin/dashboard/stats
// Returns: { documents: N, users: N, oauth_clients: N, external_services: N,
//            zim_archives: N, confluence_spaces: N, git_templates: N,
//            queue: { pending: N, completed: N, failed: N } }
```

Define a `dashboardStatsRepo` interface (where consumed) with count methods. Wire counts from existing repositories.

**Document soft-delete endpoints** — verify these exist in `DocumentHandler`:
```go
// POST   /api/documents/{uuid}/restore   — restore soft-deleted document
// DELETE /api/documents/{uuid}/purge     — permanently delete
// DELETE /api/admin/documents/purge?older_than_days=30 — bulk purge
```

If missing, add them. The `DocumentRepository` already has `SoftDelete`. Add `Restore` (clear `deleted_at`) and `Purge` (hard delete + remove file) methods if needed.

**Route registration** — add new routes in `internal/server/routes.go`:
```go
// Inside /api route group, add admin sub-group:
r.Route("/admin", func(r chi.Router) {
    // TODO: Add admin auth middleware (session or bearer + admin check)
    r.Get("/dashboard/stats", deps.DashboardHandler.Stats)

    r.Route("/users", func(r chi.Router) {
        r.Get("/", deps.UserHandler.List)
        r.Post("/", deps.UserHandler.Create)
        r.Get("/{id}", deps.UserHandler.Show)
        r.Put("/{id}", deps.UserHandler.Update)
        r.Delete("/{id}", deps.UserHandler.Delete)
        r.Post("/{id}/toggle-admin", deps.UserHandler.ToggleAdmin)
    })

    r.Delete("/documents/purge", deps.DocumentHandler.BulkPurge)
})

// Document restore/purge (in existing /api/documents group):
r.Post("/{uuid}/restore", deps.DocumentHandler.Restore)
r.Delete("/{uuid}/purge", deps.DocumentHandler.Purge)
```

### 2. Shared Components — `frontend/src/components/shared/`

**`DataTable.vue`** — generic wrapper around TanStack Table:
```vue
<script setup lang="ts" generic="T">
import { FlexRender, getCoreRowModel, useVueTable } from '@tanstack/vue-table'
import type { ColumnDef } from '@tanstack/vue-table'

const props = defineProps<{
  data: T[]
  columns: ColumnDef<T, any>[]
  loading?: boolean
}>()
</script>

<template>
  <div class="overflow-hidden shadow ring-1 ring-black ring-opacity-5 rounded-lg">
    <table class="min-w-full divide-y divide-gray-300">
      <thead class="bg-gray-50">
        <tr>
          <th v-for="header in table.getHeaderGroups()[0].headers" :key="header.id"
              class="px-3 py-3.5 text-left text-sm font-semibold text-gray-900">
            <FlexRender :render="header.column.columnDef.header" :props="header.getContext()" />
          </th>
        </tr>
      </thead>
      <tbody class="divide-y divide-gray-200 bg-white">
        <!-- rows -->
      </tbody>
    </table>
  </div>
</template>
```

**`Pagination.vue`** — page navigation with page size selector:
```vue
<script setup lang="ts">
defineProps<{
  page: number
  perPage: number
  total: number
}>()

defineEmits<{
  'update:page': [page: number]
  'update:perPage': [perPage: number]
}>()
</script>
```

**`StatusBadge.vue`** — colored badge for document status:
```vue
<script setup lang="ts">
defineProps<{ status: string }>()
// Map status → color: uploaded=yellow, extracted=blue, indexed=green, failed=red, index_failed=orange
</script>
```

**`EmptyState.vue`** — placeholder for empty lists (icon, title, description, action button).

**`ConfirmDialog.vue`** — Headless UI Dialog for destructive action confirmation:
```vue
<script setup lang="ts">
import { Dialog, DialogPanel, DialogTitle } from '@headlessui/vue'
import { ExclamationTriangleIcon } from '@heroicons/vue/24/outline'

defineProps<{
  open: boolean
  title: string
  message: string
  confirmLabel?: string
  variant?: 'danger' | 'warning'
}>()

defineEmits<{
  confirm: []
  cancel: []
}>()
</script>
```

**`SearchInput.vue`** — debounced search input with clear button (300ms debounce).

### 3. Documents Store — `frontend/src/stores/documents.ts`

```ts
import { defineStore } from 'pinia'
import { ref } from 'vue'
// Import generated API client functions

export const useDocumentsStore = defineStore('documents', () => {
  const documents = ref<Document[]>([])
  const currentDocument = ref<Document | null>(null)
  const total = ref(0)
  const loading = ref(false)
  const error = ref<string | null>(null)

  async function fetchDocuments(params: { page?: number; per_page?: number; search?: string; status?: string; file_type?: string }) { /* ... */ }
  async function fetchDocument(uuid: string) { /* ... */ }
  async function uploadDocument(formData: FormData) { /* ... */ }
  async function deleteDocument(uuid: string) { /* soft delete */ }
  async function restoreDocument(uuid: string) { /* ... */ }
  async function purgeDocument(uuid: string) { /* hard delete */ }
  async function bulkPurge(olderThanDays: number) { /* ... */ }

  return { documents, currentDocument, total, loading, error, fetchDocuments, fetchDocument, uploadDocument, deleteDocument, restoreDocument, purgeDocument, bulkPurge }
})
```

### 4. Dashboard — `frontend/src/views/DashboardView.vue`

- Fetch stats from `GET /api/admin/dashboard/stats`
- Display stat cards in a responsive grid (2 cols on mobile, 3-4 on desktop)
- Each card: icon, label, count, optional link to the corresponding list page
- Conditionally show external service cards only if counts > 0
- Queue stats card with pending/completed/failed counts

### 5. Document List — `frontend/src/views/DocumentListView.vue`

- Toolbar: search input, file type filter (Headless UI `Listbox`), status filter, "Upload" button, "Trash" link
- TanStack Table with columns: Title, File Type, Status (StatusBadge), Size, Created, Actions (view, delete)
- Sortable columns (Title, Created, Size)
- Server-side pagination via `Pagination` component
- Click row → navigate to document detail
- Delete button → `ConfirmDialog` → soft delete → toast

### 6. Document Upload — `frontend/src/components/documents/UploadModal.vue`

Headless UI Dialog with multi-step flow:

1. **File selection**: drag-and-drop zone or file picker. Validate file type (.pdf, .docx, .xlsx, .html, .md, .txt) and size (50 MB max) client-side.
2. **AI analysis** (optional): if user clicks "Analyze", POST to `/api/documents/analyze` with file. Display suggested title, description, tags from response.
3. **Metadata form**: title, description, tags (comma-separated input), visibility toggle (Headless UI `Switch`)
4. **Submit**: POST multipart form to `/api/documents`, show progress, toast on success, close modal, refresh list

### 7. Document Detail — `frontend/src/views/DocumentDetailView.vue`

- Metadata panel: title, description, file type, status, size, word count, content hash, created/updated timestamps, tags
- Download button linking to `/api/documents/{uuid}/download`
- **Content viewer** (`ContentViewer` component):
  - Markdown: render with `marked` + `dompurify`, wrapped in `prose` class
  - HTML: sanitize with `dompurify`, render in `prose` class
  - Plain text: display in `<pre>` with monospace font
- Soft-delete button (if not already deleted) → `ConfirmDialog`

### 8. Content Viewer — `frontend/src/components/documents/ContentViewer.vue`

```vue
<script setup lang="ts">
import { computed } from 'vue'
import { marked } from 'marked'
import DOMPurify from 'dompurify'

const props = defineProps<{
  content: string
  fileType: string // 'markdown' | 'html' | 'pdf' | 'docx' | 'xlsx'
}>()

const renderedContent = computed(() => {
  if (props.fileType === 'markdown') {
    return DOMPurify.sanitize(marked(props.content) as string)
  }
  if (props.fileType === 'html') {
    return DOMPurify.sanitize(props.content)
  }
  return null // plain text fallback
})
</script>

<template>
  <div v-if="renderedContent" class="prose prose-sm max-w-none" v-html="renderedContent" />
  <pre v-else class="whitespace-pre-wrap text-sm text-gray-700 font-mono bg-gray-50 p-4 rounded-lg">{{ content }}</pre>
</template>
```

### 9. Document Trash — `frontend/src/views/DocumentTrashView.vue`

- List soft-deleted documents (pass `?status=deleted` or `?deleted=true` filter)
- Columns: Title, File Type, Deleted At, Actions (restore, purge)
- Restore → POST `/api/documents/{uuid}/restore` → toast
- Purge → `ConfirmDialog` with strong warning → DELETE `/api/documents/{uuid}/purge` → toast
- Bulk purge button → `ConfirmDialog` with days input → DELETE `/api/admin/documents/purge?older_than_days=N`

### 10. Users — `frontend/src/views/UserListView.vue`

- TanStack Table: Name, Email, Admin (badge), OIDC Subject, Created, Actions
- "Create User" button → `UserModal` (create mode)
- Click row → inline edit via `UserModal` (edit mode)
- Admin toggle → Headless UI `Switch` inline in table, POST to toggle-admin endpoint
- Delete → `ConfirmDialog` → DELETE

**`frontend/src/components/users/UserModal.vue`**:
- Headless UI `Dialog`
- Fields: name, email, admin toggle
- Validates email format
- Emits `created` or `updated` event on success

### 11. SSE Event Integration — `frontend/src/composables/useDocumentEvents.ts`

Composable that connects SSE and reacts to document pipeline events:

```ts
import { useSSE } from '@/composables/useSSE'
import { useDocumentsStore } from '@/stores/documents'
import { toast } from 'vue-sonner'

export function useDocumentEvents() {
  const { connect, on, connected } = useSSE()
  const documents = useDocumentsStore()

  function start() {
    connect()

    on('job.completed', (event) => {
      if (event.job_kind === 'document_extract') {
        toast.success('Document extracted successfully')
        documents.refreshCurrent()
      }
      if (event.job_kind === 'document_index') {
        toast.success('Document indexed successfully')
        documents.refreshCurrent()
      }
    })

    on('job.failed', (event) => {
      if (event.job_kind.startsWith('document_')) {
        toast.error(`Document processing failed: ${event.error}`)
      }
    })
  }

  return { start, connected }
}
```

Use this in `AppLayout.vue` — call `start()` on mount so SSE events trigger toasts globally.

### 12. Notifications Store — `frontend/src/stores/notifications.ts`

```ts
import { defineStore } from 'pinia'
import { ref } from 'vue'

export interface Notification {
  id: string
  type: 'success' | 'error' | 'info' | 'warning'
  title: string
  message?: string
  timestamp: Date
}

export const useNotificationsStore = defineStore('notifications', () => {
  const notifications = ref<Notification[]>([])

  function add(notification: Omit<Notification, 'id' | 'timestamp'>) {
    notifications.value.push({
      ...notification,
      id: crypto.randomUUID(),
      timestamp: new Date(),
    })
  }

  function remove(id: string) {
    notifications.value = notifications.value.filter(n => n.id !== id)
  }

  return { notifications, add, remove }
})
```

### 13. Tests

**Vue component tests** (Vitest + @vue/test-utils):

- `DataTable.vue` — renders headers and rows, handles empty state
- `Pagination.vue` — displays correct page numbers, emits page changes
- `StatusBadge.vue` — renders correct colors for each status
- `ConfirmDialog.vue` — opens/closes, emits confirm/cancel
- `ContentViewer.vue` — renders markdown as HTML, sanitizes, handles plain text
- `UploadModal.vue` — file selection, form submission, validation
- `DashboardView.vue` — renders stat cards from mocked API
- `DocumentListView.vue` — renders table, search filtering, navigation
- `UserListView.vue` — renders table, admin toggle

**Store tests**:
- `documents.ts` — fetchDocuments, uploadDocument, deleteDocument with mocked fetch
- `auth.ts` — fetchUser authenticated/unauthenticated
- `notifications.ts` — add/remove

### 14. Verification

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

1. **Go API stubs**: dashboard handler, user CRUD, document restore/purge, route registration
2. **Dashboard + Document list**: DashboardView, DocumentListView, DataTable, Pagination, StatusBadge
3. **Upload + Viewer**: UploadModal, ContentViewer, DocumentDetailView
4. **Soft-delete + Users**: DocumentTrashView, UserListView, UserModal, ConfirmDialog
5. **SSE integration + tests**: useDocumentEvents, notifications store, all Vue tests

Use `/commit` after each checkpoint.
