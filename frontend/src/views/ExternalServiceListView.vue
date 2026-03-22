<script setup lang="ts">
import { ref, watch, computed, h } from 'vue'
import { toast } from 'vue-sonner'
import {
  PencilSquareIcon,
  TrashIcon,
  ArrowUpIcon,
  ArrowDownIcon,
  HeartIcon,
  ArrowPathIcon,
} from '@heroicons/vue/24/outline'
import { Switch } from '@headlessui/vue'
import type { ColumnDef } from '@tanstack/vue-table'

import DataTable from '../components/shared/DataTable.vue'
import Pagination from '../components/shared/Pagination.vue'
import EmptyState from '../components/shared/EmptyState.vue'
import ConfirmDialog from '../components/shared/ConfirmDialog.vue'
import ExternalServiceModal from '../components/external-services/ExternalServiceModal.vue'

import { useExternalServicesStore } from '../stores/externalServices'
import type { ExternalService } from '../stores/externalServices'

const store = useExternalServicesStore()

const page = ref(1)
const perPage = ref(20)
const typeFilter = ref('')

const syncingUUIDs = ref<Set<string>>(new Set())

const showModal = ref(false)
const editTarget = ref<ExternalService | null>(null)
const deleteTarget = ref<ExternalService | null>(null)
const showDeleteDialog = computed(() => deleteTarget.value !== null)

async function fetchServices(): Promise<void> {
  const offset = (page.value - 1) * perPage.value
  try {
    await store.fetchServices({
      type: typeFilter.value || undefined,
      limit: perPage.value,
      offset,
    })
  } catch {
    toast.error('Failed to load external services')
  }
}

watch([typeFilter], () => {
  page.value = 1
  fetchServices()
}, { immediate: true })

watch([page, perPage], () => {
  fetchServices()
})

function openCreateModal(): void {
  editTarget.value = null
  showModal.value = true
}

function openEditModal(service: ExternalService): void {
  editTarget.value = service
  showModal.value = true
}

function handleRowClick(row: ExternalService): void {
  openEditModal(row)
}

function handleModalClose(): void {
  showModal.value = false
  editTarget.value = null
}

function handleModalSaved(): void {
  showModal.value = false
  editTarget.value = null
  fetchServices()
}

async function handleDeleteConfirm(): Promise<void> {
  if (deleteTarget.value === null) {
    return
  }
  const serviceName = deleteTarget.value.name
  try {
    await store.deleteService(deleteTarget.value.uuid)
    toast.success(`Service "${serviceName}" deleted`)
    deleteTarget.value = null
    fetchServices()
  } catch {
    toast.error(`Failed to delete "${serviceName}"`)
  }
}

function handleDeleteCancel(): void {
  deleteTarget.value = null
}

async function handleToggleEnabled(service: ExternalService): Promise<void> {
  try {
    const updated = await store.updateService(service.uuid, {
      is_enabled: !service.is_enabled,
    })
    const label = updated.is_enabled ? 'enabled' : 'disabled'
    toast.success(`Service "${service.name}" ${label}`)
    fetchServices()
  } catch {
    toast.error(`Failed to toggle "${service.name}"`)
  }
}

async function handleSync(service: ExternalService): Promise<void> {
  syncingUUIDs.value = new Set([...syncingUUIDs.value, service.uuid])
  try {
    await store.syncService(service.uuid)
    toast.success(`Sync queued for "${service.name}"`)
  } catch {
    toast.error(`Failed to queue sync for "${service.name}"`)
  } finally {
    const next = new Set(syncingUUIDs.value)
    next.delete(service.uuid)
    syncingUUIDs.value = next
  }
}

async function handleHealthCheck(service: ExternalService): Promise<void> {
  try {
    await store.checkHealth(service.uuid)
    toast.success(`Health check passed for "${service.name}"`)
    fetchServices()
  } catch (e) {
    const message = e instanceof Error ? e.message : 'Health check failed'
    if (message.includes('not yet available') || message.includes('Not Implemented')) {
      toast.info(`Health check is not yet available for "${service.name}"`)
    } else {
      toast.error(`Health check failed for "${service.name}": ${message}`)
    }
  }
}

