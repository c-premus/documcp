import { describe, it, expect, vi } from 'vitest'
import { mount, flushPromises } from '@vue/test-utils'
import ZimArchiveListView from '@/views/ZimArchiveListView.vue'
import { setupViewTest, stubFetch } from '@/__tests__/testHelpers/viewHarness'

vi.mock('vue-router', () => ({
  useRouter: () => ({ push: vi.fn() }),
  RouterLink: { template: '<a><slot/></a>' },
}))

vi.mock('vue-sonner', () => ({
  toast: { success: vi.fn(), error: vi.fn() },
}))

describe('ZimArchiveListView', () => {
  setupViewTest({ defaultFetch: { data: [], meta: { total: 0 } } })

  function mountView() {
    return mount(ZimArchiveListView, {
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
          EmptyState: { template: '<div data-testid="empty-state"/>', props: ['title'] },
          StatusBadge: true,
        },
      },
    })
  }

  it('renders title', async () => {
    const wrapper = mountView()
    await flushPromises()
    expect(wrapper.text()).toContain('ZIM Archives')
  })

  it('fetches archives on mount', async () => {
    mountView()
    await flushPromises()
    expect(fetch).toHaveBeenCalled()
    const url = (fetch as ReturnType<typeof vi.fn>).mock.calls[0]![0] as string
    expect(url).toContain('/api/zim/archives')
  })

  it('shows empty state when no archives', async () => {
    const wrapper = mountView()
    await flushPromises()
    expect(wrapper.find('[data-testid="empty-state"]').exists()).toBe(true)
  })

  it('shows data table when archives exist', async () => {
    stubFetch({
      data: [
        {
          uuid: 'zim-1',
          name: 'wikipedia_en',
          title: 'Wikipedia',
          language: 'en',
          article_count: 1000,
          media_count: 500,
          file_size: 1048576,
          file_size_human: '1 MB',
          tags: [],
        },
      ],
      meta: { total: 1 },
    })

    const wrapper = mountView()
    await flushPromises()
    expect(wrapper.find('[data-testid="data-table"]').exists()).toBe(true)
  })

  it('has category and language filter dropdowns', async () => {
    const wrapper = mountView()
    await flushPromises()
    const selects = wrapper.findAll('select')
    expect(selects.length).toBe(2)
  })
})
