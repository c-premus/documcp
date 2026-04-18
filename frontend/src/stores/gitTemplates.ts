import { defineStore } from 'pinia'
import { ref } from 'vue'
import { apiFetch, buildQuery } from '@/api/helpers'
import { withLoading } from '@/composables/useAsyncAction'

export interface GitTemplate {
  readonly uuid: string
  readonly name: string
  readonly slug: string
  readonly description?: string
  readonly repository_url: string
  readonly branch: string
  readonly category?: string
  readonly tags: string[]
  readonly is_public: boolean
  readonly status: string
  readonly error_message?: string
  readonly file_count: number
  readonly total_size_bytes: number
  readonly last_synced_at?: string
  readonly last_commit_sha?: string
  readonly created_at?: string
  readonly updated_at?: string
}

export interface GitTemplateFile {
  readonly path: string
  readonly filename: string
  readonly extension?: string
  readonly size_bytes: number
  readonly is_essential: boolean
  readonly content_hash?: string
}

export interface TreeItem {
  readonly name: string
  readonly path: string
  readonly type: 'file' | 'directory'
  children?: TreeItem[]
}

interface ListParams {
  readonly category?: string
  readonly per_page?: number
  readonly offset?: number
}

interface ListResponse {
  readonly data: GitTemplate[]
  readonly meta: { readonly total: number; readonly limit: number; readonly offset: number }
}

interface SingleResponse {
  readonly data: GitTemplate
  readonly message?: string
}

interface MessageResponse {
  readonly message: string
}

interface StructureResponse {
  readonly data: {
    readonly uuid: string
    readonly name: string
    readonly file_tree: string[]
    readonly essential_files: string[]
    readonly variables: string[]
    readonly files: GitTemplateFile[]
    readonly file_count: number
    readonly total_size: number
  }
}

interface FileContentResponse {
  readonly data: {
    readonly path: string
    readonly filename: string
    readonly size_bytes: number
    readonly is_essential: boolean
    readonly content: string
  }
}

interface ValidateUrlResponse {
  readonly valid: boolean
  readonly error?: string
}

export interface CreateTemplateParams {
  readonly name: string
  readonly repository_url: string
  readonly branch: string
  readonly category: string
  readonly description?: string
  readonly tags?: string[]
  readonly is_public?: boolean
  readonly git_token?: string
}

export function buildTree(files: GitTemplateFile[]): TreeItem[] {
  const root: TreeItem[] = []

  for (const file of files) {
    const parts = file.path.split('/')
    let currentLevel = root

    for (let i = 0; i < parts.length; i++) {
      const part = parts[i]
      if (part === undefined) {
        continue
      }

      const isLast = i === parts.length - 1
      const existingNode = currentLevel.find((node) => node.name === part)

      if (isLast) {
        if (existingNode === undefined) {
          currentLevel.push({
            name: part,
            path: file.path,
            type: 'file',
          })
        }
      } else {
        if (existingNode !== undefined) {
          if (existingNode.children === undefined) {
            existingNode.children = []
          }
          currentLevel = existingNode.children
        } else {
          const dirPath = parts.slice(0, i + 1).join('/')
          const newDir: TreeItem = {
            name: part,
            path: dirPath,
            type: 'directory',
            children: [],
          }
          currentLevel.push(newDir)
          currentLevel = newDir.children!
        }
      }
    }
  }

  sortTree(root)
  return root
}

function sortTree(items: TreeItem[]): void {
  items.sort((a, b) => {
    if (a.type === 'directory' && b.type === 'file') {
      return -1
    }
    if (a.type === 'file' && b.type === 'directory') {
      return 1
    }
    return a.name.localeCompare(b.name)
  })
  for (const item of items) {
    if (item.children !== undefined) {
      sortTree(item.children)
    }
  }
}

export const useGitTemplatesStore = defineStore('gitTemplates', () => {
  const templates = ref<GitTemplate[]>([])
  const total = ref(0)
  const loading = ref(false)
  const loaded = ref(false)
  const error = ref<string | null>(null)

  async function fetchTemplates(params?: ListParams): Promise<ListResponse> {
    return withLoading(
      loading,
      error,
      async () => {
        const query = buildQuery({
          category: params?.category,
          per_page: params?.per_page,
          offset: params?.offset,
        })
        const response = await apiFetch<ListResponse>(`/api/git-templates${query}`)
        templates.value = response.data
        total.value = response.meta.total
        loaded.value = true
        return response
      },
      'Failed to fetch git templates',
    )
  }

  async function createTemplate(params: CreateTemplateParams): Promise<GitTemplate> {
    return withLoading(
      loading,
      error,
      async () => {
        const response = await apiFetch<SingleResponse>('/api/git-templates', {
          method: 'POST',
          headers: { 'Content-Type': 'application/json' },
          body: JSON.stringify(params),
        })
        templates.value = [response.data, ...templates.value]
        total.value += 1
        return response.data
      },
      'Failed to create git template',
    )
  }

  async function deleteTemplate(uuid: string): Promise<MessageResponse> {
    return withLoading(
      loading,
      error,
      async () => {
        const response = await apiFetch<MessageResponse>(`/api/git-templates/${uuid}`, {
          method: 'DELETE',
        })
        templates.value = templates.value.filter((t) => t.uuid !== uuid)
        return response
      },
      'Failed to delete git template',
    )
  }

  async function syncTemplate(uuid: string): Promise<MessageResponse> {
    try {
      const response = await apiFetch<MessageResponse>(`/api/git-templates/${uuid}/sync`, {
        method: 'POST',
      })
      return response
    } catch (e) {
      error.value = e instanceof Error ? e.message : 'Failed to sync git template'
      throw e
    }
  }

  async function fetchStructure(uuid: string): Promise<StructureResponse['data']> {
    return withLoading(
      loading,
      error,
      async () => {
        const response = await apiFetch<StructureResponse>(`/api/git-templates/${uuid}/structure`)
        return response.data
      },
      'Failed to fetch template structure',
    )
  }

  async function readFile(uuid: string, path: string): Promise<FileContentResponse['data']> {
    try {
      const encodedPath = path.split('/').map(encodeURIComponent).join('/')
      const response = await apiFetch<FileContentResponse>(
        `/api/git-templates/${uuid}/files/${encodedPath}`,
      )
      return response.data
    } catch (e) {
      error.value = e instanceof Error ? e.message : 'Failed to read file'
      throw e
    }
  }

  async function updateTemplate(
    uuid: string,
    params: Partial<CreateTemplateParams>,
  ): Promise<GitTemplate> {
    return withLoading(
      loading,
      error,
      async () => {
        const response = await apiFetch<SingleResponse>(`/api/git-templates/${uuid}`, {
          method: 'PUT',
          headers: { 'Content-Type': 'application/json' },
          body: JSON.stringify(params),
        })
        const index = templates.value.findIndex((t) => t.uuid === uuid)
        if (index !== -1) {
          templates.value[index] = response.data
        }
        return response.data
      },
      'Failed to update git template',
    )
  }

  async function validateUrl(url: string): Promise<ValidateUrlResponse> {
    const response = await apiFetch<ValidateUrlResponse>('/api/admin/git-templates/validate-url', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ url }),
    })
    return response
  }

  return {
    templates,
    total,
    loading,
    loaded,
    error,
    fetchTemplates,
    createTemplate,
    updateTemplate,
    deleteTemplate,
    syncTemplate,
    fetchStructure,
    readFile,
    validateUrl,
  }
})
