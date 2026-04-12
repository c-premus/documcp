import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'
import { createPinia, setActivePinia } from 'pinia'
import { useSSEStore } from '@/stores/sse'
import type { SSEEvent } from '@/stores/sse'

function mockSSEEvent(overrides: Partial<SSEEvent> = {}): SSEEvent {
  return {
    type: 'job.completed',
    job_kind: 'index_document',
    job_id: 1,
    queue: 'default',
    timestamp: '2026-01-01T00:00:00Z',
    ...overrides,
  }
}

class MockEventSource {
  onopen: ((ev: Event) => void) | null = null
  onerror: ((ev: Event) => void) | null = null
  onmessage: ((ev: MessageEvent) => void) | null = null
  url: string
  close = vi.fn()
  constructor(url: string) {
    this.url = url
  }
}

let lastCreatedES: MockEventSource | null = null
let esInstanceCount = 0

function stubEventSource() {
  lastCreatedES = null
  esInstanceCount = 0

  class TrackedEventSource extends MockEventSource {
    constructor(url: string) {
      super(url)
      // eslint-disable-next-line @typescript-eslint/no-this-alias
      lastCreatedES = this
      esInstanceCount++
    }
  }

  vi.stubGlobal('EventSource', TrackedEventSource)
}

describe('sse store', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
    stubEventSource()
  })

  afterEach(() => {
    vi.restoreAllMocks()
    vi.unstubAllGlobals()
  })

  it('has correct initial state', () => {
    const store = useSSEStore()

    expect(store.connected).toBe(false)
    expect(store.lastEvent).toBeNull()
  })

  describe('connect', () => {
    it('creates EventSource with default URL', () => {
      const store = useSSEStore()
      store.connect()

      expect(lastCreatedES).not.toBeNull()
      expect(lastCreatedES!.url).toBe('/api/events/stream')
    })

    it('creates EventSource with custom URL', () => {
      const store = useSSEStore()
      store.connect('/custom/stream')

      expect(lastCreatedES!.url).toBe('/custom/stream')
    })

    it('sets connected to true on open', () => {
      const store = useSSEStore()
      store.connect()

      lastCreatedES!.onopen!(new Event('open'))

      expect(store.connected).toBe(true)
    })

    it('is idempotent — second call is a no-op', () => {
      const store = useSSEStore()
      store.connect()
      store.connect()

      expect(esInstanceCount).toBe(1)
    })
  })

  describe('onerror', () => {
    it('sets connected to false', () => {
      const store = useSSEStore()
      store.connect()

      lastCreatedES!.onopen!(new Event('open'))
      expect(store.connected).toBe(true)

      lastCreatedES!.onerror!(new Event('error'))
      expect(store.connected).toBe(false)
    })
  })

  describe('onmessage', () => {
    it('parses JSON and sets lastEvent', () => {
      const store = useSSEStore()
      store.connect()

      const event = mockSSEEvent()
      lastCreatedES!.onmessage!(new MessageEvent('message', { data: JSON.stringify(event) }))

      expect(store.lastEvent).toEqual(event)
    })

    it('dispatches to registered listeners matching event type', () => {
      const store = useSSEStore()
      const handler = vi.fn()
      store.on('job.completed', handler)
      store.connect()

      const event = mockSSEEvent({ type: 'job.completed' })
      lastCreatedES!.onmessage!(new MessageEvent('message', { data: JSON.stringify(event) }))

      expect(handler).toHaveBeenCalledWith(event)
    })

    it('does not call handlers for non-matching event types', () => {
      const store = useSSEStore()
      const handler = vi.fn()
      store.on('job.failed', handler)
      store.connect()

      const event = mockSSEEvent({ type: 'job.completed' })
      lastCreatedES!.onmessage!(new MessageEvent('message', { data: JSON.stringify(event) }))

      expect(handler).not.toHaveBeenCalled()
    })

    it('dispatches to multiple handlers for the same type', () => {
      const store = useSSEStore()
      const handler1 = vi.fn()
      const handler2 = vi.fn()
      store.on('job.completed', handler1)
      store.on('job.completed', handler2)
      store.connect()

      const event = mockSSEEvent({ type: 'job.completed' })
      lastCreatedES!.onmessage!(new MessageEvent('message', { data: JSON.stringify(event) }))

      expect(handler1).toHaveBeenCalledWith(event)
      expect(handler2).toHaveBeenCalledWith(event)
    })
  })

  describe('on', () => {
    it('returns a cleanup function that removes the handler', () => {
      const store = useSSEStore()
      const handler = vi.fn()
      const cleanup = store.on('job.completed', handler)
      store.connect()

      cleanup()

      const event = mockSSEEvent({ type: 'job.completed' })
      lastCreatedES!.onmessage!(new MessageEvent('message', { data: JSON.stringify(event) }))

      expect(handler).not.toHaveBeenCalled()
    })

    it('only removes the specific handler on cleanup', () => {
      const store = useSSEStore()
      const handler1 = vi.fn()
      const handler2 = vi.fn()
      const cleanup1 = store.on('job.completed', handler1)
      store.on('job.completed', handler2)
      store.connect()

      cleanup1()

      const event = mockSSEEvent({ type: 'job.completed' })
      lastCreatedES!.onmessage!(new MessageEvent('message', { data: JSON.stringify(event) }))

      expect(handler1).not.toHaveBeenCalled()
      expect(handler2).toHaveBeenCalledWith(event)
    })
  })

  describe('disconnect', () => {
    it('calls close on EventSource and sets connected to false', () => {
      const store = useSSEStore()
      store.connect()

      const es = lastCreatedES!
      es.onopen!(new Event('open'))
      expect(store.connected).toBe(true)

      store.disconnect()

      expect(es.close).toHaveBeenCalled()
      expect(store.connected).toBe(false)
    })

    it('allows reconnecting after disconnect', () => {
      const store = useSSEStore()
      store.connect()
      store.disconnect()
      store.connect()

      expect(esInstanceCount).toBe(2)
    })

    it('is safe to call when not connected', () => {
      const store = useSSEStore()
      expect(() => store.disconnect()).not.toThrow()
      expect(store.connected).toBe(false)
    })
  })
})
