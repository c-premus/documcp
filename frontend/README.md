# DocuMCP Admin Panel

Vue 3 + TypeScript SPA for managing DocuMCP. Built with Vite, Tailwind CSS v4, and Pinia.

## Development

```bash
npm ci                 # Install dependencies
npm run dev            # Vite dev server with HMR
npm run build          # OpenAPI codegen + vue-tsc + Vite build
npm run test           # Vitest
npm run test:coverage  # Tests with coverage thresholds
npm run lint           # vue-tsc + ESLint
npm run lint:fix       # ESLint --fix + Prettier
npm run format         # Prettier write
```

## API Client

The API client is auto-generated from `docs/contracts/openapi.yaml` using `@hey-api/openapi-ts`. Run `npm run build` to regenerate after spec changes. Generated files are in `src/api/generated/` and excluded from linting.

## Project Structure

```
src/
  api/            Generated API client + wrapper
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
