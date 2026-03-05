import { defineStore } from 'pinia'
import { ref } from 'vue'
import type { SSEEvent } from '@/composables/useSSE'

export const useNotificationsStore = defineStore('notifications', () => {
  const events = ref<SSEEvent[]>([])

  function addEvent(event: SSEEvent) {
    events.value.push(event)
    if (events.value.length > 100) {
      events.value = events.value.slice(-50)
    }
  }

  function clear() {
    events.value = []
  }

  return { events, addEvent, clear }
})
