import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'
import { createPinia, setActivePinia } from 'pinia'
import { mount, flushPromises } from '@vue/test-utils'
import DocumentListView from '@/views/DocumentListView.vue'

vi.mock('vue-router', () => ({
  useRouter: () => ({ push: vi.fn() }),
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

describe('DocumentListView', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
    stubFetch({ data: [], meta: { total: 0, limit: 10, offset: 0 } })
  })

  afterEach(() => {
    vi.restoreAllMocks()
    vi.unstubAllGlobals()
  })

  function mountView() {
    return mount(DocumentListView, {
      shallow: true,
      global: {
        stubs: {
          RouterLink: { template: '<a><slot/></a>' },
        },
      },
    })
  }

  it('renders title', async () => {
    const wrapper = mountView()
    await flushPromises()
    expect(wrapper.text()).toContain('Documents')
  })

  it('fetches documents on mount', async () => {
    mountView()
    await flushPromises()
    expect(fetch).toHaveBeenCalled()
    const url = (fetch as ReturnType<typeof vi.fn>).mock.calls[0]![0] as string
    expect(url).toContain('/api/documents')
  })

  it('shows empty state when no documents', async () => {
    const wrapper = mountView()
    await flushPromises()
    expect(wrapper.findComponent({ name: 'EmptyState' }).exists()).toBe(true)
  })

  it('shows data table when documents exist', async () => {
    stubFetch({
      data: [
        {
          uuid: 'doc-1',
          title: 'Test',
          file_type: 'pdf',
          file_size: 1024,
          status: 'indexed',
          tags: [],
          created_at: '2026-01-01T00:00:00Z',
          updated_at: '2026-01-01T00:00:00Z',
        },
      ],
      meta: { total: 1, limit: 10, offset: 0 },
    })

    const wrapper = mountView()
    await flushPromises()
    expect(wrapper.findComponent({ name: 'DataTable' }).exists()).toBe(true)
    expect(wrapper.findComponent({ name: 'Pagination' }).exists()).toBe(true)
  })

  it('has upload button', async () => {
    const wrapper = mountView()
    await flushPromises()
    expect(wrapper.text()).toContain('Upload')
  })

  it('has filter dropdowns', async () => {
    const wrapper = mountView()
    await flushPromises()
    expect(wrapper.findAll('select').length).toBe(2)
  })
})
