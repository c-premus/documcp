import { describe, it, expect } from 'vitest'
import { mount } from '@vue/test-utils'
import TruncatedText from '@/components/shared/TruncatedText.vue'

// The `mono` and `maxWidth` props each map to a Tailwind class; the class IS
// the observable contract (callers customize typography by passing one of
// these props). The class assertions below are prop-passthrough checks, not
// the Tailwind-class-string theater that the 2026-04-15 audit called out.
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
    // Contract check: default typography is not monospace.
    expect(wrapper.classes()).not.toContain('font-mono')
  })

  it('opts into monospace when mono=true', () => {
    const wrapper = mount(TruncatedText, { props: { value: 'hi', mono: true } })
    // Contract check: `mono` prop passthrough to `font-mono`.
    expect(wrapper.classes()).toContain('font-mono')
  })

  it('accepts a custom maxWidth class', () => {
    const wrapper = mount(TruncatedText, {
      props: { value: 'hi', maxWidth: 'max-w-[8rem]' },
    })
    // Contract check: caller-supplied maxWidth is applied verbatim.
    expect(wrapper.classes()).toContain('max-w-[8rem]')
  })

  it('defaults maxWidth to max-w-xs', () => {
    const wrapper = mount(TruncatedText, { props: { value: 'hi' } })
    // Contract check: documented default width.
    expect(wrapper.classes()).toContain('max-w-xs')
  })
})
