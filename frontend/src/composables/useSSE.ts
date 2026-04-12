import { ref, onUnmounted } from 'vue'

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

export function useSSE(url = '/api/events/stream') {
  const connected = ref(false)
  const lastEvent = ref<SSEEvent | null>(null)
  let eventSource: EventSource | null = null
  let reconnectDelay = 1000
  let reconnectTimer: ReturnType<typeof setTimeout> | null = null
  const listeners = new Map<string, Set<(event: SSEEvent) => void>>()

  function connect() {
    if (eventSource !== null) return
    eventSource = new EventSource(url, { withCredentials: true })

    eventSource.onopen = () => {
      connected.value = true
      reconnectDelay = 1000
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
        return
      }
      if (typeof event.type !== 'string') return
      lastEvent.value = event
      const handlers = listeners.get(event.type)
      if (handlers) {
        handlers.forEach((fn) => fn(event))
      }
    }
  }

  function scheduleReconnect() {
    if (reconnectTimer !== null) return
    reconnectTimer = setTimeout(() => {
      reconnectTimer = null
      reconnectDelay = Math.min(reconnectDelay * 2, 30000)
      connect()
    }, reconnectDelay)
  }

  function on(eventType: string, handler: (event: SSEEvent) => void) {
    if (!listeners.has(eventType)) {
      listeners.set(eventType, new Set())
    }
    listeners.get(eventType)!.add(handler)
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

  onUnmounted(disconnect)

  return { connected, lastEvent, connect, disconnect, on }
}
