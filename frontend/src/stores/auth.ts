import { defineStore } from 'pinia'
import { ref, computed } from 'vue'

export interface User {
  readonly id: number
  readonly email: string
  readonly name: string
  readonly is_admin: boolean
}

export const useAuthStore = defineStore('auth', () => {
  const user = ref<User | null>(null)
  const loading = ref(true)

  const isAuthenticated = computed(() => user.value !== null)
  const isAdmin = computed(() => user.value?.is_admin ?? false)

  async function fetchUser() {
    try {
      const response = await fetch('/api/auth/me')
      if (response.ok) {
        user.value = (await response.json()) as User
      } else {
        user.value = null
      }
    } catch {
      user.value = null
    } finally {
      loading.value = false
    }
  }

  function logout() {
    fetch('/auth/logout', { method: 'POST' }).finally(() => {
      user.value = null
      window.location.href = '/'
    })
  }

  return { user, loading, isAuthenticated, isAdmin, fetchUser, logout }
})
