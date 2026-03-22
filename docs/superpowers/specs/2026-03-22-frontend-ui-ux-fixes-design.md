# Frontend UI/UX Fixes — Design Spec

**Date**: 2026-03-22
**Status**: Approved
**Scope**: Vue 3 SPA (`frontend/src/`)

---

## Overview

A single comprehensive pass fixing five categories of issues identified in a UI/UX audit:
layout restructure, logo placement, pagination bugs and polish, cursor consistency, and
targeted accessibility fixes.

---

## 1. Layout Restructure

### Problem
- `AppHeader` sits inside the `lg:pl-64` content div → only as wide as the content column
- `AppSidebar` uses `inset-y-0` → spans full viewport height, overlapping behind the header
- App name/branding lives in the sidebar, not the top bar

### Solution

**`AppLayout.vue`**
- Move `<AppHeader />` outside the `lg:pl-64` wrapper so it spans full width
- Add `pt-16` to the content wrapper to clear the fixed header
- Structure:
  ```
  <div class="min-h-screen bg-bg-page">
    <AppHeader />                       ← full width, above everything
    <AppSidebar />
    <div class="lg:pl-64 pt-16">
      <main>...</main>
    </div>
  </div>
  ```

**`AppHeader.vue`**
- Change from `sticky` to `fixed top-0 left-0 right-0 z-20 h-16`
- Left side: logo + "DocuMCP" wordmark
- Right side: SSE indicator, theme toggle, username, logout button
- Logo: `<img src="/admin/logo-concept-1-transparent.svg" alt="" aria-hidden="true" class="h-8 w-8 shrink-0" />`
- Wordmark: `<span class="text-lg font-semibold text-text-primary">DocuMCP</span>`

**`AppSidebar.vue`**
- Change `inset-y-0` to `top-16 bottom-0` so sidebar starts below the header
- Remove the `h-16` branding block (`<div class="flex items-center h-16 px-6 border-b ...">`) entirely — branding is now in the header
- Nav items start from the top of the sidebar with `py-4`

---

## 2. Pagination Fixes

### 2a. ZIM Archive offset bug

**Problem**: `ZimArchiveListView.fetchData()` passes `per_page` to the store but never passes
`page`/`offset`, so the API always returns page 1 capped at `perPage`. Changing page has no
effect.

**Fix**:
- Add `offset?: number` to `ListParams` interface in `stores/zimArchives.ts`
- Pass `offset: params?.offset` to `buildQuery` in `fetchArchives()`
- In `ZimArchiveListView.fetchData()`, pass `offset: (page.value - 1) * perPage.value`

### 2b. Per-page select hidden on small tables

**Problem**: The per-page dropdown appears on tables with 1–2 rows where it is meaningless.

**Fix**: In `Pagination.vue`, add:
```ts
const MIN_PAGE_SIZE = PAGE_SIZE_OPTIONS[0] // 10
const showPerPage = computed(() => props.total > MIN_PAGE_SIZE)
```
Wrap the per-page label + select in `v-if="showPerPage"`.

### 2c. Duplicate `id="page-size"` (a11y)

**Problem**: `Pagination.vue` hardcodes `id="page-size"` on the select and `for="page-size"` on
the label. If two paginated tables exist on the same page, IDs duplicate, breaking label
association for screen readers.

**Fix**: Use Vue 3.5's `useId()`:
```ts
const pageSizeId = useId()
```
Bind `:id="pageSizeId"` on the select, `:for="pageSizeId"` on the label.

---

## 3. Cursor Consistency

**Problem**: Interactive elements inconsistently declare `cursor-pointer`. `ThemeToggle`'s button
has no pointer cursor. `DataTable.vue` currently hardcodes `cursor-pointer` on every `<tr>`
unconditionally — including tables with no row-click navigation (e.g. OAuth clients, trash).

**Fix**: Add `cursor-pointer` explicitly to:

| File | Element |
|------|---------|
| `ThemeToggle.vue` | `<button>` |
| `AppHeader.vue` | logout `<button>` |
| `AppSidebar.vue` | all `<router-link>` elements (4 nav groups, same class string) |
| `OAuthClientListView.vue` | revoke `h(...)` button in actions column |
| `DataTable.vue` | `<tr>` rows only when `clickable` prop is true |

**`DataTable.vue` clickable prop**:
- Add `readonly clickable?: boolean` to `defineProps`
- **Remove** `cursor-pointer` from the static `class` string on `<tr>`
- Apply `cursor-pointer` only via `:class="{ 'cursor-pointer': clickable }"`
- Views with row-click navigation pass `:clickable="true"`: `DocumentListView`,
  `ZimArchiveListView`, `GitTemplateListView`, `UserListView`, `ExternalServiceListView`
- Views without row-click navigation (`DocumentTrashView`, `OAuthClientListView`) do **not** pass
  `:clickable="true"`

---

## 4. Accessibility Fixes

### 4a. ZIM filter selects missing labels

**Problem**: The Category and Language `<select>` elements in `ZimArchiveListView.vue` have no
`<label>`, relying only on positional context.

**Fix**: Add `sr-only` labels with matching `for`/`id` pairs:
```html
<label for="zim-category-filter" class="sr-only">Category</label>
<select id="zim-category-filter" v-model="categoryFilter" ...>

<label for="zim-language-filter" class="sr-only">Language</label>
<select id="zim-language-filter" v-model="languageFilter" ...>
```

### 4b. Action button aria-labels missing row context

**Problem**: The revoke button in `OAuthClientListView.vue` has `aria-label="Revoke client"` on
every row. Screen readers cannot distinguish between rows.

**Fix**: Include client name in the label:
```ts
'aria-label': `Revoke client ${client.client_name}`,
```

---

## Files Changed

| File | Change |
|------|--------|
| `frontend/src/components/layout/AppLayout.vue` | Move header outside pl-64, add pt-16 to content |
| `frontend/src/components/layout/AppHeader.vue` | fixed full-width, logo + wordmark on left |
| `frontend/src/components/layout/AppSidebar.vue` | top-16 offset, remove branding block |
| `frontend/src/components/layout/ThemeToggle.vue` | add cursor-pointer |
| `frontend/src/components/shared/Pagination.vue` | showPerPage computed, useId() for label |
| `frontend/src/components/shared/DataTable.vue` | clickable prop → cursor-pointer on tr |
| `frontend/src/stores/zimArchives.ts` | add offset to ListParams |
| `frontend/src/views/ZimArchiveListView.vue` | pass offset in fetchData, :clickable="true" |
| `frontend/src/views/DocumentListView.vue` | `:clickable="true"` |
| `frontend/src/views/GitTemplateListView.vue` | `:clickable="true"` |
| `frontend/src/views/UserListView.vue` | `:clickable="true"` |
| `frontend/src/views/ExternalServiceListView.vue` | `:clickable="true"` |
| `frontend/src/views/OAuthClientListView.vue` | cursor-pointer + contextual aria-label on revoke button only (no `:clickable`) |

---

## Out of Scope

- Comprehensive a11y sweep (planned follow-up session)
- Mobile/responsive sidebar (no hamburger menu yet)
- Logo appearance in light mode (near-white fill may be hard to see; tracked as known issue)
