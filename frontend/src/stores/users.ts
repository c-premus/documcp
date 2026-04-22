import { defineStore } from 'pinia'
import { ref } from 'vue'
import { apiFetch, buildQuery } from '@/api/helpers'
import { withLoading } from '@/composables/useAsyncAction'

export interface User {
  readonly id: number
  readonly name: string
  readonly email: string
  readonly oidc_sub: string
  readonly oidc_provider: string
  readonly is_admin: boolean
  readonly created_at: string
  readonly updated_at: string
}

interface ListParams {
  readonly limit?: number
  readonly offset?: number
  readonly q?: string
}

interface ListResponse {
  readonly data: User[]
  readonly meta: { readonly total: number }
}

interface SingleResponse {
  readonly data: User
}

export const useUsersStore = defineStore('users', () => {
  const users = ref<User[]>([])
  const total = ref(0)
  const loading = ref(false)
  const error = ref<string | null>(null)

  async function fetchUsers(params?: ListParams): Promise<ListResponse> {
    return withLoading(
      loading,
      error,
      async () => {
        const query = buildQuery({
          limit: params?.limit,
          offset: params?.offset,
          q: params?.q,
        })
        const response = await apiFetch<ListResponse>(`/api/admin/users${query}`)
        users.value = response.data
        total.value = response.meta.total
        return response
      },
      'Failed to fetch users',
    )
  }

  async function toggleAdmin(id: number): Promise<User> {
    return withLoading(
      loading,
      error,
      async () => {
        const response = await apiFetch<SingleResponse>(`/api/admin/users/${id}/toggle-admin`, {
          method: 'POST',
        })
        const idx = users.value.findIndex((u) => u.id === id)
        if (idx !== -1) {
          users.value[idx] = response.data
        }
        return response.data
      },
      'Failed to toggle admin',
    )
  }

  async function deleteUser(id: number): Promise<void> {
    return withLoading(
      loading,
      error,
      async () => {
        await apiFetch(`/api/admin/users/${id}`, { method: 'DELETE' })
        users.value = users.value.filter((u) => u.id !== id)
      },
      'Failed to delete user',
    )
  }

  return {
    users,
    total,
    loading,
    error,
    fetchUsers,
    toggleAdmin,
    deleteUser,
  }
})
