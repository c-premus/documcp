import { describe, it, expect } from 'vitest'
import { mount } from '@vue/test-utils'
import ContentViewer from '@/components/documents/ContentViewer.vue'

function mountViewer(props: { content: string; fileType: string }) {
  return mount(ContentViewer, { props })
}

describe('ContentViewer', () => {
  describe('markdown rendering', () => {
    it('renders markdown as HTML for "markdown" file type', () => {
      const wrapper = mountViewer({ content: '# Hello World', fileType: 'markdown' })

      const prose = wrapper.find('.prose')
      expect(prose.exists()).toBe(true)
      expect(prose.html()).toContain('<h1')
      expect(prose.text()).toContain('Hello World')
    })

    it('renders markdown as HTML for "md" file type', () => {
      const wrapper = mountViewer({ content: '**bold text**', fileType: 'md' })

      const prose = wrapper.find('.prose')
      expect(prose.exists()).toBe(true)
      expect(prose.html()).toContain('<strong>')
      expect(prose.text()).toContain('bold text')
    })

    it('renders markdown paragraphs', () => {
      const wrapper = mountViewer({ content: 'A paragraph of text.', fileType: 'markdown' })

      const prose = wrapper.find('.prose')
      expect(prose.html()).toContain('<p>')
    })
  })

  describe('HTML rendering', () => {
    it('renders sanitized HTML for "html" file type', () => {
      const wrapper = mountViewer({ content: '<p>Hello</p>', fileType: 'html' })

      const prose = wrapper.find('.prose')
      expect(prose.exists()).toBe(true)
      expect(prose.html()).toContain('<p>Hello</p>')
    })

    it('renders sanitized HTML for "htm" file type', () => {
      const wrapper = mountViewer({ content: '<em>emphasized</em>', fileType: 'htm' })

      const prose = wrapper.find('.prose')
      expect(prose.exists()).toBe(true)
      expect(prose.html()).toContain('<em>')
    })
  })

  describe('sanitization', () => {
    it('strips script tags from markdown content', () => {
      const wrapper = mountViewer({
        content: '<script>alert("xss")</script><p>safe</p>',
        fileType: 'markdown',
      })

      expect(wrapper.html()).not.toContain('<script>')
      expect(wrapper.html()).not.toContain('alert')
    })

    it('strips script tags from HTML content', () => {
      const wrapper = mountViewer({
        content: '<p>safe</p><script>alert("xss")</script>',
        fileType: 'html',
      })

      expect(wrapper.html()).not.toContain('<script>')
      expect(wrapper.text()).toContain('safe')
    })
  })

  describe('plain text rendering', () => {
    it('renders plain text in pre element for txt file type', () => {
      const wrapper = mountViewer({ content: 'plain text content', fileType: 'txt' })

      const pre = wrapper.find('pre')
      expect(pre.exists()).toBe(true)
      expect(pre.text()).toBe('plain text content')
      expect(wrapper.find('.prose').exists()).toBe(false)
    })

    it('renders plain text for unknown file types', () => {
      const wrapper = mountViewer({ content: 'some content', fileType: 'pdf' })

      const pre = wrapper.find('pre')
      expect(pre.exists()).toBe(true)
      expect(pre.text()).toBe('some content')
    })

    it('is case-insensitive for file type matching', () => {
      const wrapper = mountViewer({ content: '# Title', fileType: 'MARKDOWN' })

      const prose = wrapper.find('.prose')
      expect(prose.exists()).toBe(true)
      expect(prose.html()).toContain('<h1')
    })
  })

  describe('empty content', () => {
    it('handles empty string content for markdown gracefully', () => {
      const wrapper = mountViewer({ content: '', fileType: 'markdown' })

      // Should render without error - empty markdown produces empty prose or falls to pre
      expect(wrapper.exists()).toBe(true)
    })

    it('handles empty string content for plain text gracefully', () => {
      const wrapper = mountViewer({ content: '', fileType: 'txt' })

      const pre = wrapper.find('pre')
      expect(pre.exists()).toBe(true)
      expect(pre.text()).toBe('')
    })
  })
})
