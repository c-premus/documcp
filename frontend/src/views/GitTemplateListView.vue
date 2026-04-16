<script setup lang="ts">
import { ref, computed, watch, h } from 'vue'
import { useRouter } from 'vue-router'
import { toast } from 'vue-sonner'
import type { ColumnDef } from '@tanstack/vue-table'

import DataTable from '../components/shared/DataTable.vue'
import Pagination from '../components/shared/Pagination.vue'
import EmptyState from '../components/shared/EmptyState.vue'
import ConfirmDialog from '../components/shared/ConfirmDialog.vue'
import CategoryBadge from '../components/shared/CategoryBadge.vue'
import TruncatedText from '../components/shared/TruncatedText.vue'
import RelativeTimeCell from '../components/shared/RelativeTimeCell.vue'
import GitTemplateCreateModal from '../components/git-templates/GitTemplateCreateModal.vue'
import GitTemplateEditModal from '../components/git-templates/GitTemplateEditModal.vue'
import GitTemplateRowActions from '../components/git-templates/GitTemplateRowActions.vue'
import { useAuthStore } from '@/stores/auth'
import { useGitTemplatesStore } from '../stores/gitTemplates'
import type { GitTemplate } from '../stores/gitTemplates'

const CATEGORY_PALETTE: Readonly<Record<string, string>> = {
  claude: 'bg-violet-100 text-violet-800 dark:bg-violet-900/30 dark:text-violet-300',
  'memory-bank': 'bg-amber-100 text-amber-800 dark:bg-amber-900/30 dark:text-amber-300',
  project: 'bg-emerald-100 text-emerald-800 dark:bg-emerald-900/30 dark:text-emerald-300',
}

const router = useRouter()
const auth = useAuthStore()
const store = useGitTemplatesStore()

const page = ref(1)
const perPage = ref(50)

const showCreateModal = ref(false)
const deleteTarget = ref<GitTemplate | null>(null)
const showDeleteDialog = computed(() => deleteTarget.value !== null)
const editTarget = ref<GitTemplate | null>(null)
const showEditModal = computed(() => editTarget.value !== null)
const syncingUuids = ref<Set<string>>(new Set())

function fetchData(): void {
  store
    .fetchTemplates({
      per_page: perPage.value,
      offset: (page.value - 1) * perPage.value,
    })
    .catch(() => {
      toast.error('Failed to load git templates')
    })
}

watch(
  [page, perPage],
  () => {
    fetchData()
  },
  { immediate: true },
)

function handleRowClick(row: GitTemplate): void {
  router.push(`/git-templates/${row.uuid}/files`)
}

async function handleSync(template: GitTemplate): Promise<void> {
  syncingUuids.value.add(template.uuid)
  try {
    await store.syncTemplate(template.uuid)
    toast.success(`Sync started for "${template.name}"`)
    await fetchData()
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
    await fetchData()
  } catch {
    toast.error(`Failed to delete "${name}"`)
  }
}

function handleDeleteCancel(): void {
  deleteTarget.value = null
}

function handleEditClose(): void {
  editTarget.value = null
}

function handleEditSaved(): void {
  editTarget.value = null
  fetchData()
}

function handleCreateClose(): void {
  showCreateModal.value = false
}

function handleCreateSaved(): void {
  showCreateModal.value = false
  fetchData()
}

const baseColumns: ColumnDef<GitTemplate, unknown>[] = [
  {
    accessorKey: 'name',
    header: 'Name',
    enableSorting: true,
  },
  {
    accessorKey: 'category',
    header: 'Category',
    enableSorting: true,
    meta: { className: 'w-28 hidden sm:table-cell' },
    cell: ({ getValue }) => {
      const value = getValue<string | undefined>()
      if (value === undefined || value === '') {
        return '-'
      }
      return h(CategoryBadge, { value, palette: CATEGORY_PALETTE })
    },
  },
  {
    accessorKey: 'repository_url',
    header: 'Repository',
    enableSorting: false,
    meta: { className: 'hidden lg:table-cell' },
    cell: ({ getValue }) => h(TruncatedText, { value: getValue<string>(), mono: true }),
  },
  {
    accessorKey: 'branch',
    header: 'Branch',
    enableSorting: false,
    meta: { className: 'w-24 hidden md:table-cell' },
    cell: ({ getValue }) =>
      h('span', { class: 'font-mono text-xs text-text-muted' }, getValue<string>()),
  },
  {
    accessorKey: 'last_synced_at',
    header: 'Last Sync',
    enableSorting: true,
    meta: { className: 'w-36 hidden md:table-cell' },
    cell: ({ getValue }) =>
      h(RelativeTimeCell, { value: getValue<string | undefined>() ?? null, fallback: 'Never' }),
  },
  {
    accessorKey: 'file_count',
    header: 'Files',
    enableSorting: true,
    meta: { className: 'w-16 hidden sm:table-cell' },
  },
]

const actionsColumn: ColumnDef<GitTemplate, unknown> = {
  id: 'actions',
  header: 'Actions',
  enableSorting: false,
  meta: { className: 'w-20' },
  cell: ({ row }) =>
    h(GitTemplateRowActions, {
      template: row.original,
      syncing: syncingUuids.value.has(row.original.uuid),
      onEdit: (template: GitTemplate) => {
        editTarget.value = template
      },
      onSync: handleSync,
      onDelete: (template: GitTemplate) => {
        deleteTarget.value = template
      },
    }),
}

const columns = computed(() => (auth.isAdmin ? [...baseColumns, actionsColumn] : baseColumns))
</script>

<template>
  <div>
    <!-- Toolbar -->
    <div class="flex flex-wrap items-center gap-2 sm:gap-4 mb-4">
      <h1 class="text-2xl font-bold text-text-primary">Git Templates</h1>

      <div class="flex-1" />

      <button
        v-if="auth.isAdmin"
        type="button"
        class="bg-indigo-600 text-white rounded-md px-4 py-2 text-sm font-medium hover:bg-indigo-500"
        @click="showCreateModal = true"
      >
        Add Template
      </button>
    </div>

    <!-- Empty State -->
    <EmptyState
      v-if="!store.loading && store.templates.length === 0"
      :title="auth.isAdmin ? 'No git templates' : 'No git templates available'"
      :description="
        auth.isAdmin
          ? 'Add your first git template to get started.'
          : 'There are no git templates configured yet.'
      "
    >
      <template v-if="auth.isAdmin" #action>
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
    <template v-else>
      <DataTable
        :data="store.templates"
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

    <!-- Edit Modal -->
    <GitTemplateEditModal
      :open="showEditModal"
      :template="editTarget"
      @close="handleEditClose"
      @saved="handleEditSaved"
    />

    <!-- Create Modal -->
    <GitTemplateCreateModal
      :open="showCreateModal"
      @close="handleCreateClose"
      @saved="handleCreateSaved"
    />
  </div>
</template>
