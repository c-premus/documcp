import { describe, it, expect, beforeEach } from 'vitest'
import { createPinia, setActivePinia } from 'pinia'
import { useNotificationsStore } from '@/stores/notifications'
import type { SSEEvent } from '@/stores/sse'

function mockEvent(overrides: Partial<SSEEvent> = {}): SSEEvent {
  return {
    type: 'job.completed',
    job_kind: 'sync',
    job_id: 1,
    queue: 'default',
    timestamp: '2026-01-01T00:00:00Z',
    ...overrides,
  }
}

describe('notifications store', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
  })

  it('has correct initial state', () => {
    const store = useNotificationsStore()

    expect(store.events).toEqual([])
  })

  describe('addEvent', () => {
    it('adds SSE event to events array', () => {
      const store = useNotificationsStore()
      const event = mockEvent()

      store.addEvent(event)

      expect(store.events).toHaveLength(1)
      expect(store.events[0]).toEqual(event)
    })

    it('accumulates multiple events', () => {
      const store = useNotificationsStore()

      store.addEvent(mockEvent({ job_id: 1 }))
      store.addEvent(mockEvent({ job_id: 2 }))
      store.addEvent(mockEvent({ job_id: 3 }))

      expect(store.events).toHaveLength(3)
    })

    it('trims array to last 50 when exceeding 100 events', () => {
      const store = useNotificationsStore()

      for (let i = 0; i < 101; i++) {
        store.addEvent(mockEvent({ job_id: i }))
      }

      expect(store.events).toHaveLength(50)
      // Should keep the last 50 (ids 51-100)
      expect(store.events[0]!.job_id).toBe(51)
      expect(store.events[49]!.job_id).toBe(100)
    })

    it('does not trim at exactly 100 events', () => {
      const store = useNotificationsStore()

      for (let i = 0; i < 100; i++) {
        store.addEvent(mockEvent({ job_id: i }))
      }

      expect(store.events).toHaveLength(100)
    })
  })

  describe('clear', () => {
    it('empties events array', () => {
      const store = useNotificationsStore()

      store.addEvent(mockEvent({ job_id: 1 }))
      store.addEvent(mockEvent({ job_id: 2 }))
      expect(store.events).toHaveLength(2)

      store.clear()

      expect(store.events).toEqual([])
    })
  })
})
