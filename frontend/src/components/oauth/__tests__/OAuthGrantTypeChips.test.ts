import { describe, it, expect } from 'vitest'
import { mount } from '@vue/test-utils'
import OAuthGrantTypeChips from '@/components/oauth/OAuthGrantTypeChips.vue'

describe('OAuthGrantTypeChips', () => {
  it('renders one chip per grant type', () => {
    const wrapper = mount(OAuthGrantTypeChips, {
      props: {
        grantTypes: [
          'authorization_code',
          'refresh_token',
          'urn:ietf:params:oauth:grant-type:device_code',
        ],
      },
    })
    const chips = wrapper.findAll('span')
    expect(chips).toHaveLength(3)
  })

  it('replaces underscores with spaces so grant names read naturally', () => {
    const wrapper = mount(OAuthGrantTypeChips, {
      props: { grantTypes: ['authorization_code', 'refresh_token'] },
    })
    expect(wrapper.text()).toContain('authorization code')
    expect(wrapper.text()).toContain('refresh token')
    expect(wrapper.text()).not.toContain('_')
  })

  it('renders nothing when the array is empty', () => {
    const wrapper = mount(OAuthGrantTypeChips, { props: { grantTypes: [] } })
    expect(wrapper.findAll('span')).toHaveLength(0)
  })
})
