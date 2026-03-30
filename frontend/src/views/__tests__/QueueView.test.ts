import { describe, it, expect, vi, beforeAll, afterEach } from 'vitest'
import { mount, flushPromises } from '@vue/test-utils'
import { createPinia, setActivePinia } from 'pinia'
import QueueView from '@/views/QueueView.vue'
import { useQueueStore } from '@/stores/queue'

// HeadlessUI Tabs and Dialog require ResizeObserver
beforeAll(() => {
  vi.stubGlobal(
    'ResizeObserver',
    class ResizeObserver {
      observe() {}
      unobserve() {}
      disconnect() {}
    },
  )
})

afterEach(() => {
  document.body.innerHTML = ''
  vi.restoreAllMocks()
})

function mountView() {
  const pinia = createPinia()
  setActivePinia(pinia)

  // Stub store fetch methods to prevent real API calls
  const store = useQueueStore()
  vi.spyOn(store, 'fetchStats').mockResolvedValue({
    available: 5,
    running: 2,
    retryable: 1,
    discarded: 3,
    cancelled: 0,
  })
  vi.spyOn(store, 'fetchFailedJobs').mockResolvedValue({
    jobs: [],
    count: 0,
  })

  const wrapper = mount(QueueView, {
    attachTo: document.body,
    global: {
      plugins: [pinia],
      stubs: {
        DataTable: { template: '<div data-testid="data-table"><slot /></div>' },
        StatusBadge: { template: '<span />' },
      },
    },
  })

  return { wrapper, store }
}

describe('QueueView', () => {
  it('renders the heading', async () => {
    const { wrapper } = mountView()
    await flushPromises()

    expect(wrapper.text()).toContain('Queue Management')
  })

  it('renders stat cards when stats are loaded', async () => {
    const { wrapper, store } = mountView()
    await flushPromises()

    // The mock resolves but doesn't set store.stats -- patch it manually
    store.$patch({
      stats: { available: 5, running: 2, retryable: 1, discarded: 3, cancelled: 0 },
    })
    await flushPromises()

    const text = wrapper.text()
    expect(text).toContain('Available')
    expect(text).toContain('Running')
    expect(text).toContain('Retryable')
    expect(text).toContain('Discarded')
    expect(text).toContain('Cancelled')
    expect(text).toContain('5')
    expect(text).toContain('2')
    expect(text).toContain('1')
    expect(text).toContain('3')
  })

  it('renders Stats and Failed Jobs tabs', async () => {
    const { wrapper } = mountView()
    await flushPromises()

    const text = wrapper.text()
    expect(text).toContain('Stats')
    expect(text).toContain('Failed Jobs')
  })

  it('shows empty state for failed jobs when there are none', async () => {
    const { wrapper } = mountView()
    await flushPromises()

    // Click the "Failed Jobs" tab button to switch panels
    const tabButtons = wrapper.findAll('button')
    const failedJobsTab = tabButtons.find((b) => b.text().includes('Failed Jobs'))
    expect(failedJobsTab).toBeTruthy()
    await failedJobsTab!.trigger('click')
    await flushPromises()

    expect(wrapper.text()).toContain('No failed jobs')
    expect(wrapper.text()).toContain('All jobs are running smoothly')
  })

  it('calls fetchStats and fetchFailedJobs on mount', async () => {
    const { store } = mountView()
    await flushPromises()

    expect(store.fetchStats).toHaveBeenCalledOnce()
    expect(store.fetchFailedJobs).toHaveBeenCalledOnce()
  })

  it('shows spinner when stats are null', async () => {
    const pinia = createPinia()
    setActivePinia(pinia)
    const store = useQueueStore()

    // Stats remain null (never resolves during test)
    vi.spyOn(store, 'fetchStats').mockReturnValue(new Promise(() => {}))
    vi.spyOn(store, 'fetchFailedJobs').mockResolvedValue({ jobs: [], count: 0 })

    const wrapper = mount(QueueView, {
      attachTo: document.body,
      global: {
        plugins: [pinia],
        stubs: {
          DataTable: { template: '<div />' },
          StatusBadge: { template: '<span />' },
        },
      },
    })
    await flushPromises()

    // Stats panel is default, spinner should be visible
    const spinner = wrapper.find('.animate-spin')
    expect(spinner.exists()).toBe(true)
  })
})
