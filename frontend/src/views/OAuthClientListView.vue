<script setup lang="ts">
import { ref, watch, computed, h } from 'vue'
import { toast } from 'vue-sonner'
import { formatDistanceToNow } from 'date-fns'
import { NoSymbolIcon } from '@heroicons/vue/24/outline'
import type { ColumnDef } from '@tanstack/vue-table'

import { apiFetch } from '@/api/helpers'
import DataTable from '../components/shared/DataTable.vue'
import Pagination from '../components/shared/Pagination.vue'
import SearchInput from '../components/shared/SearchInput.vue'
import EmptyState from '../components/shared/EmptyState.vue'
import ConfirmDialog from '../components/shared/ConfirmDialog.vue'
import StatusBadge from '../components/shared/StatusBadge.vue'
import OAuthClientCreateModal from '../components/oauth/OAuthClientCreateModal.vue'
import SecretDisplayModal from '../components/oauth/SecretDisplayModal.vue'

import type { OAuthClient } from '../stores/oauthClients'

interface ListResponse {
  readonly data: OAuthClient[]
  readonly meta: {
    readonly total: number
    readonly limit: number
    readonly offset: number
  }
}

interface RevokeResponse {
  readonly message: string
}

function buildQuery(params: Record<string, string | number | undefined>): string {
  const search = new URLSearchParams()
  for (const [key, value] of Object.entries(params)) {
    if (value !== undefined && value !== '') {
      search.set(key, String(value))
    }
  }
  const qs = search.toString()
  return qs ? `?${qs}` : ''
}

const clients = ref<OAuthClient[]>([])
const total = ref(0)
const loading = ref(false)

const searchQuery = ref('')
const page = ref(1)
const perPage = ref(20)

const showCreateModal = ref(false)
const revokeTarget = ref<OAuthClient | null>(null)
const showRevokeDialog = computed(() => revokeTarget.value !== null)

const showSecretModal = ref(false)
const createdClientId = ref('')
const createdClientSecret = ref('')

async function fetchClients(): Promise<void> {
  loading.value = true
  try {
    const offset = (page.value - 1) * perPage.value
    const query = buildQuery({
      q: searchQuery.value || undefined,
      limit: perPage.value,
      offset,
    })
    const response = await apiFetch<ListResponse>(`/api/admin/oauth-clients${query}`)
    clients.value = response.data
    total.value = response.meta.total
  } catch {
    toast.error('Failed to load OAuth clients')
  } finally {
    loading.value = false
  }
}

watch([searchQuery], () => {
  page.value = 1
  fetchClients()
}, { immediate: true })

watch([page, perPage], () => {
  fetchClients()
})

function openCreateModal(): void {
  showCreateModal.value = true
}

function handleCreateModalClose(): void {
  showCreateModal.value = false
}

function handleClientCreated(payload: { clientId: string; clientSecret: string }): void {
  showCreateModal.value = false
  createdClientId.value = payload.clientId
  createdClientSecret.value = payload.clientSecret
  showSecretModal.value = true
  fetchClients()
}

function handleSecretModalClose(): void {
  showSecretModal.value = false
  createdClientId.value = ''
  createdClientSecret.value = ''
}

async function handleRevokeConfirm(): Promise<void> {
  if (revokeTarget.value === null) {
    return
  }
  const clientName = revokeTarget.value.client_name
  const clientDbId = revokeTarget.value.id
  try {
    await apiFetch<RevokeResponse>(`/api/admin/oauth-clients/${clientDbId}/revoke`, {
      method: 'POST',
    })
    toast.success(`Client "${clientName}" revoked`)
    revokeTarget.value = null
    fetchClients()
  } catch {
    toast.error(`Failed to revoke "${clientName}"`)
  }
}

function handleRevokeCancel(): void {
  revokeTarget.value = null
}

