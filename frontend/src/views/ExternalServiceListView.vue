<script setup lang="ts">
import { ref, watch, computed, h } from 'vue'
import { toast } from 'vue-sonner'
import type { ColumnDef } from '@tanstack/vue-table'

import DataTable from '../components/shared/DataTable.vue'
import Pagination from '../components/shared/Pagination.vue'
import EmptyState from '../components/shared/EmptyState.vue'
import ConfirmDialog from '../components/shared/ConfirmDialog.vue'
import CategoryBadge from '../components/shared/CategoryBadge.vue'
import TruncatedText from '../components/shared/TruncatedText.vue'
import ToggleCell from '../components/shared/ToggleCell.vue'
import PriorityReorder from '../components/shared/PriorityReorder.vue'
import ExternalServiceModal from '../components/external-services/ExternalServiceModal.vue'
import ExternalServiceRowActions from '../components/external-services/ExternalServiceRowActions.vue'
import ExternalServiceMobileCard from '../components/external-services/ExternalServiceMobileCard.vue'

import { useExternalServicesStore } from '../stores/externalServices'
import type { ExternalService } from '../stores/externalServices'

const TYPE_PALETTE: Readonly<Record<string, string>> = {
  kiwix: 'bg-blue-100 text-blue-800 dark:bg-blue-900/30 dark:text-blue-300',
}

const HEALTH_PALETTE: Readonly<Record<string, string>> = {
  healthy: 'bg-green-100 text-green-800 dark:bg-green-900/30 dark:text-green-300',
  unhealthy: 'bg-red-100 text-red-800 dark:bg-red-900/30 dark:text-red-300',
}

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

watch(
  [typeFilter],
  () => {
    page.value = 1
    fetchServices()
  },
  { immediate: true },
)

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

const sortedByPriority = computed(() => [...store.services].sort((a, b) => a.priority - b.priority))

async function handleMovePriority(
  service: ExternalService,
  direction: 'up' | 'down',
): Promise<void> {
  const sorted = sortedByPriority.value
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

  const order = reordered.map((s, i) => ({ uuid: s.uuid, priority: i }))
  try {
    await store.reorderServices(order)
    toast.success('Service order updated')
    fetchServices()
  } catch {
    toast.error('Failed to reorder services')
  }
}

function canMoveUp(service: ExternalService): boolean {
  const sorted = sortedByPriority.value
  return sorted.findIndex((s) => s.uuid === service.uuid) > 0
}

function canMoveDown(service: ExternalService): boolean {
  const sorted = sortedByPriority.value
  const idx = sorted.findIndex((s) => s.uuid === service.uuid)
  return idx !== -1 && idx < sorted.length - 1
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
    meta: { className: 'w-28 hidden sm:table-cell' },
    cell: ({ getValue }) => h(CategoryBadge, { value: getValue<string>(), palette: TYPE_PALETTE }),
  },
  {
    accessorKey: 'base_url',
    header: 'Base URL',
    enableSorting: false,
    meta: { className: 'hidden lg:table-cell' },
    cell: ({ getValue }) => h(TruncatedText, { value: getValue<string>(), mono: true }),
  },
  {
    accessorKey: 'status',
    header: 'Health',
    enableSorting: false,
    meta: { className: 'w-28' },
    cell: ({ getValue }) =>
      h(CategoryBadge, { value: getValue<string>(), palette: HEALTH_PALETTE }),
  },
  {
    accessorKey: 'priority',
    header: 'Priority',
    enableSorting: true,
    meta: { className: 'w-28 hidden md:table-cell' },
    cell: ({ row }) =>
      h(PriorityReorder, {
        priority: row.original.priority,
        canMoveUp: canMoveUp(row.original),
        canMoveDown: canMoveDown(row.original),
        onUp: () => handleMovePriority(row.original, 'up'),
        onDown: () => handleMovePriority(row.original, 'down'),
      }),
  },
  {
    accessorKey: 'is_enabled',
    header: 'Enabled',
    enableSorting: false,
    meta: { className: 'w-20 hidden sm:table-cell' },
    cell: ({ row }) =>
      h(ToggleCell, {
        modelValue: row.original.is_enabled,
        label: `Toggle ${row.original.name} enabled`,
        'onUpdate:modelValue': () => handleToggleEnabled(row.original),
      }),
  },
  {
    id: 'actions',
    header: 'Actions',
    enableSorting: false,
    meta: { className: 'w-36' },
    cell: ({ row }) =>
      h(ExternalServiceRowActions, {
        service: row.original,
        syncing: syncingUUIDs.value.has(row.original.uuid),
        onSync: handleSync,
        onHealthCheck: handleHealthCheck,
        onEdit: openEditModal,
        onDelete: (service: ExternalService) => {
          deleteTarget.value = service
        },
      }),
  },
]
</script>

<template>
  <div>
    <!-- Toolbar -->
    <div class="flex flex-wrap items-center gap-2 sm:gap-4 mb-4">
      <h1 class="text-2xl font-bold text-text-primary">External Services</h1>

      <label for="service-type-filter" class="sr-only">Filter by service type</label>
      <select
        id="service-type-filter"
        v-model="typeFilter"
        class="rounded-md border-border-input bg-bg-surface text-text-secondary shadow-sm focus:border-indigo-500 focus:ring-indigo-500 dark:focus:border-indigo-400 dark:focus:ring-indigo-400 sm:text-sm"
      >
        <option value="">All Types</option>
        <option value="kiwix">Kiwix</option>
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
        :clickable="true"
        @row-click="handleRowClick"
      >
        <template #mobile-card="{ row }">
          <ExternalServiceMobileCard
            :service="row as ExternalService"
            :syncing="syncingUUIDs.has((row as ExternalService).uuid)"
            :can-move-up="canMoveUp(row as ExternalService)"
            :can-move-down="canMoveDown(row as ExternalService)"
            @sync="handleSync"
            @health-check="handleHealthCheck"
            @edit="openEditModal"
            @delete="(service: ExternalService) => (deleteTarget = service)"
            @toggle-enabled="handleToggleEnabled"
            @move-priority="handleMovePriority"
          />
        </template>
      </DataTable>

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
