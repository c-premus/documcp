import { describe, it, expect, vi, beforeEach } from 'vitest'
import { mount, flushPromises } from '@vue/test-utils'
import ContentViewer from '@/components/documents/ContentViewer.vue'

const mermaidInitialize = vi.fn()
const mermaidRender = vi.fn()

vi.mock('mermaid', () => ({
  default: {
    initialize: (config: Record<string, unknown>) => mermaidInitialize(config),
    render: (id: string, source: string) => mermaidRender(id, source),
  },
}))

beforeEach(() => {
  mermaidInitialize.mockReset()
  mermaidRender.mockReset()
})

function mountViewer(props: { content: string; fileType: string }) {
  return mount(ContentViewer, { props, attachTo: document.body })
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

  describe('mermaid rendering', () => {
    it('does not load mermaid when no diagram blocks are present', async () => {
      mountViewer({ content: '# Just a heading', fileType: 'markdown' })
      await flushPromises()

      expect(mermaidRender).not.toHaveBeenCalled()
    })

    it('replaces a fenced mermaid block with the rendered SVG', async () => {
      mermaidRender.mockResolvedValue({ svg: '<svg data-testid="diag"><g/></svg>' })

      const wrapper = mountViewer({
        content: '```mermaid\ngraph TD\n  A --> B\n```\n',
        fileType: 'markdown',
      })
      await flushPromises()

      expect(mermaidRender).toHaveBeenCalledTimes(1)
      const [, source] = mermaidRender.mock.calls[0]!
      expect(source).toContain('graph TD')
      expect(source).toContain('A --> B')

      const diagram = wrapper.find('.mermaid-diagram')
      expect(diagram.exists()).toBe(true)
      expect(diagram.find('[data-testid="diag"]').exists()).toBe(true)
      expect(wrapper.find('code.language-mermaid').exists()).toBe(false)
    })

    it('leaves the code block intact when mermaid render fails', async () => {
      mermaidRender.mockRejectedValue(new Error('parse error'))
      const consoleSpy = vi.spyOn(console, 'warn').mockImplementation(() => {})

      const wrapper = mountViewer({
        content: '```mermaid\ninvalid syntax\n```\n',
        fileType: 'markdown',
      })
      await flushPromises()

      expect(mermaidRender).toHaveBeenCalledTimes(1)
      expect(wrapper.find('.mermaid-diagram').exists()).toBe(false)
      expect(wrapper.find('code.language-mermaid').exists()).toBe(true)

      consoleSpy.mockRestore()
    })

    it('does not re-render an already-processed mermaid block on content updates', async () => {
      mermaidRender.mockResolvedValue({ svg: '<svg/>' })

      const wrapper = mountViewer({
        content: '```mermaid\ngraph TD\n  A --> B\n```\n',
        fileType: 'markdown',
      })
      await flushPromises()
      expect(mermaidRender).toHaveBeenCalledTimes(1)

      // Force a re-render by changing fileType (renderedContent recomputes).
      await wrapper.setProps({ fileType: 'md' })
      await flushPromises()

      // Still 1 — the data-mermaid-rendered marker prevents the re-walk
      // from picking up the same source twice. The content is the same
      // diagram so a second render call would be wasted work.
      expect(mermaidRender).toHaveBeenCalledTimes(1)
    })
  })
})
