<script setup lang="ts">
import { ref, watch, computed, h } from 'vue'
import { toast } from 'vue-sonner'
import { formatDistanceToNow } from 'date-fns'
import { ArrowPathIcon, TrashIcon } from '@heroicons/vue/24/outline'
import type { ColumnDef } from '@tanstack/vue-table'

import { useDocumentsStore } from '../stores/documents'
import type { Document } from '../stores/documents'
import DataTable from '../components/shared/DataTable.vue'
import Pagination from '../components/shared/Pagination.vue'
import EmptyState from '../components/shared/EmptyState.vue'
import ConfirmDialog from '../components/shared/ConfirmDialog.vue'

const store = useDocumentsStore()

const page = ref(1)
const perPage = ref(10)

const purgeTarget = ref<Document | null>(null)
const showPurgeDialog = computed(() => purgeTarget.value !== null)

const showBulkPurgeDialog = ref(false)
const bulkPurgeDays = ref(30)

const columns: ColumnDef<Document, unknown>[] = [
  {
    accessorKey: 'title',
    header: 'Title',
    enableSorting: true,
  },
  {
    accessorKey: 'file_type',
    header: 'File Type',
    size: 100,
    enableSorting: true,
    cell: ({ getValue }) => {
      const value = getValue<string>()
      return value.toUpperCase()
    },
  },
  {
    accessorKey: 'updated_at',
    header: 'Deleted At',
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
            class: 'text-text-muted hover:text-green-600 dark:hover:text-green-400',
            title: 'Restore document',
            'aria-label': 'Restore document',
            onClick: (event: MouseEvent) => {
              event.stopPropagation()
              handleRestore(row.original)
            },
          },
          [h(ArrowPathIcon, { class: 'h-5 w-5' })],
        ),
        h(
          'button',
          {
            type: 'button',
            class: 'text-text-muted hover:text-red-600 dark:hover:text-red-400',
            title: 'Permanently delete',
            'aria-label': 'Permanently delete',
            onClick: (event: MouseEvent) => {
              event.stopPropagation()
              purgeTarget.value = row.original
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
  store.fetchDeletedDocuments({
    limit: perPage.value,
    offset,
  })
}

watch(
  [page, perPage],
  () => {
    fetchData()
  },
  { immediate: true },
)

async function handleRestore(doc: Document): Promise<void> {
  try {
    await store.restoreDocument(doc.uuid)
    toast.success(`"${doc.title}" restored successfully`)
    fetchData()
  } catch {
    toast.error(`Failed to restore "${doc.title}"`)
  }
}

async function handlePurgeConfirm(): Promise<void> {
  if (purgeTarget.value === null) {
    return
  }
  const title = purgeTarget.value.title
  const uuid = purgeTarget.value.uuid
  try {
    await store.purgeDocument(uuid)
    toast.success(`"${title}" permanently deleted`)
    purgeTarget.value = null
    fetchData()
  } catch {
    toast.error(`Failed to purge "${title}"`)
  }
}

function handlePurgeCancel(): void {
  purgeTarget.value = null
}

async function handleBulkPurgeConfirm(): Promise<void> {
  try {
    const result = await store.bulkPurge(bulkPurgeDays.value)
    toast.success(`Purged ${result.count} document(s) older than ${bulkPurgeDays.value} days`)
    showBulkPurgeDialog.value = false
    fetchData()
  } catch {
    toast.error('Failed to bulk purge documents')
  }
}

function handleBulkPurgeCancel(): void {
  showBulkPurgeDialog.value = false
}
</script>

<template>
  <div>
    <!-- Header -->
    <div class="mb-4">
      <RouterLink
        to="/documents"
        class="text-sm text-indigo-600 hover:text-indigo-500 dark:text-indigo-400 dark:hover:text-indigo-300"
      >
        &larr; Documents
      </RouterLink>

      <div class="mt-2 flex items-center justify-between">
        <div>
          <h1 class="text-2xl font-bold text-text-primary">Trash</h1>
          <p class="mt-1 text-sm text-text-muted">Soft-deleted documents</p>
        </div>

        <button
          type="button"
          class="rounded-md border border-red-300 dark:border-red-700 px-4 py-2 text-sm font-medium text-red-700 dark:text-red-300 hover:bg-red-50 dark:hover:bg-red-900/20"
          @click="showBulkPurgeDialog = true"
        >
          Purge All Older Than...
        </button>
      </div>
    </div>

    <!-- Empty State -->
    <EmptyState
      v-if="!store.loading && store.documents.length === 0"
      title="Trash is empty"
      description="No soft-deleted documents found."
    >
      <template #icon>
        <TrashIcon class="mx-auto h-12 w-12 text-text-disabled" />
      </template>
    </EmptyState>

    <!-- Data Table -->
    <template v-else>
      <DataTable :data="store.documents" :columns="columns" :loading="store.loading" />

      <Pagination
        :page="page"
        :per-page="perPage"
        :total="store.total"
        @update:page="page = $event"
        @update:per-page="perPage = $event"
      />
    </template>

    <!-- Single Purge Confirmation Dialog -->
    <ConfirmDialog
      :open="showPurgeDialog"
      title="Permanently Delete Document"
      :message="`This will PERMANENTLY delete &quot;${purgeTarget?.title ?? ''}&quot;. This cannot be undone.`"
      confirm-label="Purge"
      variant="danger"
      @confirm="handlePurgeConfirm"
      @cancel="handlePurgeCancel"
    />

    <!-- Bulk Purge Dialog -->
    <ConfirmDialog
      :open="showBulkPurgeDialog"
      title="Bulk Purge Documents"
      :message="`Permanently delete all trashed documents older than the specified number of days. This cannot be undone.`"
      confirm-label="Purge All"
      variant="danger"
      @confirm="handleBulkPurgeConfirm"
      @cancel="handleBulkPurgeCancel"
    >
      <template #default>
        <div class="mt-3">
          <label for="bulk-purge-days" class="block text-sm font-medium text-text-secondary">
            Older than (days)
          </label>
          <input
            id="bulk-purge-days"
            v-model.number="bulkPurgeDays"
            type="number"
            min="1"
            class="mt-1 block w-full rounded-md border border-border-input bg-bg-surface text-text-primary px-3 py-2 text-sm shadow-sm focus:border-indigo-500 focus:outline-none focus:ring-1 focus:ring-indigo-500 dark:focus:border-indigo-400 dark:focus:ring-indigo-400"
          />
        </div>
      </template>
    </ConfirmDialog>
  </div>
</template>