async function handleMovePriority(service: ExternalService, direction: 'up' | 'down'): Promise<void> {
  const sorted = [...store.services].sort((a, b) => a.priority - b.priority)
  const currentIndex = sorted.findIndex((s) => s.uuid === service.uuid)
  if (currentIndex === -1) {
    return
  }

  const targetIndex = direction === 'up' ? currentIndex - 1 : currentIndex + 1
  if (targetIndex < 0 || targetIndex >= sorted.length) {
    return
  }

  const reordered = [...sorted]
  const temp = reordered[currentIndex]
  const target = reordered[targetIndex]
  if (temp === undefined || target === undefined) {
    return
  }
  reordered[currentIndex] = target
  reordered[targetIndex] = temp

  const serviceIds = reordered.map((s) => s.priority)
  try {
    await store.reorderServices(serviceIds)
    toast.success('Service order updated')
    fetchServices()
  } catch {
    toast.error('Failed to reorder services')
  }
}

function healthStatusStyle(status: string): { bg: string; text: string } {
  switch (status) {
    case 'healthy':
      return { bg: 'bg-green-100 dark:bg-green-900/30', text: 'text-green-800 dark:text-green-300' }
    case 'unhealthy':
      return { bg: 'bg-red-100 dark:bg-red-900/30', text: 'text-red-800 dark:text-red-300' }
    default:
      return { bg: 'bg-gray-100 dark:bg-gray-800', text: 'text-gray-800 dark:text-gray-300' }
  }
}

function typeBadgeStyle(type: string): { bg: string; text: string } {
  switch (type) {
    case 'kiwix':
      return { bg: 'bg-blue-100 dark:bg-blue-900/30', text: 'text-blue-800 dark:text-blue-300' }
    case 'confluence':
      return { bg: 'bg-purple-100 dark:bg-purple-900/30', text: 'text-purple-800 dark:text-purple-300' }
    default:
      return { bg: 'bg-gray-100 dark:bg-gray-800', text: 'text-gray-800 dark:text-gray-300' }
  }
}

