<script setup lang="ts">
import { ref, watch, computed, h } from 'vue'
import { useRouter } from 'vue-router'
import { toast } from 'vue-sonner'
import { formatDistanceToNow } from 'date-fns'
import type { ColumnDef } from '@tanstack/vue-table'

import { useZimArchivesStore } from '../stores/zimArchives'
import type { ZimArchive } from '../stores/zimArchives'
import DataTable from '../components/shared/DataTable.vue'
import Pagination from '../components/shared/Pagination.vue'
import StatusBadge from '../components/shared/StatusBadge.vue'
import SearchInput from '../components/shared/SearchInput.vue'
import EmptyState from '../components/shared/EmptyState.vue'

const CATEGORY_OPTIONS = ['All', 'devdocs', 'wikipedia', 'stack_exchange', 'other'] as const
const LANGUAGE_OPTIONS = ['All', 'en', 'es', 'fr', 'de', 'zh', 'ja', 'pt', 'ru'] as const

const router = useRouter()
const store = useZimArchivesStore()

const search = ref('')
const categoryFilter = ref('All')
const languageFilter = ref('All')
const page = ref(1)
const perPage = ref(50)

const hasActiveFilters = computed(
  () => search.value !== '' || categoryFilter.value !== 'All' || languageFilter.value !== 'All',
)

const columns: ColumnDef<ZimArchive, unknown>[] = [
  {
    accessorKey: 'name',
    header: 'Name',
    enableSorting: true,
    cell: ({ getValue }) => {
      const value = getValue<string>()
      return h(
        'span',
        {
          class:
            'text-indigo-600 dark:text-indigo-400 font-medium hover:text-indigo-800 dark:hover:text-indigo-300',
        },
        value,
      )
    },
  },
  {
    accessorKey: 'title',
    header: 'Title',
    enableSorting: true,
    meta: { className: 'hidden lg:table-cell' },
  },
  {
    accessorKey: 'category',
    header: 'Category',
    enableSorting: true,
    meta: { className: 'w-28' },
    cell: ({ getValue }) => {
      const value = getValue<string | undefined>()
      if (!value) {
        return ''
      }
      return h(StatusBadge, { status: value })
    },
  },
  {
    accessorKey: 'language',
    header: 'Language',
    enableSorting: true,
    meta: { className: 'w-16 hidden sm:table-cell' },
    cell: ({ getValue }) => {
      const value = getValue<string>()
      return value.toUpperCase()
    },
  },
  {
    accessorKey: 'has_fulltext_index',
    header: 'Search',
    enableSorting: true,
    meta: { className: 'w-20 hidden md:table-cell' },
    cell: ({ getValue }) => {
      const value = getValue<boolean>()
      return h(StatusBadge, { status: value ? 'fulltext' : 'title only' })
    },
  },
  {
    accessorKey: 'article_count',
    header: 'Articles',
    enableSorting: true,
    meta: { className: 'w-24 hidden md:table-cell' },
    cell: ({ getValue }) => {
      const value = getValue<number>()
      return value.toLocaleString()
    },
  },
  {
    accessorKey: 'file_size_human',
    header: 'Size',
    enableSorting: false,
    meta: { className: 'w-24 hidden md:table-cell' },
  },
  {
    accessorKey: 'last_synced_at',
    header: 'Last Synced',
    enableSorting: true,
    meta: { className: 'w-36 hidden md:table-cell' },
    cell: ({ getValue }) => {
      const value = getValue<string | undefined>()
      if (!value) {
        return 'Never'
      }
      return formatDistanceToNow(new Date(value), { addSuffix: true })
    },
  },
]

function fetchData(): void {
  store
    .fetchArchives({
      query: search.value || undefined,
      category: categoryFilter.value !== 'All' ? categoryFilter.value : undefined,
      language: languageFilter.value !== 'All' ? languageFilter.value : undefined,
      per_page: perPage.value,
      offset: (page.value - 1) * perPage.value,
    })
    .catch(() => {
      toast.error('Failed to load ZIM archives')
    })
}

watch(
  [search, categoryFilter, languageFilter],
  () => {
    page.value = 1
    fetchData()
  },
  { immediate: true },
)

watch([page, perPage], () => {
  fetchData()
})

function handleRowClick(row: ZimArchive): void {
  router.push(`/zim-archives/${row.name}`)
}
</script>

<template>
  <div>
    <!-- Toolbar -->
    <div class="flex flex-wrap items-center gap-2 sm:gap-4 mb-4">
      <h1 class="text-2xl font-bold text-text-primary">ZIM Archives</h1>

      <SearchInput
        v-model="search"
        placeholder="Search archives..."
        class="w-full sm:w-auto sm:max-w-sm"
      />

      <label for="zim-category-filter" class="sr-only">Category</label>
      <select
        id="zim-category-filter"
        v-model="categoryFilter"
        class="rounded-md border border-border-input bg-bg-surface py-1.5 pl-3 pr-8 text-sm text-text-secondary focus:border-indigo-500 focus:outline-none focus:ring-1 focus:ring-indigo-500 dark:focus:border-indigo-400 dark:focus:ring-indigo-400"
      >
        <option v-for="opt in CATEGORY_OPTIONS" :key="opt" :value="opt">
          {{ opt === 'All' ? 'All Categories' : opt }}
        </option>
      </select>

      <label for="zim-language-filter" class="sr-only">Language</label>
      <select
        id="zim-language-filter"
        v-model="languageFilter"
        class="rounded-md border border-border-input bg-bg-surface py-1.5 pl-3 pr-8 text-sm text-text-secondary focus:border-indigo-500 focus:outline-none focus:ring-1 focus:ring-indigo-500 dark:focus:border-indigo-400 dark:focus:ring-indigo-400"
      >
        <option v-for="opt in LANGUAGE_OPTIONS" :key="opt" :value="opt">
          {{ opt === 'All' ? 'All Languages' : opt.toUpperCase() }}
        </option>
      </select>
    </div>

    <!-- Empty State -->
    <EmptyState
      v-if="!store.loading && store.archives.length === 0 && !hasActiveFilters"
      title="No ZIM archives"
      description="No ZIM archives have been configured yet."
    />

    <!-- No Results -->
    <EmptyState
      v-else-if="!store.loading && store.archives.length === 0 && hasActiveFilters"
      title="No matching archives"
      description="Try adjusting your search or filters."
    />

    <!-- Data Table -->
    <template v-else>
      <DataTable
        :data="store.archives"
        :columns="columns"
        :loading="store.loading"
        :clickable="true"
        @row-click="handleRowClick"
      />

      <Pagination
        :page="page"
        :per-page="perPage"
        :total="store.total"
        @update:page="page = $event"
        @update:per-page="perPage = $event"
      />
    </template>
  </div>
</template>
