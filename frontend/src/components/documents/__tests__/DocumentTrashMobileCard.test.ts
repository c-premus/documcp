import { describe, it, expect } from 'vitest'
import { mount } from '@vue/test-utils'
import DocumentTrashMobileCard from '@/components/documents/DocumentTrashMobileCard.vue'
import type { Document } from '@/stores/documents'

const DOC: Document = {
  uuid: 'doc-1',
  title: 'Old Plan',
  description: '',
  file_type: 'pdf',
  file_size: 1024,
  mime_type: 'application/pdf',
  word_count: 0,
  is_public: false,
  has_file: true,
  status: 'indexed',
  content_hash: '',
  tags: [],
  created_at: '2026-01-01T00:00:00Z',
  updated_at: '2026-01-15T00:00:00Z',
  processed_at: '2026-01-01T00:00:00Z',
}

describe('DocumentTrashMobileCard', () => {
  it('renders the title and "Deleted" label', () => {
    const wrapper = mount(DocumentTrashMobileCard, { props: { document: DOC, canPurge: true } })
    expect(wrapper.get('h3').text()).toBe('Old Plan')
    expect(wrapper.text()).toContain('Deleted')
  })

  it('shows only restore for non-admins', () => {
    const wrapper = mount(DocumentTrashMobileCard, { props: { document: DOC, canPurge: false } })
    const labels = wrapper.findAll('button').map((b) => b.attributes('aria-label'))
    expect(labels).toEqual(['Restore document'])
  })

  it('shows restore + purge for admins', () => {
    const wrapper = mount(DocumentTrashMobileCard, { props: { document: DOC, canPurge: true } })
    const labels = wrapper.findAll('button').map((b) => b.attributes('aria-label'))
    expect(labels).toEqual(['Restore document', 'Permanently delete'])
  })

  it('emits restore + purge with the document payload', async () => {
    const wrapper = mount(DocumentTrashMobileCard, { props: { document: DOC, canPurge: true } })
    await wrapper.get('[aria-label="Restore document"]').trigger('click')
    await wrapper.get('[aria-label="Permanently delete"]').trigger('click')

    expect(wrapper.emitted('restore')![0]).toEqual([DOC])
    expect(wrapper.emitted('purge')![0]).toEqual([DOC])
  })
})
