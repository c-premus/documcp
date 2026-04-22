import { describe, it, expect, vi } from 'vitest'
import { mount, flushPromises } from '@vue/test-utils'
import DashboardView from '@/views/DashboardView.vue'
import {
  setAdmin,
  setNonAdmin,
  setupViewTest,
  stubFetch,
} from '@/__tests__/testHelpers/viewHarness'

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
  git_templates: number
  queue?: { pending: number; completed: number; failed: number }
}

const sampleStats: DashboardStats = {
  documents: 42,
  users: 5,
  oauth_clients: 3,
  external_services: 2,
  zim_archives: 7,
  git_templates: 10,
}

describe('DashboardView', () => {
  setupViewTest()

  describe('admin', () => {
    it('fetches stats on mount', async () => {
      setAdmin()
      stubFetch({ data: sampleStats })

      mount(DashboardView)
      await flushPromises()

      expect(fetch).toHaveBeenCalledWith('/api/admin/dashboard/stats', undefined)
    })

    it('shows loading state initially', () => {
      setAdmin()
      vi.stubGlobal('fetch', vi.fn().mockReturnValue(new Promise(() => {})))

      const wrapper = mount(DashboardView)

      expect(wrapper.find('.animate-spin').exists()).toBe(true)
    })

    it('renders stat cards with correct counts', async () => {
      setAdmin()
      stubFetch({ data: sampleStats })

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
      expect(wrapper.text()).toContain('Git Templates')
      expect(wrapper.text()).toContain('10')
    })

    it('hides loading spinner after stats load', async () => {
      setAdmin()
      stubFetch({ data: sampleStats })

      const wrapper = mount(DashboardView)
      await flushPromises()

      expect(wrapper.find('.animate-spin').exists()).toBe(false)
    })

    it('shows error state on fetch failure', async () => {
      setAdmin()
      stubFetch({ message: 'Server unavailable' }, false)

      const wrapper = mount(DashboardView)
      await flushPromises()

      expect(wrapper.text()).toContain('Server unavailable')
      expect(wrapper.find('.animate-spin').exists()).toBe(false)
    })

    it('shows queue stats section when queue data is present', async () => {
      setAdmin()
      const statsWithQueue: DashboardStats = {
        ...sampleStats,
        queue: { pending: 12, completed: 89, failed: 3 },
      }
      stubFetch({ data: statsWithQueue })

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
      setAdmin()
      stubFetch({ data: sampleStats })

      const wrapper = mount(DashboardView)
      await flushPromises()

      expect(wrapper.text()).not.toContain('Job Queue')
    })

    it('renders the dashboard heading with system overview', async () => {
      setAdmin()
      stubFetch({ data: sampleStats })

      const wrapper = mount(DashboardView)
      await flushPromises()

      expect(wrapper.find('h1').text()).toBe('Dashboard')
      expect(wrapper.text()).toContain('System overview')
    })
  })

  describe('non-admin', () => {
    it('does not fetch stats', async () => {
      setNonAdmin()

      mount(DashboardView)
      await flushPromises()

      expect(fetch).not.toHaveBeenCalled()
    })

    it('does not show loading spinner', async () => {
      setNonAdmin()

      const wrapper = mount(DashboardView)
      await flushPromises()

      expect(wrapper.find('.animate-spin').exists()).toBe(false)
    })

    it('shows welcome message with user name', async () => {
      setNonAdmin()

      const wrapper = mount(DashboardView)
      await flushPromises()

      expect(wrapper.find('h1').text()).toBe('Dashboard')
      expect(wrapper.text()).toContain('Welcome, Regular User')
    })

    it('renders quick links', async () => {
      setNonAdmin()

      const wrapper = mount(DashboardView)
      await flushPromises()

      expect(wrapper.text()).toContain('Documents')
      expect(wrapper.text()).toContain('ZIM Archives')
      expect(wrapper.text()).toContain('Git Templates')
    })
  })
})
