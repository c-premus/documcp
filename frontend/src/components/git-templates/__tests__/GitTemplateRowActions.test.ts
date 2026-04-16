import { describe, it, expect } from 'vitest'
import { mount } from '@vue/test-utils'
import GitTemplateRowActions from '@/components/git-templates/GitTemplateRowActions.vue'
import type { GitTemplate } from '@/stores/gitTemplates'

const TEMPLATE: GitTemplate = {
  uuid: 'tpl-1',
  name: 'claude-md',
  slug: 'claude-md',
  repository_url: 'https://git.example.com/claude',
  branch: 'main',
  tags: [],
  is_public: true,
  status: 'synced',
  file_count: 5,
  total_size_bytes: 1024,
}

function mountActions(syncing = false) {
  return mount(GitTemplateRowActions, { props: { template: TEMPLATE, syncing } })
}

describe('GitTemplateRowActions', () => {
  it('exposes three accessible buttons', () => {
    const wrapper = mountActions()
    const labels = wrapper.findAll('button').map((b) => b.attributes('aria-label'))
    expect(labels).toEqual(['Edit template', 'Sync template', 'Delete template'])
  })

  it('emits edit with the template when edit is clicked', async () => {
    const wrapper = mountActions()
    await wrapper.get('[aria-label="Edit template"]').trigger('click')
    expect(wrapper.emitted('edit')![0]).toEqual([TEMPLATE])
  })

  it('emits sync with the template when sync is clicked', async () => {
    const wrapper = mountActions()
    await wrapper.get('[aria-label="Sync template"]').trigger('click')
    expect(wrapper.emitted('sync')![0]).toEqual([TEMPLATE])
  })

  it('emits delete with the template when delete is clicked', async () => {
    const wrapper = mountActions()
    await wrapper.get('[aria-label="Delete template"]').trigger('click')
    expect(wrapper.emitted('delete')![0]).toEqual([TEMPLATE])
  })

  it('disables sync while syncing', () => {
    const wrapper = mountActions(true)
    expect(wrapper.get('[aria-label="Sync template"]').attributes('disabled')).toBeDefined()
  })

  it('does not emit sync when clicked during syncing', async () => {
    const wrapper = mountActions(true)
    await wrapper.get('[aria-label="Sync template"]').trigger('click')
    expect(wrapper.emitted('sync')).toBeUndefined()
  })
})
