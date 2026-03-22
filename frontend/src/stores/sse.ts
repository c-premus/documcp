import { defineStore } from 'pinia'
import { ref } from 'vue'
import type { SSEEvent } from '@/composables/useSSE'

export type { SSEEvent }

/**
 * Singleton SSE store — exactly one EventSource connection for the whole app.
 * AppLayout calls connect() once on mount; all other components read state or
 * register listeners via on(). Multiple calls to connect() are idempotent.
 */
export const useSSEStore = defineStore('sse', () => {
  const connected = ref(false)
  const lastEvent = ref<SSEEvent | null>(null)
  let eventSource: EventSource | null = null
  const listeners = new Map<string, Set<(event: SSEEvent) => void>>()

  function connect(url = '/api/admin/events/stream') {
    if (eventSource !== null) return // already connected

    eventSource = new EventSource(url)

    eventSource.onopen = () => {
      connected.value = true
    }
    eventSource.onerror = () => {
      connected.value = false
    }
    eventSource.onmessage = (e: MessageEvent) => {
      const event = JSON.parse(e.data as string) as SSEEvent
      lastEvent.value = event
      listeners.get(event.type)?.forEach((fn) => fn(event))
    }
  }

  /**
   * Register a handler for a named event type. Returns a cleanup function that
   * removes the handler — call it from onUnmounted to avoid memory leaks.
   */
  function on(eventType: string, handler: (event: SSEEvent) => void): () => void {
    if (!listeners.has(eventType)) {
      listeners.set(eventType, new Set())
    }
    listeners.get(eventType)!.add(handler)
    return () => {
      listeners.get(eventType)?.delete(handler)
    }
  }

  function disconnect() {
    eventSource?.close()
    eventSource = null
    connected.value = false
  }

  return { connected, lastEvent, connect, disconnect, on }
})
