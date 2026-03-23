import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'
import { createPinia, setActivePinia } from 'pinia'
import { useGitTemplatesStore } from '@/stores/gitTemplates'
import type { GitTemplate } from '@/stores/gitTemplates'

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
    it('calls correct URL and sets templates/total', async () => {
      const templates = [mockTemplate(), mockTemplate({ uuid: 'tpl-2', name: 'Second' })]
      stubFetch({ data: templates, meta: { total: 2 } })

      const store = useGitTemplatesStore()
      await store.fetchTemplates()

      expect(fetch).toHaveBeenCalledWith('/api/git-templates', undefined)
      expect(store.templates).toEqual(templates)
      expect(store.total).toBe(2)
    })

    it('passes category as query param', async () => {
      stubFetch({ data: [], meta: { total: 0 } })

      const store = useGitTemplatesStore()
      await store.fetchTemplates({ category: 'documentation' })

      const calledUrl = (fetch as ReturnType<typeof vi.fn>).mock.calls[0]![0] as string
      expect(calledUrl).toContain('/api/git-templates?')
      expect(calledUrl).toContain('category=documentation')
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

  describe('fetchStructure', () => {
    it('calls correct URL and returns structure data', async () => {
      const structureData = {
        uuid: 'tpl-1',
        name: 'Test Template',
        file_tree: ['README.md', 'src/index.ts'],
        essential_files: ['README.md'],
        variables: ['PROJECT_NAME'],
        files: [
          { path: 'README.md', filename: 'README.md', size_bytes: 512, is_essential: true },
        ],
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
        json: () => Promise.resolve({ data: { uuid: 'tpl-1', name: 'T', file_tree: [], essential_files: [], variables: [], files: [], file_count: 0, total_size: 0 } }),
      })
      await promise

      expect(store.loading).toBe(false)
    })

    it('sets error on failure', async () => {
      stubFetch({ message: 'Template not found' }, false)

      const store = useGitTemplatesStore()
      await expect(store.fetchStructure('bad-id')).rejects.toThrow('Template not found')

      expect(store.error).toBe('Template not found')
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

      expect(fetch).toHaveBeenCalledWith(
        '/api/git-templates/tpl-1/files/src/index.ts',
        undefined,
      )
      expect(result).toEqual(fileData)
    })

    it('sets error on failure', async () => {
      stubFetch({ message: 'File not found' }, false)

      const store = useGitTemplatesStore()
      await expect(store.readFile('tpl-1', 'missing.txt')).rejects.toThrow('File not found')

      expect(store.error).toBe('File not found')
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
