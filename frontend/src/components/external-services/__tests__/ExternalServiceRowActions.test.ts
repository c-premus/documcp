import { describe, it, expect } from 'vitest'
import { mount } from '@vue/test-utils'
import ExternalServiceRowActions from '@/components/external-services/ExternalServiceRowActions.vue'
import type { ExternalService } from '@/stores/externalServices'

const KIWIX_SERVICE: ExternalService = {
  uuid: 'svc-1',
  name: 'Kiwix Main',
  slug: 'kiwix-main',
  type: 'kiwix',
  base_url: 'https://kiwix.example.com',
  priority: 0,
  status: 'healthy',
  is_enabled: true,
  is_env_managed: false,
  error_count: 0,
  consecutive_failures: 0,
}

const NON_KIWIX_SERVICE: ExternalService = {
  ...KIWIX_SERVICE,
  uuid: 'svc-2',
  type: 'other',
}

function mountActions(service: ExternalService, syncing = false) {
  return mount(ExternalServiceRowActions, { props: { service, syncing } })
}

describe('ExternalServiceRowActions', () => {
  it('renders all four accessible buttons for a kiwix service', () => {
    const wrapper = mountActions(KIWIX_SERVICE)
    const labels = wrapper.findAll('button').map((b) => b.attributes('aria-label'))
    expect(labels).toEqual(['Sync now', 'Health check', 'Edit service', 'Delete service'])
  })

  it('hides the sync button for non-kiwix services', () => {
    const wrapper = mountActions(NON_KIWIX_SERVICE)
    const labels = wrapper.findAll('button').map((b) => b.attributes('aria-label'))
    expect(labels).toEqual(['Health check', 'Edit service', 'Delete service'])
  })

  it('emits sync with the service when sync is clicked', async () => {
    const wrapper = mountActions(KIWIX_SERVICE)
    await wrapper.get('[aria-label="Sync now"]').trigger('click')
    expect(wrapper.emitted('sync')![0]).toEqual([KIWIX_SERVICE])
  })

  it('emits healthCheck with the service when health-check is clicked', async () => {
    const wrapper = mountActions(KIWIX_SERVICE)
    await wrapper.get('[aria-label="Health check"]').trigger('click')
    expect(wrapper.emitted('healthCheck')![0]).toEqual([KIWIX_SERVICE])
  })

  it('emits edit with the service when edit is clicked', async () => {
    const wrapper = mountActions(KIWIX_SERVICE)
    await wrapper.get('[aria-label="Edit service"]').trigger('click')
    expect(wrapper.emitted('edit')![0]).toEqual([KIWIX_SERVICE])
  })

  it('emits delete with the service when delete is clicked', async () => {
    const wrapper = mountActions(KIWIX_SERVICE)
    await wrapper.get('[aria-label="Delete service"]').trigger('click')
    expect(wrapper.emitted('delete')![0]).toEqual([KIWIX_SERVICE])
  })

  it('disables the sync button while syncing', () => {
    const wrapper = mountActions(KIWIX_SERVICE, true)
    expect(wrapper.get('[aria-label="Sync now"]').attributes('disabled')).toBeDefined()
  })

  it('does not emit sync when the button is clicked during syncing', async () => {
    const wrapper = mountActions(KIWIX_SERVICE, true)
    await wrapper.get('[aria-label="Sync now"]').trigger('click')
    expect(wrapper.emitted('sync')).toBeUndefined()
  })
})
