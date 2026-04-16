import { describe, it, expect } from 'vitest'
import { mount } from '@vue/test-utils'
import DocumentRowActions from '@/components/documents/DocumentRowActions.vue'
import type { Document } from '@/stores/documents'

const DOC: Document = {
  uuid: 'doc-1',
  title: 'Test',
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
  updated_at: '2026-01-01T00:00:00Z',
  processed_at: '2026-01-01T00:00:00Z',
}

function mountActions() {
  return mount(DocumentRowActions, { props: { document: DOC } })
}

describe('DocumentRowActions', () => {
  it('exposes three accessible buttons with descriptive labels', () => {
    const wrapper = mountActions()
    const buttons = wrapper.findAll('button')
    expect(buttons).toHaveLength(3)

    const labels = buttons.map((b) => b.attributes('aria-label'))
    expect(labels).toEqual(['Edit document', 'View document', 'Delete document'])

    for (const b of buttons) {
      expect(b.attributes('type')).toBe('button')
    }
  })

  it('emits edit with the document when the edit button is clicked', async () => {
    const wrapper = mountActions()
    await wrapper.get('[aria-label="Edit document"]').trigger('click')

    expect(wrapper.emitted('edit')).toHaveLength(1)
    expect(wrapper.emitted('edit')![0]).toEqual([DOC])
    expect(wrapper.emitted('view')).toBeUndefined()
    expect(wrapper.emitted('delete')).toBeUndefined()
  })

  it('emits view with the document when the view button is clicked', async () => {
    const wrapper = mountActions()
    await wrapper.get('[aria-label="View document"]').trigger('click')

    expect(wrapper.emitted('view')).toHaveLength(1)
    expect(wrapper.emitted('view')![0]).toEqual([DOC])
  })

  it('emits delete with the document when the delete button is clicked', async () => {
    const wrapper = mountActions()
    await wrapper.get('[aria-label="Delete document"]').trigger('click')

    expect(wrapper.emitted('delete')).toHaveLength(1)
    expect(wrapper.emitted('delete')![0]).toEqual([DOC])
  })

  it('stops click propagation so row-level handlers do not also fire', async () => {
    let rowClicked = false
    const wrapper = mount(
      {
        components: { DocumentRowActions },
        props: ['document'],
        template:
          '<div @click="onRow"><DocumentRowActions :document="document" @edit="()=>{}" @view="()=>{}" @delete="()=>{}"/></div>',
        methods: {
          onRow(this: { rowClicked: boolean }) {
            rowClicked = true
          },
        },
      },
      { props: { document: DOC } },
    )

    await wrapper.get('[aria-label="Edit document"]').trigger('click')
    expect(rowClicked).toBe(false)

    await wrapper.get('[aria-label="View document"]').trigger('click')
    expect(rowClicked).toBe(false)

    await wrapper.get('[aria-label="Delete document"]').trigger('click')
    expect(rowClicked).toBe(false)
  })
})
