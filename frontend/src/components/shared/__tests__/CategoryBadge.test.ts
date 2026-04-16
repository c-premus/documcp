import { describe, it, expect } from 'vitest'
import { mount } from '@vue/test-utils'
import CategoryBadge from '@/components/shared/CategoryBadge.vue'

const PALETTE = {
  kiwix: 'bg-blue-100 text-blue-800',
  confluence: 'bg-violet-100 text-violet-800',
} as const

describe('CategoryBadge', () => {
  it('renders the value as the visible label', () => {
    expect(mount(CategoryBadge, { props: { value: 'kiwix', palette: PALETTE } }).text()).toBe(
      'kiwix',
    )
  })

  it('replaces underscores with spaces in the visible label', () => {
    expect(mount(CategoryBadge, { props: { value: 'memory_bank', palette: PALETTE } }).text()).toBe(
      'memory bank',
    )
  })

  it('applies palette class when value matches a key', () => {
    const wrapper = mount(CategoryBadge, { props: { value: 'kiwix', palette: PALETTE } })
    expect(wrapper.classes()).toContain('bg-blue-100')
  })

  it('falls back to gray palette when value is unknown', () => {
    const wrapper = mount(CategoryBadge, {
      props: { value: 'unknown-value', palette: PALETTE },
    })
    expect(wrapper.classes()).toContain('bg-gray-100')
  })

  it('reacts to prop changes', async () => {
    const wrapper = mount(CategoryBadge, { props: { value: 'kiwix', palette: PALETTE } })
    expect(wrapper.text()).toBe('kiwix')
    await wrapper.setProps({ value: 'confluence' })
    expect(wrapper.text()).toBe('confluence')
  })
})
