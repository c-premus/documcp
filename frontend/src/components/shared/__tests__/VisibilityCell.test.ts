import { describe, it, expect } from 'vitest'
import { mount } from '@vue/test-utils'
import VisibilityCell from '@/components/shared/VisibilityCell.vue'
import StatusBadge from '@/components/shared/StatusBadge.vue'

describe('VisibilityCell', () => {
  it('renders "public" label when isPublic=true', () => {
    const wrapper = mount(VisibilityCell, { props: { isPublic: true } })
    expect(wrapper.text()).toBe('public')
  })

  it('renders "private" label when isPublic=false', () => {
    const wrapper = mount(VisibilityCell, { props: { isPublic: false } })
    expect(wrapper.text()).toBe('private')
  })

  it('delegates rendering to StatusBadge with the derived status prop', () => {
    const wrapper = mount(VisibilityCell, { props: { isPublic: true } })
    const badge = wrapper.findComponent(StatusBadge)
    expect(badge.exists()).toBe(true)
    expect(badge.props('status')).toBe('public')
  })
})
