<script setup lang="ts">
import { ref, computed, h } from 'vue'
import { useRouter } from 'vue-router'
import { toast } from 'vue-sonner'
import { formatDistanceToNow } from 'date-fns'
import { TrashIcon, ArrowPathIcon } from '@heroicons/vue/24/outline'
import type { ColumnDef } from '@tanstack/vue-table'

import DataTable from '../components/shared/DataTable.vue'
import EmptyState from '../components/shared/EmptyState.vue'
import ConfirmDialog from '../components/shared/ConfirmDialog.vue'
import GitTemplateCreateModal from '../components/git-templates/GitTemplateCreateModal.vue'
import { useGitTemplatesStore } from '../stores/gitTemplates'
import type { GitTemplate } from '../stores/gitTemplates'

const router = useRouter()
const store = useGitTemplatesStore()

const templates = ref<GitTemplate[]>([])
const total = ref(0)
const loading = ref(false)

const showCreateModal = ref(false)
const deleteTarget = ref<GitTemplate | null>(null)
const showDeleteDialog = computed(() => deleteTarget.value !== null)
const syncingUuids = ref<Set<string>>(new Set())

async function fetchTemplates(): Promise<void> {
  loading.value = true
  try {
    const response = await store.fetchTemplates()
    templates.value = response.data
    total.value = response.meta.total
  } catch {
    toast.error('Failed to load git templates')
  } finally {
    loading.value = false
  }
}

fetchTemplates()

function handleRowClick(row: GitTemplate): void {
  router.push(`/git-templates/${row.uuid}/files`)
}

async function handleSync(template: GitTemplate): Promise<void> {
  syncingUuids.value.add(template.uuid)
  try {
    await store.syncTemplate(template.uuid)
    toast.success(`Sync started for "${template.name}"`)
    await fetchTemplates()
  } catch {
    toast.error(`Failed to sync "${template.name}"`)
  } finally {
    syncingUuids.value.delete(template.uuid)
  }
}

async function handleDeleteConfirm(): Promise<void> {
  if (deleteTarget.value === null) {
    return
  }
  const name = deleteTarget.value.name
  try {
    await store.deleteTemplate(deleteTarget.value.uuid)
    toast.success(`Template "${name}" deleted`)
    deleteTarget.value = null
    await fetchTemplates()
  } catch {
    toast.error(`Failed to delete "${name}"`)
  }
}

function handleDeleteCancel(): void {
  deleteTarget.value = null
}

function handleCreateClose(): void {
  showCreateModal.value = false
}

function handleCreateSaved(): void {
  showCreateModal.value = false
  fetchTemplates()
}

function truncate(value: string, maxLength: number): string {
  if (value.length <= maxLength) {
    return value
  }
  return `${value.slice(0, maxLength)}...`
}

function categoryBadgeClasses(category: string): string {
  const styles: Readonly<Record<string, string>> = {
    claude: 'bg-violet-100 text-violet-800 dark:bg-violet-900/30 dark:text-violet-300',
    'memory-bank': 'bg-amber-100 text-amber-800 dark:bg-amber-900/30 dark:text-amber-300',
    project: 'bg-emerald-100 text-emerald-800 dark:bg-emerald-900/30 dark:text-emerald-300',
  }
  return styles[category] ?? 'bg-gray-100 text-gray-800 dark:bg-gray-800 dark:text-gray-300'
}

