<script setup lang="ts">
import { ref, onMounted, onUnmounted, h, computed } from 'vue'
import { toast } from 'vue-sonner'
import { formatDistanceToNow } from 'date-fns'
import { ArrowPathIcon, TrashIcon } from '@heroicons/vue/24/outline'
import { TabGroup, TabList, Tab, TabPanels, TabPanel } from '@headlessui/vue'
import type { ColumnDef } from '@tanstack/vue-table'

import { useQueueStore } from '../stores/queue'
import type { FailedJob } from '../stores/queue'
import DataTable from '../components/shared/DataTable.vue'
import StatusBadge from '../components/shared/StatusBadge.vue'
import EmptyState from '../components/shared/EmptyState.vue'
import ConfirmDialog from '../components/shared/ConfirmDialog.vue'

const store = useQueueStore()

const deleteTarget = ref<FailedJob | null>(null)
const showDeleteDialog = computed(() => deleteTarget.value !== null)

let refreshInterval: ReturnType<typeof setInterval> | null = null

onMounted(async () => {
  await Promise.all([store.fetchStats(), store.fetchFailedJobs()])
  refreshInterval = setInterval(() => {
    store.fetchStats()
  }, 30000)
})

onUnmounted(() => {
  if (refreshInterval !== null) {
    clearInterval(refreshInterval)
  }
})

async function handleRetry(job: FailedJob): Promise<void> {
  try {
    await store.retryJob(job.id)
    toast.success(`Job ${job.kind} #${job.id} queued for retry`)
    await store.fetchFailedJobs()
  } catch {
    toast.error(`Failed to retry job #${job.id}`)
  }
}

async function handleDeleteConfirm(): Promise<void> {
  if (deleteTarget.value === null) {
    return
  }
  const id = deleteTarget.value.id
  try {
    await store.deleteJob(id)
    toast.success(`Job #${id} deleted`)
    deleteTarget.value = null
    await store.fetchFailedJobs()
  } catch {
    toast.error(`Failed to delete job #${id}`)
  }
}

function handleDeleteCancel(): void {
  deleteTarget.value = null
}

function truncateError(job: FailedJob): string {
  if (!job.errors || job.errors.length === 0) {
    return ''
  }
  const lastError = job.errors[job.errors.length - 1]
  if (!lastError) {
    return ''
  }
  const msg = lastError.error
  if (msg.length > 80) {
    return `${msg.slice(0, 80)}...`
  }
  return msg
}

