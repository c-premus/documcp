import { describe, it, expect } from 'vitest'
import { mount } from '@vue/test-utils'
import ScopeGrantRowActions from '@/components/oauth/ScopeGrantRowActions.vue'
import type { ScopeGrant } from '@/stores/oauthClients'

const GRANT: ScopeGrant = {
  id: 42,
  scope: 'documents:read',
  granted_by: 1,
  granted_by_email: 'admin@example.com',
  granted_by_name: 'Admin',
  granted_at: '2026-04-01T00:00:00Z',
  expires_at: '2026-05-01T00:00:00Z',
}

describe('ScopeGrantRowActions', () => {
  it('aria-label includes the scope so screen readers disambiguate rows', () => {
    const wrapper = mount(ScopeGrantRowActions, { props: { grant: GRANT } })
    expect(wrapper.get('button').attributes('aria-label')).toBe('Revoke grant documents:read')
  })

  it('emits revoke with the grant when clicked', async () => {
    const wrapper = mount(ScopeGrantRowActions, { props: { grant: GRANT } })
    await wrapper.get('button').trigger('click')
    expect(wrapper.emitted('revoke')).toHaveLength(1)
    expect(wrapper.emitted('revoke')![0]).toEqual([GRANT])
  })

  it('stops click propagation so row click handlers do not fire', async () => {
    let rowClicked = false
    const wrapper = mount(
      {
        components: { ScopeGrantRowActions },
        props: ['grant'],
        template:
          '<div @click="onRow"><ScopeGrantRowActions :grant="grant" @revoke="() => {}"/></div>',
        methods: {
          onRow() {
            rowClicked = true
          },
        },
      },
      { props: { grant: GRANT } },
    )

    await wrapper.get('button').trigger('click')
    expect(rowClicked).toBe(false)
  })
})
