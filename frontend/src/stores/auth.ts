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
        const body = await response.json()
        user.value = body.data as User
      } else {
        user.value = null
      }
    } catch {
      user.value = null
    } finally {
      loading.value = false
    }
  }

  async function logout(options: { revokeOAuth?: boolean } = {}) {
    user.value = null
    const url = options.revokeOAuth ? '/auth/logout?revoke_oauth=true' : '/auth/logout'
    try {
      const response = await fetch(url, { method: 'POST' })
      if (response.ok) {
        const body = await response.json()
        if (body.redirect_url) {
          window.location.href = body.redirect_url
          return
        }
      }
    } catch {
      // Fall through to default redirect.
    }
    window.location.href = '/'
  }

  return { user, loading, isAuthenticated, isAdmin, fetchUser, logout }
})
