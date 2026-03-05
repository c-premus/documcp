import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'
import { createPinia, setActivePinia } from 'pinia'
import { authGuard } from '@/auth/authGuard'
import { useAuthStore } from '@/stores/auth'
import type { RouteLocationNormalized, NavigationGuardNext } from 'vue-router'

function makeRoute(overrides: Partial<RouteLocationNormalized> = {}): RouteLocationNormalized {
  return {
    fullPath: '/dashboard',
    path: '/dashboard',
    name: undefined,
    params: {},
    query: {},
    hash: '',
    matched: [],
    redirectedFrom: undefined,
    meta: {},
    ...overrides,
  } as RouteLocationNormalized
}

describe('authGuard', () => {
  let next: NavigationGuardNext

  beforeEach(() => {
    setActivePinia(createPinia())
    next = vi.fn()
    vi.stubGlobal('fetch', vi.fn().mockResolvedValue({ ok: false }))
  })

  afterEach(() => {
    vi.restoreAllMocks()
    vi.unstubAllGlobals()
  })

  it('redirects to login when not authenticated', async () => {
    const locationMock = { href: '' }
    vi.stubGlobal('location', locationMock)

    const auth = useAuthStore()
    auth.$patch({ loading: false, user: null })

    const to = makeRoute({ fullPath: '/documents' })
    await authGuard(to, makeRoute(), next)

    expect(locationMock.href).toBe('/auth/login?redirect=%2Fadmin%2Fdocuments')
    expect(next).not.toHaveBeenCalled()
  })

  it('calls next() when authenticated', async () => {
    const auth = useAuthStore()
    auth.$patch({
      loading: false,
      user: { id: 1, email: 'test@example.com', name: 'Test', is_admin: false },
    })

    await authGuard(makeRoute(), makeRoute(), next)

    expect(next).toHaveBeenCalledWith()
  })

  it('redirects to dashboard when admin required but user is not admin', async () => {
    const auth = useAuthStore()
    auth.$patch({
      loading: false,
      user: { id: 1, email: 'test@example.com', name: 'Test', is_admin: false },
    })

    const to = makeRoute({ meta: { requiresAdmin: true } })
    await authGuard(to, makeRoute(), next)

    expect(next).toHaveBeenCalledWith({ name: 'dashboard' })
  })

  it('allows admin access when user is admin', async () => {
    const auth = useAuthStore()
    auth.$patch({
      loading: false,
      user: { id: 1, email: 'admin@example.com', name: 'Admin', is_admin: true },
    })

    const to = makeRoute({ meta: { requiresAdmin: true } })
    await authGuard(to, makeRoute(), next)

    expect(next).toHaveBeenCalledWith()
  })

  it('fetches user first if still loading', async () => {
    const mockUser = { id: 1, email: 'test@example.com', name: 'Test', is_admin: false }
    vi.stubGlobal(
      'fetch',
      vi.fn().mockResolvedValue({
        ok: true,
        json: () => Promise.resolve(mockUser),
      }),
    )

    const auth = useAuthStore()
    // loading defaults to true, so fetchUser should be called

    await authGuard(makeRoute(), makeRoute(), next)

    expect(fetch).toHaveBeenCalledWith('/api/auth/me')
    expect(auth.user).toEqual(mockUser)
    expect(next).toHaveBeenCalledWith()
  })
})
