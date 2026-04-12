import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'
import { createPinia, setActivePinia } from 'pinia'
import { defineComponent } from 'vue'
import { mount } from '@vue/test-utils'
import type { SSEEvent } from '@/composables/useSSE'
import { useDocumentEvents } from '@/composables/useDocumentEvents'
import { useSSEStore } from '@/stores/sse'
import { useDocumentsStore } from '@/stores/documents'
import { useZimArchivesStore } from '@/stores/zimArchives'
import { useGitTemplatesStore } from '@/stores/gitTemplates'
import { useExternalServicesStore } from '@/stores/externalServices'
import { useNotificationsStore } from '@/stores/notifications'
import { useAuthStore } from '@/stores/auth'
import { useQueueStore } from '@/stores/queue'

vi.mock('vue-sonner', () => ({
  toast: {
    success: vi.fn(),
    error: vi.fn(),
    info: vi.fn(),
  },
}))

function makeEvent(overrides: Partial<SSEEvent> = {}): SSEEvent {
  return {
    type: 'job.completed',
    job_kind: 'document_extract',
    job_id: 1,
    queue: 'default',
    timestamp: '2026-03-25T00:00:00Z',
    ...overrides,
  }
}

function mountComposable() {
  let result!: ReturnType<typeof useDocumentEvents>
  const wrapper = mount(
    defineComponent({
      setup() {
        result = useDocumentEvents()
        return {}
      },
      render() {
        return null
      },
    }),
  )
  return { result, wrapper }
}

