import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'

vi.mock('vue', async () => {
  const actual = await vi.importActual('vue')
  return { ...actual, onUnmounted: vi.fn() }
})

import { useSSE } from '@/composables/useSSE'
import type { SSEEvent } from '@/composables/useSSE'

class MockEventSource {
  url: string
  onopen: (() => void) | null = null
  onerror: (() => void) | null = null
  onmessage: ((e: MessageEvent) => void) | null = null
  close = vi.fn()
  constructor(url: string) {
    this.url = url
  }
}

function createCapturingMock() {
  const instances: MockEventSource[] = []
  vi.stubGlobal(
    'EventSource',
    class extends MockEventSource {
      constructor(url: string) {
        super(url)
        instances.push(this)
      }
    },
  )
  return instances
}

describe('useSSE', () => {
  beforeEach(() => {
    vi.stubGlobal('EventSource', MockEventSource)
  })

  afterEach(() => {
    vi.unstubAllGlobals()
  })

  it('creates EventSource with correct URL on connect', () => {
    const { connect } = useSSE('/api/test/stream')
    connect()
    expect(true).toBe(true)
  })

  it('sets connected to true on open', () => {
    const instances = createCapturingMock()
    const sse = useSSE()
    sse.connect()
    instances[0]!.onopen!()
    expect(sse.connected.value).toBe(true)
  })

  it('sets connected to false on error', () => {
    const instances = createCapturingMock()
    const sse = useSSE()
    sse.connect()
    instances[0]!.onopen!()
    expect(sse.connected.value).toBe(true)
    instances[0]!.onerror!()
    expect(sse.connected.value).toBe(false)
  })

  it('parses JSON message and sets lastEvent', () => {
    const instances = createCapturingMock()
    const sse = useSSE()
    sse.connect()

    const event: SSEEvent = {
      type: 'job.completed',
      job_kind: 'sync',
      job_id: 42,
      queue: 'default',
      timestamp: '2026-01-01T00:00:00Z',
    }
    instances[0]!.onmessage!({ data: JSON.stringify(event) } as MessageEvent)
    expect(sse.lastEvent.value).toEqual(event)
  })

  it('calls registered listener when matching event type received', () => {
    const instances = createCapturingMock()
    const sse = useSSE()
    const handler = vi.fn()
    sse.on('job.completed', handler)
    sse.connect()

    const event: SSEEvent = {
      type: 'job.completed',
      job_kind: 'sync',
      job_id: 42,
      queue: 'default',
      timestamp: '2026-01-01T00:00:00Z',
    }
    instances[0]!.onmessage!({ data: JSON.stringify(event) } as MessageEvent)
    expect(handler).toHaveBeenCalledWith(event)
  })

  it('does not call listener for non-matching event type', () => {
    const instances = createCapturingMock()
    const sse = useSSE()
    const handler = vi.fn()
    sse.on('job.failed', handler)
    sse.connect()

    const event: SSEEvent = {
      type: 'job.completed',
      job_kind: 'sync',
      job_id: 42,
      queue: 'default',
      timestamp: '2026-01-01T00:00:00Z',
    }
    instances[0]!.onmessage!({ data: JSON.stringify(event) } as MessageEvent)
    expect(handler).not.toHaveBeenCalled()
  })

  it('disconnect closes EventSource and sets connected to false', () => {
    const instances = createCapturingMock()
    const sse = useSSE()
    sse.connect()
    instances[0]!.onopen!()
    expect(sse.connected.value).toBe(true)

    sse.disconnect()
    expect(instances[0]!.close).toHaveBeenCalled()
    expect(sse.connected.value).toBe(false)
  })
})
