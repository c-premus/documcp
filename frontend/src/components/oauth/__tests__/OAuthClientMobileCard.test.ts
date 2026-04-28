import { describe, it, expect } from 'vitest'
import { mount } from '@vue/test-utils'
import { createMemoryHistory, createRouter } from 'vue-router'
import OAuthClientMobileCard from '@/components/oauth/OAuthClientMobileCard.vue'
import type { OAuthClient } from '@/stores/oauthClients'

const CLIENT: OAuthClient = {
  id: 42,
  client_id: 'cli_xyz_long_identifier',
  client_name: 'My MCP Client',
  redirect_uris: ['https://example.com/cb'],
  grant_types: ['authorization_code', 'refresh_token'],
  response_types: ['code'],
  token_endpoint_auth_method: 'client_secret_basic',
  scope: 'mcp:read',
  last_used_at: '2026-04-20T00:00:00Z',
  created_at: '2026-01-01T00:00:00Z',
  updated_at: '2026-01-01T00:00:00Z',
}

function mountCard(overrides: Partial<OAuthClient> = {}) {
  const router = createRouter({
    history: createMemoryHistory(),
    routes: [{ path: '/:pathMatch(.*)*', component: { template: '<div />' } }],
  })
  return mount(OAuthClientMobileCard, {
    props: { client: { ...CLIENT, ...overrides } },
    global: { plugins: [router] },
  })
}

describe('OAuthClientMobileCard', () => {
  it('renders the client name as a link to the detail view', () => {
    const wrapper = mountCard()
    const link = wrapper.get('a')
    expect(link.text()).toBe('My MCP Client')
    expect(link.attributes('href')).toBe('/oauth-clients/42')
  })

  it('shows the client_id and a humanized auth method', () => {
    const wrapper = mountCard()
    expect(wrapper.text()).toContain('cli_xyz_long_identifier')
    expect(wrapper.text()).toContain('client secret basic')
  })

  it('includes a delete button labeled with the client name', () => {
    const wrapper = mountCard()
    const button = wrapper.get(`[aria-label="Delete client My MCP Client"]`)
    expect(button.exists()).toBe(true)
  })

  it('emits delete with the client when the delete button fires', async () => {
    const wrapper = mountCard()
    await wrapper.get(`[aria-label="Delete client My MCP Client"]`).trigger('click')
    expect(wrapper.emitted('delete')![0]).toEqual([CLIENT])
  })
})
