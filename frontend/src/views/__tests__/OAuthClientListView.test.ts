import { describe, it, expect, vi } from 'vitest'
import { mount, flushPromises } from '@vue/test-utils'
import OAuthClientListView from '@/views/OAuthClientListView.vue'
import { setupViewTest } from '@/__tests__/testHelpers/viewHarness'

vi.mock('vue-router', () => ({
  useRouter: () => ({ push: vi.fn() }),
  RouterLink: { template: '<a><slot/></a>' },
}))

vi.mock('vue-sonner', () => ({
  toast: { success: vi.fn(), error: vi.fn() },
}))

describe('OAuthClientListView', () => {
  setupViewTest({ defaultFetch: { data: [], meta: { total: 0, limit: 20, offset: 0 } } })

  function mountView() {
    return mount(OAuthClientListView, {
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
          StatusBadge: true,
          OAuthClientCreateModal: {
            template: '<div data-testid="create-modal"/>',
            props: ['open'],
          },
          SecretDisplayModal: { template: '<div data-testid="secret-modal"/>', props: ['open'] },
          NoSymbolIcon: true,
        },
      },
    })
  }

  it('renders title', async () => {
    const wrapper = mountView()
    await flushPromises()
    expect(wrapper.text()).toContain('OAuth Clients')
  })

  it('fetches clients on mount', async () => {
    mountView()
    await flushPromises()
    expect(fetch).toHaveBeenCalled()
    const url = (fetch as ReturnType<typeof vi.fn>).mock.calls[0]![0] as string
    expect(url).toContain('/api/admin/oauth-clients')
  })

  it('shows empty state when no clients', async () => {
    const wrapper = mountView()
    await flushPromises()
    expect(wrapper.find('[data-testid="empty-state"]').exists()).toBe(true)
  })

  it('has create client button', async () => {
    const wrapper = mountView()
    await flushPromises()
    const btn = wrapper.findAll('button').find((b) => b.text().includes('Create Client'))
    expect(btn).toBeDefined()
  })
})
