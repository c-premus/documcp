import { defineStore } from 'pinia'
import { ref } from 'vue'
import { apiFetch, buildQuery } from '@/api/helpers'
import { withLoading } from '@/composables/useAsyncAction'

export interface OAuthClient {
  readonly id: number
  readonly client_id: string
  readonly client_name: string
  readonly redirect_uris: string[]
  readonly grant_types: string[]
  readonly response_types: string[]
  readonly token_endpoint_auth_method: string
  readonly scope: string
  readonly last_used_at: string | null
  readonly created_at: string
  readonly updated_at: string
}

export interface ScopeGrant {
  readonly id: number
  readonly scope: string
  readonly granted_by: number
  readonly granted_by_email: string | null
  readonly granted_by_name: string | null
  readonly granted_at: string
  readonly expires_at: string | null
}

export interface CreateClientRequest {
  readonly client_name: string
  readonly redirect_uris: string[]
  readonly grant_types: string[]
  readonly token_endpoint_auth_method: string
  readonly scope: string
}

export interface CreatedClient {
  readonly id: number
  readonly client_id: string
  readonly client_secret: string
  readonly client_name: string
}

interface ListParams {
  readonly limit?: number
  readonly offset?: number
  readonly q?: string
}

interface ListResponse {
  readonly data: OAuthClient[]
  readonly meta: {
    readonly total: number
    readonly limit: number
    readonly offset: number
  }
}

interface ShowResponse {
  readonly data: OAuthClient
}

interface ScopeGrantsResponse {
  readonly data: ScopeGrant[]
}

interface CreateResponse {
  readonly data: CreatedClient
  readonly message: string
}

export const useOAuthClientsStore = defineStore('oauthClients', () => {
  const clients = ref<OAuthClient[]>([])
  const total = ref(0)
  const loading = ref(false)
  const error = ref<string | null>(null)
  const currentClient = ref<OAuthClient | null>(null)
  const scopeGrants = ref<ScopeGrant[]>([])

  async function fetchClients(params?: ListParams): Promise<ListResponse> {
    return withLoading(
      loading,
      error,
      async () => {
        const query = buildQuery({
          limit: params?.limit,
          offset: params?.offset,
          q: params?.q,
        })
        const response = await apiFetch<ListResponse>(`/api/admin/oauth-clients${query}`)
        clients.value = response.data
        total.value = response.meta.total
        return response
      },
      'Failed to fetch OAuth clients',
    )
  }

  async function createClient(request: CreateClientRequest): Promise<CreatedClient> {
    return withLoading(
      loading,
      error,
      async () => {
        const response = await apiFetch<CreateResponse>('/api/admin/oauth-clients', {
          method: 'POST',
          headers: { 'Content-Type': 'application/json' },
          body: JSON.stringify(request),
        })
        return response.data
      },
      'Failed to create OAuth client',
    )
  }

  async function deleteClient(id: number): Promise<void> {
    return withLoading(
      loading,
      error,
      async () => {
        await apiFetch(`/api/admin/oauth-clients/${id}`, {
          method: 'DELETE',
        })
        clients.value = clients.value.filter((c) => c.id !== id)
      },
      'Failed to delete OAuth client',
    )
  }

  async function fetchClient(id: number): Promise<OAuthClient> {
    return withLoading(
      loading,
      error,
      async () => {
        const response = await apiFetch<ShowResponse>(`/api/admin/oauth-clients/${id}`)
        currentClient.value = response.data
        return response.data
      },
      'Failed to fetch OAuth client',
    )
  }

  async function fetchScopeGrants(clientId: number): Promise<ScopeGrant[]> {
    try {
      const response = await apiFetch<ScopeGrantsResponse>(
        `/api/admin/oauth-clients/${clientId}/scope-grants`,
      )
      scopeGrants.value = response.data
      return response.data
    } catch (e) {
      error.value = e instanceof Error ? e.message : 'Failed to fetch scope grants'
      throw e
    }
  }

  async function revokeScopeGrant(clientId: number, grantId: number): Promise<void> {
    try {
      await apiFetch(`/api/admin/oauth-clients/${clientId}/scope-grants/${grantId}`, {
        method: 'DELETE',
      })
      scopeGrants.value = scopeGrants.value.filter((g) => g.id !== grantId)
    } catch (e) {
      error.value = e instanceof Error ? e.message : 'Failed to revoke scope grant'
      throw e
    }
  }

  return {
    clients,
    total,
    loading,
    error,
    currentClient,
    scopeGrants,
    fetchClients,
    fetchClient,
    fetchScopeGrants,
    revokeScopeGrant,
    createClient,
    deleteClient,
  }
})
