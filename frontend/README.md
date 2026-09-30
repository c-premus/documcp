# DocuMCP Admin Panel

Vue 3 + TypeScript SPA for managing DocuMCP. Built with Vite, Tailwind CSS v4, and Pinia.

## Development

```bash
npm ci                 # Install dependencies
npm run dev            # Vite dev server with HMR
npm run build          # vue-tsc -b + Vite build -> ../web/frontend/dist/
npm run preview        # Serve the production build locally
npm run test           # Vitest (single run)
npm run test:watch     # Vitest in watch mode
npm run test:coverage  # Tests with coverage thresholds
npm run lint           # vue-tsc -b + ESLint
npm run lint:fix       # ESLint --fix + Prettier
npm run format         # Prettier write
npm run format:check   # Prettier check
```

Type-check with `vue-tsc -b` (what `build` and `lint` run). `tsconfig.json`
is a solution-style root with no files of its own, so `vue-tsc --noEmit`
checks nothing and always passes.

`web/frontend/dist/` is committed and embedded into the Go binary. CI
rebuilds it and fails if the committed copy differs, so commit the rebuilt
`dist/` with any change that affects the bundle.

## API Client

Stores call the backend through `src/api/helpers.ts` (`apiFetch`). DTO shapes are hand-declared in each store against `docs/contracts/openapi.yaml`.

## Project Structure

```
src/
  api/            apiFetch wrapper + shared helpers
  auth/           Auth guard (OIDC session check)
  components/
    layout/            AppLayout, AppHeader, AppSidebar, SidebarNav, AppNotifications
    shared/            DataTable, table cell primitives, Pagination, ConfirmDialog, SearchInput, ...
    documents/         UploadModal, ContentViewer, DocumentEditModal, row actions, mobile cards
    users/             UserRowActions, UserSessionsModal, UserMobileCard
    external-services/ git-templates/ oauth/ queue/ zim/
                       Per-domain row actions, modals, and mobile cards
  composables/    useAppVersion, useAsyncAction, useDocumentEvents, useSidebar, useTheme
  router/         Vue Router config
  stores/         Pinia stores (auth, documents, sse, users, oauthClients, ...)
  utils/          DataTable feature registration, HTML sanitizing
  views/          Page components (Dashboard, DocumentList, DocumentDetail, ...)
  sentry.ts       Optional Sentry/GlitchTip init (VITE_SENTRY_DSN at build time)
```
