import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'
import { createPinia, setActivePinia } from 'pinia'
import { useGitTemplatesStore, buildTree } from '@/stores/gitTemplates'
import type { GitTemplate, GitTemplateFile } from '@/stores/gitTemplates'

function mockTemplate(overrides: Partial<GitTemplate> = {}): GitTemplate {
  return {
    uuid: 'tpl-1',
    name: 'Test Template',
    slug: 'test-template',
    description: 'A test template',
    repository_url: 'https://github.com/example/repo',
    branch: 'main',
    category: 'general',
    tags: ['test'],
    is_public: true,
    status: 'ready',
    file_count: 5,
    total_size_bytes: 2048,
    created_at: '2026-01-01T00:00:00Z',
    updated_at: '2026-01-01T00:00:00Z',
    ...overrides,
  }
}

function stubFetch(response: unknown, ok = true) {
  vi.stubGlobal(
    'fetch',
    vi.fn().mockResolvedValue({
      ok,
      status: ok ? 200 : 500,
      statusText: 'Internal Server Error',
      json: () => Promise.resolve(response),
    }),
  )
}

describe('gitTemplates store', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
    vi.stubGlobal('fetch', vi.fn())
  })

  afterEach(() => {
    vi.restoreAllMocks()
    vi.unstubAllGlobals()
  })

  it('has correct initial state', () => {
    const store = useGitTemplatesStore()

    expect(store.templates).toEqual([])
    expect(store.total).toBe(0)
    expect(store.loading).toBe(false)
    expect(store.error).toBeNull()
  })

  describe('fetchTemplates', () => {
    it('calls correct URL with query params and sets templates/total', async () => {
      const templates = [mockTemplate(), mockTemplate({ uuid: 'tpl-2', name: 'Second' })]
      stubFetch({ data: templates, meta: { total: 2, limit: 10, offset: 0 } })

      const store = useGitTemplatesStore()
      await store.fetchTemplates({ per_page: 10, offset: 0, category: 'docs' })

      const calledUrl = (fetch as ReturnType<typeof vi.fn>).mock.calls[0]![0] as string
      expect(calledUrl).toContain('/api/git-templates?')
      expect(calledUrl).toContain('per_page=10')
      expect(calledUrl).toContain('offset=0')
      expect(calledUrl).toContain('category=docs')
      expect(store.templates).toEqual(templates)
      expect(store.total).toBe(2)
    })

    it('calls URL without query when no params', async () => {
      stubFetch({ data: [], meta: { total: 0, limit: 10, offset: 0 } })

      const store = useGitTemplatesStore()
      await store.fetchTemplates()

      expect(fetch).toHaveBeenCalledWith('/api/git-templates', undefined)
    })

    it('sets loading true during fetch', async () => {
      let resolvePromise: (value: unknown) => void
      vi.stubGlobal(
        'fetch',
        vi.fn().mockReturnValue(
          new Promise((resolve) => {
            resolvePromise = resolve
          }),
        ),
      )

      const store = useGitTemplatesStore()
      const promise = store.fetchTemplates()

      expect(store.loading).toBe(true)

      resolvePromise!({
        ok: true,
        json: () => Promise.resolve({ data: [], meta: { total: 0 } }),
      })
      await promise

      expect(store.loading).toBe(false)
    })

    it('sets error on failure', async () => {
      stubFetch({ message: 'Forbidden' }, false)

      const store = useGitTemplatesStore()
      await expect(store.fetchTemplates()).rejects.toThrow('Forbidden')

      expect(store.error).toBe('Forbidden')
      expect(store.loading).toBe(false)
    })
  })

  describe('createTemplate', () => {
    it('POSTs payload and returns created template', async () => {
      const template = mockTemplate()
      stubFetch({ data: template, message: 'Template created' })

      const store = useGitTemplatesStore()
      const params = {
        name: 'New Template',
        repository_url: 'https://github.com/example/repo',
        branch: 'main',
        category: 'general',
        description: 'A new template',
        tags: ['test'],
        is_public: true,
      }
      const result = await store.createTemplate(params)

      expect(fetch).toHaveBeenCalledWith('/api/git-templates', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify(params),
      })
      expect(result).toEqual(template)
    })

    it('sets loading true during create', async () => {
      let resolvePromise: (value: unknown) => void
      vi.stubGlobal(
        'fetch',
        vi.fn().mockReturnValue(
          new Promise((resolve) => {
            resolvePromise = resolve
          }),
        ),
      )

      const store = useGitTemplatesStore()
      const promise = store.createTemplate({
        name: 'T',
        repository_url: 'https://github.com/x/y',
        branch: 'main',
        category: 'general',
      })

      expect(store.loading).toBe(true)

      resolvePromise!({
        ok: true,
        json: () => Promise.resolve({ data: mockTemplate() }),
      })
      await promise

      expect(store.loading).toBe(false)
    })

    it('sets error on failure', async () => {
      stubFetch({ message: 'Validation failed' }, false)

      const store = useGitTemplatesStore()
      await expect(
        store.createTemplate({
          name: '',
          repository_url: '',
          branch: '',
          category: '',
        }),
      ).rejects.toThrow('Validation failed')

      expect(store.error).toBe('Validation failed')
      expect(store.loading).toBe(false)
    })
  })

  describe('deleteTemplate', () => {
    it('DELETEs and removes from local array', async () => {
      stubFetch({ message: 'Deleted' })

      const store = useGitTemplatesStore()
      store.$patch({
        templates: [mockTemplate({ uuid: 'tpl-1' }), mockTemplate({ uuid: 'tpl-2' })],
      })

      const result = await store.deleteTemplate('tpl-1')

      expect(fetch).toHaveBeenCalledWith('/api/git-templates/tpl-1', { method: 'DELETE' })
      expect(result).toEqual({ message: 'Deleted' })
      expect(store.templates).toHaveLength(1)
      expect(store.templates[0]!.uuid).toBe('tpl-2')
    })

    it('does not modify array when uuid not found', async () => {
      stubFetch({ message: 'Deleted' })

      const store = useGitTemplatesStore()
      store.$patch({ templates: [mockTemplate({ uuid: 'tpl-1' })] })

      await store.deleteTemplate('tpl-999')

      expect(store.templates).toHaveLength(1)
      expect(store.templates[0]!.uuid).toBe('tpl-1')
    })

    it('sets loading true during delete', async () => {
      let resolvePromise: (value: unknown) => void
      vi.stubGlobal(
        'fetch',
        vi.fn().mockReturnValue(
          new Promise((resolve) => {
            resolvePromise = resolve
          }),
        ),
      )

      const store = useGitTemplatesStore()
      const promise = store.deleteTemplate('tpl-1')

      expect(store.loading).toBe(true)

      resolvePromise!({
        ok: true,
        json: () => Promise.resolve({ message: 'Deleted' }),
      })
      await promise

      expect(store.loading).toBe(false)
    })

    it('sets error on failure', async () => {
      stubFetch({ message: 'Cannot delete' }, false)

      const store = useGitTemplatesStore()
      await expect(store.deleteTemplate('tpl-1')).rejects.toThrow('Cannot delete')

      expect(store.error).toBe('Cannot delete')
      expect(store.loading).toBe(false)
    })
  })

  describe('syncTemplate', () => {
    it('POSTs to sync endpoint and returns message', async () => {
      stubFetch({ message: 'Sync started' })

      const store = useGitTemplatesStore()
      const result = await store.syncTemplate('tpl-1')

      expect(fetch).toHaveBeenCalledWith('/api/git-templates/tpl-1/sync', { method: 'POST' })
      expect(result).toEqual({ message: 'Sync started' })
    })

    it('does not set loading during sync', async () => {
      stubFetch({ message: 'Sync started' })

      const store = useGitTemplatesStore()
      await store.syncTemplate('tpl-1')

      expect(store.loading).toBe(false)
    })

    it('sets error on failure', async () => {
      stubFetch({ message: 'Sync failed' }, false)

      const store = useGitTemplatesStore()
      await expect(store.syncTemplate('tpl-1')).rejects.toThrow('Sync failed')

      expect(store.error).toBe('Sync failed')
    })
  })

  describe('fetchStructure', () => {
    it('calls correct URL and returns structure data', async () => {
      const structureData = {
        uuid: 'tpl-1',
        name: 'Test Template',
        file_tree: ['README.md', 'src/index.ts'],
        essential_files: ['README.md'],
        variables: ['PROJECT_NAME'],
        files: [{ path: 'README.md', filename: 'README.md', size_bytes: 512, is_essential: true }],
        file_count: 2,
        total_size: 1024,
      }
      stubFetch({ data: structureData })

      const store = useGitTemplatesStore()
      const result = await store.fetchStructure('tpl-1')

      expect(fetch).toHaveBeenCalledWith('/api/git-templates/tpl-1/structure', undefined)
      expect(result).toEqual(structureData)
    })

    it('sets loading true during fetch', async () => {
      let resolvePromise: (value: unknown) => void
      vi.stubGlobal(
        'fetch',
        vi.fn().mockReturnValue(
          new Promise((resolve) => {
            resolvePromise = resolve
          }),
        ),
      )

      const store = useGitTemplatesStore()
      const promise = store.fetchStructure('tpl-1')

      expect(store.loading).toBe(true)

      resolvePromise!({
        ok: true,
        json: () =>
          Promise.resolve({
            data: {
              uuid: 'tpl-1',
              name: 'T',
              file_tree: [],
              essential_files: [],
              variables: [],
              files: [],
              file_count: 0,
              total_size: 0,
            },
          }),
      })
      await promise

      expect(store.loading).toBe(false)
    })

    it('sets error on failure', async () => {
      stubFetch({ message: 'Template not found' }, false)

      const store = useGitTemplatesStore()
      await expect(store.fetchStructure('bad-id')).rejects.toThrow('Template not found')

      expect(store.error).toBe('Template not found')
      expect(store.loading).toBe(false)
    })
  })

  describe('readFile', () => {
    it('calls correct URL with encoded path and returns file content', async () => {
      const fileData = {
        path: 'src/index.ts',
        filename: 'index.ts',
        size_bytes: 256,
        is_essential: false,
        content: 'console.log("hello")',
      }
      stubFetch({ data: fileData })

      const store = useGitTemplatesStore()
      const result = await store.readFile('tpl-1', 'src/index.ts')

      expect(fetch).toHaveBeenCalledWith('/api/git-templates/tpl-1/files/src/index.ts', undefined)
      expect(result).toEqual(fileData)
    })

    it('encodes special characters in path segments', async () => {
      const fileData = {
        path: 'docs/my file.md',
        filename: 'my file.md',
        size_bytes: 128,
        is_essential: false,
        content: '# Hello',
      }
      stubFetch({ data: fileData })

      const store = useGitTemplatesStore()
      await store.readFile('tpl-1', 'docs/my file.md')

      const calledUrl = (fetch as ReturnType<typeof vi.fn>).mock.calls[0]![0] as string
      expect(calledUrl).toBe('/api/git-templates/tpl-1/files/docs/my%20file.md')
    })

    it('sets error on failure', async () => {
      stubFetch({ message: 'File not found' }, false)

      const store = useGitTemplatesStore()
      await expect(store.readFile('tpl-1', 'missing.txt')).rejects.toThrow('File not found')

      expect(store.error).toBe('File not found')
    })
  })

  describe('validateUrl', () => {
    it('POSTs url to validate-url endpoint and returns result', async () => {
      stubFetch({ valid: true })

      const store = useGitTemplatesStore()
      const result = await store.validateUrl('https://github.com/example/repo')

      expect(fetch).toHaveBeenCalledWith('/api/admin/git-templates/validate-url', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ url: 'https://github.com/example/repo' }),
      })
      expect(result).toEqual({ valid: true })
    })

    it('returns invalid result with error message', async () => {
      stubFetch({ valid: false, error: 'Repository not found' })

      const store = useGitTemplatesStore()
      const result = await store.validateUrl('https://github.com/example/nonexistent')

      expect(result).toEqual({ valid: false, error: 'Repository not found' })
    })

    it('throws on network failure', async () => {
      stubFetch({ message: 'Server error' }, false)

      const store = useGitTemplatesStore()
      await expect(store.validateUrl('https://github.com/example/repo')).rejects.toThrow(
        'Server error',
      )
    })
  })

  describe('error handling', () => {
    it('uses statusText when response body has no message', async () => {
      vi.stubGlobal(
        'fetch',
        vi.fn().mockResolvedValue({
          ok: false,
          status: 500,
          statusText: 'Internal Server Error',
          json: () => Promise.reject(new Error('parse error')),
        }),
      )

      const store = useGitTemplatesStore()
      await expect(store.fetchTemplates()).rejects.toThrow('Internal Server Error')

      expect(store.error).toBe('Internal Server Error')
    })

    it('clears error before each request', async () => {
      const store = useGitTemplatesStore()
      store.$patch({ error: 'previous error' })

      stubFetch({ data: [], meta: { total: 0 } })
      await store.fetchTemplates()

      expect(store.error).toBeNull()
    })
  })
})

