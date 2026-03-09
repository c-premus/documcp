import { defineStore } from 'pinia'
import { ref } from 'vue'

export interface QueueStats {
  readonly available: number
  readonly running: number
  readonly retryable: number
  readonly discarded: number
  readonly cancelled: number
}

export interface AttemptError {
  readonly at: string
  readonly error: string
  readonly trace: string
}

export interface FailedJob {
  readonly id: number
  readonly kind: string
  readonly queue: string
  readonly state: string
  readonly attempt: number
  readonly max_attempts: number
  readonly created_at: string
  readonly errors?: AttemptError[]
}

async function api<T>(url: string, options?: RequestInit): Promise<T> {
  const res = await fetch(url, options)
  if (!res.ok) {
    const body = await res.json().catch(() => ({ message: res.statusText }))
    throw new Error(body.message || res.statusText)
  }
  return res.json() as Promise<T>
}

export const useQueueStore = defineStore('queue', () => {
  const stats = ref<QueueStats | null>(null)
  const failedJobs = ref<FailedJob[]>([])
  const failedCount = ref(0)
  const loading = ref(false)

  async function fetchStats(): Promise<QueueStats> {
    loading.value = true
    try {
      const response = await api<{data: QueueStats}>('/api/admin/queue/stats')
      stats.value = response.data
      return response.data
    } finally {
      loading.value = false
    }
  }

  async function fetchFailedJobs(limit?: number): Promise<{data: FailedJob[], meta: {count: number}}> {
    loading.value = true
    try {
      const qs = limit ? `?limit=${limit}` : ''
      const data = await api<{data: FailedJob[], meta: {count: number}}>(`/api/admin/queue/failed${qs}`)
      failedJobs.value = data.data
      failedCount.value = data.meta.count
      return data
    } finally {
      loading.value = false
    }
  }

  async function retryJob(id: number): Promise<void> {
    await api<{ id: number; state: string }>(`/api/admin/queue/failed/${id}/retry`, {
      method: 'POST',
    })
  }

  async function deleteJob(id: number): Promise<void> {
    await api<{ id: number; state: string }>(`/api/admin/queue/failed/${id}`, {
      method: 'DELETE',
    })
  }

  return {
    stats,
    failedJobs,
    failedCount,
    loading,
    fetchStats,
    fetchFailedJobs,
    retryJob,
    deleteJob,
  }
})
