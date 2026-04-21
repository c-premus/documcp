import { describe, it, expect } from 'vitest'
import { mount } from '@vue/test-utils'
import OAuthScopeChips from '@/components/oauth/OAuthScopeChips.vue'

describe('OAuthScopeChips', () => {
  it('splits a space-delimited scope string into one chip per scope', () => {
    const wrapper = mount(OAuthScopeChips, {
      props: { scope: 'documents:read documents:write mcp:access' },
    })
    const chips = wrapper.findAll('span')
    expect(chips).toHaveLength(3)
    expect(chips[0]!.text()).toBe('documents:read')
    expect(chips[1]!.text()).toBe('documents:write')
    expect(chips[2]!.text()).toBe('mcp:access')
  })

  it('accepts an array directly', () => {
    const wrapper = mount(OAuthScopeChips, {
      props: { scope: ['read', 'write'] },
    })
    expect(wrapper.findAll('span')).toHaveLength(2)
  })

  it('collapses repeated whitespace without producing empty chips', () => {
    const wrapper = mount(OAuthScopeChips, {
      props: { scope: '  documents:read   mcp:access   ' },
    })
    expect(wrapper.findAll('span')).toHaveLength(2)
  })

  it('renders nothing for an empty scope', () => {
    const wrapper = mount(OAuthScopeChips, { props: { scope: '' } })
    expect(wrapper.findAll('span')).toHaveLength(0)
  })
})
