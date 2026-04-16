import { describe, it, expect } from 'vitest'
import { mount } from '@vue/test-utils'
import PriorityReorder from '@/components/shared/PriorityReorder.vue'

function mountReorder(priority: number, canMoveUp: boolean, canMoveDown: boolean) {
  return mount(PriorityReorder, {
    props: { priority, canMoveUp, canMoveDown },
  })
}

describe('PriorityReorder', () => {
  it('displays the priority number', () => {
    const wrapper = mountReorder(3, true, true)
    expect(wrapper.text()).toContain('3')
  })

  it('exposes two accessible buttons with directional labels', () => {
    const wrapper = mountReorder(0, true, true)
    expect(wrapper.get('[aria-label="Move up"]').exists()).toBe(true)
    expect(wrapper.get('[aria-label="Move down"]').exists()).toBe(true)
  })

  it('emits up when the up button is clicked', async () => {
    const wrapper = mountReorder(0, true, true)
    await wrapper.get('[aria-label="Move up"]').trigger('click')
    expect(wrapper.emitted('up')).toHaveLength(1)
    expect(wrapper.emitted('down')).toBeUndefined()
  })

  it('emits down when the down button is clicked', async () => {
    const wrapper = mountReorder(0, true, true)
    await wrapper.get('[aria-label="Move down"]').trigger('click')
    expect(wrapper.emitted('down')).toHaveLength(1)
    expect(wrapper.emitted('up')).toBeUndefined()
  })

  it('disables the up button when canMoveUp is false', () => {
    const wrapper = mountReorder(0, false, true)
    expect(wrapper.get('[aria-label="Move up"]').attributes('disabled')).toBeDefined()
  })

  it('disables the down button when canMoveDown is false', () => {
    const wrapper = mountReorder(0, true, false)
    expect(wrapper.get('[aria-label="Move down"]').attributes('disabled')).toBeDefined()
  })

  it('stops click propagation on both buttons', async () => {
    let rowClicked = false
    const wrapper = mount(
      {
        components: { PriorityReorder },
        props: ['priority', 'canMoveUp', 'canMoveDown'],
        template:
          '<div @click="onRow"><PriorityReorder :priority="priority" :can-move-up="canMoveUp" :can-move-down="canMoveDown" @up="() => {}" @down="() => {}"/></div>',
        methods: {
          onRow() {
            rowClicked = true
          },
        },
      },
      { props: { priority: 0, canMoveUp: true, canMoveDown: true } },
    )

    await wrapper.get('[aria-label="Move up"]').trigger('click')
    expect(rowClicked).toBe(false)

    await wrapper.get('[aria-label="Move down"]').trigger('click')
    expect(rowClicked).toBe(false)
  })
})
