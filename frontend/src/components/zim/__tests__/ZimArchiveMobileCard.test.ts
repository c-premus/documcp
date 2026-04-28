import { describe, it, expect } from 'vitest'
import { mount } from '@vue/test-utils'
import ZimArchiveMobileCard from '@/components/zim/ZimArchiveMobileCard.vue'
import type { ZimArchive } from '@/stores/zimArchives'

const ARCHIVE: ZimArchive = {
  uuid: 'arc-1',
  name: 'wikipedia_en_2024_03',
  title: 'Wikipedia (English)',
  description: 'Wikipedia mirror',
  language: 'en',
  category: 'wikipedia',
  article_count: 6543210,
  media_count: 0,
  file_size: 102400000000,
  file_size_human: '95.2 GB',
  has_fulltext_index: true,
  tags: [],
  last_synced_at: '2026-04-25T00:00:00Z',
}

function mountCard(overrides: Partial<ZimArchive> = {}) {
  return mount(ZimArchiveMobileCard, { props: { archive: { ...ARCHIVE, ...overrides } } })
}

describe('ZimArchiveMobileCard', () => {
  it('renders the archive name and title', () => {
    const wrapper = mountCard()
    expect(wrapper.get('h3').text()).toBe('wikipedia_en_2024_03')
    expect(wrapper.text()).toContain('Wikipedia (English)')
  })

  it('shows category and search-type badges as accessible status elements', () => {
    const wrapper = mountCard()
    const statuses = wrapper.findAll('[role="status"]').map((s) => s.text())
    expect(statuses).toContain('wikipedia')
    expect(statuses).toContain('fulltext')
  })

  it('falls back to "title only" when fulltext index is absent', () => {
    const wrapper = mountCard({ has_fulltext_index: false })
    const statuses = wrapper.findAll('[role="status"]').map((s) => s.text())
    expect(statuses).toContain('title only')
  })

  it('renders article count, file size, language, and last-synced relative time', () => {
    const wrapper = mountCard()
    expect(wrapper.text()).toContain('6,543,210 articles')
    expect(wrapper.text()).toContain('95.2 GB')
    expect(wrapper.text()).toContain('EN')
    expect(wrapper.text()).toContain('synced')
  })

  it('shows "Never synced" when last_synced_at is missing', () => {
    const wrapper = mountCard({ last_synced_at: undefined })
    expect(wrapper.text()).toContain('Never synced')
  })
})
