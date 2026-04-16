import { defineStore } from 'pinia'
import { ref } from 'vue'
import type { SSEEvent } from '@/stores/sse'

export interface Notification {
  readonly id: string
  readonly type: 'success' | 'error' | 'info' | 'warning'
  readonly title: string
  readonly message?: string
  readonly timestamp: Date
}

export const useNotificationsStore = defineStore('notifications', () => {
  const events = ref<SSEEvent[]>([])
  const notifications = ref<Notification[]>([])

  function addEvent(event: SSEEvent) {
    events.value.push(event)
    if (events.value.length > 100) {
      events.value = events.value.slice(-50)
    }
  }

  function add(notification: Omit<Notification, 'id' | 'timestamp'>) {
    notifications.value.push({
      ...notification,
      id: crypto.randomUUID(),
      timestamp: new Date(),
    })
  }

  function remove(id: string) {
    notifications.value = notifications.value.filter((n) => n.id !== id)
  }

  function clear() {
    events.value = []
  }

  return { events, notifications, addEvent, add, remove, clear }
})
