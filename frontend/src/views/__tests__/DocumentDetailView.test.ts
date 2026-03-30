import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'
import { createPinia, setActivePinia } from 'pinia'
import { mount, flushPromises } from '@vue/test-utils'
import DocumentDetailView from '@/views/DocumentDetailView.vue'
import { useDocumentsStore } from '@/stores/documents'

const pushMock = vi.fn()

vi.mock('vue-router', () => ({
  useRouter: () => ({ push: pushMock }),
  RouterLink: { template: '<a><slot/></a>' },
}))

vi.mock('vue-sonner', () => ({
  toast: { success: vi.fn(), error: vi.fn() },
}))

function stubFetch(response: unknown, ok = true) {
  vi.stubGlobal(
    'fetch',
    vi.fn().mockResolvedValue({
      ok,
      status: ok ? 200 : 500,
      statusText: 'Internal Server Error',
      json: () => Promise.resolve(response),
    }),
  )
}

function mockDocument(overrides = {}) {
  return {
    uuid: 'doc-1',
    title: 'Test Document',
    description: 'A test',
    file_type: 'markdown',
    file_size: 2048,
    mime_type: 'text/markdown',
    word_count: 200,
    is_public: false,
    has_file: true,
    status: 'indexed',
    content_hash: 'abc123def456',
    tags: ['test', 'docs'],
    created_at: '2026-01-01T00:00:00Z',
    updated_at: '2026-01-15T00:00:00Z',
    processed_at: '2026-01-01T00:00:00Z',
    content: '# Hello World',
    ...overrides,
  }
}

describe('DocumentDetailView', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
    stubFetch({ data: mockDocument() })
  })

  afterEach(() => {
    vi.restoreAllMocks()
    vi.unstubAllGlobals()
    pushMock.mockReset()
  })

  function mountView(uuid = 'doc-1') {
    return mount(DocumentDetailView, {
      props: { uuid },
      global: {
        stubs: {
          StatusBadge: {
            template: '<span data-testid="status-badge">{{ status }}</span>',
            props: ['status'],
          },
          ConfirmDialog: { template: '<div data-testid="confirm-dialog"/>', props: ['open'] },
          ContentViewer: {
            template: '<div data-testid="content-viewer"/>',
            props: ['content', 'fileType'],
          },
          RouterLink: { template: '<a><slot/></a>' },
          ArrowDownTrayIcon: true,
          TrashIcon: true,
        },
      },
    })
  }

  it('fetches document on mount', async () => {
    mountView('doc-1')
    await flushPromises()
    const url = (fetch as ReturnType<typeof vi.fn>).mock.calls[0]![0] as string
    expect(url).toContain('/api/documents/doc-1')
  })

  it('shows loading spinner when loading', async () => {
    const store = useDocumentsStore()
    store.$patch({ loading: true, currentDocument: null })
    const wrapper = mountView()
    expect(wrapper.find('.animate-spin').exists()).toBe(true)
  })

  it('shows error state when error and no document', async () => {
    const store = useDocumentsStore()
    store.$patch({ error: 'Not found', currentDocument: null, loading: false })
    const wrapper = mountView()
    expect(wrapper.text()).toContain('Not found')
    expect(wrapper.find('button').text()).toContain('Retry')
  })

  it('shows document title when loaded', async () => {
    const store = useDocumentsStore()
    store.$patch({ currentDocument: mockDocument(), loading: false })
    const wrapper = mountView()
    await flushPromises()
    expect(wrapper.text()).toContain('Test Document')
  })

  it('shows metadata fields', async () => {
    const store = useDocumentsStore()
    store.$patch({ currentDocument: mockDocument(), loading: false })
    const wrapper = mountView()
    await flushPromises()
    expect(wrapper.text()).toContain('markdown')
    expect(wrapper.text()).toContain('200')
    expect(wrapper.text()).toContain('test')
    expect(wrapper.text()).toContain('docs')
  })

  it('shows content viewer when content available', async () => {
    const store = useDocumentsStore()
    store.$patch({ currentDocument: mockDocument(), loading: false })
    const wrapper = mountView()
    await flushPromises()
    expect(wrapper.find('[data-testid="content-viewer"]').exists()).toBe(true)
  })

  it('has download link and delete button', async () => {
    const store = useDocumentsStore()
    store.$patch({ currentDocument: mockDocument(), loading: false })
    const wrapper = mountView()
    await flushPromises()
    expect(wrapper.find('a[href*="download"]').exists()).toBe(true)
    const deleteBtn = wrapper.findAll('button').find((b) => b.text().includes('Delete'))
    expect(deleteBtn).toBeDefined()
  })
})
