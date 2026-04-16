import { describe, it, expect } from 'vitest'
import { mount } from '@vue/test-utils'
import UserRowActions from '@/components/users/UserRowActions.vue'

const USER = { id: 42, name: 'Alice' }

describe('UserRowActions', () => {
  it('renders a single delete button with aria-label', () => {
    const wrapper = mount(UserRowActions, { props: { user: USER } })
    const buttons = wrapper.findAll('button')
    expect(buttons).toHaveLength(1)
    expect(buttons[0]!.attributes('aria-label')).toBe('Delete user')
  })

  it('emits delete with the user when clicked', async () => {
    const wrapper = mount(UserRowActions, { props: { user: USER } })
    await wrapper.get('[aria-label="Delete user"]').trigger('click')
    expect(wrapper.emitted('delete')).toHaveLength(1)
    expect(wrapper.emitted('delete')![0]).toEqual([USER])
  })

  it('stops click propagation', async () => {
    let rowClicked = false
    const wrapper = mount(
      {
        components: { UserRowActions },
        props: ['user'],
        template: '<div @click="onRow"><UserRowActions :user="user" @delete="() => {}"/></div>',
        methods: {
          onRow() {
            rowClicked = true
          },
        },
      },
      { props: { user: USER } },
    )

    await wrapper.get('[aria-label="Delete user"]').trigger('click')
    expect(rowClicked).toBe(false)
  })
})
