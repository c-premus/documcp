import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'
import { createPinia, setActivePinia } from 'pinia'
import { useQueueStore } from '@/stores/queue'
import type { QueueStats, FailedJob } from '@/stores/queue'

function mockStats(overrides: Partial<QueueStats> = {}): QueueStats {
  return {
    available: 5,
    running: 2,
    retryable: 1,
    discarded: 0,
    cancelled: 0,
    ...overrides,
  }
}

function mockFailedJob(overrides: Partial<FailedJob> = {}): FailedJob {
  return {
    id: 1,
    kind: 'document.process',
    queue: 'default',
    state: 'retryable',
    attempt: 3,
    max_attempts: 5,
    created_at: '2026-01-01T00:00:00Z',
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

describe('queue store', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
    vi.stubGlobal('fetch', vi.fn())
  })

  afterEach(() => {
    vi.restoreAllMocks()
    vi.unstubAllGlobals()
  })

  it('has correct initial state', () => {
    const store = useQueueStore()

    expect(store.stats).toBeNull()
    expect(store.failedJobs).toEqual([])
    expect(store.failedCount).toBe(0)
    expect(store.loading).toBe(false)
  })

  describe('fetchStats', () => {
    it('calls correct URL and sets stats', async () => {
      const stats = mockStats()
      stubFetch({ data: stats })

      const store = useQueueStore()
      const result = await store.fetchStats()

      expect(fetch).toHaveBeenCalledWith('/api/admin/queue/stats', undefined)
      expect(store.stats).toEqual(stats)
      expect(result).toEqual(stats)
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

      const store = useQueueStore()
      const promise = store.fetchStats()

      expect(store.loading).toBe(true)

      resolvePromise!({
        ok: true,
        json: () => Promise.resolve({ data: mockStats() }),
      })
      await promise

      expect(store.loading).toBe(false)
    })

    it('sets loading false even on failure', async () => {
      stubFetch({ message: 'Server error' }, false)

      const store = useQueueStore()
      await expect(store.fetchStats()).rejects.toThrow('Server error')

      expect(store.loading).toBe(false)
    })
  })

  describe('fetchFailedJobs', () => {
    it('calls correct URL and sets failedJobs/failedCount', async () => {
      const jobs = [mockFailedJob(), mockFailedJob({ id: 2 })]
      stubFetch({ data: jobs, meta: { count: 2 } })

      const store = useQueueStore()
      const result = await store.fetchFailedJobs()

      expect(fetch).toHaveBeenCalledWith('/api/admin/queue/failed', undefined)
      expect(store.failedJobs).toEqual(jobs)
      expect(store.failedCount).toBe(2)
      expect(result).toEqual({ data: jobs, meta: { count: 2 } })
    })

    it('appends limit query param when provided', async () => {
      stubFetch({ data: [], meta: { count: 0 } })

      const store = useQueueStore()
      await store.fetchFailedJobs(25)

      expect(fetch).toHaveBeenCalledWith('/api/admin/queue/failed?limit=25', undefined)
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

      const store = useQueueStore()
      const promise = store.fetchFailedJobs()

      expect(store.loading).toBe(true)

      resolvePromise!({
        ok: true,
        json: () => Promise.resolve({ data: [], meta: { count: 0 } }),
      })
      await promise

      expect(store.loading).toBe(false)
    })
  })

  describe('retryJob', () => {
    it('POSTs to retry endpoint', async () => {
      stubFetch({ id: 42, state: 'available' })

      const store = useQueueStore()
      await store.retryJob(42)

      expect(fetch).toHaveBeenCalledWith('/api/admin/queue/failed/42/retry', {
        method: 'POST',
      })
    })

    it('throws on failure', async () => {
      stubFetch({ message: 'Job not found' }, false)

      const store = useQueueStore()
      await expect(store.retryJob(999)).rejects.toThrow('Job not found')
    })
  })

  describe('deleteJob', () => {
    it('DELETEs failed job by id', async () => {
      stubFetch({ id: 42, state: 'cancelled' })

      const store = useQueueStore()
      await store.deleteJob(42)

      expect(fetch).toHaveBeenCalledWith('/api/admin/queue/failed/42', {
        method: 'DELETE',
      })
    })

    it('throws on failure', async () => {
      stubFetch({ message: 'Job not found' }, false)

      const store = useQueueStore()
      await expect(store.deleteJob(999)).rejects.toThrow('Job not found')
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

      const store = useQueueStore()
      await expect(store.fetchStats()).rejects.toThrow('Internal Server Error')
    })
  })
})