const columns: ColumnDef<FailedJob, unknown>[] = [
  {
    accessorKey: 'id',
    header: 'ID',
    size: 60,
  },
  {
    accessorKey: 'kind',
    header: 'Job Kind',
  },
  {
    accessorKey: 'queue',
    header: 'Queue',
    size: 80,
  },
  {
    accessorKey: 'state',
    header: 'State',
    cell: ({ getValue }) => {
      const value = getValue<string>()
      return h(StatusBadge, { status: value })
    },
  },
  {
    accessorKey: 'attempt',
    header: 'Attempts',
    size: 80,
    cell: ({ row }) => {
      return `${row.original.attempt}/${row.original.max_attempts}`
    },
  },
  {
    id: 'error',
    header: 'Error',
    cell: ({ row }) => {
      const msg = truncateError(row.original)
      return h('span', { class: 'text-red-600 dark:text-red-400 text-xs', title: msg }, msg)
    },
  },
  {
    accessorKey: 'created_at',
    header: 'Created',
    cell: ({ getValue }) => {
      const value = getValue<string>()
      return formatDistanceToNow(new Date(value), { addSuffix: true })
    },
  },
  {
    id: 'actions',
    header: 'Actions',
    cell: ({ row }) => {
      return h('div', { class: 'flex items-center gap-2' }, [
        h(
          'button',
          {
            type: 'button',
            class: 'text-text-muted hover:text-indigo-600 dark:hover:text-indigo-400',
            title: 'Retry job',
            'aria-label': 'Retry job',
            onClick: (event: MouseEvent) => {
              event.stopPropagation()
              handleRetry(row.original)
            },
          },
          [h(ArrowPathIcon, { class: 'h-5 w-5' })],
        ),
        h(
          'button',
          {
            type: 'button',
            class: 'text-text-muted hover:text-red-600 dark:hover:text-red-400',
            title: 'Delete job',
            'aria-label': 'Delete job',
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
</script>

<template>
  <div>
    <h1 class="text-2xl font-bold text-text-primary mb-6">Queue Management</h1>

    <TabGroup>
      <TabList class="flex space-x-1 rounded-xl bg-bg-active p-1 mb-6">
        <Tab
          v-slot="{ selected }"
          as="template"
        >
          <button
            :class="[
              'w-full rounded-lg py-2.5 text-sm font-medium leading-5',
              selected
                ? 'bg-bg-surface text-indigo-700 dark:text-indigo-300 shadow'
                : 'text-text-muted hover:bg-bg-surface/50 hover:text-text-primary',
            ]"
          >
            Stats
          </button>
        </Tab>
        <Tab
          v-slot="{ selected }"
          as="template"
        >
          <button
            :class="[
              'w-full rounded-lg py-2.5 text-sm font-medium leading-5',
              selected
                ? 'bg-bg-surface text-indigo-700 dark:text-indigo-300 shadow'
                : 'text-text-muted hover:bg-bg-surface/50 hover:text-text-primary',
            ]"
          >
            Failed Jobs ({{ store.failedCount }})
          </button>
        </Tab>
      </TabList>

      <TabPanels>
        <!-- Stats Panel -->
        <TabPanel>
          <div v-if="store.stats" class="grid grid-cols-2 gap-4 sm:grid-cols-3 lg:grid-cols-5">
            <div class="rounded-lg bg-bg-surface p-4 shadow ring-1 ring-black/5 dark:ring-white/10">
              <dt class="text-sm font-medium text-text-muted">Available</dt>
              <dd class="mt-1 text-2xl font-semibold text-text-primary">{{ store.stats.available }}</dd>
            </div>
            <div class="rounded-lg bg-bg-surface p-4 shadow ring-1 ring-black/5 dark:ring-white/10">
              <dt class="text-sm font-medium text-text-muted">Running</dt>
              <dd class="mt-1 text-2xl font-semibold text-indigo-600 dark:text-indigo-400">{{ store.stats.running }}</dd>
            </div>
            <div class="rounded-lg bg-bg-surface p-4 shadow ring-1 ring-black/5 dark:ring-white/10">
              <dt class="text-sm font-medium text-text-muted">Retryable</dt>
              <dd class="mt-1 text-2xl font-semibold text-yellow-600 dark:text-yellow-400">{{ store.stats.retryable }}</dd>
            </div>
            <div class="rounded-lg bg-bg-surface p-4 shadow ring-1 ring-black/5 dark:ring-white/10">
              <dt class="text-sm font-medium text-text-muted">Discarded</dt>
              <dd class="mt-1 text-2xl font-semibold text-red-600 dark:text-red-400">{{ store.stats.discarded }}</dd>
            </div>
            <div class="rounded-lg bg-bg-surface p-4 shadow ring-1 ring-black/5 dark:ring-white/10">
              <dt class="text-sm font-medium text-text-muted">Cancelled</dt>
              <dd class="mt-1 text-2xl font-semibold text-text-muted">{{ store.stats.cancelled }}</dd>
            </div>
          </div>
          <div v-else class="flex items-center justify-center py-12">
            <div class="h-8 w-8 animate-spin rounded-full border-4 border-border-input border-t-indigo-600 dark:border-t-indigo-400" />
          </div>
        </TabPanel>

        <!-- Failed Jobs Panel -->
        <TabPanel>
          <EmptyState
            v-if="!store.loading && store.failedJobs.length === 0"
            title="No failed jobs"
            description="All jobs are running smoothly."
          />

          <DataTable
            v-else
            :data="store.failedJobs"
            :columns="columns"
            :loading="store.loading"
          />
        </TabPanel>
      </TabPanels>
    </TabGroup>

    <!-- Delete Confirmation -->
    <ConfirmDialog
      :open="showDeleteDialog"
      title="Delete Job"
      :message="`Are you sure you want to delete job #${deleteTarget?.id ?? ''}? This action cannot be undone.`"
      confirm-label="Delete"
      variant="danger"
      @confirm="handleDeleteConfirm"
      @cancel="handleDeleteCancel"
    />
  </div>
</template>
