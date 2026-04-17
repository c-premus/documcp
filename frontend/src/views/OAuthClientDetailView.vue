<script setup lang="ts">
import { ref, h, watch, onMounted } from 'vue'
import { RouterLink } from 'vue-router'
import { toast } from 'vue-sonner'
import { format, formatDistanceToNow } from 'date-fns'
import { TrashIcon } from '@heroicons/vue/24/outline'
import type { ColumnDef } from '@tanstack/vue-table'

import { useOAuthClientsStore, type ScopeGrant } from '@/stores/oauthClients'
import DataTable from '@/components/shared/DataTable.vue'
import EmptyState from '@/components/shared/EmptyState.vue'
import ConfirmDialog from '@/components/shared/ConfirmDialog.vue'

const props = defineProps<{
  readonly id: number
}>()

const store = useOAuthClientsStore()
const revokeTarget = ref<ScopeGrant | null>(null)
const revoking = ref(false)

function formatDate(dateString: string): string {
  return format(new Date(dateString), 'MMM d, yyyy h:mm a')
}

function formatRelative(dateString: string | null): string {
  if (!dateString) return 'Never'
  return formatDistanceToNow(new Date(dateString), { addSuffix: true })
}

function formatGranter(grant: ScopeGrant): string {
  return grant.granted_by_email || grant.granted_by_name || `User #${grant.granted_by}`
}

async function loadClient(id: number): Promise<void> {
  try {
    await Promise.all([store.fetchClient(id), store.fetchScopeGrants(id)])
  } catch {
    // error is set in store
  }
}

async function handleRevoke(): Promise<void> {
  if (!revokeTarget.value) return
  revoking.value = true
  try {
    await store.revokeScopeGrant(props.id, revokeTarget.value.id)
    toast.success('Scope grant revoked')
  } catch {
    toast.error('Failed to revoke scope grant')
  } finally {
    revoking.value = false
    revokeTarget.value = null
  }
}

const grantColumns: ColumnDef<ScopeGrant, unknown>[] = [
  {
    accessorKey: 'scope',
    header: 'Scope',
    enableSorting: false,
    cell: ({ getValue }) => {
      const value = getValue<string>()
      return h(
        'div',
        { class: 'flex flex-wrap gap-1' },
        value.split(' ').map((s) =>
          h(
            'span',
            {
              class:
                'inline-flex items-center rounded-full bg-green-100 dark:bg-green-900/30 px-2 py-0.5 text-xs font-medium text-green-800 dark:text-green-300',
            },
            s,
          ),
        ),
      )
    },
  },
  {
    accessorKey: 'granted_by',
    header: 'Granted By',
    enableSorting: false,
    cell: ({ row }) => formatGranter(row.original),
  },
  {
    accessorKey: 'granted_at',
    header: 'Granted',
    enableSorting: false,
    cell: ({ getValue }) => formatRelative(getValue<string>()),
  },
  {
    accessorKey: 'expires_at',
    header: 'Expires',
    enableSorting: false,
    cell: ({ getValue }) => formatRelative(getValue<string | null>()),
  },
  {
    id: 'actions',
    header: '',
    enableSorting: false,
    meta: { className: 'w-16' },
    cell: ({ row }) => {
      const grant = row.original
      return h(
        'button',
        {
          type: 'button',
          class: 'cursor-pointer text-text-muted hover:text-red-600 dark:hover:text-red-400',
          title: 'Revoke grant',
          'aria-label': 'Revoke grant',
          onClick: (event: MouseEvent) => {
            event.stopPropagation()
            revokeTarget.value = grant
          },
        },
        [h(TrashIcon, { class: 'h-5 w-5' })],
      )
    },
  },
]

onMounted(() => {
  loadClient(props.id)
})

watch(
  () => props.id,
  (newId) => {
    loadClient(newId)
  },
)
</script>

