# DocuMCP Admin Panel

Vue 3 + TypeScript SPA for managing DocuMCP. Built with Vite, Tailwind CSS v4, and Pinia.

## Development

```bash
npm ci                 # Install dependencies
npm run dev            # Vite dev server with HMR
npm run build          # vue-tsc + Vite build
npm run test           # Vitest
npm run test:coverage  # Tests with coverage thresholds
npm run lint           # vue-tsc + ESLint
npm run lint:fix       # ESLint --fix + Prettier
npm run format         # Prettier write
```

## API Client

Stores call the backend through `src/api/helpers.ts` (`apiFetch`). DTO shapes are hand-declared in each store against `docs/contracts/openapi.yaml`.

## Project Structure

```
src/
  api/            apiFetch wrapper + shared helpers
  auth/           Auth guard (OIDC session check)
  components/
    layout/       AppLayout, Sidebar, Header, Notifications
    shared/       DataTable, Pagination, StatusBadge, ConfirmDialog, SearchInput
    documents/    UploadModal, ContentViewer
    users/        UserModal
  composables/    useSSE, useDocumentEvents, useTheme
  router/         Vue Router config
  stores/         Pinia stores (auth, documents, notifications, queue, sse, ...)
  views/          Page components (Dashboard, DocumentList, DocumentDetail, ...)
```
