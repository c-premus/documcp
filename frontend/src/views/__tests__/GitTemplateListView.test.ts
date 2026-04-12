import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'
import { createPinia, setActivePinia } from 'pinia'
import { mount, flushPromises } from '@vue/test-utils'
import GitTemplateListView from '@/views/GitTemplateListView.vue'

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

describe('GitTemplateListView', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
    stubFetch({ data: [], meta: { total: 0, limit: 50, offset: 0 } })
  })

  afterEach(() => {
    vi.restoreAllMocks()
    vi.unstubAllGlobals()
  })

  function mountView() {
    return mount(GitTemplateListView, {
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
            template: '<div data-testid="empty-state"><slot name="action"/></div>',
            props: ['title'],
          },
          ConfirmDialog: { template: '<div data-testid="confirm-dialog"/>', props: ['open'] },
          GitTemplateCreateModal: {
            template: '<div data-testid="create-modal"/>',
            props: ['open'],
          },
          ArrowPathIcon: true,
          TrashIcon: true,
        },
      },
    })
  }

  it('renders title', async () => {
    const wrapper = mountView()
    await flushPromises()
    expect(wrapper.text()).toContain('Git Templates')
  })

  it('fetches templates on mount', async () => {
    mountView()
    await flushPromises()
    expect(fetch).toHaveBeenCalled()
    const url = (fetch as ReturnType<typeof vi.fn>).mock.calls[0]![0] as string
    expect(url).toContain('/api/git-templates')
  })

  it('shows empty state when no templates', async () => {
    const wrapper = mountView()
    await flushPromises()
    expect(wrapper.find('[data-testid="empty-state"]').exists()).toBe(true)
  })

  it('shows data table when templates exist', async () => {
    stubFetch({
      data: [
        {
          uuid: 'tpl-1',
          name: 'Test Template',
          slug: 'test-template',
          repository_url: 'https://git.example.com/repo',
          branch: 'main',
          tags: [],
          is_public: true,
          status: 'synced',
          file_count: 10,
          total_size_bytes: 5000,
        },
      ],
      meta: { total: 1, limit: 50, offset: 0 },
    })

    const wrapper = mountView()
    await flushPromises()
    expect(wrapper.find('[data-testid="data-table"]').exists()).toBe(true)
  })

  it('has add template button for admin', async () => {
    const { useAuthStore } = await import('@/stores/auth')
    const auth = useAuthStore()
    auth.user = { id: 1, email: 'admin@test.com', name: 'Admin', is_admin: true }

    const wrapper = mountView()
    await flushPromises()
    const btn = wrapper.findAll('button').find((b) => b.text().includes('Add Template'))
    expect(btn).toBeDefined()
  })

  it('hides add template button for non-admin', async () => {
    const { useAuthStore } = await import('@/stores/auth')
    const auth = useAuthStore()
    auth.user = { id: 2, email: 'user@test.com', name: 'User', is_admin: false }

    const wrapper = mountView()
    await flushPromises()
    const btn = wrapper.findAll('button').find((b) => b.text().includes('Add Template'))
    expect(btn).toBeUndefined()
  })
})
