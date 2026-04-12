import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'
import { createPinia, setActivePinia } from 'pinia'
import { mount, flushPromises } from '@vue/test-utils'
import DocumentTrashView from '@/views/DocumentTrashView.vue'

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

describe('DocumentTrashView', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
    stubFetch({ data: [], total: 0 })
  })

  afterEach(() => {
    vi.restoreAllMocks()
    vi.unstubAllGlobals()
  })

  function mountView() {
    return mount(DocumentTrashView, {
      global: {
        stubs: {
          DataTable: {
            template: '<div data-testid="data-table"/>',
            props: ['data', 'columns', 'loading'],
          },
          Pagination: {
            template: '<div data-testid="pagination"/>',
            props: ['page', 'perPage', 'total'],
          },
          EmptyState: {
            template: '<div data-testid="empty-state"/>',
            props: ['title', 'description'],
          },
          ConfirmDialog: {
            template: '<div data-testid="confirm-dialog"/>',
            props: ['open', 'title', 'message'],
          },
          RouterLink: { template: '<a><slot/></a>' },
          ArrowPathIcon: true,
          TrashIcon: true,
        },
      },
    })
  }

  it('renders title and back link', async () => {
    const wrapper = mountView()
    await flushPromises()
    expect(wrapper.text()).toContain('Trash')
    expect(wrapper.text()).toContain('Documents')
  })

  it('fetches deleted documents on mount', async () => {
    mountView()
    await flushPromises()
    expect(fetch).toHaveBeenCalled()
    const url = (fetch as ReturnType<typeof vi.fn>).mock.calls[0]![0] as string
    expect(url).toContain('/api/documents/trash')
  })

  it('shows empty state when no documents', async () => {
    const wrapper = mountView()
    await flushPromises()
    expect(wrapper.find('[data-testid="empty-state"]').exists()).toBe(true)
  })

  it('shows data table when deleted documents exist', async () => {
    stubFetch({
      data: [
        {
          uuid: 'del-1',
          title: 'Deleted Doc',
          description: '',
          file_type: 'pdf',
          file_size: 512,
          status: 'indexed',
          tags: [],
          created_at: '2026-01-01T00:00:00Z',
          updated_at: '2026-01-01T00:00:00Z',
        },
      ],
      total: 1,
    })

    const wrapper = mountView()
    await flushPromises()
    expect(wrapper.find('[data-testid="data-table"]').exists()).toBe(true)
  })

  it('has bulk purge button for admin', async () => {
    const { useAuthStore } = await import('@/stores/auth')
    const auth = useAuthStore()
    auth.user = { id: 1, email: 'admin@test.com', name: 'Admin', is_admin: true }

    const wrapper = mountView()
    await flushPromises()
    const btn = wrapper.findAll('button').find((b) => b.text().includes('Purge All'))
    expect(btn).toBeDefined()
  })

  it('hides bulk purge button for non-admin', async () => {
    const { useAuthStore } = await import('@/stores/auth')
    const auth = useAuthStore()
    auth.user = { id: 2, email: 'user@test.com', name: 'User', is_admin: false }

    const wrapper = mountView()
    await flushPromises()
    const btn = wrapper.findAll('button').find((b) => b.text().includes('Purge All'))
    expect(btn).toBeUndefined()
  })
})
