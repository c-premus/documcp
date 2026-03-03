# Phase 8: Vue 3 + TypeScript Frontend Scaffold

You are implementing DocuMCP-go. Run `/memory-bank` first to load project context.

**Depends on**: Phase 7 (SSE endpoint at `GET /api/events/stream`)

## Your Task

Create a `frontend/` directory with Vue 3 + TypeScript + Vite. Auto-generate an API client from the OpenAPI spec. Set up OIDC auth, routing, Pinia stores, layout shell, and SSE composable. Embed built assets in the Go binary via `//go:embed`. The old templ+htmx admin UI continues to exist — it is NOT deleted until Phase 11.

**Primary agent**: `typescript-writer`
**Secondary agents**: `go-writer`, `test-generator`

## Architecture Decisions

1. **Vite builds to `web/frontend/dist/`** — Go embeds this directory via `//go:embed all:dist` in `web/frontend/handler.go`
2. **API client** auto-generated from `docs/contracts/openapi.yaml` using `@hey-api/openapi-ts`
3. **Headless UI** (`@headlessui/vue`) + **Tailwind CSS** for accessible, unstyled components — NOT a full component library
4. **TanStack Table** (`@tanstack/vue-table`) for headless data tables — NOT a pre-built table component
5. **OIDC auth via redirect** — user clicks "Login" → redirects to existing `GET /auth/login` → OIDC flow → session cookie set → redirected back. New `GET /api/auth/me` endpoint returns current user from session.
6. **SPA fallback** — all `/admin/*` requests served by Go. Unmatched paths return `index.html` so Vue Router handles client-side routing.
7. **Coexistence** — old templ+htmx admin at `/admin/` stays mounted. New Vue SPA mounts at `/app/*` (or adjust as needed to avoid conflict). Phase 11 swaps the mount point.

## Steps

### 1. Create Frontend Project

```bash
cd /workspaces/DocuMCP-go
npm create vite@latest frontend -- --template vue-ts
cd frontend
```

### 2. Install Dependencies

**Production**:
```bash
npm install vue-router@4 pinia @headlessui/vue @heroicons/vue/24/outline @heroicons/vue/24/solid @heroicons/vue/20/solid
npm install @tanstack/vue-table vue-sonner
npm install marked dompurify @types/dompurify date-fns
npm install @hey-api/client-fetch
```

**Development**:
```bash
npm install -D tailwindcss @tailwindcss/forms @tailwindcss/typography postcss autoprefixer
npm install -D @hey-api/openapi-ts
npm install -D vitest @vue/test-utils jsdom @types/node
npm install -D typescript vue-tsc
```

### 3. Configure Tailwind — `frontend/tailwind.config.js`

```js
/** @type {import('tailwindcss').Config} */
export default {
  content: ['./index.html', './src/**/*.{vue,js,ts,jsx,tsx}'],
  theme: { extend: {} },
  plugins: [
    require('@tailwindcss/forms'),
    require('@tailwindcss/typography'),
  ],
}
```

Create `frontend/postcss.config.js`:
```js
export default {
  plugins: {
    tailwindcss: {},
    autoprefixer: {},
  },
}
```

Add to `frontend/src/style.css`:
```css
@tailwind base;
@tailwind components;
@tailwind utilities;
```

### 4. Configure Vite — `frontend/vite.config.ts`

```ts
import { defineConfig } from 'vite'
import vue from '@vitejs/plugin-vue'
import { resolve } from 'path'

export default defineConfig({
  plugins: [vue()],
  resolve: {
    alias: { '@': resolve(__dirname, 'src') },
  },
  build: {
    outDir: '../web/frontend/dist',
    emptyOutDir: true,
  },
  server: {
    proxy: {
      '/api': 'http://localhost:8080',
      '/auth': 'http://localhost:8080',
      '/oauth': 'http://localhost:8080',
    },
  },
})
```

### 5. OpenAPI Client Generation

Add to `frontend/package.json` scripts:
```json
{
  "scripts": {
    "dev": "vite",
    "build": "vue-tsc --noEmit && vite build",
    "preview": "vite preview",
    "test": "vitest run",
    "test:watch": "vitest",
    "lint": "vue-tsc --noEmit",
    "generate-api": "openapi-ts -i ../docs/contracts/openapi.yaml -o src/api/generated -c @hey-api/client-fetch"
  }
}
```

Create `frontend/openapi-ts.config.ts`:
```ts
import { defineConfig } from '@hey-api/openapi-ts'

export default defineConfig({
  client: '@hey-api/client-fetch',
  input: '../docs/contracts/openapi.yaml',
  output: 'src/api/generated',
})
```

