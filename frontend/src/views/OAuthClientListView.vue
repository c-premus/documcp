<script setup lang="ts">
import { storeToRefs } from 'pinia'
import { ref, watch, computed, h } from 'vue'
import { RouterLink } from 'vue-router'
import { toast } from 'vue-sonner'
import type { ColumnDef } from '@tanstack/vue-table'

import DataTable from '../components/shared/DataTable.vue'
import Pagination from '../components/shared/Pagination.vue'
import SearchInput from '../components/shared/SearchInput.vue'
import EmptyState from '../components/shared/EmptyState.vue'
import ConfirmDialog from '../components/shared/ConfirmDialog.vue'
import TruncatedText from '../components/shared/TruncatedText.vue'
import RelativeTimeCell from '../components/shared/RelativeTimeCell.vue'
import OAuthClientCreateModal from '../components/oauth/OAuthClientCreateModal.vue'
import OAuthGrantTypeChips from '../components/oauth/OAuthGrantTypeChips.vue'
import OAuthClientRowActions from '../components/oauth/OAuthClientRowActions.vue'
import SecretDisplayModal from '../components/oauth/SecretDisplayModal.vue'

import { useOAuthClientsStore, type OAuthClient } from '../stores/oauthClients'

const oauthStore = useOAuthClientsStore()
const { clients, total, loading } = storeToRefs(oauthStore)

const searchQuery = ref('')
const page = ref(1)
const perPage = ref(20)

const showCreateModal = ref(false)
const deleteTarget = ref<OAuthClient | null>(null)
const showDeleteDialog = computed(() => deleteTarget.value !== null)

const showSecretModal = ref(false)
const createdClientId = ref('')
const createdClientSecret = ref('')

async function loadClients(): Promise<void> {
  try {
    await oauthStore.fetchClients({
      q: searchQuery.value || undefined,
      limit: perPage.value,
      offset: (page.value - 1) * perPage.value,
    })
  } catch {
    toast.error('Failed to load OAuth clients')
  }
}

watch(
  [searchQuery],
  () => {
    page.value = 1
    loadClients()
  },
  { immediate: true },
)

watch([page, perPage], () => {
  loadClients()
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
  loadClients()
}

function handleSecretModalClose(): void {
  showSecretModal.value = false
  createdClientId.value = ''
  createdClientSecret.value = ''
}

async function handleDeleteConfirm(): Promise<void> {
  if (deleteTarget.value === null) {
    return
  }
  const clientName = deleteTarget.value.client_name
  const clientDbId = deleteTarget.value.id
  try {
    await oauthStore.deleteClient(clientDbId)
    toast.success(`Client "${clientName}" deleted`)
    deleteTarget.value = null
    loadClients()
  } catch {
    toast.error(`Failed to delete "${clientName}"`)
  }
}

function handleDeleteCancel(): void {
  deleteTarget.value = null
}

const columns: ColumnDef<OAuthClient, unknown>[] = [
  {
    accessorKey: 'client_name',
    header: 'Client Name',
    enableSorting: true,
    cell: ({ row }) => {
      const client = row.original
      return h(
        RouterLink,
        {
          to: `/oauth-clients/${client.id}`,
          class: 'font-medium text-indigo-600 dark:text-indigo-400 hover:underline',
        },
        () => client.client_name,
      )
    },
  },
  {
    accessorKey: 'client_id',
    header: 'Client ID',
    enableSorting: false,
    meta: { className: 'hidden lg:table-cell' },
    cell: ({ getValue }) =>
      h(TruncatedText, { value: getValue<string>(), mono: true, maxWidth: 'max-w-[12rem]' }),
  },
  {
    accessorKey: 'grant_types',
    header: 'Grant Types',
    enableSorting: false,
    meta: { className: 'hidden sm:table-cell' },
    cell: ({ getValue }) => h(OAuthGrantTypeChips, { grantTypes: getValue<string[]>() }),
  },
  {
    accessorKey: 'token_endpoint_auth_method',
    header: 'Auth Method',
    enableSorting: false,
    meta: { className: 'hidden lg:table-cell' },
    cell: ({ getValue }) => getValue<string>().replace(/_/g, ' '),
  },
  {
    accessorKey: 'created_at',
    header: 'Created',
    enableSorting: true,
    meta: { className: 'w-36 hidden md:table-cell' },
    cell: ({ getValue }) => h(RelativeTimeCell, { value: getValue<string>() }),
  },
  {
    accessorKey: 'last_used_at',
    header: 'Last Used',
    enableSorting: true,
    meta: { className: 'w-36 hidden md:table-cell' },
    cell: ({ getValue }) =>
      h(RelativeTimeCell, { value: getValue<string | null>() ?? null, fallback: '—' }),
  },
  {
    id: 'actions',
    header: 'Actions',
    enableSorting: false,
    meta: { className: 'w-20' },
    cell: ({ row }) =>
      h(OAuthClientRowActions, {
        client: row.original,
        onDelete: (client: OAuthClient) => {
          deleteTarget.value = client
        },
      }),
  },
]
</script>

<template>
  <div>
    <!-- Toolbar -->
    <div class="flex flex-wrap items-center gap-2 sm:gap-4 mb-4">
      <h1 class="text-2xl font-bold text-text-primary">OAuth Clients</h1>

      <SearchInput
        v-model="searchQuery"
        placeholder="Search clients..."
        class="w-full sm:w-auto sm:max-w-sm"
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
      <DataTable :data="clients" :columns="columns" :loading="loading" />

      <Pagination
        :page="page"
        :per-page="perPage"
        :total="total"
        @update:page="page = $event"
        @update:per-page="perPage = $event"
      />
    </template>

    <!-- Delete Confirmation Dialog -->
    <ConfirmDialog
      :open="showDeleteDialog"
      title="Delete OAuth Client"
      :message="`Are you sure you want to delete &quot;${deleteTarget?.client_name ?? ''}&quot;? This will permanently remove the client and revoke all its tokens.`"
      confirm-label="Delete"
      variant="danger"
      @confirm="handleDeleteConfirm"
      @cancel="handleDeleteCancel"
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
