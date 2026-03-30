import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'
import { createPinia, setActivePinia } from 'pinia'
import { mount, flushPromises } from '@vue/test-utils'
import UserListView from '@/views/UserListView.vue'

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

describe('UserListView', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
    stubFetch({ data: [], meta: { total: 0 } })
  })

  afterEach(() => {
    vi.restoreAllMocks()
    vi.unstubAllGlobals()
  })

  function mountView() {
    return mount(UserListView, {
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
          SearchInput: { template: '<input data-testid="search"/>', props: ['modelValue'] },
          EmptyState: {
            template: '<div data-testid="empty-state"><slot name="action"/></div>',
            props: ['title'],
          },
          ConfirmDialog: { template: '<div data-testid="confirm-dialog"/>', props: ['open'] },
          UserModal: { template: '<div data-testid="user-modal"/>', props: ['open', 'user'] },
          Switch: true,
          PencilSquareIcon: true,
          TrashIcon: true,
        },
      },
    })
  }

  it('renders title', async () => {
    const wrapper = mountView()
    await flushPromises()
    expect(wrapper.text()).toContain('Users')
  })

  it('fetches users on mount', async () => {
    mountView()
    await flushPromises()
    expect(fetch).toHaveBeenCalled()
    const url = (fetch as ReturnType<typeof vi.fn>).mock.calls[0]![0] as string
    expect(url).toContain('/api/admin/users')
  })

  it('shows empty state when no users', async () => {
    const wrapper = mountView()
    await flushPromises()
    expect(wrapper.find('[data-testid="empty-state"]').exists()).toBe(true)
  })

  it('has create user button', async () => {
    const wrapper = mountView()
    await flushPromises()
    const btn = wrapper.findAll('button').find((b) => b.text().includes('Create User'))
    expect(btn).toBeDefined()
  })
})
