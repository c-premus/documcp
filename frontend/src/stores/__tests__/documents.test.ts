import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'
import { createPinia, setActivePinia } from 'pinia'
import { useDocumentsStore } from '@/stores/documents'
import type { Document } from '@/stores/documents'

function mockDocument(overrides: Partial<Document> = {}): Document {
  return {
    uuid: 'doc-1',
    title: 'Test Document',
    description: 'A test document',
    file_type: 'markdown',
    file_size: 1024,
    mime_type: 'text/markdown',
    word_count: 100,
    is_public: false,
    status: 'indexed',
    content_hash: 'abc123',
    tags: ['test'],
    created_at: '2026-01-01T00:00:00Z',
    updated_at: '2026-01-01T00:00:00Z',
    processed_at: '2026-01-01T00:00:00Z',
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

describe('documents store', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
    vi.stubGlobal('fetch', vi.fn())
  })

  afterEach(() => {
    vi.restoreAllMocks()
    vi.unstubAllGlobals()
  })

  it('has correct initial state', () => {
    const store = useDocumentsStore()

    expect(store.documents).toEqual([])
    expect(store.currentDocument).toBeNull()
    expect(store.total).toBe(0)
    expect(store.loading).toBe(false)
    expect(store.error).toBeNull()
  })

  describe('fetchDocuments', () => {
    it('calls correct URL and sets documents and total', async () => {
      const docs = [mockDocument(), mockDocument({ uuid: 'doc-2', title: 'Second' })]
      stubFetch({ data: docs, meta: { total: 2, limit: 10, offset: 0 } })

      const store = useDocumentsStore()
      await store.fetchDocuments({ limit: 10, offset: 0 })

      expect(fetch).toHaveBeenCalledWith('/api/documents?limit=10&offset=0', undefined)
      expect(store.documents).toEqual(docs)
      expect(store.total).toBe(2)
      expect(store.loading).toBe(false)
    })

    it('builds query string from params', async () => {
      stubFetch({ data: [], meta: { total: 0, limit: 25, offset: 0 } })

      const store = useDocumentsStore()
      await store.fetchDocuments({ limit: 25, q: 'search term', status: 'indexed' })

      const calledUrl = (fetch as ReturnType<typeof vi.fn>).mock.calls[0]![0] as string
      expect(calledUrl).toContain('limit=25')
      expect(calledUrl).toContain('q=search+term')
      expect(calledUrl).toContain('status=indexed')
    })

    it('calls URL without query when no params', async () => {
      stubFetch({ data: [], meta: { total: 0, limit: 10, offset: 0 } })

      const store = useDocumentsStore()
      await store.fetchDocuments()

      expect(fetch).toHaveBeenCalledWith('/api/documents', undefined)
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

      const store = useDocumentsStore()
      const promise = store.fetchDocuments()

      expect(store.loading).toBe(true)

      resolvePromise!({
        ok: true,
        json: () => Promise.resolve({ data: [], meta: { total: 0, limit: 10, offset: 0 } }),
      })
      await promise

      expect(store.loading).toBe(false)
    })

    it('sets error on failure', async () => {
      stubFetch({ message: 'Not found' }, false)

      const store = useDocumentsStore()
      await expect(store.fetchDocuments()).rejects.toThrow('Not found')

      expect(store.error).toBe('Not found')
      expect(store.loading).toBe(false)
    })
  })

  describe('fetchDocument', () => {
    it('calls correct URL and sets currentDocument', async () => {
      const doc = mockDocument({ uuid: 'abc-123' })
      stubFetch({ data: doc })

      const store = useDocumentsStore()
      const result = await store.fetchDocument('abc-123')

      expect(fetch).toHaveBeenCalledWith('/api/documents/abc-123', undefined)
      expect(store.currentDocument).toEqual(doc)
      expect(result).toEqual(doc)
    })

    it('sets error on failure', async () => {
      stubFetch({ message: 'Document not found' }, false)

      const store = useDocumentsStore()
      await expect(store.fetchDocument('bad-id')).rejects.toThrow('Document not found')

      expect(store.error).toBe('Document not found')
    })
  })

  describe('uploadDocument', () => {
    it('POSTs FormData to /api/documents', async () => {
      const doc = mockDocument()
      stubFetch({ data: doc })

      const store = useDocumentsStore()
      const formData = new FormData()
      formData.append('file', new Blob(['content']), 'test.md')

      await store.uploadDocument(formData)

      expect(fetch).toHaveBeenCalledWith('/api/documents', {
        method: 'POST',
        body: formData,
      })
    })

    it('sets error on upload failure', async () => {
      stubFetch({ message: 'Upload failed' }, false)

      const store = useDocumentsStore()
      const formData = new FormData()
      await expect(store.uploadDocument(formData)).rejects.toThrow('Upload failed')

      expect(store.error).toBe('Upload failed')
    })
  })

  describe('deleteDocument', () => {
    it('DELETEs and removes from local documents array', async () => {
      stubFetch({ message: 'Deleted' })

      const store = useDocumentsStore()
      store.$patch({
        documents: [mockDocument({ uuid: 'doc-1' }), mockDocument({ uuid: 'doc-2' })],
      })

      await store.deleteDocument('doc-1')

      expect(fetch).toHaveBeenCalledWith('/api/documents/doc-1', { method: 'DELETE' })
      expect(store.documents).toHaveLength(1)
      expect(store.documents[0]!.uuid).toBe('doc-2')
    })

    it('clears currentDocument if it matches deleted uuid', async () => {
      stubFetch({ message: 'Deleted' })

      const store = useDocumentsStore()
      const doc = mockDocument({ uuid: 'doc-1' })
      store.$patch({ currentDocument: doc, documents: [doc] })

      await store.deleteDocument('doc-1')

      expect(store.currentDocument).toBeNull()
    })

    it('keeps currentDocument if it does not match deleted uuid', async () => {
      stubFetch({ message: 'Deleted' })

      const store = useDocumentsStore()
      const doc = mockDocument({ uuid: 'doc-1' })
      store.$patch({ currentDocument: doc, documents: [doc, mockDocument({ uuid: 'doc-2' })] })

      await store.deleteDocument('doc-2')

      expect(store.currentDocument).toEqual(doc)
    })
  })

  describe('restoreDocument', () => {
    it('POSTs to restore endpoint', async () => {
      const doc = mockDocument({ uuid: 'doc-1' })
      stubFetch({ data: doc })

      const store = useDocumentsStore()
      const result = await store.restoreDocument('doc-1')

      expect(fetch).toHaveBeenCalledWith('/api/documents/doc-1/restore', { method: 'POST' })
      expect(result).toEqual(doc)
    })
  })

  describe('purgeDocument', () => {
    it('DELETEs purge endpoint and removes from local array', async () => {
      stubFetch({ message: 'Purged' })

      const store = useDocumentsStore()
      store.$patch({
        documents: [mockDocument({ uuid: 'doc-1' }), mockDocument({ uuid: 'doc-2' })],
      })

      await store.purgeDocument('doc-1')

      expect(fetch).toHaveBeenCalledWith('/api/documents/doc-1/purge', { method: 'DELETE' })
      expect(store.documents).toHaveLength(1)
      expect(store.documents[0]!.uuid).toBe('doc-2')
    })

    it('clears currentDocument if it matches purged uuid', async () => {
      stubFetch({ message: 'Purged' })

      const store = useDocumentsStore()
      const doc = mockDocument({ uuid: 'doc-1' })
      store.$patch({ currentDocument: doc, documents: [doc] })

      await store.purgeDocument('doc-1')

      expect(store.currentDocument).toBeNull()
    })
  })

  describe('refreshCurrent', () => {
    it('returns null if no currentDocument', async () => {
      const store = useDocumentsStore()
      const result = await store.refreshCurrent()

      expect(result).toBeNull()
      expect(fetch).not.toHaveBeenCalled()
    })

    it('re-fetches currentDocument by uuid', async () => {
      const doc = mockDocument({ uuid: 'doc-1', title: 'Updated Title' })
      stubFetch({ data: doc })

      const store = useDocumentsStore()
      store.$patch({ currentDocument: mockDocument({ uuid: 'doc-1', title: 'Old Title' }) })

      const result = await store.refreshCurrent()

      expect(fetch).toHaveBeenCalledWith('/api/documents/doc-1', undefined)
      expect(result).toEqual(doc)
      expect(store.currentDocument).toEqual(doc)
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

      const store = useDocumentsStore()
      await expect(store.fetchDocuments()).rejects.toThrow('Internal Server Error')

      expect(store.error).toBe('Internal Server Error')
    })

    it('clears error before each request', async () => {
      const store = useDocumentsStore()
      store.$patch({ error: 'previous error' })

      stubFetch({ data: [], meta: { total: 0, limit: 10, offset: 0 } })
      await store.fetchDocuments()

      expect(store.error).toBeNull()
    })
  })
})
