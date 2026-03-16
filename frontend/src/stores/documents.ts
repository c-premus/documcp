import { defineStore } from 'pinia'
import { ref } from 'vue'
import { apiFetch, buildQuery } from '@/api/helpers'

export interface Document {
  readonly uuid: string
  readonly title: string
  readonly description: string
  readonly file_type: string
  readonly file_size: number
  readonly mime_type: string
  readonly word_count: number
  readonly is_public: boolean
  readonly status: string
  readonly content_hash: string
  readonly tags: string[]
  readonly created_at: string
  readonly updated_at: string
  readonly processed_at: string
  readonly content?: string
}

export interface AnalyzeResult {
  readonly title: string
  readonly description: string
  readonly tags: string[]
  readonly word_count: number
  readonly reading_time: number
  readonly language: string
}

interface ListParams {
  readonly limit?: number
  readonly offset?: number
  readonly q?: string
  readonly file_type?: string
  readonly status?: string
  readonly sort?: string
  readonly order?: 'asc' | 'desc'
}

interface TrashParams {
  readonly limit?: number
  readonly offset?: number
}

interface ListResponse {
  readonly data: Document[]
  readonly meta: {
    readonly total: number
    readonly limit: number
    readonly offset: number
  }
}

interface SingleResponse {
  readonly data: Document
  readonly message?: string
}

interface DeleteResponse {
  readonly message: string
}

interface BulkPurgeResponse {
  readonly message: string
  readonly count: number
}

interface AnalyzeResponse {
  readonly data: AnalyzeResult
}

interface TrashResponse {
  readonly data: Document[]
  readonly total: number
}


export const useDocumentsStore = defineStore('documents', () => {
  const documents = ref<Document[]>([])
  const currentDocument = ref<Document | null>(null)
  const total = ref(0)
  const loading = ref(false)
  const error = ref<string | null>(null)

  async function fetchDocuments(params?: ListParams): Promise<ListResponse> {
    loading.value = true
    error.value = null
    try {
      const query = buildQuery({
        limit: params?.limit,
        offset: params?.offset,
        q: params?.q,
        file_type: params?.file_type,
        status: params?.status,
        sort: params?.sort,
        order: params?.order,
      })
      const response = await apiFetch<ListResponse>(`/api/documents${query}`)
      documents.value = response.data
      total.value = response.meta.total
      return response
    } catch (e) {
      error.value = e instanceof Error ? e.message : 'Failed to fetch documents'
      throw e
    } finally {
      loading.value = false
    }
  }

  async function fetchDocument(uuid: string): Promise<Document> {
    loading.value = true
    error.value = null
    try {
      const response = await apiFetch<SingleResponse>(`/api/documents/${uuid}`)
      currentDocument.value = response.data
      return response.data
    } catch (e) {
      error.value = e instanceof Error ? e.message : 'Failed to fetch document'
      throw e
    } finally {
      loading.value = false
    }
  }

  async function uploadDocument(formData: FormData): Promise<SingleResponse> {
    loading.value = true
    error.value = null
    try {
      const response = await apiFetch<SingleResponse>('/api/documents', {
        method: 'POST',
        body: formData,
      })
      return response
    } catch (e) {
      error.value = e instanceof Error ? e.message : 'Failed to upload document'
      throw e
    } finally {
      loading.value = false
    }
  }

  async function analyzeDocument(formData: FormData): Promise<AnalyzeResult> {
    loading.value = true
    error.value = null
    try {
      const response = await apiFetch<AnalyzeResponse>('/api/documents/analyze', {
        method: 'POST',
        body: formData,
      })
      return response.data
    } catch (e) {
      error.value = e instanceof Error ? e.message : 'Failed to analyze document'
      throw e
    } finally {
      loading.value = false
    }
  }

  async function deleteDocument(uuid: string): Promise<DeleteResponse> {
    loading.value = true
    error.value = null
    try {
      const response = await apiFetch<DeleteResponse>(`/api/documents/${uuid}`, {
        method: 'DELETE',
      })
      documents.value = documents.value.filter((d) => d.uuid !== uuid)
      if (currentDocument.value?.uuid === uuid) {
        currentDocument.value = null
      }
      return response
    } catch (e) {
      error.value = e instanceof Error ? e.message : 'Failed to delete document'
      throw e
    } finally {
      loading.value = false
    }
  }

  async function restoreDocument(uuid: string): Promise<Document> {
    loading.value = true
    error.value = null
    try {
      const response = await apiFetch<SingleResponse>(`/api/documents/${uuid}/restore`, {
        method: 'POST',
      })
      return response.data
    } catch (e) {
      error.value = e instanceof Error ? e.message : 'Failed to restore document'
      throw e
    } finally {
      loading.value = false
    }
  }

  async function purgeDocument(uuid: string): Promise<DeleteResponse> {
    loading.value = true
    error.value = null
    try {
      const response = await apiFetch<DeleteResponse>(`/api/documents/${uuid}/purge`, {
        method: 'DELETE',
      })
      documents.value = documents.value.filter((d) => d.uuid !== uuid)
      if (currentDocument.value?.uuid === uuid) {
        currentDocument.value = null
      }
      return response
    } catch (e) {
      error.value = e instanceof Error ? e.message : 'Failed to purge document'
      throw e
    } finally {
      loading.value = false
    }
  }

  async function bulkPurge(olderThanDays: number): Promise<BulkPurgeResponse> {
    loading.value = true
    error.value = null
    try {
      const query = buildQuery({ older_than_days: olderThanDays })
      const response = await apiFetch<BulkPurgeResponse>(`/api/admin/documents/purge${query}`, {
        method: 'DELETE',
      })
      return response
    } catch (e) {
      error.value = e instanceof Error ? e.message : 'Failed to bulk purge documents'
      throw e
    } finally {
      loading.value = false
    }
  }

  async function fetchDeletedDocuments(params?: TrashParams): Promise<TrashResponse> {
    loading.value = true
    error.value = null
    try {
      const query = buildQuery({
        limit: params?.limit,
        offset: params?.offset,
      })
      const response = await apiFetch<TrashResponse>(`/api/documents/trash${query}`)
      documents.value = response.data
      total.value = response.total
      return response
    } catch (e) {
      error.value = e instanceof Error ? e.message : 'Failed to fetch deleted documents'
      throw e
    } finally {
      loading.value = false
    }
  }

  async function refreshCurrent(): Promise<Document | null> {
    if (currentDocument.value === null) {
      return null
    }
    return fetchDocument(currentDocument.value.uuid)
  }

  return {
    documents,
    currentDocument,
    total,
    loading,
    error,
    fetchDocuments,
    fetchDocument,
    uploadDocument,
    analyzeDocument,
    deleteDocument,
    restoreDocument,
    purgeDocument,
    bulkPurge,
    fetchDeletedDocuments,
    refreshCurrent,
  }
})
