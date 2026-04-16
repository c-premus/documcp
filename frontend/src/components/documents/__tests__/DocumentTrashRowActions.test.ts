import { describe, it, expect } from 'vitest'
import { mount } from '@vue/test-utils'
import DocumentTrashRowActions from '@/components/documents/DocumentTrashRowActions.vue'
import type { Document } from '@/stores/documents'

const DOC: Document = {
  uuid: 'doc-99',
  title: 'Trashed Document',
  description: '',
  file_type: 'pdf',
  file_size: 2048,
  mime_type: 'application/pdf',
  word_count: 0,
  is_public: false,
  has_file: true,
  status: 'indexed',
  content_hash: '',
  tags: [],
  created_at: '2026-01-01T00:00:00Z',
  updated_at: '2026-04-01T00:00:00Z',
  processed_at: '2026-01-01T00:00:00Z',
}

describe('DocumentTrashRowActions', () => {
  it('exposes a restore button for all users', () => {
    const wrapper = mount(DocumentTrashRowActions, {
      props: { document: DOC, canPurge: false },
    })
    expect(wrapper.get('[aria-label="Restore document"]').exists()).toBe(true)
  })

  it('hides the purge button when canPurge is false', () => {
    const wrapper = mount(DocumentTrashRowActions, {
      props: { document: DOC, canPurge: false },
    })
    expect(wrapper.find('[aria-label="Permanently delete"]').exists()).toBe(false)
  })

  it('shows the purge button when canPurge is true', () => {
    const wrapper = mount(DocumentTrashRowActions, {
      props: { document: DOC, canPurge: true },
    })
    expect(wrapper.get('[aria-label="Permanently delete"]').exists()).toBe(true)
  })

  it('emits restore with the document when restore is clicked', async () => {
    const wrapper = mount(DocumentTrashRowActions, {
      props: { document: DOC, canPurge: true },
    })
    await wrapper.get('[aria-label="Restore document"]').trigger('click')
    expect(wrapper.emitted('restore')![0]).toEqual([DOC])
  })

  it('emits purge with the document when purge is clicked', async () => {
    const wrapper = mount(DocumentTrashRowActions, {
      props: { document: DOC, canPurge: true },
    })
    await wrapper.get('[aria-label="Permanently delete"]').trigger('click')
    expect(wrapper.emitted('purge')![0]).toEqual([DOC])
  })
})
