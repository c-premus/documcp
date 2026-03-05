import { defineStore } from 'pinia'
import { ref } from 'vue'

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
  name: string
  path: string
  type: 'file' | 'directory'
  children?: TreeItem[]
}

interface ListResponse {
  readonly data: GitTemplate[]
  readonly meta: { readonly total: number }
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

interface CreateTemplateParams {
  readonly name: string
  readonly repository_url: string
  readonly branch: string
  readonly category: string
  readonly description?: string
  readonly tags?: string[]
  readonly is_public?: boolean
}

async function api<T>(url: string, options?: RequestInit): Promise<T> {
  const res = await fetch(url, options)
  if (!res.ok) {
    const body = await res.json().catch(() => ({ message: res.statusText }))
    throw new Error(body.message || res.statusText)
  }
  return res.json() as Promise<T>
}

function buildQuery(params: Record<string, string | number | undefined>): string {
  const search = new URLSearchParams()
  for (const [key, value] of Object.entries(params)) {
    if (value !== undefined && value !== '') {
      search.set(key, String(value))
    }
  }
  const qs = search.toString()
  return qs ? `?${qs}` : ''
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
  const error = ref<string | null>(null)

  async function fetchTemplates(category?: string): Promise<ListResponse> {
    loading.value = true
    error.value = null
    try {
      const query = buildQuery({ category })
      const response = await api<ListResponse>(`/api/git-templates${query}`)
      templates.value = response.data
      total.value = response.meta.total
      return response
    } catch (e) {
      error.value = e instanceof Error ? e.message : 'Failed to fetch git templates'
      throw e
    } finally {
      loading.value = false
    }
  }

  async function createTemplate(params: CreateTemplateParams): Promise<GitTemplate> {
    loading.value = true
    error.value = null
    try {
      const response = await api<SingleResponse>('/api/git-templates', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify(params),
      })
      return response.data
    } catch (e) {
      error.value = e instanceof Error ? e.message : 'Failed to create git template'
      throw e
    } finally {
      loading.value = false
    }
  }

  async function deleteTemplate(uuid: string): Promise<MessageResponse> {
    loading.value = true
    error.value = null
    try {
      const response = await api<MessageResponse>(`/api/git-templates/${uuid}`, {
        method: 'DELETE',
      })
      templates.value = templates.value.filter((t) => t.uuid !== uuid)
      return response
    } catch (e) {
      error.value = e instanceof Error ? e.message : 'Failed to delete git template'
      throw e
    } finally {
      loading.value = false
    }
  }

  async function syncTemplate(uuid: string): Promise<MessageResponse> {
    try {
      const response = await api<MessageResponse>(`/api/git-templates/${uuid}/sync`, {
        method: 'POST',
      })
      return response
    } catch (e) {
      error.value = e instanceof Error ? e.message : 'Failed to sync git template'
      throw e
    }
  }

  async function fetchStructure(uuid: string): Promise<StructureResponse['data']> {
    loading.value = true
    error.value = null
    try {
      const response = await api<StructureResponse>(`/api/git-templates/${uuid}/structure`)
      return response.data
    } catch (e) {
      error.value = e instanceof Error ? e.message : 'Failed to fetch template structure'
      throw e
    } finally {
      loading.value = false
    }
  }

  async function readFile(uuid: string, path: string): Promise<FileContentResponse['data']> {
    try {
      const response = await api<FileContentResponse>(
        `/api/git-templates/${uuid}/files/${encodeURIComponent(path)}`,
      )
      return response.data
    } catch (e) {
      error.value = e instanceof Error ? e.message : 'Failed to read file'
      throw e
    }
  }

  async function validateUrl(url: string): Promise<ValidateUrlResponse> {
    const response = await api<ValidateUrlResponse>('/api/admin/git-templates/validate-url', {
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
    error,
    fetchTemplates,
    createTemplate,
    deleteTemplate,
    syncTemplate,
    fetchStructure,
    readFile,
    validateUrl,
  }
})
