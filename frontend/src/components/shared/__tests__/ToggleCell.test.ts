import { describe, it, expect } from 'vitest'
import { mount } from '@vue/test-utils'
import ToggleCell from '@/components/shared/ToggleCell.vue'

describe('ToggleCell', () => {
  it('exposes the label prop via aria-label', () => {
    const wrapper = mount(ToggleCell, {
      props: { modelValue: false, label: 'Toggle admin for Alice' },
    })
    const toggle = wrapper.get('[role="switch"]')
    expect(toggle.attributes('aria-label')).toBe('Toggle admin for Alice')
  })

  it('reflects modelValue through aria-checked', () => {
    const off = mount(ToggleCell, { props: { modelValue: false, label: 'x' } })
    expect(off.get('[role="switch"]').attributes('aria-checked')).toBe('false')

    const on = mount(ToggleCell, { props: { modelValue: true, label: 'x' } })
    expect(on.get('[role="switch"]').attributes('aria-checked')).toBe('true')
  })

  it('emits update:modelValue with the flipped boolean on click', async () => {
    const wrapper = mount(ToggleCell, {
      props: { modelValue: false, label: 'x' },
    })
    await wrapper.get('[role="switch"]').trigger('click')
    expect(wrapper.emitted('update:modelValue')).toHaveLength(1)
    expect(wrapper.emitted('update:modelValue')![0]).toEqual([true])
  })

  it('stops click propagation so row-click handlers do not fire', async () => {
    let rowClicked = false
    const wrapper = mount(
      {
        components: { ToggleCell },
        props: ['modelValue', 'label'],
        template:
          '<div @click="onRow"><ToggleCell :model-value="modelValue" :label="label" @update:model-value="() => {}"/></div>',
        methods: {
          onRow() {
            rowClicked = true
          },
        },
      },
      { props: { modelValue: false, label: 'x' } },
    )

    await wrapper.get('[role="switch"]').trigger('click')
    expect(rowClicked).toBe(false)
  })
})