Run `npm run generate-api` to generate typed API client.

### 6. API Client Wrapper — `frontend/src/api/client.ts`

```ts
import { client } from './generated'

// Configure the base client with auth headers
client.setConfig({
  baseUrl: import.meta.env.VITE_API_BASE_URL || '',
  headers: {
    'Content-Type': 'application/json',
  },
})

// Add response interceptor for 401 → redirect to login
client.interceptors.response.use((response) => {
  if (response.status === 401) {
    window.location.href = '/auth/login?redirect=' + encodeURIComponent(window.location.pathname)
  }
  return response
})

export { client }
```

### 7. Auth Store — `frontend/src/stores/auth.ts`

```ts
import { defineStore } from 'pinia'
import { ref, computed } from 'vue'

export interface User {
  id: number
  email: string
  name: string
  is_admin: boolean
}

export const useAuthStore = defineStore('auth', () => {
  const user = ref<User | null>(null)
  const loading = ref(true)

  const isAuthenticated = computed(() => user.value !== null)
  const isAdmin = computed(() => user.value?.is_admin ?? false)

  async function fetchUser() {
    try {
      const response = await fetch('/api/auth/me')
      if (response.ok) {
        user.value = await response.json()
      } else {
        user.value = null
      }
    } catch {
      user.value = null
    } finally {
      loading.value = false
    }
  }

  function logout() {
    // POST to /auth/logout, then clear state
    fetch('/auth/logout', { method: 'POST' }).finally(() => {
      user.value = null
      window.location.href = '/'
    })
  }

  return { user, loading, isAuthenticated, isAdmin, fetchUser, logout }
})
```

### 8. Auth Guard — `frontend/src/auth/authGuard.ts`

```ts
import type { NavigationGuard } from 'vue-router'
import { useAuthStore } from '@/stores/auth'

export const authGuard: NavigationGuard = async (to, _from, next) => {
  const auth = useAuthStore()

  if (auth.loading) {
    await auth.fetchUser()
  }

  if (!auth.isAuthenticated) {
    window.location.href = '/auth/login?redirect=' + encodeURIComponent(to.fullPath)
    return
  }

  if (to.meta.requiresAdmin && !auth.isAdmin) {
    next({ name: 'dashboard' })
    return
  }

  next()
}
```

### 9. Router — `frontend/src/router/index.ts`

All admin routes lazy-loaded. Use `authGuard` as a global `beforeEach`:

```ts
import { createRouter, createWebHistory } from 'vue-router'
import { authGuard } from '@/auth/authGuard'

const router = createRouter({
  history: createWebHistory('/app'),
  routes: [
    {
      path: '/',
      redirect: '/dashboard',
    },
    {
      path: '/dashboard',
      name: 'dashboard',
      component: () => import('@/views/DashboardView.vue'),
    },
    // Documents
    {
      path: '/documents',
      name: 'documents',
      component: () => import('@/views/DocumentListView.vue'),
    },
    {
      path: '/documents/trash',
      name: 'documents-trash',
      component: () => import('@/views/DocumentTrashView.vue'),
    },
    {
      path: '/documents/:uuid',
      name: 'document-detail',
      component: () => import('@/views/DocumentDetailView.vue'),
      props: true,
    },
    // Users
    {
      path: '/users',
      name: 'users',
      component: () => import('@/views/UserListView.vue'),
      meta: { requiresAdmin: true },
    },
    // OAuth Clients
    {
      path: '/oauth-clients',
      name: 'oauth-clients',
      component: () => import('@/views/OAuthClientListView.vue'),
      meta: { requiresAdmin: true },
    },
    // External Services
    {
      path: '/external-services',
      name: 'external-services',
      component: () => import('@/views/ExternalServiceListView.vue'),
      meta: { requiresAdmin: true },
    },
    // ZIM Archives
    {
      path: '/zim-archives',
      name: 'zim-archives',
      component: () => import('@/views/ZimArchiveListView.vue'),
    },
    {
      path: '/zim-archives/:archive',
      name: 'zim-archive-browse',
      component: () => import('@/views/ZimArchiveBrowseView.vue'),
      props: true,
    },
    // Confluence Spaces
    {
      path: '/confluence-spaces',
      name: 'confluence-spaces',
      component: () => import('@/views/ConfluenceSpaceListView.vue'),
    },
    // Git Templates
    {
      path: '/git-templates',
      name: 'git-templates',
      component: () => import('@/views/GitTemplateListView.vue'),
    },
    {
      path: '/git-templates/:uuid/files',
      name: 'git-template-files',
      component: () => import('@/views/GitTemplateFilesView.vue'),
      props: true,
    },
    // Queue
    {
      path: '/queue',
      name: 'queue',
      component: () => import('@/views/QueueView.vue'),
      meta: { requiresAdmin: true },
    },
  ],
})

router.beforeEach(authGuard)

export default router
```