describe('useDocumentEvents', () => {
  let sseStore: ReturnType<typeof useSSEStore>
  let onHandlers: Map<string, (event: SSEEvent) => void>

  beforeEach(() => {
    setActivePinia(createPinia())
    vi.stubGlobal('fetch', vi.fn())

    sseStore = useSSEStore()
    onHandlers = new Map()

    // Intercept sseStore.on to capture registered handlers
    vi.spyOn(sseStore, 'on').mockImplementation((eventType: string, handler) => {
      onHandlers.set(eventType, handler)
      return vi.fn()
    })
  })

  afterEach(() => {
    vi.restoreAllMocks()
    vi.unstubAllGlobals()
  })

  it('registers handlers for job.completed, job.failed, and job.snoozed', () => {
    const { result } = mountComposable()
    result.start()

    expect(sseStore.on).toHaveBeenCalledWith('job.completed', expect.any(Function))
    expect(sseStore.on).toHaveBeenCalledWith('job.failed', expect.any(Function))
    expect(sseStore.on).toHaveBeenCalledWith('job.snoozed', expect.any(Function))
  })

  it('exposes connected from sseStore', () => {
    const { result } = mountComposable()
    expect(result.connected).toBe(false)
  })

  describe('job.completed — document_extract', () => {
    it('shows success toast and refreshes documents', async () => {
      const { toast } = await import('vue-sonner')
      const documents = useDocumentsStore()
      const notifications = useNotificationsStore()
      vi.spyOn(documents, 'refreshCurrent').mockResolvedValue(null)
      vi.spyOn(notifications, 'addEvent')

      const { result } = mountComposable()
      result.start()

      const event = makeEvent({ type: 'job.completed', job_kind: 'document_extract' })
      onHandlers.get('job.completed')!(event)

      expect(toast.success).toHaveBeenCalledWith('Document extracted successfully')
      expect(notifications.addEvent).toHaveBeenCalledWith(event)
      expect(documents.refreshCurrent).toHaveBeenCalled()
    })

    it('fetches document list only when loaded flag is set (lazy refresh)', async () => {
      const documents = useDocumentsStore()
      vi.spyOn(documents, 'refreshCurrent').mockResolvedValue(null)
      vi.spyOn(documents, 'fetchDocuments').mockResolvedValue({
        data: [],
        meta: { total: 0, limit: 10, offset: 0 },
      })

      // Not loaded — should NOT fetch list
      const { result } = mountComposable()
      result.start()

      const event = makeEvent({ type: 'job.completed', job_kind: 'document_extract' })
      onHandlers.get('job.completed')!(event)
      expect(documents.fetchDocuments).not.toHaveBeenCalled()

      // With loaded flag set — should fetch list
      documents.$patch({ loaded: true })
      onHandlers.get('job.completed')!(event)
      expect(documents.fetchDocuments).toHaveBeenCalled()
    })
  })

  describe('job.completed — scheduler jobs', () => {
    it('refreshes zimArchives and externalServices on sync_kiwix when loaded', async () => {
      const { toast } = await import('vue-sonner')
      const zimArchives = useZimArchivesStore()
      const externalServices = useExternalServicesStore()
      zimArchives.$patch({ loaded: true })
      externalServices.$patch({ loaded: true })
      vi.spyOn(zimArchives, 'fetchArchives').mockResolvedValue({ data: [], meta: { total: 0 } })
      vi.spyOn(externalServices, 'fetchServices').mockResolvedValue({
        data: [],
        meta: { total: 0, limit: 10, offset: 0 },
      })

      const { result } = mountComposable()
      result.start()

      onHandlers.get('job.completed')!(makeEvent({ type: 'job.completed', job_kind: 'sync_kiwix' }))

      expect(zimArchives.fetchArchives).toHaveBeenCalled()
      expect(externalServices.fetchServices).toHaveBeenCalled()
      expect(toast.info).toHaveBeenCalledWith('Kiwix sync completed')
    })

    it('does not refresh zimArchives or externalServices on sync_kiwix when not loaded', () => {
      const zimArchives = useZimArchivesStore()
      const externalServices = useExternalServicesStore()
      vi.spyOn(zimArchives, 'fetchArchives').mockResolvedValue({ data: [], meta: { total: 0 } })
      vi.spyOn(externalServices, 'fetchServices').mockResolvedValue({
        data: [],
        meta: { total: 0, limit: 10, offset: 0 },
      })

      const { result } = mountComposable()
      result.start()

      onHandlers.get('job.completed')!(makeEvent({ type: 'job.completed', job_kind: 'sync_kiwix' }))

      expect(zimArchives.fetchArchives).not.toHaveBeenCalled()
      expect(externalServices.fetchServices).not.toHaveBeenCalled()
    })

    it('refreshes gitTemplates on sync_git_templates when loaded', async () => {
      const gitTemplates = useGitTemplatesStore()
      gitTemplates.$patch({ loaded: true })
      vi.spyOn(gitTemplates, 'fetchTemplates').mockResolvedValue({
        data: [],
        meta: { total: 0, limit: 50, offset: 0 },
      })

      const { result } = mountComposable()
      result.start()

      onHandlers.get('job.completed')!(
        makeEvent({ type: 'job.completed', job_kind: 'sync_git_templates' }),
      )

      expect(gitTemplates.fetchTemplates).toHaveBeenCalled()
    })

    it('refreshes externalServices on health_check_services when loaded', async () => {
      const externalServices = useExternalServicesStore()
      externalServices.$patch({ loaded: true })
      vi.spyOn(externalServices, 'fetchServices').mockResolvedValue({
        data: [],
        meta: { total: 0, limit: 10, offset: 0 },
      })

      const { result } = mountComposable()
      result.start()

      onHandlers.get('job.completed')!(
        makeEvent({ type: 'job.completed', job_kind: 'health_check_services' }),
      )

      expect(externalServices.fetchServices).toHaveBeenCalled()
    })
  })

  describe('job.completed — queue stats refresh', () => {
    it('refreshes queue stats when stats are loaded', () => {
      const auth = useAuthStore()
      auth.user = { id: 1, email: 'admin@test.com', name: 'Admin', is_admin: true }

      const queue = useQueueStore()
      queue.$patch({
        stats: { available: 0, running: 0, retryable: 0, discarded: 0, cancelled: 0 },
      })
      vi.spyOn(queue, 'fetchStats').mockResolvedValue({
        available: 0,
        running: 0,
        retryable: 0,
        discarded: 0,
        cancelled: 0,
      })

      const { result } = mountComposable()
      result.start()

      onHandlers.get('job.completed')!(makeEvent({ type: 'job.completed', job_kind: 'sync_kiwix' }))

      expect(queue.fetchStats).toHaveBeenCalled()
    })

    it('does not refresh queue stats when stats are not loaded', () => {
      const queue = useQueueStore()
      vi.spyOn(queue, 'fetchStats').mockResolvedValue({
        available: 0,
        running: 0,
        retryable: 0,
        discarded: 0,
        cancelled: 0,
      })

      const { result } = mountComposable()
      result.start()

      onHandlers.get('job.completed')!(makeEvent({ type: 'job.completed', job_kind: 'sync_kiwix' }))

      expect(queue.fetchStats).not.toHaveBeenCalled()
    })
  })

  describe('job.failed', () => {
    it('shows error toast for document failures', async () => {
      const { toast } = await import('vue-sonner')
      const notifications = useNotificationsStore()
      vi.spyOn(notifications, 'addEvent')

      const { result } = mountComposable()
      result.start()

      const event = makeEvent({
        type: 'job.failed',
        job_kind: 'document_extract',
        error: 'extraction failed',
      })
      onHandlers.get('job.failed')!(event)

      expect(toast.error).toHaveBeenCalledWith('Document processing failed: extraction failed')
      expect(notifications.addEvent).toHaveBeenCalledWith(event)
    })

    it('fetches document list on failure only when loaded', () => {
      const documents = useDocumentsStore()
      vi.spyOn(documents, 'fetchDocuments').mockResolvedValue({
        data: [],
        meta: { total: 0, limit: 10, offset: 0 },
      })

      const { result } = mountComposable()
      result.start()

      const event = makeEvent({ type: 'job.failed', job_kind: 'document_extract' })

      // Not loaded — should NOT fetch
      onHandlers.get('job.failed')!(event)
      expect(documents.fetchDocuments).not.toHaveBeenCalled()

      // Loaded — should fetch
      documents.$patch({ loaded: true })
      onHandlers.get('job.failed')!(event)
      expect(documents.fetchDocuments).toHaveBeenCalled()
    })

    it('refreshes queue stats and failed jobs on failure when stats loaded', () => {
      const auth = useAuthStore()
      auth.user = { id: 1, email: 'admin@test.com', name: 'Admin', is_admin: true }

      const queue = useQueueStore()
      queue.$patch({
        stats: { available: 0, running: 0, retryable: 0, discarded: 0, cancelled: 0 },
      })
      vi.spyOn(queue, 'fetchStats').mockResolvedValue({
        available: 0,
        running: 0,
        retryable: 0,
        discarded: 0,
        cancelled: 0,
      })
      vi.spyOn(queue, 'fetchFailedJobs').mockResolvedValue({
        data: [],
        meta: { count: 0 },
      })

      const { result } = mountComposable()
      result.start()

      onHandlers.get('job.failed')!(
        makeEvent({ type: 'job.failed', job_kind: 'document_extract', error: 'fail' }),
      )

      expect(queue.fetchStats).toHaveBeenCalled()
      expect(queue.fetchFailedJobs).toHaveBeenCalled()
    })
  })

  describe('job.snoozed', () => {
    it('adds event to notifications', () => {
      const notifications = useNotificationsStore()
      vi.spyOn(notifications, 'addEvent')

      const { result } = mountComposable()
      result.start()

      const event = makeEvent({ type: 'job.snoozed', job_kind: 'document_extract' })
      onHandlers.get('job.snoozed')!(event)

      expect(notifications.addEvent).toHaveBeenCalledWith(event)
    })
  })
})
