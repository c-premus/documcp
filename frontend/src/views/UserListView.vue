<script setup lang="ts">
import { ref, watch, computed, h } from 'vue'
import { toast } from 'vue-sonner'
import { formatDistanceToNow } from 'date-fns'
import { PencilSquareIcon, TrashIcon } from '@heroicons/vue/24/outline'
import { Switch } from '@headlessui/vue'
import type { ColumnDef } from '@tanstack/vue-table'

import { apiFetch } from '@/api/helpers'
import DataTable from '../components/shared/DataTable.vue'
import Pagination from '../components/shared/Pagination.vue'
import SearchInput from '../components/shared/SearchInput.vue'
import EmptyState from '../components/shared/EmptyState.vue'
import ConfirmDialog from '../components/shared/ConfirmDialog.vue'
import UserModal from '../components/users/UserModal.vue'

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

const showModal = ref(false)
const editTarget = ref<User | null>(null)
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

watch([searchQuery], () => {
  page.value = 1
  fetchUsers()
}, { immediate: true })

watch([page, perPage], () => {
  fetchUsers()
})

async function handleToggleAdmin(user: User): Promise<void> {
  try {
    const response = await apiFetch<SingleResponse>(
      `/api/admin/users/${user.id}/toggle-admin`,
      { method: 'POST' },
    )
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

function openCreateModal(): void {
  editTarget.value = null
  showModal.value = true
}

function openEditModal(user: User): void {
  editTarget.value = user
  showModal.value = true
}

function handleRowClick(row: User): void {
  openEditModal(row)
}

function handleModalClose(): void {
  showModal.value = false
  editTarget.value = null
}

function handleModalSaved(): void {
  showModal.value = false
  editTarget.value = null
  fetchUsers()
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

function truncate(value: string, maxLength: number): string {
  if (value.length <= maxLength) {
    return value
  }
  return `${value.slice(0, maxLength)}...`
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
  },
  {
    accessorKey: 'is_admin',
    header: 'Admin',
    enableSorting: false,
    cell: ({ row }) => {
      const user = row.original
      return h(Switch, {
        'modelValue': user.is_admin,
        'class': [
          'relative inline-flex h-6 w-11 shrink-0 cursor-pointer rounded-full border-2 border-transparent transition-colors duration-200 ease-in-out focus:outline-none focus-visible:ring-2 focus-visible:ring-indigo-600 focus-visible:ring-offset-2',
          user.is_admin ? 'bg-indigo-600' : 'bg-gray-200',
        ],
        'onClick': (event: MouseEvent) => {
          event.stopPropagation()
        },
        'onUpdate:modelValue': () => {
          handleToggleAdmin(user)
        },
      }, {
        default: () => h('span', {
          class: [
            'pointer-events-none inline-block h-5 w-5 transform rounded-full bg-white shadow ring-0 transition duration-200 ease-in-out',
            user.is_admin ? 'translate-x-5' : 'translate-x-0',
          ],
        }),
      })
    },
  },
  {
    accessorKey: 'oidc_sub',
    header: 'OIDC Subject',
    enableSorting: false,
    cell: ({ getValue }) => {
      const value = getValue<string>()
      return truncate(value, 20)
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
    id: 'actions',
    header: 'Actions',
    enableSorting: false,
    cell: ({ row }) => {
      return h('div', { class: 'flex items-center gap-2' }, [
        h(
          'button',
          {
            type: 'button',
            class: 'text-gray-500 hover:text-indigo-600',
            title: 'Edit user',
            onClick: (event: MouseEvent) => {
              event.stopPropagation()
              openEditModal(row.original)
            },
          },
          [h(PencilSquareIcon, { class: 'h-5 w-5' })],
        ),
        h(
          'button',
          {
            type: 'button',
            class: 'text-gray-500 hover:text-red-600',
            title: 'Delete user',
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
    <!-- Toolbar -->
    <div class="flex items-center gap-4 mb-4">
      <h1 class="text-2xl font-bold text-gray-900">Users</h1>

      <SearchInput
        v-model="searchQuery"
        placeholder="Search users..."
        class="flex-1 max-w-sm"
      />

      <button
        type="button"
        class="bg-indigo-600 text-white rounded-md px-4 py-2 text-sm font-medium hover:bg-indigo-500"
        @click="openCreateModal"
      >
        Create User
      </button>
    </div>

    <!-- Empty State -->
    <EmptyState
      v-if="!loading && users.length === 0 && searchQuery === ''"
      title="No users yet"
      description="Create your first user to get started."
    >
      <template #action>
        <button
          type="button"
          class="bg-indigo-600 text-white rounded-md px-4 py-2 text-sm font-medium hover:bg-indigo-500"
          @click="openCreateModal"
        >
          Create User
        </button>
      </template>
    </EmptyState>

    <!-- Data Table -->
    <template v-else>
      <DataTable
        :data="users"
        :columns="columns"
        :loading="loading"
        @row-click="handleRowClick"
      />

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
      :message="`Are you sure you want to delete &quot;${deleteTarget?.name ?? ''}&quot;? This action cannot be undone.`"
      confirm-label="Delete"
      variant="danger"
      @confirm="handleDeleteConfirm"
      @cancel="handleDeleteCancel"
    />

    <!-- Create / Edit Modal -->
    <UserModal
      :open="showModal"
      :user="editTarget"
      @close="handleModalClose"
      @saved="handleModalSaved"
    />
  </div>
</template>
