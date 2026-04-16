import { describe, it, expect } from 'vitest'
import { mount } from '@vue/test-utils'
import TruncatedText from '@/components/shared/TruncatedText.vue'

describe('TruncatedText', () => {
  it('renders value as visible text', () => {
    expect(mount(TruncatedText, { props: { value: 'hello world' } }).text()).toBe('hello world')
  })

  it('exposes the full value via the title attribute for hover discovery', () => {
    const value = 'a long url that will be visually truncated'
    const wrapper = mount(TruncatedText, { props: { value } })
    expect(wrapper.attributes('title')).toBe(value)
  })

  it('is not monospace by default', () => {
    const wrapper = mount(TruncatedText, { props: { value: 'hi' } })
    expect(wrapper.classes()).not.toContain('font-mono')
  })

  it('opts into monospace when mono=true', () => {
    const wrapper = mount(TruncatedText, { props: { value: 'hi', mono: true } })
    expect(wrapper.classes()).toContain('font-mono')
  })

  it('accepts a custom maxWidth class', () => {
    const wrapper = mount(TruncatedText, {
      props: { value: 'hi', maxWidth: 'max-w-[8rem]' },
    })
    expect(wrapper.classes()).toContain('max-w-[8rem]')
  })

  it('defaults maxWidth to max-w-xs', () => {
    const wrapper = mount(TruncatedText, { props: { value: 'hi' } })
    expect(wrapper.classes()).toContain('max-w-xs')
  })
})
