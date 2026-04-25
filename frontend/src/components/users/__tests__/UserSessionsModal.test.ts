import { describe, it, expect, vi, beforeAll, beforeEach, afterEach } from 'vitest'
import { mount, flushPromises } from '@vue/test-utils'
import { createPinia, setActivePinia } from 'pinia'

import UserSessionsModal from '@/components/users/UserSessionsModal.vue'
import { useUsersStore, type User } from '@/stores/users'

// HeadlessUI Dialog requires ResizeObserver which jsdom doesn't provide.
beforeAll(() => {
  vi.stubGlobal(
    'ResizeObserver',
    class ResizeObserver {
      observe() {}
      unobserve() {}
      disconnect() {}
    },
  )
})

const USER: User = {
  id: 7,
  name: 'Carol',
  email: 'carol@example.com',
  oidc_sub: 'sub-7',
  oidc_provider: 'https://idp.example.com',
  is_admin: false,
  created_at: '2026-01-01T00:00:00Z',
  updated_at: '2026-01-01T00:00:00Z',
}

function stubFetch(response: unknown, ok = true) {
  vi.stubGlobal(
    'fetch',
    vi.fn().mockResolvedValue({
      ok,
      status: ok ? 200 : 500,
      statusText: ok ? 'OK' : 'Internal Server Error',
      json: () => Promise.resolve(response),
    }),
  )
}

describe('UserSessionsModal', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
  })

  afterEach(() => {
    vi.restoreAllMocks()
    // Clean up portaled content from document.body
    document.body.innerHTML = ''
  })

  it('fetches sessions when opened and renders one row per session ID', async () => {
    stubFetch({ data: ['session-aaaa-bbbb', 'session-cccc-dddd'] })

    const wrapper = mount(UserSessionsModal, {
      props: { open: true, user: USER },
      attachTo: document.body,
    })

    await flushPromises()

    const calledUrl = (fetch as ReturnType<typeof vi.fn>).mock.calls[0]![0] as string
    expect(calledUrl).toBe('/api/admin/users/7/sessions')

    expect(document.querySelectorAll('[aria-label^="Session "]').length).toBe(2)

    wrapper.unmount()
  })

  it('renders an empty-state message when there are no sessions', async () => {
    stubFetch({ data: [] })

    const wrapper = mount(UserSessionsModal, {
      props: { open: true, user: USER },
      attachTo: document.body,
    })
    await flushPromises()

    const text = document.body.textContent ?? ''
    expect(text).toContain('No active sessions')

    wrapper.unmount()
  })

  it('revoke-one DELETEs and removes the session from the list', async () => {
    const fetchMock = vi
      .fn()
      .mockResolvedValueOnce({
        ok: true,
        status: 200,
        statusText: 'OK',
        json: () => Promise.resolve({ data: ['s1'] }),
      })
      .mockResolvedValueOnce({
        ok: true,
        status: 200,
        statusText: 'OK',
        json: () => Promise.resolve({ message: 'session revoked' }),
      })
    vi.stubGlobal('fetch', fetchMock)

    const wrapper = mount(UserSessionsModal, {
      props: { open: true, user: USER },
      attachTo: document.body,
    })
    await flushPromises()

    const revokeBtn = document.querySelector(
      '[aria-label="Revoke session s1"]',
    ) as HTMLButtonElement
    expect(revokeBtn).not.toBeNull()
    revokeBtn.click()
    await flushPromises()

    const url = fetchMock.mock.calls[1]![0] as string
    const opts = fetchMock.mock.calls[1]![1] as RequestInit
    expect(url).toBe('/api/admin/users/7/sessions/s1')
    expect(opts.method).toBe('DELETE')

    const store = useUsersStore()
    expect(store.sessionIDs).toEqual([])

    wrapper.unmount()
  })

  it('revoke-all DELETEs the collection and clears state', async () => {
    const fetchMock = vi
      .fn()
      .mockResolvedValueOnce({
        ok: true,
        status: 200,
        statusText: 'OK',
        json: () => Promise.resolve({ data: ['s1', 's2'] }),
      })
      .mockResolvedValueOnce({
        ok: true,
        status: 200,
        statusText: 'OK',
        json: () => Promise.resolve({ data: { revoked: 2 } }),
      })
    vi.stubGlobal('fetch', fetchMock)

    const wrapper = mount(UserSessionsModal, {
      props: { open: true, user: USER },
      attachTo: document.body,
    })
    await flushPromises()

    const allButtons = Array.from(document.querySelectorAll('button')) as HTMLButtonElement[]
    const revokeAllBtn = allButtons.find((b) => b.textContent?.trim() === 'Revoke all')
    expect(revokeAllBtn).toBeDefined()
    revokeAllBtn!.click()
    await flushPromises()

    const url = fetchMock.mock.calls[1]![0] as string
    const opts = fetchMock.mock.calls[1]![1] as RequestInit
    expect(url).toBe('/api/admin/users/7/sessions')
    expect(opts.method).toBe('DELETE')

    const store = useUsersStore()
    expect(store.sessionIDs).toEqual([])

    wrapper.unmount()
  })

  it('emits close when the close icon is clicked', async () => {
    stubFetch({ data: [] })

    const wrapper = mount(UserSessionsModal, {
      props: { open: true, user: USER },
      attachTo: document.body,
    })
    await flushPromises()

    const closeBtn = document.querySelector('[aria-label="Close"]') as HTMLButtonElement
    closeBtn.click()

    expect(wrapper.emitted('close')).toHaveLength(1)
    wrapper.unmount()
  })
})
