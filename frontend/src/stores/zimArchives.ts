import { defineStore } from 'pinia'
import { ref } from 'vue'
import { apiFetch, buildQuery } from '@/api/helpers'

export interface ZimArchive {
  readonly uuid: string
  readonly name: string
  readonly title: string
  readonly description?: string
  readonly language: string
  readonly category?: string
  readonly creator?: string
  readonly publisher?: string
  readonly article_count: number
  readonly media_count: number
  readonly file_size: number
  readonly file_size_human: string
  readonly tags: readonly string[]
  readonly last_synced_at?: string
}

export interface ZimSearchResult {
  readonly title: string
  readonly path: string
  readonly snippet?: string
  readonly score?: number
}

export interface ZimArticle {
  readonly archive_name: string
  readonly path: string
  readonly title: string
  readonly content: string
  readonly mime_type: string
}

interface ListParams {
  readonly query?: string
  readonly category?: string
  readonly language?: string
  readonly per_page?: number
  readonly offset?: number
}

interface ListResponse {
  readonly data: ZimArchive[]
  readonly meta: {
    readonly total: number
  }
}

interface SearchResponse {
  readonly data: ZimSearchResult[]
  readonly meta: {
    readonly archive: string
    readonly query: string
    readonly total: number
  }
}

interface ArticleResponse {
  readonly data: ZimArticle
}

export const useZimArchivesStore = defineStore('zimArchives', () => {
  const archives = ref<ZimArchive[]>([])
  const total = ref(0)
  const currentArticle = ref<ZimArticle | null>(null)
  const searchResults = ref<ZimSearchResult[]>([])
  const loading = ref(false)
  const searchLoading = ref(false)
  const articleLoading = ref(false)
  const error = ref<string | null>(null)

  async function fetchArchives(params?: ListParams): Promise<ListResponse> {
    loading.value = true
    error.value = null
    try {
      const query = buildQuery({
        query: params?.query,
        category: params?.category,
        language: params?.language,
        per_page: params?.per_page,
        offset: params?.offset,
      })
      const response = await apiFetch<ListResponse>(`/api/zim/archives${query}`)
      archives.value = response.data
      total.value = response.meta.total
      return response
    } catch (e) {
      error.value = e instanceof Error ? e.message : 'Failed to fetch ZIM archives'
      throw e
    } finally {
      loading.value = false
    }
  }

  async function searchArticles(
    archive: string,
    q: string,
    limit?: number,
  ): Promise<ZimSearchResult[]> {
    searchLoading.value = true
    error.value = null
    try {
      const query = buildQuery({ q, limit })
      const response = await apiFetch<SearchResponse>(
        `/api/zim/archives/${encodeURIComponent(archive)}/search${query}`,
      )
      searchResults.value = response.data
      return response.data
    } catch (e) {
      error.value = e instanceof Error ? e.message : 'Failed to search articles'
      throw e
    } finally {
      searchLoading.value = false
    }
  }

  async function readArticle(archive: string, path: string): Promise<ZimArticle> {
    articleLoading.value = true
    error.value = null
    try {
      const response = await apiFetch<ArticleResponse>(
        `/api/zim/archives/${encodeURIComponent(archive)}/articles/${encodeURIComponent(path)}`,
      )
      currentArticle.value = response.data
      return response.data
    } catch (e) {
      error.value = e instanceof Error ? e.message : 'Failed to read article'
      throw e
    } finally {
      articleLoading.value = false
    }
  }

  function clearSearch(): void {
    searchResults.value = []
  }

  function clearArticle(): void {
    currentArticle.value = null
  }

  return {
    archives,
    total,
    currentArticle,
    searchResults,
    loading,
    searchLoading,
    articleLoading,
    error,
    fetchArchives,
    searchArticles,
    readArticle,
    clearSearch,
    clearArticle,
  }
})
