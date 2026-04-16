import { describe, it, expect } from 'vitest'
import { mount } from '@vue/test-utils'
import StatusBadge from '@/components/shared/StatusBadge.vue'

function mountBadge(status: string) {
  return mount(StatusBadge, { props: { status } })
}

describe('StatusBadge', () => {
  it('exposes role="status" so assistive tech reads it as a status region', () => {
    const wrapper = mountBadge('uploaded')
    expect(wrapper.get('[role="status"]').exists()).toBe(true)
  })

  it('renders the status string as visible text', () => {
    expect(mountBadge('uploaded').text()).toBe('uploaded')
    expect(mountBadge('indexed').text()).toBe('indexed')
    expect(mountBadge('failed').text()).toBe('failed')
  })

  it('replaces underscores with spaces in the display label', () => {
    expect(mountBadge('index_failed').text()).toBe('index failed')
  })

  it('preserves unknown statuses in display text (does not drop them)', () => {
    expect(mountBadge('custom_unknown_state').text()).toBe('custom unknown state')
  })

  it('reacts to prop changes', async () => {
    const wrapper = mountBadge('uploaded')
    expect(wrapper.text()).toBe('uploaded')
    await wrapper.setProps({ status: 'indexed' })
    expect(wrapper.text()).toBe('indexed')
  })
})
