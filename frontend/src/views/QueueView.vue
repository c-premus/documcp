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
      return h('span', { class: 'text-red-600 text-xs', title: msg }, msg)
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
            class: 'text-gray-500 hover:text-indigo-600',
            title: 'Retry job',
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
            class: 'text-gray-500 hover:text-red-600',
            title: 'Delete job',
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
    <h1 class="text-2xl font-bold text-gray-900 mb-6">Queue Management</h1>

    <TabGroup>
      <TabList class="flex space-x-1 rounded-xl bg-gray-100 p-1 mb-6">
        <Tab
          v-slot="{ selected }"
          as="template"
        >
          <button
            :class="[
              'w-full rounded-lg py-2.5 text-sm font-medium leading-5',
              selected
                ? 'bg-white text-indigo-700 shadow'
                : 'text-gray-600 hover:bg-white/[0.5] hover:text-gray-800',
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
                ? 'bg-white text-indigo-700 shadow'
                : 'text-gray-600 hover:bg-white/[0.5] hover:text-gray-800',
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
            <div class="rounded-lg bg-white p-4 shadow ring-1 ring-black/5">
              <dt class="text-sm font-medium text-gray-500">Available</dt>
              <dd class="mt-1 text-2xl font-semibold text-gray-900">{{ store.stats.available }}</dd>
            </div>
            <div class="rounded-lg bg-white p-4 shadow ring-1 ring-black/5">
              <dt class="text-sm font-medium text-gray-500">Running</dt>
              <dd class="mt-1 text-2xl font-semibold text-indigo-600">{{ store.stats.running }}</dd>
            </div>
            <div class="rounded-lg bg-white p-4 shadow ring-1 ring-black/5">
              <dt class="text-sm font-medium text-gray-500">Retryable</dt>
              <dd class="mt-1 text-2xl font-semibold text-yellow-600">{{ store.stats.retryable }}</dd>
            </div>
            <div class="rounded-lg bg-white p-4 shadow ring-1 ring-black/5">
              <dt class="text-sm font-medium text-gray-500">Discarded</dt>
              <dd class="mt-1 text-2xl font-semibold text-red-600">{{ store.stats.discarded }}</dd>
            </div>
            <div class="rounded-lg bg-white p-4 shadow ring-1 ring-black/5">
              <dt class="text-sm font-medium text-gray-500">Cancelled</dt>
              <dd class="mt-1 text-2xl font-semibold text-gray-500">{{ store.stats.cancelled }}</dd>
            </div>
          </div>
          <div v-else class="flex items-center justify-center py-12">
            <div class="h-8 w-8 animate-spin rounded-full border-4 border-gray-300 border-t-indigo-600" />
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
