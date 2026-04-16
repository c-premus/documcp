import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'
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
    vi.stubGlobal('crypto', {
      randomUUID: vi.fn().mockReturnValue('test-uuid-123'),
    })
  })

  afterEach(() => {
    vi.restoreAllMocks()
    vi.unstubAllGlobals()
  })

  it('has correct initial state', () => {
    const store = useNotificationsStore()

    expect(store.events).toEqual([])
    expect(store.notifications).toEqual([])
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

  describe('add', () => {
    it('creates notification with generated ID and timestamp', () => {
      const store = useNotificationsStore()

      store.add({ type: 'success', title: 'Upload complete' })

      expect(store.notifications).toHaveLength(1)
      expect(store.notifications[0]!.id).toBe('test-uuid-123')
      expect(store.notifications[0]!.type).toBe('success')
      expect(store.notifications[0]!.title).toBe('Upload complete')
      expect(store.notifications[0]!.timestamp).toBeInstanceOf(Date)
    })

    it('includes optional message', () => {
      const store = useNotificationsStore()

      store.add({ type: 'error', title: 'Failed', message: 'Something went wrong' })

      expect(store.notifications[0]!.message).toBe('Something went wrong')
    })

    it('supports all notification types', () => {
      const store = useNotificationsStore()
      const types = ['success', 'error', 'info', 'warning'] as const

      for (const type of types) {
        store.add({ type, title: `${type} notification` })
      }

      expect(store.notifications).toHaveLength(4)
      expect(store.notifications.map((n) => n.type)).toEqual([...types])
    })
  })

  describe('remove', () => {
    it('removes notification by ID', () => {
      const store = useNotificationsStore()

      let callCount = 0
      vi.stubGlobal('crypto', {
        randomUUID: vi.fn(() => `uuid-${callCount++}`),
      })

      store.add({ type: 'success', title: 'First' })
      store.add({ type: 'info', title: 'Second' })

      expect(store.notifications).toHaveLength(2)

      store.remove('uuid-0')

      expect(store.notifications).toHaveLength(1)
      expect(store.notifications[0]!.title).toBe('Second')
    })

    it('does nothing for non-existent ID', () => {
      const store = useNotificationsStore()

      store.add({ type: 'success', title: 'First' })
      store.remove('non-existent')

      expect(store.notifications).toHaveLength(1)
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

    it('does not affect notifications array', () => {
      const store = useNotificationsStore()

      store.add({ type: 'success', title: 'Keep me' })
      store.addEvent(mockEvent())

      store.clear()

      expect(store.events).toEqual([])
      expect(store.notifications).toHaveLength(1)
    })
  })
})
