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
})
