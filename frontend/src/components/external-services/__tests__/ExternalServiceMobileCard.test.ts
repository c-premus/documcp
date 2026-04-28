import { describe, it, expect } from 'vitest'
import { mount } from '@vue/test-utils'
import ExternalServiceMobileCard from '@/components/external-services/ExternalServiceMobileCard.vue'
import type { ExternalService } from '@/stores/externalServices'

const SERVICE: ExternalService = {
  uuid: 'svc-1',
  name: 'Local Kiwix',
  slug: 'local-kiwix',
  type: 'kiwix',
  base_url: 'https://kiwix.local',
  priority: 1,
  status: 'healthy',
  is_enabled: true,
  is_env_managed: false,
  error_count: 0,
  consecutive_failures: 0,
  created_at: '2026-01-01T00:00:00Z',
  updated_at: '2026-01-01T00:00:00Z',
}

function mountCard(
  overrides: Partial<{
    service: ExternalService
    syncing: boolean
    canMoveUp: boolean
    canMoveDown: boolean
  }> = {},
) {
  return mount(ExternalServiceMobileCard, {
    props: {
      service: SERVICE,
      syncing: false,
      canMoveUp: true,
      canMoveDown: true,
      ...overrides,
    },
  })
}

describe('ExternalServiceMobileCard', () => {
  it('renders the service name and base URL', () => {
    const wrapper = mountCard()
    expect(wrapper.get('h3').text()).toBe('Local Kiwix')
    expect(wrapper.text()).toContain('https://kiwix.local')
  })

  it('exposes the row-actions cluster with all four buttons', () => {
    const wrapper = mountCard()
    const labels = wrapper.findAll('button').map((b) => b.attributes('aria-label'))
    expect(labels).toContain('Sync now')
    expect(labels).toContain('Health check')
    expect(labels).toContain('Edit service')
    expect(labels).toContain('Delete service')
  })

  it('emits toggleEnabled when the enabled switch is flipped', async () => {
    const wrapper = mountCard()
    await wrapper.get('[role="switch"]').trigger('click')
    expect(wrapper.emitted('toggleEnabled')![0]).toEqual([SERVICE])
  })

  it('emits movePriority up/down with direction', async () => {
    const wrapper = mountCard()
    await wrapper.get('[aria-label="Move up"]').trigger('click')
    await wrapper.get('[aria-label="Move down"]').trigger('click')
    expect(wrapper.emitted('movePriority')![0]).toEqual([SERVICE, 'up'])
    expect(wrapper.emitted('movePriority')![1]).toEqual([SERVICE, 'down'])
  })

  it('disables priority buttons at boundaries', () => {
    const wrapper = mountCard({ canMoveUp: false, canMoveDown: false })
    expect(wrapper.get('[aria-label="Move up"]').attributes('disabled')).toBeDefined()
    expect(wrapper.get('[aria-label="Move down"]').attributes('disabled')).toBeDefined()
  })

  it('emits sync / healthCheck / edit / delete with the service payload', async () => {
    const wrapper = mountCard()
    await wrapper.get('[aria-label="Sync now"]').trigger('click')
    await wrapper.get('[aria-label="Health check"]').trigger('click')
    await wrapper.get('[aria-label="Edit service"]').trigger('click')
    await wrapper.get('[aria-label="Delete service"]').trigger('click')
    expect(wrapper.emitted('sync')![0]).toEqual([SERVICE])
    expect(wrapper.emitted('healthCheck')![0]).toEqual([SERVICE])
    expect(wrapper.emitted('edit')![0]).toEqual([SERVICE])
    expect(wrapper.emitted('delete')![0]).toEqual([SERVICE])
  })
})