const columns: ColumnDef<GitTemplate, unknown>[] = [
  {
    accessorKey: 'name',
    header: 'Name',
    enableSorting: true,
  },
  {
    accessorKey: 'category',
    header: 'Category',
    enableSorting: true,
    cell: ({ getValue }) => {
      const value = getValue<string | undefined>()
      if (value === undefined || value === '') {
        return '-'
      }
      return h(
        'span',
        {
          class: [
            'inline-flex items-center rounded-full px-2.5 py-0.5 text-xs font-medium capitalize',
            categoryBadgeClasses(value),
          ],
        },
        value,
      )
    },
  },
  {
    accessorKey: 'repository_url',
    header: 'Repository',
    enableSorting: false,
    cell: ({ getValue }) => {
      const value = getValue<string>()
      return h(
        'span',
        { class: 'font-mono text-xs text-text-muted', title: value },
        truncate(value, 40),
      )
    },
  },
  {
    accessorKey: 'branch',
    header: 'Branch',
    enableSorting: false,
    cell: ({ getValue }) => {
      const value = getValue<string>()
      return h(
        'span',
        { class: 'font-mono text-xs text-text-muted' },
        value,
      )
    },
  },
  {
    accessorKey: 'last_synced_at',
    header: 'Last Sync',
    enableSorting: true,
    cell: ({ getValue }) => {
      const value = getValue<string | undefined>()
      if (value === undefined || value === '') {
        return 'Never'
      }
      return formatDistanceToNow(new Date(value), { addSuffix: true })
    },
  },
  {
    accessorKey: 'file_count',
    header: 'Files',
    enableSorting: true,
  },
  {
    id: 'actions',
    header: 'Actions',
    enableSorting: false,
    cell: ({ row }) => {
      const template = row.original
      const isSyncing = syncingUuids.value.has(template.uuid)
      return h('div', { class: 'flex items-center gap-2' }, [
        h(
          'button',
          {
            type: 'button',
            class: [
              'text-text-muted hover:text-indigo-600 dark:hover:text-indigo-400',
              isSyncing ? 'animate-spin' : '',
            ],
            title: 'Sync template',
            'aria-label': 'Sync template',
            disabled: isSyncing,
            onClick: (event: MouseEvent) => {
              event.stopPropagation()
              handleSync(template)
            },
          },
          [h(ArrowPathIcon, { class: 'h-5 w-5' })],
        ),
        h(
          'button',
          {
            type: 'button',
            class: 'text-text-muted hover:text-red-600 dark:hover:text-red-400',
            title: 'Delete template',
            'aria-label': 'Delete template',
            onClick: (event: MouseEvent) => {
              event.stopPropagation()
              deleteTarget.value = template
            },
          },
          [h(TrashIcon, { class: 'h-5 w-5' })],
        ),
      ])
    },
  },
]
</script>

<template>
  <div>
    <!-- Toolbar -->
    <div class="flex items-center gap-4 mb-4">
      <h1 class="text-2xl font-bold text-text-primary">Git Templates</h1>

      <div class="flex-1" />

      <button
        type="button"
        class="bg-indigo-600 text-white rounded-md px-4 py-2 text-sm font-medium hover:bg-indigo-500"
        @click="showCreateModal = true"
      >
        Add Template
      </button>
    </div>

    <!-- Empty State -->
    <EmptyState
      v-if="!loading && templates.length === 0"
      title="No git templates"
      description="Add your first git template to get started."
    >
      <template #action>
        <button
          type="button"
          class="bg-indigo-600 text-white rounded-md px-4 py-2 text-sm font-medium hover:bg-indigo-500"
          @click="showCreateModal = true"
        >
          Add Template
        </button>
      </template>
    </EmptyState>

    <!-- Data Table -->
    <DataTable
      v-else
      :data="templates"
      :columns="columns"
      :loading="loading"
      @row-click="handleRowClick"
    />

    <!-- Delete Confirmation Dialog -->
    <ConfirmDialog
      :open="showDeleteDialog"
      title="Delete Template"
      :message="`Are you sure you want to delete &quot;${deleteTarget?.name ?? ''}&quot;? This action cannot be undone.`"
      confirm-label="Delete"
      variant="danger"
      @confirm="handleDeleteConfirm"
      @cancel="handleDeleteCancel"
    />

    <!-- Create Modal -->
    <GitTemplateCreateModal
      :open="showCreateModal"
      @close="handleCreateClose"
      @saved="handleCreateSaved"
    />
  </div>
</template>