### 10. SSE Composable — `frontend/src/composables/useSSE.ts`

```ts
import { ref, onUnmounted } from 'vue'

export interface SSEEvent {
  type: string
  job_kind: string
  job_id: number
  queue: string
  attempt?: number
  error?: string
  timestamp: string
}

export function useSSE(url = '/api/events/stream') {
  const connected = ref(false)
  const lastEvent = ref<SSEEvent | null>(null)
  let eventSource: EventSource | null = null
  const listeners = new Map<string, Set<(event: SSEEvent) => void>>()

  function connect() {
    eventSource = new EventSource(url)

    eventSource.onopen = () => { connected.value = true }
    eventSource.onerror = () => {
      connected.value = false
      // Auto-reconnect is built into EventSource
    }
    eventSource.onmessage = (e) => {
      const event: SSEEvent = JSON.parse(e.data)
      lastEvent.value = event
      const handlers = listeners.get(event.type)
      if (handlers) {
        handlers.forEach(fn => fn(event))
      }
    }
  }

  function on(eventType: string, handler: (event: SSEEvent) => void) {
    if (!listeners.has(eventType)) {
      listeners.set(eventType, new Set())
    }
    listeners.get(eventType)!.add(handler)
  }

  function disconnect() {
    eventSource?.close()
    eventSource = null
    connected.value = false
  }

  onUnmounted(disconnect)

  return { connected, lastEvent, connect, disconnect, on }
}
```

### 11. Pinia Stores (Stubs)

Create stub stores in `frontend/src/stores/` — these will be fleshed out in Phases 9-10:

- `auth.ts` — already defined above
- `documents.ts` — list, detail, upload, delete actions
- `notifications.ts` — toast message queue, SSE event handling

Each store follows the composition API pattern with `defineStore('name', () => { ... })`.

### 12. Layout Components — `frontend/src/components/layout/`

**`AppLayout.vue`** — wraps all authenticated pages:
```vue
<template>
  <div class="min-h-screen bg-gray-50">
    <AppSidebar />
    <div class="lg:pl-64">
      <AppHeader />
      <main class="px-4 py-6 sm:px-6 lg:px-8">
        <router-view />
      </main>
    </div>
    <AppNotifications />
  </div>
</template>
```

**`AppSidebar.vue`** — navigation links matching router routes. Use `@heroicons/vue` for icons. Highlight active route.

**`AppHeader.vue`** — user menu (name, email, logout), SSE connection indicator.

**`AppNotifications.vue`** — wrapper for `vue-sonner` `Toaster` component.

### 13. Placeholder Views — `frontend/src/views/`

Create placeholder `.vue` files for every route. Each should be a minimal single-file component:

```vue
<script setup lang="ts">
// Phase 9/10 will implement this
</script>

<template>
  <div>
    <h1 class="text-2xl font-bold text-gray-900">Page Title</h1>
    <p class="mt-2 text-gray-600">Coming in Phase 9.</p>
  </div>
</template>
```

Create these files:
- `DashboardView.vue`
- `DocumentListView.vue`, `DocumentDetailView.vue`, `DocumentTrashView.vue`
- `UserListView.vue`
- `OAuthClientListView.vue`
- `ExternalServiceListView.vue`
- `ZimArchiveListView.vue`, `ZimArchiveBrowseView.vue`
- `ConfluenceSpaceListView.vue`
- `GitTemplateListView.vue`, `GitTemplateFilesView.vue`
- `QueueView.vue`

### 14. App Entry — `frontend/src/App.vue` and `main.ts`

**`main.ts`**:
```ts
import { createApp } from 'vue'
import { createPinia } from 'pinia'
import App from './App.vue'
import router from './router'
import './style.css'

const app = createApp(App)
app.use(createPinia())
app.use(router)
app.mount('#app')
```

**`App.vue`**:
```vue
<script setup lang="ts">
import AppLayout from '@/components/layout/AppLayout.vue'
import { useAuthStore } from '@/stores/auth'

const auth = useAuthStore()
auth.fetchUser()
</script>

<template>
  <AppLayout v-if="auth.isAuthenticated">
    <router-view />
  </AppLayout>
  <div v-else-if="auth.loading" class="flex items-center justify-center min-h-screen">
    <p class="text-gray-500">Loading...</p>
  </div>
</template>
```

