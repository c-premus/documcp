import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'
import { mount, flushPromises } from '@vue/test-utils'
import DashboardView from '@/views/DashboardView.vue'

vi.mock('vue-router', () => ({
  useRouter: () => ({ push: vi.fn() }),
  RouterLink: { template: '<a><slot /></a>', props: ['to'] },
}))

interface DashboardStats {
  documents: number
  users: number
  oauth_clients: number
  external_services: number
  zim_archives: number
  confluence_spaces: number
  git_templates: number
  queue?: { pending: number; completed: number; failed: number }
}

function stubFetchStats(stats: DashboardStats) {
  vi.stubGlobal(
    'fetch',
    vi.fn().mockResolvedValue({
      ok: true,
      json: () => Promise.resolve({ data: stats }),
    }),
  )
}

function stubFetchError(message: string) {
  vi.stubGlobal(
    'fetch',
    vi.fn().mockResolvedValue({
      ok: false,
      statusText: 'Internal Server Error',
      json: () => Promise.resolve({ message }),
    }),
  )
}

const sampleStats: DashboardStats = {
  documents: 42,
  users: 5,
  oauth_clients: 3,
  external_services: 2,
  zim_archives: 7,
  confluence_spaces: 4,
  git_templates: 10,
}

describe('DashboardView', () => {
  beforeEach(() => {
    vi.stubGlobal('fetch', vi.fn())
  })

  afterEach(() => {
    vi.restoreAllMocks()
    vi.unstubAllGlobals()
  })

  it('fetches stats on mount', async () => {
    stubFetchStats(sampleStats)

    mount(DashboardView)
    await flushPromises()

    expect(fetch).toHaveBeenCalledWith('/api/admin/dashboard/stats')
  })

  it('shows loading state initially', () => {
    // Never resolving fetch to keep loading state
    vi.stubGlobal(
      'fetch',
      vi.fn().mockReturnValue(new Promise(() => {})),
    )

    const wrapper = mount(DashboardView)

    expect(wrapper.find('.animate-spin').exists()).toBe(true)
  })

  it('renders stat cards with correct counts', async () => {
    stubFetchStats(sampleStats)

    const wrapper = mount(DashboardView)
    await flushPromises()

    expect(wrapper.text()).toContain('Documents')
    expect(wrapper.text()).toContain('42')
    expect(wrapper.text()).toContain('Users')
    expect(wrapper.text()).toContain('5')
    expect(wrapper.text()).toContain('OAuth Clients')
    expect(wrapper.text()).toContain('3')
    expect(wrapper.text()).toContain('External Services')
    expect(wrapper.text()).toContain('2')
    expect(wrapper.text()).toContain('ZIM Archives')
    expect(wrapper.text()).toContain('7')
    expect(wrapper.text()).toContain('Confluence Spaces')
    expect(wrapper.text()).toContain('4')
    expect(wrapper.text()).toContain('Git Templates')
    expect(wrapper.text()).toContain('10')
  })

  it('hides loading spinner after stats load', async () => {
    stubFetchStats(sampleStats)

    const wrapper = mount(DashboardView)
    await flushPromises()

    expect(wrapper.find('.animate-spin').exists()).toBe(false)
  })

  it('shows error state on fetch failure', async () => {
    stubFetchError('Server unavailable')

    const wrapper = mount(DashboardView)
    await flushPromises()

    expect(wrapper.text()).toContain('Server unavailable')
    expect(wrapper.find('.animate-spin').exists()).toBe(false)
  })

  it('shows queue stats section when queue data is present', async () => {
    const statsWithQueue: DashboardStats = {
      ...sampleStats,
      queue: { pending: 12, completed: 89, failed: 3 },
    }
    stubFetchStats(statsWithQueue)

    const wrapper = mount(DashboardView)
    await flushPromises()

    expect(wrapper.text()).toContain('Job Queue')
    expect(wrapper.text()).toContain('Pending')
    expect(wrapper.text()).toContain('12')
    expect(wrapper.text()).toContain('Completed')
    expect(wrapper.text()).toContain('89')
    expect(wrapper.text()).toContain('Failed')
    expect(wrapper.text()).toContain('3')
  })

  it('does not show queue section when queue data is absent', async () => {
    stubFetchStats(sampleStats)

    const wrapper = mount(DashboardView)
    await flushPromises()

    expect(wrapper.text()).not.toContain('Job Queue')
  })

  it('renders the dashboard heading', async () => {
    stubFetchStats(sampleStats)

    const wrapper = mount(DashboardView)
    await flushPromises()

    expect(wrapper.find('h1').text()).toBe('Dashboard')
    expect(wrapper.text()).toContain('System overview')
  })
})
