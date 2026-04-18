import { defineStore } from 'pinia'
import { ref } from 'vue'
import { apiFetch, buildQuery } from '@/api/helpers'
import { withLoading } from '@/composables/useAsyncAction'

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
  const loaded = ref(false)
  const error = ref<string | null>(null)

  async function fetchServices(params?: ListParams): Promise<ListResponse> {
    return withLoading(
      loading,
      error,
      async () => {
        const query = buildQuery({
          limit: params?.limit,
          offset: params?.offset,
          type: params?.type,
          status: params?.status,
        })
        const response = await apiFetch<ListResponse>(`/api/external-services${query}`)
        services.value = response.data
        total.value = response.meta.total
        loaded.value = true
        return response
      },
      'Failed to fetch external services',
    )
  }

  async function createService(payload: CreateServicePayload): Promise<ExternalService> {
    return withLoading(
      loading,
      error,
      async () => {
        const response = await apiFetch<SingleResponse>('/api/external-services', {
          method: 'POST',
          headers: { 'Content-Type': 'application/json' },
          body: JSON.stringify(payload),
        })
        services.value = [response.data, ...services.value]
        total.value += 1
        return response.data
      },
      'Failed to create service',
    )
  }

  async function updateService(
    uuid: string,
    payload: UpdateServicePayload,
  ): Promise<ExternalService> {
    return withLoading(
      loading,
      error,
      async () => {
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
      },
      'Failed to update service',
    )
  }

  async function deleteService(uuid: string): Promise<MessageResponse> {
    return withLoading(
      loading,
      error,
      async () => {
        const response = await apiFetch<MessageResponse>(`/api/external-services/${uuid}`, {
          method: 'DELETE',
        })
        services.value = services.value.filter((s) => s.uuid !== uuid)
        return response
      },
      'Failed to delete service',
    )
  }

  async function syncService(uuid: string): Promise<MessageResponse> {
    try {
      const response = await apiFetch<MessageResponse>(`/api/external-services/${uuid}/sync`, {
        method: 'POST',
      })
      return response
    } catch (e) {
      error.value = e instanceof Error ? e.message : 'Failed to enqueue sync'
      throw e
    }
  }

  async function checkHealth(uuid: string): Promise<MessageResponse> {
    try {
      const response = await apiFetch<MessageResponse>(
        `/api/external-services/${uuid}/health-check`,
        {
          method: 'POST',
        },
      )
      return response
    } catch (e) {
      if (e instanceof Error && e.message.includes('Not Implemented')) {
        throw new Error('Health check is not yet available', { cause: e })
      }
      throw e
    }
  }

  async function reorderServices(
    order: ReadonlyArray<{ readonly uuid: string; readonly priority: number }>,
  ): Promise<MessageResponse> {
    return withLoading(
      loading,
      error,
      async () => {
        return apiFetch<MessageResponse>('/api/admin/external-services/reorder', {
          method: 'PUT',
          headers: { 'Content-Type': 'application/json' },
          body: JSON.stringify({ order }),
        })
      },
      'Failed to reorder services',
    )
  }

  return {
    services,
    total,
    loading,
    loaded,
    error,
    fetchServices,
    createService,
    updateService,
    deleteService,
    syncService,
    checkHealth,
    reorderServices,
  }
})