const columns: ColumnDef<OAuthClient, unknown>[] = [
  {
    accessorKey: 'client_name',
    header: 'Client Name',
    enableSorting: true,
  },
  {
    accessorKey: 'client_id',
    header: 'Client ID',
    enableSorting: false,
    cell: ({ getValue }) => {
      const value = getValue<string>()
      if (value.length <= 24) {
        return value
      }
      return `${value.slice(0, 24)}...`
    },
  },
  {
    accessorKey: 'grant_types',
    header: 'Grant Types',
    enableSorting: false,
    cell: ({ getValue }) => {
      const types = getValue<string[]>()
      return h(
        'div',
        { class: 'flex flex-wrap gap-1' },
        types.map((gt) =>
          h(
            'span',
            {
              class:
                'inline-flex items-center rounded-full bg-blue-100 dark:bg-blue-900/30 px-2 py-0.5 text-xs font-medium text-blue-800 dark:text-blue-300',
            },
            gt.replace(/_/g, ' '),
          ),
        ),
      )
    },
  },
  {
    accessorKey: 'is_active',
    header: 'Status',
    enableSorting: false,
    cell: ({ getValue }) => {
      const active = getValue<boolean>()
      return h(StatusBadge, {
        status: active ? 'active' : 'revoked',
      })
    },
  },
  {
    accessorKey: 'token_endpoint_auth_method',
    header: 'Auth Method',
    enableSorting: false,
    cell: ({ getValue }) => {
      const value = getValue<string>()
      return value.replace(/_/g, ' ')
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
    accessorKey: 'last_used_at',
    header: 'Last Used',
    enableSorting: true,
    cell: ({ getValue }) => {
      const value = getValue<string | null>()
      if (value === null) return '—'
      return formatDistanceToNow(new Date(value), { addSuffix: true })
    },
  },
  {
    id: 'actions',
    header: 'Actions',
    enableSorting: false,
    cell: ({ row }) => {
      const client = row.original
      if (!client.is_active) {
        return h(
          'span',
          { class: 'text-xs text-text-disabled' },
          'Revoked',
        )
      }
      return h(
        'button',
        {
          type: 'button',
          class: 'cursor-pointer text-text-muted hover:text-red-600 dark:hover:text-red-400',
          title: `Revoke client ${client.client_name}`,
          'aria-label': `Revoke client ${client.client_name}`,
          onClick: (event: MouseEvent) => {
            event.stopPropagation()
            revokeTarget.value = client
          },
        },
        [h(NoSymbolIcon, { class: 'h-5 w-5' })],
      )
    },
  },
]
</script>

<template>
  <div>
    <!-- Toolbar -->
    <div class="flex items-center gap-4 mb-4">
      <h1 class="text-2xl font-bold text-text-primary">OAuth Clients</h1>

      <SearchInput
        v-model="searchQuery"
        placeholder="Search clients..."
        class="flex-1 max-w-sm"
      />

      <button
        type="button"
        class="bg-indigo-600 text-white rounded-md px-4 py-2 text-sm font-medium hover:bg-indigo-500"
        @click="openCreateModal"
      >
        Create Client
      </button>
    </div>

    <!-- Empty State -->
    <EmptyState
      v-if="!loading && clients.length === 0 && searchQuery === ''"
      title="No OAuth clients yet"
      description="Create your first OAuth client to get started."
    >
      <template #action>
        <button
          type="button"
          class="bg-indigo-600 text-white rounded-md px-4 py-2 text-sm font-medium hover:bg-indigo-500"
          @click="openCreateModal"
        >
          Create Client
        </button>
      </template>
    </EmptyState>

    <!-- Data Table -->
    <template v-else>
      <DataTable
        :data="clients"
        :columns="columns"
        :loading="loading"
      />

      <Pagination
        :page="page"
        :per-page="perPage"
        :total="total"
        @update:page="page = $event"
        @update:per-page="perPage = $event"
      />
    </template>

    <!-- Revoke Confirmation Dialog -->
    <ConfirmDialog
      :open="showRevokeDialog"
      title="Revoke OAuth Client"
      :message="`Are you sure you want to revoke &quot;${revokeTarget?.client_name ?? ''}&quot;? This client will no longer be able to authenticate.`"
      confirm-label="Revoke"
      variant="danger"
      @confirm="handleRevokeConfirm"
      @cancel="handleRevokeCancel"
    />

    <!-- Create Client Modal -->
    <OAuthClientCreateModal
      :open="showCreateModal"
      @close="handleCreateModalClose"
      @created="handleClientCreated"
    />

    <!-- Secret Display Modal -->
    <SecretDisplayModal
      :open="showSecretModal"
      :client-id="createdClientId"
      :client-secret="createdClientSecret"
      @close="handleSecretModalClose"
    />
  </div>
</template>