### 15. Go Auth Endpoint — `internal/handler/api/auth_handler.go`

New `GET /api/auth/me` endpoint that reads the session cookie and returns the current user:

```go
package api

import (
    "encoding/json"
    "net/http"
    "log/slog"

    "github.com/gorilla/sessions"
)

// AuthHandler provides authentication-related API endpoints.
type AuthHandler struct {
    sessionStore sessions.Store
    userRepo     authUserRepo
    logger       *slog.Logger
}

// authUserRepo finds users by ID (defined where consumed).
type authUserRepo interface {
    FindUserByID(ctx context.Context, id int64) (*model.User, error)
}

func NewAuthHandler(store sessions.Store, repo authUserRepo, logger *slog.Logger) *AuthHandler {
    return &AuthHandler{sessionStore: store, userRepo: repo, logger: logger}
}

// Me returns the currently authenticated user from the session cookie.
// GET /api/auth/me
func (h *AuthHandler) Me(w http.ResponseWriter, r *http.Request) {
    session, err := h.sessionStore.Get(r, "documcp_session")
    if err != nil {
        http.Error(w, "Unauthorized", http.StatusUnauthorized)
        return
    }

    userID, ok := session.Values["user_id"].(int64)
    if !ok {
        http.Error(w, "Unauthorized", http.StatusUnauthorized)
        return
    }

    user, err := h.userRepo.FindUserByID(r.Context(), userID)
    if err != nil {
        http.Error(w, "Unauthorized", http.StatusUnauthorized)
        return
    }

    w.Header().Set("Content-Type", "application/json")
    json.NewEncoder(w).Encode(map[string]any{
        "id":       user.ID,
        "email":    user.Email,
        "name":     user.Name,
        "is_admin": user.IsAdmin,
    })
}
```

### 16. Go SPA Handler — `web/frontend/handler.go`

Embed built frontend assets and serve with SPA fallback:

```go
package frontend

import (
    "embed"
    "io/fs"
    "net/http"
    "strings"
)

//go:embed all:dist
var distFS embed.FS

// Handler returns an http.Handler that serves the embedded Vue SPA.
// It serves static files from dist/ and falls back to index.html for
// unmatched paths (SPA client-side routing).
func Handler() http.Handler {
    dist, err := fs.Sub(distFS, "dist")
    if err != nil {
        panic("frontend dist not embedded: " + err.Error())
    }
    fileServer := http.FileServer(http.FS(dist))

    return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        // Try to serve a static file first
        path := strings.TrimPrefix(r.URL.Path, "/")
        if path == "" {
            path = "index.html"
        }

        // Check if the file exists in the embedded FS
        if f, err := dist.Open(path); err == nil {
            f.Close()
            fileServer.ServeHTTP(w, r)
            return
        }

        // Fallback to index.html for SPA routing
        r.URL.Path = "/"
        fileServer.ServeHTTP(w, r)
    })
}
```

**Important**: Create a placeholder `web/frontend/dist/index.html` with minimal content so the `//go:embed` directive doesn't fail when building Go without running the frontend build first:

```html
<!DOCTYPE html>
<html><body><p>Run frontend build first</p></body></html>
```

### 17. Mount SPA in Routes — `internal/server/routes.go`

Add to `Deps`:
```go
SPAHandler   http.Handler // nil if frontend not embedded
AuthHandler  *apihandler.AuthHandler
```

Add route mounting:
```go
// Auth me endpoint (session-based, no bearer token)
if deps.AuthHandler != nil {
    r.Get("/api/auth/me", deps.AuthHandler.Me)
}

// Vue SPA (after all other routes)
if deps.SPAHandler != nil {
    r.Get("/app/*", http.StripPrefix("/app", deps.SPAHandler).ServeHTTP)
    r.Get("/app", http.RedirectHandler("/app/", http.StatusMovedPermanently).ServeHTTP)
}
```

### 18. Wire in App — `internal/app/app.go`

```go
import frontend "git.999.haus/chris/DocuMCP-go/web/frontend"

// In New():
authH := apihandler.NewAuthHandler(sessionStore, oauthRepo, logger)
spaHandler := frontend.Handler()

// Add to Deps:
srv.RegisterRoutes(server.Deps{
    // ... existing deps ...
    AuthHandler: authH,
    SPAHandler:  spaHandler,
})
```

