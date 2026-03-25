import type { NavigationGuard } from 'vue-router'
import { useAuthStore } from '@/stores/auth'

export const authGuard: NavigationGuard = async (to, _from, next) => {
  const auth = useAuthStore()

  if (auth.loading) {
    await auth.fetchUser()
  }

  if (!auth.isAuthenticated) {
    window.location.href = '/auth/login?redirect=' + encodeURIComponent('/admin' + to.fullPath)
    return
  }

  if (to.meta.requiresAdmin && !auth.isAdmin) {
    next({ name: 'dashboard' })
    return
  }

  next()
}