describe('buildTree', () => {
  it('returns empty array for empty input', () => {
    expect(buildTree([])).toEqual([])
  })

  it('returns single file node for single file', () => {
    const files: GitTemplateFile[] = [
      { path: 'README.md', filename: 'README.md', size_bytes: 100, is_essential: true },
    ]

    const result = buildTree(files)

    expect(result).toEqual([{ name: 'README.md', path: 'README.md', type: 'file' }])
  })

  it('creates directory hierarchy for nested paths', () => {
    const files: GitTemplateFile[] = [
      { path: 'src/utils/helper.ts', filename: 'helper.ts', size_bytes: 200, is_essential: false },
    ]

    const result = buildTree(files)

    expect(result).toEqual([
      {
        name: 'src',
        path: 'src',
        type: 'directory',
        children: [
          {
            name: 'utils',
            path: 'src/utils',
            type: 'directory',
            children: [{ name: 'helper.ts', path: 'src/utils/helper.ts', type: 'file' }],
          },
        ],
      },
    ])
  })

  it('sorts directories before files', () => {
    const files: GitTemplateFile[] = [
      { path: 'zebra.txt', filename: 'zebra.txt', size_bytes: 10, is_essential: false },
      { path: 'src/index.ts', filename: 'index.ts', size_bytes: 50, is_essential: false },
      { path: 'alpha.txt', filename: 'alpha.txt', size_bytes: 10, is_essential: false },
    ]

    const result = buildTree(files)

    expect(result[0]!.type).toBe('directory')
    expect(result[0]!.name).toBe('src')
    expect(result[1]!.type).toBe('file')
    expect(result[2]!.type).toBe('file')
  })

  it('sorts files at same level alphabetically', () => {
    const files: GitTemplateFile[] = [
      { path: 'charlie.txt', filename: 'charlie.txt', size_bytes: 10, is_essential: false },
      { path: 'alpha.txt', filename: 'alpha.txt', size_bytes: 10, is_essential: false },
      { path: 'bravo.txt', filename: 'bravo.txt', size_bytes: 10, is_essential: false },
    ]

    const result = buildTree(files)

    expect(result.map((n) => n.name)).toEqual(['alpha.txt', 'bravo.txt', 'charlie.txt'])
  })

  it('groups multiple files under the same directory', () => {
    const files: GitTemplateFile[] = [
      { path: 'src/a.ts', filename: 'a.ts', size_bytes: 10, is_essential: false },
      { path: 'src/b.ts', filename: 'b.ts', size_bytes: 20, is_essential: false },
    ]

    const result = buildTree(files)

    expect(result).toHaveLength(1)
    expect(result[0]!.name).toBe('src')
    expect(result[0]!.children).toHaveLength(2)
    expect(result[0]!.children![0]!.name).toBe('a.ts')
    expect(result[0]!.children![1]!.name).toBe('b.ts')
  })

  it('handles mixed root files and nested directories', () => {
    const files: GitTemplateFile[] = [
      { path: 'README.md', filename: 'README.md', size_bytes: 100, is_essential: true },
      { path: 'src/index.ts', filename: 'index.ts', size_bytes: 50, is_essential: false },
      { path: 'docs/guide.md', filename: 'guide.md', size_bytes: 80, is_essential: false },
      { path: '.gitignore', filename: '.gitignore', size_bytes: 30, is_essential: false },
    ]

    const result = buildTree(files)

    // Directories first (docs, src), then files (.gitignore, README.md)
    expect(result[0]!.type).toBe('directory')
    expect(result[1]!.type).toBe('directory')
    expect(result[2]!.type).toBe('file')
    expect(result[3]!.type).toBe('file')
  })
})
