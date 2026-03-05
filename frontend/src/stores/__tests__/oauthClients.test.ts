import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'
import { createPinia, setActivePinia } from 'pinia'
import { useOAuthClientsStore } from '@/stores/oauthClients'
import type { OAuthClient, CreatedClient } from '@/stores/oauthClients'

function mockClient(overrides: Partial<OAuthClient> = {}): OAuthClient {
  return {
    id: 1,
    client_id: 'client-abc',
    client_name: 'Test Client',
    redirect_uris: ['https://example.com/callback'],
    grant_types: ['authorization_code'],
    response_types: ['code'],
    token_endpoint_auth_method: 'client_secret_basic',
    scope: 'read write',
    is_active: true,
    created_at: '2026-01-01T00:00:00Z',
    updated_at: '2026-01-01T00:00:00Z',
    ...overrides,
  }
}

function mockCreatedClient(overrides: Partial<CreatedClient> = {}): CreatedClient {
  return {
    id: 1,
    client_id: 'client-abc',
    client_secret: 'secret-xyz',
    client_name: 'Test Client',
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

describe('oauthClients store', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
    vi.stubGlobal('fetch', vi.fn())
  })

  afterEach(() => {
    vi.restoreAllMocks()
    vi.unstubAllGlobals()
  })

  it('has correct initial state', () => {
    const store = useOAuthClientsStore()

    expect(store.clients).toEqual([])
    expect(store.total).toBe(0)
    expect(store.loading).toBe(false)
    expect(store.error).toBeNull()
  })

  describe('fetchClients', () => {
    it('calls correct URL with query params and sets clients/total', async () => {
      const clients = [mockClient(), mockClient({ id: 2, client_id: 'client-def' })]
      stubFetch({ data: clients, meta: { total: 2, limit: 10, offset: 0 } })

      const store = useOAuthClientsStore()
      await store.fetchClients({ limit: 10, offset: 0, q: 'test' })

      const calledUrl = (fetch as ReturnType<typeof vi.fn>).mock.calls[0]![0] as string
      expect(calledUrl).toContain('/api/admin/oauth-clients?')
      expect(calledUrl).toContain('limit=10')
      expect(calledUrl).toContain('offset=0')
      expect(calledUrl).toContain('q=test')
      expect(store.clients).toEqual(clients)
      expect(store.total).toBe(2)
    })

    it('calls URL without query when no params', async () => {
      stubFetch({ data: [], meta: { total: 0, limit: 10, offset: 0 } })

      const store = useOAuthClientsStore()
      await store.fetchClients()

      expect(fetch).toHaveBeenCalledWith('/api/admin/oauth-clients', undefined)
    })

    it('sets loading true during fetch', async () => {
      let resolvePromise: (value: unknown) => void
      vi.stubGlobal(
        'fetch',
        vi.fn().mockReturnValue(
          new Promise((resolve) => {
            resolvePromise = resolve
          }),
        ),
      )

      const store = useOAuthClientsStore()
      const promise = store.fetchClients()

      expect(store.loading).toBe(true)

      resolvePromise!({
        ok: true,
        json: () => Promise.resolve({ data: [], meta: { total: 0, limit: 10, offset: 0 } }),
      })
      await promise

      expect(store.loading).toBe(false)
    })

    it('sets error on failure', async () => {
      stubFetch({ message: 'Unauthorized' }, false)

      const store = useOAuthClientsStore()
      await expect(store.fetchClients()).rejects.toThrow('Unauthorized')

      expect(store.error).toBe('Unauthorized')
      expect(store.loading).toBe(false)
    })
  })

  describe('createClient', () => {
    it('POSTs payload and returns created client with secret', async () => {
      const created = mockCreatedClient()
      stubFetch({ data: created, message: 'Client created' })

      const store = useOAuthClientsStore()
      const request = {
        client_name: 'New Client',
        redirect_uris: ['https://example.com/cb'],
        grant_types: ['authorization_code'],
        token_endpoint_auth_method: 'client_secret_basic',
        scope: 'read',
      }
      const result = await store.createClient(request)

      expect(fetch).toHaveBeenCalledWith('/api/admin/oauth-clients', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify(request),
      })
      expect(result).toEqual(created)
    })

    it('sets error on failure', async () => {
      stubFetch({ message: 'Invalid redirect URI' }, false)

      const store = useOAuthClientsStore()
      await expect(
        store.createClient({
          client_name: 'Bad',
          redirect_uris: ['not-a-url'],
          grant_types: ['authorization_code'],
          token_endpoint_auth_method: 'client_secret_basic',
          scope: 'read',
        }),
      ).rejects.toThrow('Invalid redirect URI')

      expect(store.error).toBe('Invalid redirect URI')
    })
  })

  describe('revokeClient', () => {
    it('POSTs to revoke endpoint and marks client inactive locally', async () => {
      stubFetch({ message: 'Client revoked' })

      const store = useOAuthClientsStore()
      store.$patch({
        clients: [mockClient({ id: 1, is_active: true }), mockClient({ id: 2 })],
      })

      const result = await store.revokeClient(1)

      expect(fetch).toHaveBeenCalledWith('/api/admin/oauth-clients/1/revoke', {
        method: 'POST',
      })
      expect(result).toEqual({ message: 'Client revoked' })
      expect(store.clients[0]!.is_active).toBe(false)
    })

    it('does not modify array when id not found', async () => {
      stubFetch({ message: 'Client revoked' })

      const store = useOAuthClientsStore()
      store.$patch({ clients: [mockClient({ id: 1, is_active: true })] })

      await store.revokeClient(99)

      expect(store.clients[0]!.is_active).toBe(true)
    })

    it('sets error on failure', async () => {
      stubFetch({ message: 'Not found' }, false)

      const store = useOAuthClientsStore()
      await expect(store.revokeClient(999)).rejects.toThrow('Not found')

      expect(store.error).toBe('Not found')
    })
  })

  describe('error handling', () => {
    it('uses statusText when response body has no message', async () => {
      vi.stubGlobal(
        'fetch',
        vi.fn().mockResolvedValue({
          ok: false,
          status: 500,
          statusText: 'Internal Server Error',
          json: () => Promise.reject(new Error('parse error')),
        }),
      )

      const store = useOAuthClientsStore()
      await expect(store.fetchClients()).rejects.toThrow('Internal Server Error')

      expect(store.error).toBe('Internal Server Error')
    })

    it('clears error before each request', async () => {
      const store = useOAuthClientsStore()
      store.$patch({ error: 'previous error' })

      stubFetch({ data: [], meta: { total: 0, limit: 10, offset: 0 } })
      await store.fetchClients()

      expect(store.error).toBeNull()
    })
  })
})
