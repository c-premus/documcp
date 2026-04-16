<script setup lang="ts">
import { ref, watch, computed, h } from 'vue'
import { toast } from 'vue-sonner'
import type { ColumnDef } from '@tanstack/vue-table'

import { apiFetch } from '@/api/helpers'
import DataTable from '../components/shared/DataTable.vue'
import Pagination from '../components/shared/Pagination.vue'
import SearchInput from '../components/shared/SearchInput.vue'
import EmptyState from '../components/shared/EmptyState.vue'
import ConfirmDialog from '../components/shared/ConfirmDialog.vue'
import ToggleCell from '../components/shared/ToggleCell.vue'
import TruncatedText from '../components/shared/TruncatedText.vue'
import RelativeTimeCell from '../components/shared/RelativeTimeCell.vue'
import UserRowActions from '../components/users/UserRowActions.vue'

interface User {
  readonly id: number
  readonly name: string
  readonly email: string
  readonly oidc_sub: string
  readonly oidc_provider: string
  readonly is_admin: boolean
  readonly created_at: string
  readonly updated_at: string
}

interface ListResponse {
  readonly data: User[]
  readonly meta: { readonly total: number }
}

interface SingleResponse {
  readonly data: User
}

interface DeleteResponse {
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

const users = ref<User[]>([])
const total = ref(0)
const loading = ref(false)

const searchQuery = ref('')
const page = ref(1)
const perPage = ref(20)

const deleteTarget = ref<User | null>(null)
const showDeleteDialog = computed(() => deleteTarget.value !== null)

async function fetchUsers(): Promise<void> {
  loading.value = true
  try {
    const offset = (page.value - 1) * perPage.value
    const query = buildQuery({
      q: searchQuery.value || undefined,
      limit: perPage.value,
      offset,
    })
    const response = await apiFetch<ListResponse>(`/api/admin/users${query}`)
    users.value = response.data
    total.value = response.meta.total
  } catch {
    toast.error('Failed to load users')
  } finally {
    loading.value = false
  }
}

watch(
  [searchQuery],
  () => {
    page.value = 1
    fetchUsers()
  },
  { immediate: true },
)

watch([page, perPage], () => {
  fetchUsers()
})

async function handleToggleAdmin(user: User): Promise<void> {
  try {
    const response = await apiFetch<SingleResponse>(`/api/admin/users/${user.id}/toggle-admin`, {
      method: 'POST',
    })
    const index = users.value.findIndex((u) => u.id === user.id)
    if (index !== -1) {
      users.value[index] = response.data
    }
    const label = response.data.is_admin ? 'granted' : 'revoked'
    toast.success(`Admin ${label} for ${response.data.name}`)
  } catch {
    toast.error(`Failed to toggle admin for ${user.name}`)
  }
}

async function handleDeleteConfirm(): Promise<void> {
  if (deleteTarget.value === null) {
    return
  }
  const name = deleteTarget.value.name
  try {
    await apiFetch<DeleteResponse>(`/api/admin/users/${deleteTarget.value.id}`, {
      method: 'DELETE',
    })
    toast.success(`User "${name}" deleted`)
    deleteTarget.value = null
    fetchUsers()
  } catch {
    toast.error(`Failed to delete "${name}"`)
  }
}

function handleDeleteCancel(): void {
  deleteTarget.value = null
}

const columns: ColumnDef<User, unknown>[] = [
  {
    accessorKey: 'name',
    header: 'Name',
    enableSorting: true,
  },
  {
    accessorKey: 'email',
    header: 'Email',
    enableSorting: true,
    meta: { className: 'hidden md:table-cell' },
  },
  {
    accessorKey: 'is_admin',
    header: 'Admin',
    enableSorting: false,
    meta: { className: 'w-20' },
    cell: ({ row }) =>
      h(ToggleCell, {
        modelValue: row.original.is_admin,
        label: `Toggle admin for ${row.original.name}`,
        'onUpdate:modelValue': () => handleToggleAdmin(row.original),
      }),
  },
  {
    accessorKey: 'oidc_sub',
    header: 'OIDC Subject',
    enableSorting: false,
    meta: { className: 'hidden lg:table-cell' },
    cell: ({ getValue }) => h(TruncatedText, { value: getValue<string>() }),
  },
  {
    accessorKey: 'created_at',
    header: 'Created',
    enableSorting: true,
    meta: { className: 'w-36 hidden md:table-cell' },
    cell: ({ getValue }) => h(RelativeTimeCell, { value: getValue<string>() }),
  },
  {
    id: 'actions',
    header: 'Actions',
    enableSorting: false,
    meta: { className: 'w-20' },
    cell: ({ row }) =>
      h(UserRowActions, {
        user: row.original,
        onDelete: (user) => {
          deleteTarget.value = user as User
        },
      }),
  },
]
</script>

<template>
  <div>
    <!-- Toolbar -->
    <div class="flex flex-wrap items-center gap-2 sm:gap-4 mb-4">
      <h1 class="text-2xl font-bold text-text-primary">Users</h1>

      <SearchInput
        v-model="searchQuery"
        placeholder="Search users..."
        class="w-full sm:w-auto sm:max-w-sm"
      />
    </div>

    <!-- OIDC-only notice -->
    <p class="text-sm text-text-muted mb-4">
      Users are provisioned on first OIDC login. Name and email sync from identity-provider claims
      on every sign-in. Admin membership follows <code>OIDC_ADMIN_GROUPS</code> when configured; the
      admin toggle below is effective only when no admin group is set.
    </p>

    <!-- Empty State -->
    <EmptyState
      v-if="!loading && users.length === 0 && searchQuery === ''"
      title="No users yet"
      description="Users appear here after their first OIDC sign-in."
    />

    <!-- Data Table -->
    <template v-else>
      <DataTable :data="users" :columns="columns" :loading="loading" />

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
      title="Delete User"
      :message="`Are you sure you want to delete &quot;${deleteTarget?.name ?? ''}&quot;? If the user still has an active account at your identity provider, they will be re-provisioned on next login.`"
      confirm-label="Delete"
      variant="danger"
      @confirm="handleDeleteConfirm"
      @cancel="handleDeleteCancel"
    />
  </div>
</template>
