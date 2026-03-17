<script setup lang="ts">
import { ref, watch, computed, h } from 'vue'
import { useRouter } from 'vue-router'
import { toast } from 'vue-sonner'
import { formatDistanceToNow } from 'date-fns'
import { EyeIcon, TrashIcon } from '@heroicons/vue/24/outline'
import type { ColumnDef } from '@tanstack/vue-table'

import { useDocumentsStore } from '../stores/documents'
import type { Document } from '../stores/documents'
import DataTable from '../components/shared/DataTable.vue'
import Pagination from '../components/shared/Pagination.vue'
import StatusBadge from '../components/shared/StatusBadge.vue'
import SearchInput from '../components/shared/SearchInput.vue'
import EmptyState from '../components/shared/EmptyState.vue'
import ConfirmDialog from '../components/shared/ConfirmDialog.vue'
import UploadModal from '../components/documents/UploadModal.vue'

const FILE_TYPE_OPTIONS = ['All', 'PDF', 'DOCX', 'XLSX', 'HTML', 'Markdown', 'Text'] as const
const STATUS_OPTIONS = ['All', 'Uploaded', 'Extracted', 'Indexed', 'Failed'] as const

const router = useRouter()
const store = useDocumentsStore()

const search = ref('')
const fileTypeFilter = ref('All')
const statusFilter = ref('All')
const page = ref(1)
const perPage = ref(10)
const showUpload = ref(false)
const deleteTarget = ref<Document | null>(null)
const showDeleteDialog = computed(() => deleteTarget.value !== null)

const hasActiveFilters = computed(
  () => search.value !== '' || fileTypeFilter.value !== 'All' || statusFilter.value !== 'All',
)

function formatFileSize(bytes: number): string {
  if (bytes < 1024) {
    return `${bytes} B`
  }
  if (bytes < 1024 * 1024) {
    return `${(bytes / 1024).toFixed(1)} KB`
  }
  return `${(bytes / (1024 * 1024)).toFixed(1)} MB`
}

const columns: ColumnDef<Document, unknown>[] = [
  {
    accessorKey: 'title',
    header: 'Title',
    enableSorting: true,
  },
  {
    accessorKey: 'file_type',
    header: 'Type',
    size: 80,
    enableSorting: true,
    cell: ({ getValue }) => {
      const value = getValue<string>()
      return value.toUpperCase()
    },
  },
  {
    accessorKey: 'status',
    header: 'Status',
    enableSorting: true,
    cell: ({ getValue }) => {
      const value = getValue<string>()
      return h(StatusBadge, { status: value })
    },
  },
  {
    accessorKey: 'file_size',
    header: 'Size',
    enableSorting: true,
    cell: ({ getValue }) => {
      const value = getValue<number>()
      return formatFileSize(value)
    },
  },
  {
    accessorKey: 'created_at',
    header: 'Created',
    enableSorting: true,
    cell: ({ getValue }) => {
      const value = getValue<string>()
      return formatDistanceToNow(new Date(value), { addSuffix: true })
    },
  },
  {
    id: 'actions',
    header: 'Actions',
    enableSorting: false,
    cell: ({ row }) => {
      return h('div', { class: 'flex items-center gap-2' }, [
        h(
          'button',
          {
            type: 'button',
            class: 'text-text-muted hover:text-indigo-600 dark:hover:text-indigo-400',
            title: 'View document',
            'aria-label': 'View document',
            onClick: (event: MouseEvent) => {
              event.stopPropagation()
              router.push(`/documents/${row.original.uuid}`)
            },
          },
          [h(EyeIcon, { class: 'h-5 w-5' })],
        ),
        h(
          'button',
          {
            type: 'button',
            class: 'text-text-muted hover:text-red-600 dark:hover:text-red-400',
            title: 'Delete document',
            'aria-label': 'Delete document',
            onClick: (event: MouseEvent) => {
              event.stopPropagation()
              deleteTarget.value = row.original
            },
          },
          [h(TrashIcon, { class: 'h-5 w-5' })],
        ),
      ])
    },
  },
]