const columns: ColumnDef<ExternalService, unknown>[] = [
  {
    accessorKey: 'name',
    header: 'Name',
    enableSorting: true,
  },
  {
    accessorKey: 'type',
    header: 'Type',
    enableSorting: false,
    cell: ({ getValue }) => {
      const value = getValue<string>()
      const style = typeBadgeStyle(value)
      return h(
        'span',
        {
          class: `inline-flex items-center rounded-full px-2.5 py-0.5 text-xs font-medium capitalize ${style.bg} ${style.text}`,
        },
        value,
      )
    },
  },
  {
    accessorKey: 'base_url',
    header: 'Base URL',
    enableSorting: false,
    cell: ({ getValue }) => {
      const value = getValue<string>()
      if (value.length <= 40) {
        return value
      }
      return `${value.slice(0, 40)}...`
    },
  },
  {
    accessorKey: 'status',
    header: 'Health',
    enableSorting: false,
    cell: ({ getValue }) => {
      const value = getValue<string>()
      const style = healthStatusStyle(value)
      return h(
        'span',
        {
          class: `inline-flex items-center rounded-full px-2.5 py-0.5 text-xs font-medium capitalize ${style.bg} ${style.text}`,
        },
        value,
      )
    },
  },
  {
    accessorKey: 'priority',
    header: 'Priority',
    enableSorting: true,
    cell: ({ row }) => {
      const service = row.original
      return h('div', { class: 'flex items-center gap-1' }, [
        h('span', { class: 'text-sm tabular-nums' }, String(service.priority)),
        h(
          'button',
          {
            type: 'button',
            class: 'text-text-disabled hover:text-indigo-600 dark:hover:text-indigo-400 p-0.5',
            title: 'Move up',
            'aria-label': 'Move up',
            onClick: (event: MouseEvent) => {
              event.stopPropagation()
              handleMovePriority(service, 'up')
            },
          },
          [h(ArrowUpIcon, { class: 'h-4 w-4' })],
        ),
        h(
          'button',
          {
            type: 'button',
            class: 'text-text-disabled hover:text-indigo-600 dark:hover:text-indigo-400 p-0.5',
            title: 'Move down',
            'aria-label': 'Move down',
            onClick: (event: MouseEvent) => {
              event.stopPropagation()
              handleMovePriority(service, 'down')
            },
          },
          [h(ArrowDownIcon, { class: 'h-4 w-4' })],
        ),
      ])
    },
  },
  {
    accessorKey: 'is_enabled',
    header: 'Enabled',
    enableSorting: false,
    cell: ({ row }) => {
      const service = row.original
      return h(Switch, {
        'modelValue': service.is_enabled,
        'class': [
          'relative inline-flex h-6 w-11 shrink-0 cursor-pointer rounded-full border-2 border-transparent transition-colors duration-200 ease-in-out focus:outline-none focus-visible:ring-2 focus-visible:ring-indigo-600 focus-visible:ring-offset-2',
          service.is_enabled ? 'bg-indigo-600' : 'bg-gray-200 dark:bg-gray-600',
        ],
        'onClick': (event: MouseEvent) => {
          event.stopPropagation()
        },
        'onUpdate:modelValue': () => {
          handleToggleEnabled(service)
        },
      }, {
        default: () => h('span', {
          class: [
            'pointer-events-none inline-block h-5 w-5 transform rounded-full bg-white shadow ring-0 transition duration-200 ease-in-out',
            service.is_enabled ? 'translate-x-5' : 'translate-x-0',
          ],
        }),
      })
    },
  },
  {
    id: 'actions',
    header: 'Actions',
    enableSorting: false,
    cell: ({ row }) => {
      const service = row.original
      const isSyncing = syncingUUIDs.value.has(service.uuid)
      const canSync = service.type === 'kiwix' || service.type === 'confluence'
      return h('div', { class: 'flex items-center gap-2' }, [
        canSync
          ? h(
              'button',
              {
                type: 'button',
                class: `text-text-muted hover:text-indigo-600 dark:hover:text-indigo-400 ${isSyncing ? 'opacity-50 cursor-not-allowed' : ''}`,
                title: 'Sync now',
                'aria-label': 'Sync now',
                disabled: isSyncing,
                onClick: (event: MouseEvent) => {
                  event.stopPropagation()
                  if (!isSyncing) handleSync(service)
                },
              },
              [h(ArrowPathIcon, { class: `h-5 w-5 ${isSyncing ? 'animate-spin' : ''}` })],
            )
          : null,
        h(
          'button',
          {
            type: 'button',
            class: 'text-text-muted hover:text-green-600 dark:hover:text-green-400',
            title: 'Health check',
            'aria-label': 'Health check',
            onClick: (event: MouseEvent) => {
              event.stopPropagation()
              handleHealthCheck(service)
            },
          },
          [h(HeartIcon, { class: 'h-5 w-5' })],
        ),
        h(
          'button',
          {
            type: 'button',
            class: 'text-text-muted hover:text-indigo-600 dark:hover:text-indigo-400',
            title: 'Edit service',
            'aria-label': 'Edit service',
            onClick: (event: MouseEvent) => {
              event.stopPropagation()
              openEditModal(service)
            },
          },
          [h(PencilSquareIcon, { class: 'h-5 w-5' })],
        ),
        h(
          'button',
          {
            type: 'button',
            class: 'text-text-muted hover:text-red-600 dark:hover:text-red-400',
            title: 'Delete service',
            'aria-label': 'Delete service',
            onClick: (event: MouseEvent) => {
              event.stopPropagation()
              deleteTarget.value = service
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
      <h1 class="text-2xl font-bold text-text-primary">External Services</h1>

      <select
        v-model="typeFilter"
        class="rounded-md border-border-input bg-bg-surface text-text-secondary shadow-sm focus:border-indigo-500 focus:ring-indigo-500 dark:focus:border-indigo-400 dark:focus:ring-indigo-400 sm:text-sm"
      >
        <option value="">All Types</option>
        <option value="kiwix">Kiwix</option>
        <option value="confluence">Confluence</option>
      </select>

      <div class="flex-1" />

      <button
        type="button"
        class="bg-indigo-600 text-white rounded-md px-4 py-2 text-sm font-medium hover:bg-indigo-500"
        @click="openCreateModal"
      >
        Add Service
      </button>
    </div>

    <!-- Empty State -->
    <EmptyState
      v-if="!store.loading && store.services.length === 0 && typeFilter === ''"
      title="No external services"
      description="Add your first external service to get started."
    >
      <template #action>
        <button
          type="button"
          class="bg-indigo-600 text-white rounded-md px-4 py-2 text-sm font-medium hover:bg-indigo-500"
          @click="openCreateModal"
        >
          Add Service
        </button>
      </template>
    </EmptyState>

    <!-- Data Table -->
    <template v-else>
      <DataTable
        :data="store.services"
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
      title="Delete Service"
      :message="`Are you sure you want to delete &quot;${deleteTarget?.name ?? ''}&quot;? This action cannot be undone.`"
      confirm-label="Delete"
      variant="danger"
      @confirm="handleDeleteConfirm"
      @cancel="handleDeleteCancel"
    />

    <!-- Create / Edit Modal -->
    <ExternalServiceModal
      :open="showModal"
      :service="editTarget"
      @close="handleModalClose"
      @saved="handleModalSaved"
    />
  </div>
</template>
