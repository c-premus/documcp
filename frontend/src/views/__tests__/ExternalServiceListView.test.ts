import { describe, it, expect, vi, beforeAll, afterEach } from 'vitest'
import { mount, flushPromises } from '@vue/test-utils'
import { createPinia, setActivePinia } from 'pinia'
import ExternalServiceListView from '@/views/ExternalServiceListView.vue'
import { useExternalServicesStore } from '@/stores/externalServices'
import type { ExternalService } from '@/stores/externalServices'

// HeadlessUI components require ResizeObserver
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

const sampleService: ExternalService = {
  uuid: 'svc-001',
  name: 'Test Kiwix',
  slug: 'test-kiwix',
  type: 'kiwix',
  base_url: 'https://kiwix.example.com',
  priority: 0,
  status: 'healthy',
  is_enabled: true,
  is_env_managed: false,
  error_count: 0,
  consecutive_failures: 0,
}

function mountView(services: ExternalService[] = []) {
  const pinia = createPinia()
  setActivePinia(pinia)
  const store = useExternalServicesStore()

  vi.spyOn(store, 'fetchServices').mockResolvedValue({
    data: services,
    meta: { total: services.length, limit: 20, offset: 0 },
  })

  // Populate store state to match what fetchServices would set
  store.$patch({
    services,
    total: services.length,
    loading: false,
  })

  const wrapper = mount(ExternalServiceListView, {
    attachTo: document.body,
    global: {
      plugins: [pinia],
      stubs: {
        DataTable: { template: '<div data-testid="data-table"><slot /></div>' },
        Pagination: { template: '<div data-testid="pagination" />' },
        ExternalServiceModal: { template: '<div data-testid="service-modal" />' },
        ConfirmDialog: { template: '<div data-testid="confirm-dialog" />' },
      },
    },
  })

  return { wrapper, store }
}

describe('ExternalServiceListView', () => {
  it('renders the heading', async () => {
    const { wrapper } = mountView()
    await flushPromises()

    expect(wrapper.text()).toContain('External Services')
  })

  it('shows empty state when no services exist', async () => {
    const { wrapper } = mountView([])
    await flushPromises()

    expect(wrapper.text()).toContain('No external services')
    expect(wrapper.text()).toContain('Add your first external service to get started')
  })

  it('shows "Add Service" button in the toolbar', async () => {
    const { wrapper } = mountView()
    await flushPromises()

    const buttons = wrapper.findAll('button')
    const addBtn = buttons.find((b) => b.text().includes('Add Service'))
    expect(addBtn).toBeTruthy()
  })

  it('does not show empty state when services exist', async () => {
    const { wrapper } = mountView([sampleService])
    await flushPromises()

    expect(wrapper.text()).not.toContain('No external services')
  })

  it('calls fetchServices on mount', async () => {
    const { store } = mountView()
    await flushPromises()

    expect(store.fetchServices).toHaveBeenCalled()
  })

  it('renders a type filter dropdown', async () => {
    const { wrapper } = mountView()
    await flushPromises()

    const select = wrapper.find('select')
    expect(select.exists()).toBe(true)

    const options = select.findAll('option')
    const optionTexts = options.map((o) => o.text())
    expect(optionTexts).toContain('All Types')
    expect(optionTexts).toContain('Kiwix')
    expect(optionTexts).toContain('Confluence')
  })

  it('shows empty state action button that says "Add Service"', async () => {
    const { wrapper } = mountView([])
    await flushPromises()

    // The empty state includes an action button
    const buttons = wrapper.findAll('button')
    const addButtons = buttons.filter((b) => b.text().includes('Add Service'))
    // Toolbar button + empty state action button
    expect(addButtons.length).toBeGreaterThanOrEqual(2)
  })
})