function fetchData(): void {
  const offset = (page.value - 1) * perPage.value
  store.fetchDocuments({
    limit: perPage.value,
    offset,
    q: search.value || undefined,
    file_type: fileTypeFilter.value !== 'All' ? fileTypeFilter.value.toLowerCase() : undefined,
    status: statusFilter.value !== 'All' ? statusFilter.value.toLowerCase() : undefined,
  })
}

watch([search, fileTypeFilter, statusFilter], () => {
  page.value = 1
  fetchData()
}, { immediate: true })

watch([page, perPage], () => {
  fetchData()
})

function handleRowClick(row: Document): void {
  router.push(`/documents/${row.uuid}`)
}

async function handleDeleteConfirm(): Promise<void> {
  if (deleteTarget.value === null) {
    return
  }
  const title = deleteTarget.value.title
  try {
    await store.deleteDocument(deleteTarget.value.uuid)
    toast.success(`"${title}" moved to trash`)
    deleteTarget.value = null
    fetchData()
  } catch {
    toast.error(`Failed to delete "${title}"`)
  }
}

function handleDeleteCancel(): void {
  deleteTarget.value = null
}
</script>

<template>
  <div>
    <!-- Toolbar -->
    <div class="flex items-center gap-4 mb-4">
      <h1 class="text-2xl font-bold text-text-primary">Documents</h1>

      <SearchInput
        v-model="search"
        placeholder="Search documents..."
        class="flex-1 max-w-sm"
      />

      <select
        v-model="fileTypeFilter"
        class="rounded-md border border-border-input bg-bg-surface py-1.5 pl-3 pr-8 text-sm text-text-secondary focus:border-indigo-500 focus:outline-none focus:ring-1 focus:ring-indigo-500 dark:focus:border-indigo-400 dark:focus:ring-indigo-400"
      >
        <option v-for="opt in FILE_TYPE_OPTIONS" :key="opt" :value="opt">
          {{ opt === 'All' ? 'All Types' : opt }}
        </option>
      </select>

      <select
        v-model="statusFilter"
        class="rounded-md border border-border-input bg-bg-surface py-1.5 pl-3 pr-8 text-sm text-text-secondary focus:border-indigo-500 focus:outline-none focus:ring-1 focus:ring-indigo-500 dark:focus:border-indigo-400 dark:focus:ring-indigo-400"
      >
        <option v-for="opt in STATUS_OPTIONS" :key="opt" :value="opt">
          {{ opt === 'All' ? 'All Statuses' : opt }}
        </option>
      </select>

      <RouterLink
        to="/documents/trash"
        class="text-text-muted hover:text-text-secondary"
        title="Trash"
      >
        <TrashIcon class="h-5 w-5" />
      </RouterLink>

      <button
        type="button"
        class="bg-indigo-600 text-white rounded-md px-4 py-2 text-sm font-medium hover:bg-indigo-500"
        @click="showUpload = true"
      >
        Upload
      </button>
    </div>

    <!-- Empty State -->
    <EmptyState
      v-if="!store.loading && store.documents.length === 0 && !hasActiveFilters"
      title="No documents yet"
      description="Upload your first document to get started."
    >
      <template #action>
        <button
          type="button"
          class="bg-indigo-600 text-white rounded-md px-4 py-2 text-sm font-medium hover:bg-indigo-500"
          @click="showUpload = true"
        >
          Upload Document
        </button>
      </template>
    </EmptyState>

    <!-- Data Table -->
    <template v-else>
      <DataTable
        :data="store.documents"
        :columns="columns"
        :loading="store.loading"
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

    <!-- Delete Confirmation Dialog -->
    <ConfirmDialog
      :open="showDeleteDialog"
      title="Delete Document"
      :message="`Are you sure you want to delete &quot;${deleteTarget?.title ?? ''}&quot;? It will be moved to trash.`"
      confirm-label="Delete"
      variant="danger"
      @confirm="handleDeleteConfirm"
      @cancel="handleDeleteCancel"
    />

    <!-- Upload Modal -->
    <UploadModal
      :open="showUpload"
      @close="showUpload = false"
      @uploaded="showUpload = false; fetchData()"
    />
  </div>
</template>
