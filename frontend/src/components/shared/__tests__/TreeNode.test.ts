import { describe, it, expect } from 'vitest'
import { mount } from '@vue/test-utils'
import TreeNode from '@/components/shared/TreeNode.vue'

describe('TreeNode', () => {
  function mountNode(props = {}) {
    return mount(TreeNode, {
      props: {
        item: { name: 'file.ts', path: 'src/file.ts', type: 'file' as const },
        ...props,
      },
      global: {
        stubs: {
          FolderIcon: true,
          FolderOpenIcon: true,
          DocumentIcon: true,
          TreeNode: true,
        },
      },
    })
  }

  it('renders file name', () => {
    const wrapper = mountNode()
    expect(wrapper.text()).toContain('file.ts')
  })

  it('emits select with path when file is clicked', async () => {
    const wrapper = mountNode()
    await wrapper.find('button').trigger('click')
    expect(wrapper.emitted('select')?.[0]).toEqual(['src/file.ts'])
  })

  it('renders directory with aria-expanded', () => {
    const wrapper = mountNode({
      item: {
        name: 'src',
        path: 'src',
        type: 'directory',
        children: [{ name: 'index.ts', path: 'src/index.ts', type: 'file' }],
      },
    })
    const btn = wrapper.find('button')
    expect(btn.attributes('aria-expanded')).toBeDefined()
  })

  it('toggles expanded state on directory click', async () => {
    const wrapper = mountNode({
      item: {
        name: 'src',
        path: 'src',
        type: 'directory',
        children: [{ name: 'index.ts', path: 'src/index.ts', type: 'file' }],
      },
      depth: 0,
    })
    const btn = wrapper.find('button')

    // Initially expanded (depth < 2)
    expect(btn.attributes('aria-expanded')).toBe('true')

    await btn.trigger('click')
    expect(btn.attributes('aria-expanded')).toBe('false')
  })

  it('does not emit select when directory is clicked', async () => {
    const wrapper = mountNode({
      item: { name: 'src', path: 'src', type: 'directory', children: [] },
    })
    await wrapper.find('button').trigger('click')
    expect(wrapper.emitted('select')).toBeUndefined()
  })

  it('highlights selected file', () => {
    const wrapper = mountNode({
      item: { name: 'file.ts', path: 'src/file.ts', type: 'file' },
      selectedPath: 'src/file.ts',
    })
    expect(wrapper.find('button').classes()).toContain('bg-indigo-50')
  })

  it('does not highlight non-selected file', () => {
    const wrapper = mountNode({
      item: { name: 'file.ts', path: 'src/file.ts', type: 'file' },
      selectedPath: 'src/other.ts',
    })
    expect(wrapper.find('button').classes()).not.toContain('bg-indigo-50')
  })
})
