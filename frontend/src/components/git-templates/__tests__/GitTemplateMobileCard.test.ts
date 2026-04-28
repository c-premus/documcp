import { describe, it, expect } from 'vitest'
import { mount } from '@vue/test-utils'
import GitTemplateMobileCard from '@/components/git-templates/GitTemplateMobileCard.vue'
import type { GitTemplate } from '@/stores/gitTemplates'

const TEMPLATE: GitTemplate = {
  uuid: 'tpl-1',
  name: 'Memory Bank',
  slug: 'memory-bank',
  description: 'Persistent context for AI sessions',
  repository_url: 'git@github.com:c-premus/memory-bank.git',
  branch: 'main',
  category: 'memory-bank',
  tags: [],
  is_public: true,
  status: 'synced',
  file_count: 12,
  total_size_bytes: 8192,
  last_synced_at: '2026-04-25T00:00:00Z',
  created_at: '2026-01-01T00:00:00Z',
  updated_at: '2026-01-01T00:00:00Z',
}

function mountCard(
  overrides: Partial<{
    template: GitTemplate
    syncing: boolean
    isAdmin: boolean
  }> = {},
) {
  return mount(GitTemplateMobileCard, {
    props: {
      template: TEMPLATE,
      syncing: false,
      isAdmin: true,
      ...overrides,
    },
  })
}

describe('GitTemplateMobileCard', () => {
  it('renders name, description, repository URL, and branch', () => {
    const wrapper = mountCard()
    expect(wrapper.get('h3').text()).toBe('Memory Bank')
    expect(wrapper.text()).toContain('Persistent context for AI sessions')
    expect(wrapper.text()).toContain('git@github.com:c-premus/memory-bank.git')
    expect(wrapper.text()).toContain('main')
  })

  it('renders the row-actions cluster for admins', () => {
    const wrapper = mountCard({ isAdmin: true })
    const labels = wrapper.findAll('button').map((b) => b.attributes('aria-label'))
    expect(labels).toEqual(['Edit template', 'Sync template', 'Delete template'])
  })

  it('hides the row-actions cluster for non-admins', () => {
    const wrapper = mountCard({ isAdmin: false })
    expect(wrapper.findAll('button')).toHaveLength(0)
  })

  it('emits edit / sync / delete with the template payload', async () => {
    const wrapper = mountCard()
    await wrapper.get('[aria-label="Edit template"]').trigger('click')
    await wrapper.get('[aria-label="Sync template"]').trigger('click')
    await wrapper.get('[aria-label="Delete template"]').trigger('click')

    expect(wrapper.emitted('edit')![0]).toEqual([TEMPLATE])
    expect(wrapper.emitted('sync')![0]).toEqual([TEMPLATE])
    expect(wrapper.emitted('delete')![0]).toEqual([TEMPLATE])
  })

  it('disables the sync button while syncing', () => {
    const wrapper = mountCard({ syncing: true })
    expect(wrapper.get('[aria-label="Sync template"]').attributes('disabled')).toBeDefined()
  })

  it('shows file count and last-synced relative time', () => {
    const wrapper = mountCard()
    expect(wrapper.text()).toContain('12 files')
    expect(wrapper.text()).toContain('Synced')
  })
})
