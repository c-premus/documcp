import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'
import { createPinia, setActivePinia } from 'pinia'
import { useZimArchivesStore } from '@/stores/zimArchives'
import type { ZimArchive, ZimSearchResult, ZimArticle } from '@/stores/zimArchives'

function mockArchive(overrides: Partial<ZimArchive> = {}): ZimArchive {
  return {
    uuid: 'zim-1',
    name: 'wikipedia_en',
    title: 'Wikipedia English',
    description: 'Full English Wikipedia',
    language: 'en',
    category: 'wikipedia',
    creator: 'Kiwix',
    publisher: 'Kiwix',
    article_count: 6000000,
    media_count: 100000,
    file_size: 90000000000,
    file_size_human: '83.8 GB',
    tags: ['wikipedia', 'english'],
    last_synced_at: '2026-01-01T00:00:00Z',
    ...overrides,
  }
}

function mockSearchResult(overrides: Partial<ZimSearchResult> = {}): ZimSearchResult {
  return {
    title: 'Test Article',
    path: 'A/Test_Article',
    snippet: 'This is a <b>test</b> article',
    score: 0.95,
    ...overrides,
  }
}

function mockArticle(overrides: Partial<ZimArticle> = {}): ZimArticle {
  return {
    archive_name: 'wikipedia_en',
    path: 'A/Test_Article',
    title: 'Test Article',
    content: '<html><body><p>Hello world</p></body></html>',
    mime_type: 'text/html',
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

describe('zimArchives store', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
    vi.stubGlobal('fetch', vi.fn())
  })

  afterEach(() => {
    vi.restoreAllMocks()
    vi.unstubAllGlobals()
  })

  it('has correct initial state', () => {
    const store = useZimArchivesStore()

    expect(store.archives).toEqual([])
    expect(store.total).toBe(0)
    expect(store.currentArticle).toBeNull()
    expect(store.searchResults).toEqual([])
    expect(store.loading).toBe(false)
    expect(store.searchLoading).toBe(false)
    expect(store.articleLoading).toBe(false)
    expect(store.error).toBeNull()
  })

  describe('fetchArchives', () => {
    it('calls correct URL with query params and sets archives/total', async () => {
      const archives = [mockArchive(), mockArchive({ uuid: 'zim-2', name: 'wikipedia_fr' })]
      stubFetch({ data: archives, meta: { total: 2 } })

      const store = useZimArchivesStore()
      await store.fetchArchives({ per_page: 10, offset: 0, category: 'wikipedia', language: 'en' })

      const calledUrl = (fetch as ReturnType<typeof vi.fn>).mock.calls[0]![0] as string
      expect(calledUrl).toContain('/api/zim/archives?')
      expect(calledUrl).toContain('per_page=10')
      expect(calledUrl).toContain('offset=0')
      expect(calledUrl).toContain('category=wikipedia')
      expect(calledUrl).toContain('language=en')
      expect(store.archives).toEqual(archives)
      expect(store.total).toBe(2)
    })

    it('passes query param for text search', async () => {
      stubFetch({ data: [], meta: { total: 0 } })

      const store = useZimArchivesStore()
      await store.fetchArchives({ query: 'wiki' })

      const calledUrl = (fetch as ReturnType<typeof vi.fn>).mock.calls[0]![0] as string
      expect(calledUrl).toContain('query=wiki')
    })

    it('calls URL without query when no params', async () => {
      stubFetch({ data: [], meta: { total: 0 } })

      const store = useZimArchivesStore()
      await store.fetchArchives()

      expect(fetch).toHaveBeenCalledWith('/api/zim/archives', undefined)
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

      const store = useZimArchivesStore()
      const promise = store.fetchArchives()

      expect(store.loading).toBe(true)

      resolvePromise!({
        ok: true,
        json: () => Promise.resolve({ data: [], meta: { total: 0 } }),
      })
      await promise

      expect(store.loading).toBe(false)
    })

    it('sets error on failure', async () => {
      stubFetch({ message: 'Unauthorized' }, false)

      const store = useZimArchivesStore()
      await expect(store.fetchArchives()).rejects.toThrow('Unauthorized')

      expect(store.error).toBe('Unauthorized')
      expect(store.loading).toBe(false)
    })
  })

  describe('searchArticles', () => {
    it('calls correct URL with encoded archive name and query params', async () => {
      const results = [mockSearchResult(), mockSearchResult({ title: 'Second', path: 'A/Second' })]
      stubFetch({ data: results, meta: { archive: 'wikipedia_en', query: 'test', total: 2 } })

      const store = useZimArchivesStore()
      const returned = await store.searchArticles('wikipedia_en', 'test', 10)

      const calledUrl = (fetch as ReturnType<typeof vi.fn>).mock.calls[0]![0] as string
      expect(calledUrl).toContain('/api/zim/archives/wikipedia_en/search?')
      expect(calledUrl).toContain('q=test')
      expect(calledUrl).toContain('limit=10')
      expect(returned).toEqual(results)
      expect(store.searchResults).toEqual(results)
    })

    it('encodes special characters in archive name', async () => {
      stubFetch({ data: [], meta: { archive: 'my archive', query: 'q', total: 0 } })

      const store = useZimArchivesStore()
      await store.searchArticles('my archive', 'q')

      const calledUrl = (fetch as ReturnType<typeof vi.fn>).mock.calls[0]![0] as string
      expect(calledUrl).toContain('/api/zim/archives/my%20archive/search')
    })

    it('calls without limit when not provided', async () => {
      stubFetch({ data: [], meta: { archive: 'wiki', query: 'test', total: 0 } })

      const store = useZimArchivesStore()
      await store.searchArticles('wiki', 'test')

      const calledUrl = (fetch as ReturnType<typeof vi.fn>).mock.calls[0]![0] as string
      expect(calledUrl).toContain('q=test')
      expect(calledUrl).not.toContain('limit=')
    })

    it('sets searchLoading true during search', async () => {
      let resolvePromise: (value: unknown) => void
      vi.stubGlobal(
        'fetch',
        vi.fn().mockReturnValue(
          new Promise((resolve) => {
            resolvePromise = resolve
          }),
        ),
      )

      const store = useZimArchivesStore()
      const promise = store.searchArticles('wiki', 'test')

      expect(store.searchLoading).toBe(true)
      expect(store.loading).toBe(false)

      resolvePromise!({
        ok: true,
        json: () =>
          Promise.resolve({ data: [], meta: { archive: 'wiki', query: 'test', total: 0 } }),
      })
      await promise

      expect(store.searchLoading).toBe(false)
    })

    it('sets error on failure', async () => {
      stubFetch({ message: 'Search failed' }, false)

      const store = useZimArchivesStore()
      await expect(store.searchArticles('wiki', 'test')).rejects.toThrow('Search failed')

      expect(store.error).toBe('Search failed')
      expect(store.searchLoading).toBe(false)
    })
  })

  describe('readArticle', () => {
    it('calls correct URL with encoded params and sets currentArticle', async () => {
      const article = mockArticle()
      stubFetch({ data: article })

      const store = useZimArchivesStore()
      const result = await store.readArticle('wikipedia_en', 'A/Test_Article')

      const calledUrl = (fetch as ReturnType<typeof vi.fn>).mock.calls[0]![0] as string
      expect(calledUrl).toContain('/api/zim/archives/wikipedia_en/articles/A%2FTest_Article')
      expect(result).toEqual(article)
      expect(store.currentArticle).toEqual(article)
    })

    it('encodes special characters in archive and path', async () => {
      const article = mockArticle({ archive_name: 'my archive', path: 'path with spaces' })
      stubFetch({ data: article })

      const store = useZimArchivesStore()
      await store.readArticle('my archive', 'path with spaces')

      const calledUrl = (fetch as ReturnType<typeof vi.fn>).mock.calls[0]![0] as string
      expect(calledUrl).toContain('/api/zim/archives/my%20archive/articles/path%20with%20spaces')
    })

    it('sets articleLoading true during read', async () => {
      let resolvePromise: (value: unknown) => void
      vi.stubGlobal(
        'fetch',
        vi.fn().mockReturnValue(
          new Promise((resolve) => {
            resolvePromise = resolve
          }),
        ),
      )

      const store = useZimArchivesStore()
      const promise = store.readArticle('wiki', 'A/Article')

      expect(store.articleLoading).toBe(true)
      expect(store.loading).toBe(false)
      expect(store.searchLoading).toBe(false)

      resolvePromise!({
        ok: true,
        json: () => Promise.resolve({ data: mockArticle() }),
      })
      await promise

      expect(store.articleLoading).toBe(false)
    })

    it('sets error on failure', async () => {
      stubFetch({ message: 'Article not found' }, false)

      const store = useZimArchivesStore()
      await expect(store.readArticle('wiki', 'A/Missing')).rejects.toThrow('Article not found')

      expect(store.error).toBe('Article not found')
      expect(store.articleLoading).toBe(false)
    })
  })

  describe('clearSearch', () => {
    it('resets searchResults to empty array', () => {
      const store = useZimArchivesStore()
      store.$patch({ searchResults: [mockSearchResult()] })

      store.clearSearch()

      expect(store.searchResults).toEqual([])
    })
  })

  describe('clearArticle', () => {
    it('resets currentArticle to null', () => {
      const store = useZimArchivesStore()
      store.$patch({ currentArticle: mockArticle() })

      store.clearArticle()

      expect(store.currentArticle).toBeNull()
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

      const store = useZimArchivesStore()
      await expect(store.fetchArchives()).rejects.toThrow('Internal Server Error')

      expect(store.error).toBe('Internal Server Error')
    })

    it('clears error before each request', async () => {
      const store = useZimArchivesStore()
      store.$patch({ error: 'previous error' })

      stubFetch({ data: [], meta: { total: 0 } })
      await store.fetchArchives()

      expect(store.error).toBeNull()
    })

    it('clears error before searchArticles', async () => {
      const store = useZimArchivesStore()
      store.$patch({ error: 'previous error' })

      stubFetch({ data: [], meta: { archive: 'wiki', query: 'q', total: 0 } })
      await store.searchArticles('wiki', 'q')

      expect(store.error).toBeNull()
    })

    it('clears error before readArticle', async () => {
      const store = useZimArchivesStore()
      store.$patch({ error: 'previous error' })

      stubFetch({ data: mockArticle() })
      await store.readArticle('wiki', 'A/Article')

      expect(store.error).toBeNull()
    })
  })
})
