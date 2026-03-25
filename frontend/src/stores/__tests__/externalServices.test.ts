import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'
import { createPinia, setActivePinia } from 'pinia'
import { useExternalServicesStore } from '@/stores/externalServices'
import type { ExternalService } from '@/stores/externalServices'

function mockService(overrides: Partial<ExternalService> = {}): ExternalService {
  return {
    uuid: 'svc-1',
    name: 'Test Service',
    slug: 'test-service',
    type: 'llm',
    base_url: 'https://api.example.com',
    priority: 1,
    status: 'healthy',
    is_enabled: true,
    is_env_managed: false,
    error_count: 0,
    consecutive_failures: 0,
    created_at: '2026-01-01T00:00:00Z',
    updated_at: '2026-01-01T00:00:00Z',
    ...overrides,
  }
}

function stubFetch(response: unknown, ok = true) {
  vi.stubGlobal(
    'fetch',
    vi.fn().mockResolvedValue({
      ok,
      status: ok ? 200 : 500,
      statusText: 'Internal Server Error',
      json: () => Promise.resolve(response),
    }),
  )
}

describe('externalServices store', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
    vi.stubGlobal('fetch', vi.fn())
  })

  afterEach(() => {
    vi.restoreAllMocks()
    vi.unstubAllGlobals()
  })

  it('has correct initial state', () => {
    const store = useExternalServicesStore()

    expect(store.services).toEqual([])
    expect(store.total).toBe(0)
    expect(store.loading).toBe(false)
    expect(store.error).toBeNull()
  })

  describe('fetchServices', () => {
    it('calls correct URL with query params and sets services/total', async () => {
      const svcs = [mockService(), mockService({ uuid: 'svc-2', name: 'Second' })]
      stubFetch({ data: svcs, meta: { total: 2, limit: 10, offset: 0 } })

      const store = useExternalServicesStore()
      await store.fetchServices({ limit: 10, offset: 0, type: 'llm' })

      const calledUrl = (fetch as ReturnType<typeof vi.fn>).mock.calls[0]![0] as string
      expect(calledUrl).toContain('/api/external-services?')
      expect(calledUrl).toContain('limit=10')
      expect(calledUrl).toContain('offset=0')
      expect(calledUrl).toContain('type=llm')
      expect(store.services).toEqual(svcs)
      expect(store.total).toBe(2)
      expect(store.loading).toBe(false)
    })

    it('calls URL without query when no params', async () => {
      stubFetch({ data: [], meta: { total: 0, limit: 10, offset: 0 } })

      const store = useExternalServicesStore()
      await store.fetchServices()

      expect(fetch).toHaveBeenCalledWith('/api/external-services', undefined)
    })

    it('sets loading true during fetch', async () => {
      let resolvePromise: (value: unknown) => void
      vi.stubGlobal(
        'fetch',
        vi.fn().mockReturnValue(
          new Promise((resolve) => {
            resolvePromise = resolve
          }),
        ),
      )

      const store = useExternalServicesStore()
      const promise = store.fetchServices()

      expect(store.loading).toBe(true)

      resolvePromise!({
        ok: true,
        json: () => Promise.resolve({ data: [], meta: { total: 0, limit: 10, offset: 0 } }),
      })
      await promise

      expect(store.loading).toBe(false)
    })

    it('sets error on failure', async () => {
      stubFetch({ message: 'Unauthorized' }, false)

      const store = useExternalServicesStore()
      await expect(store.fetchServices()).rejects.toThrow('Unauthorized')

      expect(store.error).toBe('Unauthorized')
      expect(store.loading).toBe(false)
    })
  })

  describe('createService', () => {
    it('POSTs payload to /api/external-services', async () => {
      const svc = mockService()
      stubFetch({ data: svc })

      const store = useExternalServicesStore()
      const payload = { name: 'New Service', type: 'llm', base_url: 'https://api.new.com' }
      const result = await store.createService(payload)

      expect(fetch).toHaveBeenCalledWith('/api/external-services', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify(payload),
      })
      expect(result).toEqual(svc)
    })

    it('sets error on failure', async () => {
      stubFetch({ message: 'Validation failed' }, false)

      const store = useExternalServicesStore()
      await expect(store.createService({ name: '', type: 'llm', base_url: '' })).rejects.toThrow(
        'Validation failed',
      )

      expect(store.error).toBe('Validation failed')
    })
  })

  describe('updateService', () => {
    it('PUTs payload and updates local array', async () => {
      const updated = mockService({ uuid: 'svc-1', name: 'Updated Name' })
      stubFetch({ data: updated })

      const store = useExternalServicesStore()
      store.$patch({ services: [mockService({ uuid: 'svc-1' }), mockService({ uuid: 'svc-2' })] })

      const result = await store.updateService('svc-1', { name: 'Updated Name' })

      expect(fetch).toHaveBeenCalledWith('/api/external-services/svc-1', {
        method: 'PUT',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ name: 'Updated Name' }),
      })
      expect(result).toEqual(updated)
      expect(store.services[0]).toEqual(updated)
      expect(store.services).toHaveLength(2)
    })

    it('does not modify array when uuid not found', async () => {
      const updated = mockService({ uuid: 'svc-unknown' })
      stubFetch({ data: updated })

      const store = useExternalServicesStore()
      store.$patch({ services: [mockService({ uuid: 'svc-1' })] })

      await store.updateService('svc-unknown', { name: 'X' })

      expect(store.services[0]!.uuid).toBe('svc-1')
    })

    it('sets error on failure', async () => {
      stubFetch({ message: 'Not found' }, false)

      const store = useExternalServicesStore()
      await expect(store.updateService('bad-id', { name: 'X' })).rejects.toThrow('Not found')

      expect(store.error).toBe('Not found')
    })
  })

  describe('deleteService', () => {
    it('DELETEs and removes from local array', async () => {
      stubFetch({ message: 'Deleted' })

      const store = useExternalServicesStore()
      store.$patch({ services: [mockService({ uuid: 'svc-1' }), mockService({ uuid: 'svc-2' })] })

      await store.deleteService('svc-1')

      expect(fetch).toHaveBeenCalledWith('/api/external-services/svc-1', { method: 'DELETE' })
      expect(store.services).toHaveLength(1)
      expect(store.services[0]!.uuid).toBe('svc-2')
    })

    it('sets error on failure', async () => {
      stubFetch({ message: 'Cannot delete' }, false)

      const store = useExternalServicesStore()
      await expect(store.deleteService('svc-1')).rejects.toThrow('Cannot delete')

      expect(store.error).toBe('Cannot delete')
    })
  })

  describe('checkHealth', () => {
    it('POSTs to health-check endpoint', async () => {
      stubFetch({ message: 'OK' })

      const store = useExternalServicesStore()
      const result = await store.checkHealth('svc-1')

      expect(fetch).toHaveBeenCalledWith('/api/external-services/svc-1/health-check', {
        method: 'POST',
      })
      expect(result).toEqual({ message: 'OK' })
    })

    it('throws user-friendly error for Not Implemented', async () => {
      stubFetch({ message: 'Not Implemented' }, false)

      const store = useExternalServicesStore()
      await expect(store.checkHealth('svc-1')).rejects.toThrow('Health check is not yet available')
    })
  })

  describe('reorderServices', () => {
    it('PUTs service_ids to reorder endpoint', async () => {
      stubFetch({ message: 'Reordered' })

      const store = useExternalServicesStore()
      const result = await store.reorderServices([3, 1, 2])

      expect(fetch).toHaveBeenCalledWith('/api/admin/external-services/reorder', {
        method: 'PUT',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ service_ids: [3, 1, 2] }),
      })
      expect(result).toEqual({ message: 'Reordered' })
    })

    it('sets error on failure', async () => {
      stubFetch({ message: 'Forbidden' }, false)

      const store = useExternalServicesStore()
      await expect(store.reorderServices([1, 2])).rejects.toThrow('Forbidden')

      expect(store.error).toBe('Forbidden')
    })
  })

  describe('error handling', () => {
    it('uses statusText when response body has no message', async () => {
      vi.stubGlobal(
        'fetch',
        vi.fn().mockResolvedValue({
          ok: false,
          status: 500,
          statusText: 'Internal Server Error',
          json: () => Promise.reject(new Error('parse error')),
        }),
      )

      const store = useExternalServicesStore()
      await expect(store.fetchServices()).rejects.toThrow('Internal Server Error')

      expect(store.error).toBe('Internal Server Error')
    })

    it('clears error before each request', async () => {
      const store = useExternalServicesStore()
      store.$patch({ error: 'previous error' })

      stubFetch({ data: [], meta: { total: 0, limit: 10, offset: 0 } })
      await store.fetchServices()

      expect(store.error).toBeNull()
    })
  })
})
