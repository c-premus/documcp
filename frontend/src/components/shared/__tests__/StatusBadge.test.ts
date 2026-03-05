import { describe, it, expect } from 'vitest'
import { mount } from '@vue/test-utils'
import StatusBadge from '@/components/shared/StatusBadge.vue'

function mountBadge(status: string) {
  return mount(StatusBadge, { props: { status } })
}

describe('StatusBadge', () => {
  it('renders uploaded status with yellow classes', () => {
    const wrapper = mountBadge('uploaded')

    const span = wrapper.find('span')
    expect(span.classes()).toContain('bg-yellow-100')
    expect(span.classes()).toContain('text-yellow-800')
  })

  it('renders extracted status with blue classes', () => {
    const wrapper = mountBadge('extracted')

    const span = wrapper.find('span')
    expect(span.classes()).toContain('bg-blue-100')
    expect(span.classes()).toContain('text-blue-800')
  })

  it('renders indexed status with green classes', () => {
    const wrapper = mountBadge('indexed')

    const span = wrapper.find('span')
    expect(span.classes()).toContain('bg-green-100')
    expect(span.classes()).toContain('text-green-800')
  })

  it('renders failed status with red classes', () => {
    const wrapper = mountBadge('failed')

    const span = wrapper.find('span')
    expect(span.classes()).toContain('bg-red-100')
    expect(span.classes()).toContain('text-red-800')
  })

  it('renders index_failed status with orange classes', () => {
    const wrapper = mountBadge('index_failed')

    const span = wrapper.find('span')
    expect(span.classes()).toContain('bg-orange-100')
    expect(span.classes()).toContain('text-orange-800')
  })

  it('renders unknown status with gray classes', () => {
    const wrapper = mountBadge('unknown_status')

    const span = wrapper.find('span')
    expect(span.classes()).toContain('bg-gray-100')
    expect(span.classes()).toContain('text-gray-800')
  })

  it('displays the status text', () => {
    const wrapper = mountBadge('uploaded')

    expect(wrapper.text()).toBe('uploaded')
  })

  it('replaces underscores with spaces in display label', () => {
    const wrapper = mountBadge('index_failed')

    expect(wrapper.text()).toBe('index failed')
  })
})
