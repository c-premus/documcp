import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'
import { createPinia, setActivePinia } from 'pinia'
import { useAuthStore } from '@/stores/auth'

describe('auth store', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
    vi.stubGlobal('fetch', vi.fn())
  })

  afterEach(() => {
    vi.restoreAllMocks()
    vi.unstubAllGlobals()
  })

  it('has correct initial state', () => {
    const auth = useAuthStore()

    expect(auth.user).toBeNull()
    expect(auth.loading).toBe(true)
    expect(auth.isAuthenticated).toBe(false)
    expect(auth.isAdmin).toBe(false)
  })

  describe('fetchUser', () => {
    it('sets user and loading false on success', async () => {
      const mockUser = { id: 1, email: 'test@example.com', name: 'Test', is_admin: false }
      vi.stubGlobal(
        'fetch',
        vi.fn().mockResolvedValue({
          ok: true,
          json: () => Promise.resolve({ data: mockUser }),
        }),
      )

      const auth = useAuthStore()
      await auth.fetchUser()

      expect(auth.user).toEqual(mockUser)
      expect(auth.loading).toBe(false)
      expect(auth.isAuthenticated).toBe(true)
    })

    it('sets isAdmin true when user is admin', async () => {
      const mockUser = { id: 1, email: 'admin@example.com', name: 'Admin', is_admin: true }
      vi.stubGlobal(
        'fetch',
        vi.fn().mockResolvedValue({
          ok: true,
          json: () => Promise.resolve({ data: mockUser }),
        }),
      )

      const auth = useAuthStore()
      await auth.fetchUser()

      expect(auth.isAdmin).toBe(true)
    })

    it('sets user to null on 401 response', async () => {
      vi.stubGlobal('fetch', vi.fn().mockResolvedValue({ ok: false, status: 401 }))

      const auth = useAuthStore()
      await auth.fetchUser()

      expect(auth.user).toBeNull()
      expect(auth.loading).toBe(false)
      expect(auth.isAuthenticated).toBe(false)
    })

    it('sets user to null on network error', async () => {
      vi.stubGlobal('fetch', vi.fn().mockRejectedValue(new Error('Network error')))

      const auth = useAuthStore()
      await auth.fetchUser()

      expect(auth.user).toBeNull()
      expect(auth.loading).toBe(false)
      expect(auth.isAuthenticated).toBe(false)
    })
  })

  describe('logout', () => {
    it('calls POST /auth/logout and clears user', async () => {
      const locationMock = { href: '' }
      vi.stubGlobal('location', locationMock)

      const fetchMock = vi.fn().mockResolvedValue({ ok: true })
      vi.stubGlobal('fetch', fetchMock)

      const auth = useAuthStore()
      auth.$patch({
        user: { id: 1, email: 'test@example.com', name: 'Test', is_admin: false },
      })

      auth.logout()

      // Wait for the fetch promise and .finally() to resolve
      await vi.waitFor(() => {
        expect(fetchMock).toHaveBeenCalledWith('/auth/logout', { method: 'POST' })
        expect(auth.user).toBeNull()
        expect(locationMock.href).toBe('/')
      })
    })
  })
})
