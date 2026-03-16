import { defineStore } from 'pinia'
import { ref } from 'vue'
import { apiFetch } from '@/api/helpers'

export interface ConfluenceSpace {
  readonly key: string
  readonly name: string
  readonly type: string
  readonly description?: string
}

interface ListResponse {
  readonly data: ConfluenceSpace[]
  readonly meta: { readonly total: number }
}


export const useConfluenceSpacesStore = defineStore('confluenceSpaces', () => {
  const spaces = ref<ConfluenceSpace[]>([])
  const total = ref(0)
  const loading = ref(false)
  const error = ref<string | null>(null)

  async function fetchSpaces(): Promise<ListResponse> {
    loading.value = true
    error.value = null
    try {
      const response = await apiFetch<ListResponse>('/api/confluence/spaces')
      spaces.value = response.data
      total.value = response.meta.total
      return response
    } catch (e) {
      error.value = e instanceof Error ? e.message : 'Failed to fetch Confluence spaces'
      throw e
    } finally {
      loading.value = false
    }
  }

  return {
    spaces,
    total,
    loading,
    error,
    fetchSpaces,
  }
})
