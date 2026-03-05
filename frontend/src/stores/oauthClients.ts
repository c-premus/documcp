import { defineStore } from 'pinia'
import { ref } from 'vue'

export interface OAuthClient {
  readonly id: number
  readonly client_id: string
  readonly client_name: string
  readonly redirect_uris: string[]
  readonly grant_types: string[]
  readonly response_types: string[]
  readonly token_endpoint_auth_method: string
  readonly scope: string
  readonly is_active: boolean
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

interface RevokeResponse {
  readonly message: string
}

async function api<T>(url: string, options?: RequestInit): Promise<T> {
  const res = await fetch(url, options)
  if (!res.ok) {
    const body = await res.json().catch(() => ({ message: res.statusText }))
    throw new Error(body.message || res.statusText)
  }
  return res.json() as Promise<T>
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
      const response = await api<ListResponse>(`/api/admin/oauth-clients${query}`)
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
      const response = await api<CreateResponse>('/api/admin/oauth-clients', {
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

  async function revokeClient(id: number): Promise<RevokeResponse> {
    loading.value = true
    error.value = null
    try {
      const response = await api<RevokeResponse>(`/api/admin/oauth-clients/${id}/revoke`, {
        method: 'POST',
      })
      const index = clients.value.findIndex((c) => c.id === id)
      if (index !== -1) {
        clients.value[index] = { ...clients.value[index], is_active: false } as OAuthClient
      }
      return response
    } catch (e) {
      error.value = e instanceof Error ? e.message : 'Failed to revoke OAuth client'
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
    revokeClient,
  }
})
