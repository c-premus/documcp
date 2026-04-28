import { describe, it, expect } from 'vitest'
import { mount } from '@vue/test-utils'
import DocumentMobileCard from '@/components/documents/DocumentMobileCard.vue'
import type { Document } from '@/stores/documents'

const DOC: Document = {
  uuid: 'doc-1',
  title: 'Quarterly Plan',
  description: '',
  file_type: 'pdf',
  file_size: 2048,
  mime_type: 'application/pdf',
  word_count: 0,
  is_public: true,
  has_file: true,
  status: 'indexed',
  content_hash: '',
  tags: [],
  created_at: '2026-01-01T00:00:00Z',
  updated_at: '2026-01-01T00:00:00Z',
  processed_at: '2026-01-01T00:00:00Z',
}

function mountCard() {
  return mount(DocumentMobileCard, { props: { document: DOC } })
}

describe('DocumentMobileCard', () => {
  it('renders the document title', () => {
    const wrapper = mountCard()
    expect(wrapper.get('h3').text()).toBe('Quarterly Plan')
  })

  it('exposes status and visibility via accessible status badges', () => {
    const wrapper = mountCard()
    const statuses = wrapper.findAll('[role="status"]')

    const labels = statuses.map((s) => s.text())
    expect(labels).toContain('indexed')
    expect(labels).toContain('public')
  })

  it('renders the row-actions cluster with three buttons', () => {
    const wrapper = mountCard()

    const labels = wrapper.findAll('button').map((b) => b.attributes('aria-label'))
    expect(labels).toEqual(['Edit document', 'View document', 'Delete document'])
  })

  it('forwards edit / view / delete events with the document payload', async () => {
    const wrapper = mountCard()

    await wrapper.get('[aria-label="Edit document"]').trigger('click')
    await wrapper.get('[aria-label="View document"]').trigger('click')
    await wrapper.get('[aria-label="Delete document"]').trigger('click')

    expect(wrapper.emitted('edit')![0]).toEqual([DOC])
    expect(wrapper.emitted('view')![0]).toEqual([DOC])
    expect(wrapper.emitted('delete')![0]).toEqual([DOC])
  })
})
