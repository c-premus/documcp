<script setup lang="ts">
import { storeToRefs } from 'pinia'
import { ref, watch, computed, h } from 'vue'
import { toast } from 'vue-sonner'
import type { ColumnDef } from '@tanstack/vue-table'

import DataTable from '../components/shared/DataTable.vue'
import Pagination from '../components/shared/Pagination.vue'
import SearchInput from '../components/shared/SearchInput.vue'
import EmptyState from '../components/shared/EmptyState.vue'
import ConfirmDialog from '../components/shared/ConfirmDialog.vue'
import ToggleCell from '../components/shared/ToggleCell.vue'
import TruncatedText from '../components/shared/TruncatedText.vue'
import RelativeTimeCell from '../components/shared/RelativeTimeCell.vue'
import UserRowActions from '../components/users/UserRowActions.vue'

import { useUsersStore, type User } from '../stores/users'

const usersStore = useUsersStore()
const { users, total, loading } = storeToRefs(usersStore)

const searchQuery = ref('')
const page = ref(1)
const perPage = ref(20)

const deleteTarget = ref<User | null>(null)
const showDeleteDialog = computed(() => deleteTarget.value !== null)

async function loadUsers(): Promise<void> {
  try {
    await usersStore.fetchUsers({
      q: searchQuery.value || undefined,
      limit: perPage.value,
      offset: (page.value - 1) * perPage.value,
    })
  } catch {
    toast.error('Failed to load users')
  }
}

watch(
  [searchQuery],
  () => {
    page.value = 1
    loadUsers()
  },
  { immediate: true },
)

watch([page, perPage], () => {
  loadUsers()
})

async function handleToggleAdmin(user: User): Promise<void> {
  try {
    const updated = await usersStore.toggleAdmin(user.id)
    const label = updated.is_admin ? 'granted' : 'revoked'
    toast.success(`Admin ${label} for ${updated.name}`)
  } catch {
    toast.error(`Failed to toggle admin for ${user.name}`)
  }
}

async function handleDeleteConfirm(): Promise<void> {
  if (deleteTarget.value === null) {
    return
  }
  const name = deleteTarget.value.name
  const id = deleteTarget.value.id
  try {
    await usersStore.deleteUser(id)
    toast.success(`User "${name}" deleted`)
    deleteTarget.value = null
    loadUsers()
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