### 19. Makefile Additions

Add these targets to the existing `Makefile`:

```makefile
# Frontend
frontend-install:
	cd frontend && npm install

frontend-build: frontend-install
	cd frontend && npm run build

frontend-dev:
	cd frontend && npm run dev

frontend-test:
	cd frontend && npx vitest run

frontend-lint:
	cd frontend && npx vue-tsc --noEmit

frontend-generate-api:
	cd frontend && npm run generate-api

# Combined build
build-all: frontend-build build

# Combined test
test-all: test frontend-test
```

### 20. Dockerfile Update

Add a Node.js build stage before the Go build:

```dockerfile
# Stage 1: Frontend build
FROM node:22-alpine AS frontend
WORKDIR /app/frontend
COPY frontend/package*.json ./
RUN npm ci
COPY frontend/ ./
COPY docs/contracts/openapi.yaml ../docs/contracts/openapi.yaml
RUN npm run build

# Stage 2: Go build (existing, modified)
FROM golang:1.25-alpine AS builder
# ... existing setup ...
COPY --from=frontend /app/web/frontend/dist ./web/frontend/dist
# ... existing go build ...
```

### 21. CI Workflow Updates

Add frontend jobs to `.forgejo/workflows/ci.yaml`:

```yaml
  frontend-lint:
    runs-on: docker
    container:
      image: node:22-alpine
    steps:
      - uses: actions/checkout@v4
      - run: cd frontend && npm ci
      - run: cd frontend && npx vue-tsc --noEmit

  frontend-test:
    runs-on: docker
    container:
      image: node:22-alpine
    steps:
      - uses: actions/checkout@v4
      - run: cd frontend && npm ci
      - run: cd frontend && npx vitest run
```

If `.github/workflows/ci.yaml` exists, add the same jobs there.

### 22. Frontend Tests

Create `frontend/vitest.config.ts`:
```ts
import { defineConfig } from 'vitest/config'
import vue from '@vitejs/plugin-vue'
import { resolve } from 'path'

export default defineConfig({
  plugins: [vue()],
  resolve: {
    alias: { '@': resolve(__dirname, 'src') },
  },
  test: {
    environment: 'jsdom',
    globals: true,
  },
})
```

Write tests for:
- Auth store — `fetchUser` with mocked fetch, `logout`
- Auth guard — authenticated vs unauthenticated redirects
- SSE composable — connect, receive event, disconnect
- Layout components — render without errors, sidebar navigation links

### 23. Component Library Reference

When implementing components in later phases, use these mappings:

| UI Need | Library | Component |
|---------|---------|-----------|
| Modals/dialogs | `@headlessui/vue` | `Dialog`, `DialogPanel`, `DialogTitle` |
| Dropdowns/selects | `@headlessui/vue` | `Listbox`, `Combobox` |
| Toggles | `@headlessui/vue` | `Switch` |
| Tabs | `@headlessui/vue` | `TabGroup`, `TabList`, `Tab`, `TabPanels`, `TabPanel` |
| Menus | `@headlessui/vue` | `Menu`, `MenuButton`, `MenuItems`, `MenuItem` |
| Data tables | `@tanstack/vue-table` | `useVueTable`, `FlexRender`, `getCoreRowModel` |
| Toast notifications | `vue-sonner` | `Toaster` component, `toast()` function |
| Icons | `@heroicons/vue` | `24/outline`, `24/solid`, `20/solid` |
| File tree | Custom | Recursive `TreeNode.vue` with Tailwind |
| Forms | `@tailwindcss/forms` | Native HTML inputs styled by Tailwind |
| Markdown prose | `@tailwindcss/typography` | `prose` class on content div |

### 24. Verification

```bash
# Frontend
cd frontend
npm run generate-api
npm run build
npx vitest run
npx vue-tsc --noEmit

# Go (with embedded frontend)
cd /workspaces/DocuMCP-go
go build ./...
go test -race ./...
golangci-lint run

# Combined
make build-all
```

## Commit Checkpoints

1. **Frontend scaffold + build**: Vite project, Tailwind, tsconfig, vite.config.ts
2. **API client generation**: openapi-ts config, generated client, wrapper
3. **Auth + router + stores**: auth store, guard, router, Pinia stores, layout shell
4. **Go embed + SPA serving**: `web/frontend/handler.go`, auth handler, routes, app wiring
5. **CI/CD + tests**: Makefile, Dockerfile, CI workflow, Vitest tests

Use `/commit` after each checkpoint.
