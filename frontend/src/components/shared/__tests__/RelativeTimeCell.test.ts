import { describe, it, expect, beforeEach, afterEach, vi } from 'vitest'
import { mount } from '@vue/test-utils'
import RelativeTimeCell from '@/components/shared/RelativeTimeCell.vue'

describe('RelativeTimeCell', () => {
  beforeEach(() => {
    vi.useFakeTimers()
    vi.setSystemTime(new Date('2026-04-16T12:00:00Z'))
  })

  afterEach(() => {
    vi.useRealTimers()
  })

  it('renders a relative time label with suffix', () => {
    const wrapper = mount(RelativeTimeCell, {
      props: { value: '2026-04-16T11:00:00Z' },
    })
    expect(wrapper.text()).toContain('ago')
  })

  it('preserves the raw ISO value in the time datetime attribute for a11y', () => {
    const value = '2026-04-16T11:00:00Z'
    const wrapper = mount(RelativeTimeCell, { props: { value } })

    const time = wrapper.find('time')
    expect(time.exists()).toBe(true)
    expect(time.attributes('datetime')).toBe(value)
  })

  it('reacts to prop changes', async () => {
    const wrapper = mount(RelativeTimeCell, {
      props: { value: '2026-04-16T11:59:30Z' },
    })
    const firstLabel = wrapper.text()

    await wrapper.setProps({ value: '2026-04-15T12:00:00Z' })
    expect(wrapper.text()).not.toBe(firstLabel)
    expect(wrapper.find('time').attributes('datetime')).toBe('2026-04-15T12:00:00Z')
  })

  it('renders the fallback when value is null', () => {
    const wrapper = mount(RelativeTimeCell, {
      props: { value: null, fallback: 'Never' },
    })
    expect(wrapper.text()).toBe('Never')
    expect(wrapper.find('time').exists()).toBe(false)
  })

  it('renders the fallback when value is undefined', () => {
    const wrapper = mount(RelativeTimeCell, {
      props: { value: undefined, fallback: '—' },
    })
    expect(wrapper.text()).toBe('—')
  })

  it('renders the fallback when value is an empty string', () => {
    const wrapper = mount(RelativeTimeCell, {
      props: { value: '', fallback: 'Never' },
    })
    expect(wrapper.text()).toBe('Never')
  })

  it('renders empty string when fallback is not provided and value is empty', () => {
    const wrapper = mount(RelativeTimeCell, {
      props: { value: null },
    })
    expect(wrapper.text()).toBe('')
  })
})
