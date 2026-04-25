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

  // Session-management state is kept in the same store but separate refs so a
  // session-modal open doesn't blank the user-list loading skeleton.
  const sessionIDs = ref<string[]>([])
  const sessionsLoading = ref(false)
  const sessionsError = ref<string | null>(null)

  async function fetchUserSessions(id: number): Promise<string[]> {
    return withLoading(
      sessionsLoading,
      sessionsError,
      async () => {
        const response = await apiFetch<{ data: string[] }>(`/api/admin/users/${id}/sessions`)
        sessionIDs.value = response.data
        return response.data
      },
      'Failed to fetch sessions',
    )
  }

  async function revokeUserSession(userID: number, sessionID: string): Promise<void> {
    return withLoading(
      sessionsLoading,
      sessionsError,
      async () => {
        await apiFetch(`/api/admin/users/${userID}/sessions/${encodeURIComponent(sessionID)}`, {
          method: 'DELETE',
        })
        sessionIDs.value = sessionIDs.value.filter((s) => s !== sessionID)
      },
      'Failed to revoke session',
    )
  }

  async function revokeAllUserSessions(userID: number): Promise<number> {
    return withLoading(
      sessionsLoading,
      sessionsError,
      async () => {
        const response = await apiFetch<{ data: { revoked: number } }>(
          `/api/admin/users/${userID}/sessions`,
          { method: 'DELETE' },
        )
        sessionIDs.value = []
        return response.data.revoked
      },
      'Failed to revoke sessions',
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
    sessionIDs,
    sessionsLoading,
    sessionsError,
    fetchUserSessions,
    revokeUserSession,
    revokeAllUserSessions,
  }
})
