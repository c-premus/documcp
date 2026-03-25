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

    it('fetches document list only when documents are loaded (lazy refresh)', async () => {
      const documents = useDocumentsStore()
      vi.spyOn(documents, 'refreshCurrent').mockResolvedValue(null)
      vi.spyOn(documents, 'fetchDocuments').mockResolvedValue({
        data: [],
        meta: { total: 0, limit: 10, offset: 0 },
      })

      // No documents loaded — should NOT fetch list
      const { result } = mountComposable()
      result.start()

      const event = makeEvent({ type: 'job.completed', job_kind: 'document_index' })
      onHandlers.get('job.completed')!(event)
      expect(documents.fetchDocuments).not.toHaveBeenCalled()

      // With documents loaded — should fetch list
      documents.$patch({ documents: [{ uuid: 'doc-1' }] as never[] })
      onHandlers.get('job.completed')!(event)
      expect(documents.fetchDocuments).toHaveBeenCalled()
    })
  })

  describe('job.completed — scheduler jobs', () => {
    it('refreshes zimArchives on sync_kiwix when archives loaded', async () => {
      const { toast } = await import('vue-sonner')
      const zimArchives = useZimArchivesStore()
      zimArchives.$patch({ archives: [{ uuid: 'zim-1' }] as never[] })
      vi.spyOn(zimArchives, 'fetchArchives').mockResolvedValue({ data: [], meta: { total: 0 } })

      const { result } = mountComposable()
      result.start()

      onHandlers.get('job.completed')!(makeEvent({ type: 'job.completed', job_kind: 'sync_kiwix' }))

      expect(zimArchives.fetchArchives).toHaveBeenCalled()
      expect(toast.info).toHaveBeenCalledWith('Kiwix sync completed')
    })

    it('refreshes gitTemplates on sync_git_templates when templates loaded', async () => {
      const gitTemplates = useGitTemplatesStore()
      gitTemplates.$patch({ templates: [{ uuid: 'tpl-1' }] as never[] })
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

    it('refreshes externalServices on health_check_services when services loaded', async () => {
      const externalServices = useExternalServicesStore()
      externalServices.$patch({ services: [{ uuid: 'svc-1' }] as never[] })
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
  })

  describe('job.snoozed', () => {
    it('adds event to notifications', () => {
      const notifications = useNotificationsStore()
      vi.spyOn(notifications, 'addEvent')

      const { result } = mountComposable()
      result.start()

      const event = makeEvent({ type: 'job.snoozed', job_kind: 'document_index' })
      onHandlers.get('job.snoozed')!(event)

      expect(notifications.addEvent).toHaveBeenCalledWith(event)
    })
  })
})
