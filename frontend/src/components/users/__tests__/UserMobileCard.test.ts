import { describe, it, expect } from 'vitest'
import { mount } from '@vue/test-utils'
import UserMobileCard from '@/components/users/UserMobileCard.vue'
import type { User } from '@/stores/users'

const USER: User = {
  id: 7,
  name: 'Alice Example',
  email: 'alice@example.com',
  oidc_sub: 'sub-abc-123',
  oidc_provider: 'keycloak',
  is_admin: false,
  created_at: '2026-01-01T00:00:00Z',
  updated_at: '2026-01-01T00:00:00Z',
}

function mountCard(overrides: Partial<User> = {}) {
  return mount(UserMobileCard, { props: { user: { ...USER, ...overrides } } })
}

describe('UserMobileCard', () => {
  it('renders the user name and email', () => {
    const wrapper = mountCard()
    expect(wrapper.get('h3').text()).toBe('Alice Example')
    expect(wrapper.text()).toContain('alice@example.com')
  })

  it('exposes the row-actions cluster (sessions + delete)', () => {
    const wrapper = mountCard()
    const labels = wrapper.findAll('button').map((b) => b.attributes('aria-label'))
    expect(labels).toContain('View active sessions')
    expect(labels).toContain('Delete user')
  })

  it('emits toggleAdmin when the admin switch is flipped', async () => {
    const wrapper = mountCard()
    await wrapper.get('[role="switch"]').trigger('click')

    expect(wrapper.emitted('toggleAdmin')![0]).toEqual([{ ...USER }])
  })

  it('emits sessions and delete from the row-actions cluster', async () => {
    const wrapper = mountCard()
    await wrapper.get('[aria-label="View active sessions"]').trigger('click')
    await wrapper.get('[aria-label="Delete user"]').trigger('click')

    expect(wrapper.emitted('sessions')![0]).toEqual([{ ...USER }])
    expect(wrapper.emitted('delete')![0]).toEqual([{ ...USER }])
  })

  it('omits the OIDC line when oidc_sub is empty', () => {
    const wrapper = mountCard({ oidc_sub: '' })
    expect(wrapper.text()).not.toContain('OIDC')
  })
})
