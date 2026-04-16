import { describe, it, expect } from 'vitest'
import { mount } from '@vue/test-utils'
import FileSizeCell from '@/components/shared/FileSizeCell.vue'

function mountCell(bytes: number) {
  return mount(FileSizeCell, { props: { bytes } })
}

describe('FileSizeCell', () => {
  it('renders bytes under 1 KiB in B', () => {
    expect(mountCell(0).text()).toBe('0 B')
    expect(mountCell(512).text()).toBe('512 B')
    expect(mountCell(1023).text()).toBe('1023 B')
  })

  it('renders values 1 KiB to 1 MiB in KB with one decimal', () => {
    expect(mountCell(1024).text()).toBe('1.0 KB')
    expect(mountCell(1536).text()).toBe('1.5 KB')
    expect(mountCell(1024 * 1024 - 1).text()).toBe('1024.0 KB')
  })

  it('renders values at or above 1 MiB in MB with one decimal', () => {
    expect(mountCell(1024 * 1024).text()).toBe('1.0 MB')
    expect(mountCell(1024 * 1024 * 5).text()).toBe('5.0 MB')
  })

  it('reacts to prop changes', async () => {
    const wrapper = mountCell(100)
    expect(wrapper.text()).toBe('100 B')
    await wrapper.setProps({ bytes: 2048 })
    expect(wrapper.text()).toBe('2.0 KB')
  })
})
