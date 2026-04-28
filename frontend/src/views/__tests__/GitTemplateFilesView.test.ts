import { describe, it, expect, vi } from 'vitest'
import { mount, flushPromises } from '@vue/test-utils'
import GitTemplateFilesView from '@/views/GitTemplateFilesView.vue'
import { setupViewTest, stubFetch } from '@/__tests__/testHelpers/viewHarness'

vi.mock('vue-router', () => ({
  useRouter: () => ({ push: vi.fn() }),
  RouterLink: { template: '<a><slot/></a>' },
}))

vi.mock('vue-sonner', () => ({
  toast: { success: vi.fn(), error: vi.fn() },
}))

const STRUCTURE = {
  data: {
    uuid: 'tpl-1',
    name: 'Memory Bank',
    file_tree: ['README.md', 'src/index.ts'],
    essential_files: ['README.md'],
    variables: [],
    files: [
      {
        path: 'README.md',
        filename: 'README.md',
        size_bytes: 42,
        extension: 'md',
        content_type: 'text/markdown',
      },
      {
        path: 'src/index.ts',
        filename: 'index.ts',
        size_bytes: 100,
        extension: 'ts',
        content_type: 'text/plain',
      },
    ],
    file_count: 2,
    total_size: 142,
  },
}

function fileResponse(path: string, filename: string, content: string) {
  return {
    data: {
      path,
      filename,
      size_bytes: content.length,
      is_essential: false,
      content,
    },
  }
}

async function mountView() {
  const wrapper = mount(GitTemplateFilesView, {
    props: { uuid: 'tpl-1' },
    global: {
      stubs: {
        ArrowLeftIcon: true,
      },
    },
  })
  await flushPromises()
  return wrapper
}

describe('GitTemplateFilesView', () => {
  setupViewTest()

  it('renders the file tree on mount and hides the content area until a file is selected', async () => {
    stubFetch(STRUCTURE)
    const wrapper = await mountView()

    expect(wrapper.text()).toContain('README.md')
    expect(wrapper.text()).toContain('src')
    expect(wrapper.text()).toContain('Select a file to view its contents.')
    expect(wrapper.find('[aria-label="Back to files"]').exists()).toBe(false)
  })

  it('shows the mobile-only "Back to files" button after a file is selected', async () => {
    stubFetch(STRUCTURE)
    const wrapper = await mountView()

    stubFetch(fileResponse('README.md', 'README.md', '# Hello'))
    const readmeButton = wrapper.findAll('button').find((b) => b.text().includes('README.md'))
    expect(readmeButton).toBeDefined()
    await readmeButton!.trigger('click')
    await flushPromises()

    expect(wrapper.find('[aria-label="Back to files"]').exists()).toBe(true)
  })

  it('renders markdown content via the prose typography wrapper for .md files', async () => {
    stubFetch(STRUCTURE)
    const wrapper = await mountView()

    stubFetch(fileResponse('README.md', 'README.md', '# Title\n\nBody paragraph.'))
    const readmeButton = wrapper.findAll('button').find((b) => b.text().includes('README.md'))
    await readmeButton!.trigger('click')
    await flushPromises()

    const prose = wrapper.find('.prose')
    expect(prose.exists()).toBe(true)
    expect(prose.find('h1').text()).toBe('Title')
    expect(prose.find('p').text()).toBe('Body paragraph.')
    expect(wrapper.find('pre').exists()).toBe(false)
  })

  it('renders code files as raw text in a <pre> block (no markdown rendering)', async () => {
    stubFetch(STRUCTURE)
    const wrapper = await mountView()

    stubFetch(fileResponse('src/index.ts', 'index.ts', 'export const x = 1'))
    const tsButton = wrapper.findAll('button').find((b) => b.text().includes('index.ts'))
    expect(tsButton).toBeDefined()
    await tsButton!.trigger('click')
    await flushPromises()

    expect(wrapper.find('.prose').exists()).toBe(false)
    const pre = wrapper.find('pre')
    expect(pre.exists()).toBe(true)
    expect(pre.text()).toBe('export const x = 1')
  })

  it('clears the selection when "Back to files" is clicked', async () => {
    stubFetch(STRUCTURE)
    const wrapper = await mountView()

    stubFetch(fileResponse('README.md', 'README.md', '# Hello'))
    const readmeButton = wrapper.findAll('button').find((b) => b.text().includes('README.md'))
    await readmeButton!.trigger('click')
    await flushPromises()

    await wrapper.get('[aria-label="Back to files"]').trigger('click')

    expect(wrapper.find('[aria-label="Back to files"]').exists()).toBe(false)
    expect(wrapper.text()).toContain('Select a file to view its contents.')
  })
})
