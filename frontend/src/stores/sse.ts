import { defineStore } from 'pinia'
import { ref } from 'vue'

export interface SSEEvent {
  readonly type: string
  readonly job_kind: string
  readonly job_id: number
  readonly queue: string
  readonly attempt?: number
  readonly error?: string
  readonly timestamp: string
  readonly user_id?: number
  readonly doc_uuid?: string
}

/**
 * Singleton SSE store — exactly one EventSource connection for the whole app.
 * AppLayout calls connect() once on mount; all other components read state or
 * register listeners via on(). Multiple calls to connect() are idempotent.
 */
export const useSSEStore = defineStore('sse', () => {
  const connected = ref(false)
  const lastEvent = ref<SSEEvent | null>(null)
  let eventSource: EventSource | null = null
  let reconnectDelay = 1000
  let reconnectTimer: ReturnType<typeof setTimeout> | null = null
  let currentUrl = '/api/events/stream'
  const listeners = new Map<string, Set<(event: SSEEvent) => void>>()

  function connect(url = '/api/events/stream') {
    if (eventSource !== null) return // already connected
    currentUrl = url

    eventSource = new EventSource(url, { withCredentials: true })

    eventSource.onopen = () => {
      connected.value = true
      reconnectDelay = 1000 // reset backoff on successful connection
    }
    eventSource.onerror = () => {
      connected.value = false
      eventSource?.close()
      eventSource = null
      scheduleReconnect()
    }
    eventSource.onmessage = (e: MessageEvent) => {
      let event: SSEEvent
      try {
        event = JSON.parse(e.data as string) as SSEEvent
      } catch {
        return // ignore malformed messages
      }
      if (typeof event.type !== 'string') return
      lastEvent.value = event
      listeners.get(event.type)?.forEach((fn) => fn(event))
    }
  }

  function scheduleReconnect() {
    if (reconnectTimer !== null) return
    reconnectTimer = setTimeout(() => {
      reconnectTimer = null
      reconnectDelay = Math.min(reconnectDelay * 2, 30000) // exponential backoff, cap 30s
      connect(currentUrl)
    }, reconnectDelay)
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
    if (reconnectTimer !== null) {
      clearTimeout(reconnectTimer)
      reconnectTimer = null
    }
    eventSource?.close()
    eventSource = null
    connected.value = false
  }

  return { connected, lastEvent, connect, disconnect, on }
})
