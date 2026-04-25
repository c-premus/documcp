import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'
import { createPinia, setActivePinia } from 'pinia'
import { useUsersStore, type User } from '@/stores/users'

function mockUser(overrides: Partial<User> = {}): User {
  return {
    id: 1,
    name: 'Admin',
    email: 'admin@example.com',
    oidc_sub: 'sub-abc',
    oidc_provider: 'https://idp.example.com',
    is_admin: true,
    created_at: '2026-01-01T00:00:00Z',
    updated_at: '2026-01-01T00:00:00Z',
    ...overrides,
  }
}

function stubFetch(response: unknown, ok = true) {
  vi.stubGlobal(
    'fetch',
    vi.fn().mockResolvedValue({
      ok,
      status: ok ? 200 : 500,
      statusText: 'Internal Server Error',
      json: () => Promise.resolve(response),
    }),
  )
}

describe('users store', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
    vi.stubGlobal('fetch', vi.fn())
  })

  afterEach(() => {
    vi.restoreAllMocks()
    vi.unstubAllGlobals()
  })

  it('initial state is empty', () => {
    const store = useUsersStore()
    expect(store.users).toEqual([])
    expect(store.total).toBe(0)
    expect(store.loading).toBe(false)
    expect(store.error).toBeNull()
  })

  describe('fetchUsers', () => {
    it('calls the admin endpoint with query params and mirrors response into state', async () => {
      const users = [mockUser(), mockUser({ id: 2, name: 'User' })]
      stubFetch({ data: users, meta: { total: 2 } })

      const store = useUsersStore()
      await store.fetchUsers({ limit: 10, offset: 0, q: 'admin' })

      const calledUrl = (fetch as ReturnType<typeof vi.fn>).mock.calls[0]![0] as string
      expect(calledUrl).toContain('/api/admin/users?')
      expect(calledUrl).toContain('limit=10')
      expect(calledUrl).toContain('q=admin')
      expect(store.users).toEqual(users)
      expect(store.total).toBe(2)
    })
  })

  describe('toggleAdmin', () => {
    it('POSTs to toggle-admin and replaces the user in place', async () => {
      const user = mockUser({ is_admin: false })
      const store = useUsersStore()
      store.$patch({ users: [user] })

      const updated = { ...user, is_admin: true }
      stubFetch({ data: updated })

      const result = await store.toggleAdmin(user.id)

      expect(result).toEqual(updated)
      expect(store.users[0]?.is_admin).toBe(true)
      const opts = (fetch as ReturnType<typeof vi.fn>).mock.calls[0]![1] as RequestInit
      expect(opts.method).toBe('POST')
    })
  })

  describe('deleteUser', () => {
    it('DELETEs the user and removes it from state', async () => {
      const user = mockUser()
      const store = useUsersStore()
      store.$patch({ users: [user] })

      stubFetch({ message: 'deleted' })

      await store.deleteUser(user.id)

      expect(store.users).toHaveLength(0)
      const opts = (fetch as ReturnType<typeof vi.fn>).mock.calls[0]![1] as RequestInit
      expect(opts.method).toBe('DELETE')
    })
  })

  describe('fetchUserSessions', () => {
    it('GETs /sessions and mirrors the array into sessionIDs', async () => {
      stubFetch({ data: ['abc', 'def'] })

      const store = useUsersStore()
      const result = await store.fetchUserSessions(7)

      const calledUrl = (fetch as ReturnType<typeof vi.fn>).mock.calls[0]![0] as string
      expect(calledUrl).toBe('/api/admin/users/7/sessions')
      expect(result).toEqual(['abc', 'def'])
      expect(store.sessionIDs).toEqual(['abc', 'def'])
    })
  })

  describe('revokeUserSession', () => {
    it('DELETEs the encoded session ID and drops it from state', async () => {
      const store = useUsersStore()
      store.$patch({ sessionIDs: ['abc', 'def'] })
      stubFetch({ message: 'session revoked' })

      await store.revokeUserSession(7, 'abc')

      const calledUrl = (fetch as ReturnType<typeof vi.fn>).mock.calls[0]![0] as string
      expect(calledUrl).toBe('/api/admin/users/7/sessions/abc')
      const opts = (fetch as ReturnType<typeof vi.fn>).mock.calls[0]![1] as RequestInit
      expect(opts.method).toBe('DELETE')
      expect(store.sessionIDs).toEqual(['def'])
    })

    it('encodes session IDs that contain URL-significant characters', async () => {
      stubFetch({ message: 'session revoked' })
      const store = useUsersStore()

      await store.revokeUserSession(7, 'abc/def?x')

      const calledUrl = (fetch as ReturnType<typeof vi.fn>).mock.calls[0]![0] as string
      expect(calledUrl).toBe('/api/admin/users/7/sessions/abc%2Fdef%3Fx')
    })
  })

  describe('revokeAllUserSessions', () => {
    it('DELETEs the collection, returns the revoked count, clears state', async () => {
      const store = useUsersStore()
      store.$patch({ sessionIDs: ['abc', 'def', 'ghi'] })
      stubFetch({ data: { revoked: 3 } })

      const count = await store.revokeAllUserSessions(7)

      expect(count).toBe(3)
      expect(store.sessionIDs).toEqual([])
      const calledUrl = (fetch as ReturnType<typeof vi.fn>).mock.calls[0]![0] as string
      expect(calledUrl).toBe('/api/admin/users/7/sessions')
      const opts = (fetch as ReturnType<typeof vi.fn>).mock.calls[0]![1] as RequestInit
      expect(opts.method).toBe('DELETE')
    })
  })
})
