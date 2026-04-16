import { describe, it, expect } from 'vitest'
import { mount } from '@vue/test-utils'
import OAuthClientRowActions from '@/components/oauth/OAuthClientRowActions.vue'
import type { OAuthClient } from '@/stores/oauthClients'

const CLIENT: OAuthClient = {
  id: 7,
  client_id: 'abc-123',
  client_name: 'My Test Client',
  redirect_uris: ['https://example.com/cb'],
  grant_types: ['authorization_code'],
  response_types: ['code'],
  token_endpoint_auth_method: 'client_secret_basic',
  scope: 'read',
  last_used_at: null,
  created_at: '2026-01-01T00:00:00Z',
  updated_at: '2026-01-01T00:00:00Z',
}

describe('OAuthClientRowActions', () => {
  it('includes the client name in the aria-label so screen readers disambiguate rows', () => {
    const wrapper = mount(OAuthClientRowActions, { props: { client: CLIENT } })
    const button = wrapper.get('button')
    expect(button.attributes('aria-label')).toBe('Delete client My Test Client')
  })

  it('emits delete with the client when clicked', async () => {
    const wrapper = mount(OAuthClientRowActions, { props: { client: CLIENT } })
    await wrapper.get('button').trigger('click')
    expect(wrapper.emitted('delete')).toHaveLength(1)
    expect(wrapper.emitted('delete')![0]).toEqual([CLIENT])
  })

  it('stops click propagation', async () => {
    let rowClicked = false
    const wrapper = mount(
      {
        components: { OAuthClientRowActions },
        props: ['client'],
        template:
          '<div @click="onRow"><OAuthClientRowActions :client="client" @delete="() => {}"/></div>',
        methods: {
          onRow() {
            rowClicked = true
          },
        },
      },
      { props: { client: CLIENT } },
    )

    await wrapper.get('button').trigger('click')
    expect(rowClicked).toBe(false)
  })
})
