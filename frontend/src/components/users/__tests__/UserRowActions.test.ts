import { describe, it, expect } from 'vitest'
import { mount } from '@vue/test-utils'
import UserRowActions from '@/components/users/UserRowActions.vue'

const USER = { id: 42, name: 'Alice' }

describe('UserRowActions', () => {
  it('renders sessions and delete buttons with aria-labels', () => {
    const wrapper = mount(UserRowActions, { props: { user: USER } })
    const buttons = wrapper.findAll('button')
    expect(buttons).toHaveLength(2)
    expect(buttons[0]!.attributes('aria-label')).toBe('View active sessions')
    expect(buttons[1]!.attributes('aria-label')).toBe('Delete user')
  })

  it('emits delete with the user when clicked', async () => {
    const wrapper = mount(UserRowActions, { props: { user: USER } })
    await wrapper.get('[aria-label="Delete user"]').trigger('click')
    expect(wrapper.emitted('delete')).toHaveLength(1)
    expect(wrapper.emitted('delete')![0]).toEqual([USER])
  })

  it('emits sessions with the user when clicked', async () => {
    const wrapper = mount(UserRowActions, { props: { user: USER } })
    await wrapper.get('[aria-label="View active sessions"]').trigger('click')
    expect(wrapper.emitted('sessions')).toHaveLength(1)
    expect(wrapper.emitted('sessions')![0]).toEqual([USER])
  })

  it('stops click propagation on every action button', async () => {
    let rowClicked = false
    const wrapper = mount(
      {
        components: { UserRowActions },
        props: ['user'],
        template:
          '<div @click="onRow"><UserRowActions :user="user" @delete="() => {}" @sessions="() => {}"/></div>',
        methods: {
          onRow() {
            rowClicked = true
          },
        },
      },
      { props: { user: USER } },
    )

    await wrapper.get('[aria-label="View active sessions"]').trigger('click')
    expect(rowClicked).toBe(false)

    await wrapper.get('[aria-label="Delete user"]').trigger('click')
    expect(rowClicked).toBe(false)
  })
})
