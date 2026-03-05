import { ref, onUnmounted } from 'vue'

export interface SSEEvent {
  readonly type: string
  readonly job_kind: string
  readonly job_id: number
  readonly queue: string
  readonly attempt?: number
  readonly error?: string
  readonly timestamp: string
}

export function useSSE(url = '/api/events/stream') {
  const connected = ref(false)
  const lastEvent = ref<SSEEvent | null>(null)
  let eventSource: EventSource | null = null
  const listeners = new Map<string, Set<(event: SSEEvent) => void>>()

  function connect() {
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
      const handlers = listeners.get(event.type)
      if (handlers) {
        handlers.forEach((fn) => fn(event))
      }
    }
  }

  function on(eventType: string, handler: (event: SSEEvent) => void) {
    if (!listeners.has(eventType)) {
      listeners.set(eventType, new Set())
    }
    listeners.get(eventType)!.add(handler)
  }

  function disconnect() {
    eventSource?.close()
    eventSource = null
    connected.value = false
  }

  onUnmounted(disconnect)

  return { connected, lastEvent, connect, disconnect, on }
}
