import { defineStore } from 'pinia'
import { ref } from 'vue'
import { apiFetch, buildQuery } from '@/api/helpers'

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

interface CreateResponse {
  readonly data: CreatedClient
  readonly message: string
}

export const useOAuthClientsStore = defineStore('oauthClients', () => {
  const clients = ref<OAuthClient[]>([])
  const total = ref(0)
  const loading = ref(false)
  const error = ref<string | null>(null)

  async function fetchClients(params?: ListParams): Promise<ListResponse> {
    loading.value = true
    error.value = null
    try {
      const query = buildQuery({
        limit: params?.limit,
        offset: params?.offset,
        q: params?.q,
      })
      const response = await apiFetch<ListResponse>(`/api/admin/oauth-clients${query}`)
      clients.value = response.data
      total.value = response.meta.total
      return response
    } catch (e) {
      error.value = e instanceof Error ? e.message : 'Failed to fetch OAuth clients'
      throw e
    } finally {
      loading.value = false
    }
  }

  async function createClient(request: CreateClientRequest): Promise<CreatedClient> {
    loading.value = true
    error.value = null
    try {
      const response = await apiFetch<CreateResponse>('/api/admin/oauth-clients', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify(request),
      })
      return response.data
    } catch (e) {
      error.value = e instanceof Error ? e.message : 'Failed to create OAuth client'
      throw e
    } finally {
      loading.value = false
    }
  }

  async function deleteClient(id: number): Promise<void> {
    loading.value = true
    error.value = null
    try {
      await apiFetch(`/api/admin/oauth-clients/${id}`, {
        method: 'DELETE',
      })
      clients.value = clients.value.filter((c) => c.id !== id)
    } catch (e) {
      error.value = e instanceof Error ? e.message : 'Failed to delete OAuth client'
      throw e
    } finally {
      loading.value = false
    }
  }

  return {
    clients,
    total,
    loading,
    error,
    fetchClients,
    createClient,
    deleteClient,
  }
})