<template>
  <div class="space-y-6">
    <!-- Back link -->
    <RouterLink
      to="/oauth-clients"
      class="inline-flex items-center text-sm font-medium text-text-muted hover:text-text-secondary"
    >
      &larr; OAuth Clients
    </RouterLink>

    <!-- Loading state -->
    <div
      v-if="store.loading && !store.currentClient"
      role="status"
      aria-live="polite"
      class="flex items-center justify-center py-20"
    >
      <div
        class="h-8 w-8 animate-spin rounded-full border-4 border-border-input border-t-indigo-600 dark:border-t-indigo-400"
      />
      <span class="sr-only">Loading client...</span>
    </div>

    <!-- Error state -->
    <div
      v-else-if="store.error && !store.currentClient"
      class="rounded-lg bg-red-50 dark:bg-red-900/20 p-6 text-center"
    >
      <p class="text-sm text-red-800 dark:text-red-300">{{ store.error }}</p>
      <button
        type="button"
        class="mt-4 rounded-md bg-red-100 dark:bg-red-900/30 px-3 py-2 text-sm font-medium text-red-800 dark:text-red-300 hover:bg-red-200 dark:hover:bg-red-900/50"
        @click="loadClient(id)"
      >
        Retry
      </button>
    </div>

    <!-- Client detail -->
    <template v-else-if="store.currentClient">
      <h1 class="text-2xl font-bold text-text-primary">{{ store.currentClient.client_name }}</h1>

      <div class="grid grid-cols-1 gap-6 lg:grid-cols-3">
        <!-- Metadata sidebar -->
        <div class="rounded-lg bg-bg-surface p-6 shadow-sm lg:col-span-1">
          <h2 class="text-sm font-semibold uppercase tracking-wider text-text-muted">Details</h2>
          <dl class="mt-4 space-y-4">
            <!-- Client ID -->
            <div>
              <dt class="text-sm font-medium text-text-muted">Client ID</dt>
              <dd
                class="mt-1 block max-w-full truncate font-mono text-sm text-text-primary"
                :title="store.currentClient.client_id"
              >
                {{ store.currentClient.client_id }}
              </dd>
            </div>

            <!-- Auth Method -->
            <div>
              <dt class="text-sm font-medium text-text-muted">Auth Method</dt>
              <dd class="mt-1 text-sm text-text-primary">
                {{ store.currentClient.token_endpoint_auth_method.replace(/_/g, ' ') }}
              </dd>
            </div>

            <!-- Grant Types -->
            <div>
              <dt class="text-sm font-medium text-text-muted">Grant Types</dt>
              <dd class="mt-1 flex flex-wrap gap-1">
                <span
                  v-for="gt in store.currentClient.grant_types"
                  :key="gt"
                  class="inline-flex items-center rounded-full bg-blue-100 dark:bg-blue-900/30 px-2 py-0.5 text-xs font-medium text-blue-800 dark:text-blue-300"
                >
                  {{ gt.replace(/_/g, ' ') }}
                </span>
              </dd>
            </div>

            <!-- Redirect URIs -->
            <div>
              <dt class="text-sm font-medium text-text-muted">Redirect URIs</dt>
              <dd class="mt-1 space-y-1">
                <p
                  v-for="uri in store.currentClient.redirect_uris"
                  :key="uri"
                  class="block truncate font-mono text-sm text-text-primary"
                  :title="uri"
                >
                  {{ uri }}
                </p>
                <p
                  v-if="store.currentClient.redirect_uris.length === 0"
                  class="text-sm text-text-muted"
                >
                  None
                </p>
              </dd>
            </div>

            <!-- Scope -->
            <div>
              <dt class="text-sm font-medium text-text-muted">Base Scope</dt>
              <dd class="mt-1">
                <div v-if="store.currentClient.scope" class="flex flex-wrap gap-1">
                  <span
                    v-for="s in store.currentClient.scope.split(' ')"
                    :key="s"
                    class="inline-flex items-center rounded-full bg-indigo-50 dark:bg-indigo-900/30 px-2 py-0.5 text-xs font-medium text-indigo-700 dark:text-indigo-300"
                  >
                    {{ s }}
                  </span>
                </div>
                <span v-else class="text-sm text-text-muted">No scopes</span>
              </dd>
            </div>

            <!-- Last Used -->
            <div>
              <dt class="text-sm font-medium text-text-muted">Last Used</dt>
              <dd class="mt-1 text-sm text-text-primary">
                {{ formatRelative(store.currentClient.last_used_at) }}
              </dd>
            </div>

            <!-- Created -->
            <div>
              <dt class="text-sm font-medium text-text-muted">Created</dt>
              <dd class="mt-1 text-sm text-text-primary">
                {{ formatDate(store.currentClient.created_at) }}
              </dd>
            </div>

            <!-- Updated -->
            <div>
              <dt class="text-sm font-medium text-text-muted">Updated</dt>
              <dd class="mt-1 text-sm text-text-primary">
                {{ formatDate(store.currentClient.updated_at) }}
              </dd>
            </div>
          </dl>
        </div>

        <!-- Scope Grants -->
        <div class="rounded-lg bg-bg-surface p-6 shadow-sm lg:col-span-2">
          <h2 class="text-sm font-semibold uppercase tracking-wider text-text-muted">
            Scope Grants
          </h2>
          <p class="mt-1 text-sm text-text-muted">
            Time-bounded scope expansions granted by users during consent approval.
          </p>

          <div class="mt-4">
            <DataTable
              v-if="store.scopeGrants.length > 0"
              :data="store.scopeGrants"
              :columns="grantColumns"
              :loading="false"
            />
            <EmptyState
              v-else
              title="No active scope grants"
              description="Scope grants are created when users approve consent for scopes beyond this client's base registration."
            />
          </div>
        </div>
      </div>
    </template>

    <!-- Revoke confirmation dialog -->
    <ConfirmDialog
      :open="revokeTarget !== null"
      title="Revoke Scope Grant"
      :message="`Are you sure you want to revoke the scope grant &quot;${revokeTarget?.scope ?? ''}&quot;? The client will lose access to these scopes until a user re-approves consent.`"
      confirm-label="Revoke"
      variant="danger"
      @confirm="handleRevoke"
      @cancel="revokeTarget = null"
    />
  </div>
</template>
