import { describe, it, expect } from 'vitest'
import { mount } from '@vue/test-utils'
import FileTypeCell from '@/components/shared/FileTypeCell.vue'

describe('FileTypeCell', () => {
  it('renders the value uppercased', () => {
    expect(mount(FileTypeCell, { props: { value: 'pdf' } }).text()).toBe('PDF')
    expect(mount(FileTypeCell, { props: { value: 'markdown' } }).text()).toBe('MARKDOWN')
  })

  it('leaves already-uppercase values unchanged', () => {
    expect(mount(FileTypeCell, { props: { value: 'HTML' } }).text()).toBe('HTML')
  })

  it('renders empty string when given an empty value', () => {
    expect(mount(FileTypeCell, { props: { value: '' } }).text()).toBe('')
  })
})
