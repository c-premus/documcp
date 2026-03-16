import { defineStore } from 'pinia'
import { ref } from 'vue'
import { apiFetch, buildQuery } from '@/api/helpers'

export interface ExternalService {
  readonly uuid: string
  readonly name: string
  readonly slug: string
  readonly type: string
  readonly base_url: string
  readonly priority: number
  readonly status: string
  readonly is_enabled: boolean
  readonly is_env_managed: boolean
  readonly error_count: number
  readonly consecutive_failures: number
  readonly last_error?: string
  readonly last_error_at?: string
  readonly last_check_at?: string
  readonly last_latency_ms?: number
  readonly created_at?: string
  readonly updated_at?: string
}

interface ListParams {
  readonly limit?: number
  readonly offset?: number
  readonly type?: string
  readonly status?: string
}

interface ListResponse {
  readonly data: ExternalService[]
  readonly meta: {
    readonly total: number
    readonly limit: number
    readonly offset: number
  }
}

interface SingleResponse {
  readonly data: ExternalService
  readonly message?: string
}

interface MessageResponse {
  readonly message: string
}

interface CreateServicePayload {
  readonly name: string
  readonly type: string
  readonly base_url: string
  readonly api_key?: string
  readonly config?: Record<string, unknown>
  readonly priority?: number
}

interface UpdateServicePayload {
  readonly name?: string
  readonly type?: string
  readonly base_url?: string
  readonly api_key?: string
  readonly config?: Record<string, unknown>
  readonly priority?: number
  readonly is_enabled?: boolean
}


export const useExternalServicesStore = defineStore('externalServices', () => {
  const services = ref<ExternalService[]>([])
  const total = ref(0)
  const loading = ref(false)
  const error = ref<string | null>(null)

  async function fetchServices(params?: ListParams): Promise<ListResponse> {
    loading.value = true
    error.value = null
    try {
      const query = buildQuery({
        limit: params?.limit,
        offset: params?.offset,
        type: params?.type,
        status: params?.status,
      })
      const response = await apiFetch<ListResponse>(`/api/external-services${query}`)
      services.value = response.data
      total.value = response.meta.total
      return response
    } catch (e) {
      error.value = e instanceof Error ? e.message : 'Failed to fetch external services'
      throw e
    } finally {
      loading.value = false
    }
  }

  async function createService(payload: CreateServicePayload): Promise<ExternalService> {
    loading.value = true
    error.value = null
    try {
      const response = await apiFetch<SingleResponse>('/api/external-services', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify(payload),
      })
      return response.data
    } catch (e) {
      error.value = e instanceof Error ? e.message : 'Failed to create service'
      throw e
    } finally {
      loading.value = false
    }
  }

  async function updateService(uuid: string, payload: UpdateServicePayload): Promise<ExternalService> {
    loading.value = true
    error.value = null
    try {
      const response = await apiFetch<SingleResponse>(`/api/external-services/${uuid}`, {
        method: 'PUT',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify(payload),
      })
      const index = services.value.findIndex((s) => s.uuid === uuid)
      if (index !== -1) {
        services.value[index] = response.data
      }
      return response.data
    } catch (e) {
      error.value = e instanceof Error ? e.message : 'Failed to update service'
      throw e
    } finally {
      loading.value = false
    }
  }

  async function deleteService(uuid: string): Promise<MessageResponse> {
    loading.value = true
    error.value = null
    try {
      const response = await apiFetch<MessageResponse>(`/api/external-services/${uuid}`, {
        method: 'DELETE',
      })
      services.value = services.value.filter((s) => s.uuid !== uuid)
      return response
    } catch (e) {
      error.value = e instanceof Error ? e.message : 'Failed to delete service'
      throw e
    } finally {
      loading.value = false
    }
  }

  async function checkHealth(uuid: string): Promise<MessageResponse> {
    try {
      const response = await apiFetch<MessageResponse>(`/api/external-services/${uuid}/health-check`, {
        method: 'POST',
      })
      return response
    } catch (e) {
      if (e instanceof Error && e.message.includes('Not Implemented')) {
        throw new Error('Health check is not yet available')
      }
      throw e
    }
  }

  async function reorderServices(serviceIds: number[]): Promise<MessageResponse> {
    loading.value = true
    error.value = null
    try {
      const response = await apiFetch<MessageResponse>('/api/admin/external-services/reorder', {
        method: 'PUT',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ service_ids: serviceIds }),
      })
      return response
    } catch (e) {
      error.value = e instanceof Error ? e.message : 'Failed to reorder services'
      throw e
    } finally {
      loading.value = false
    }
  }

  return {
    services,
    total,
    loading,
    error,
    fetchServices,
    createService,
    updateService,
    deleteService,
    checkHealth,
    reorderServices,
  }
})
