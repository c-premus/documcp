import { describe, it, expect, vi, afterEach } from 'vitest'
import { apiFetch, ApiError } from '@/api/helpers'

interface StubOptions {
  ok?: boolean
  status?: number
  statusText?: string
  headers?: Record<string, string>
  json?: unknown
  jsonThrows?: boolean
}

function stubFetch(opts: StubOptions) {
  const headers = opts.headers ?? {}
  vi.stubGlobal(
    'fetch',
    vi.fn().mockResolvedValue({
      ok: opts.ok ?? false,
      status: opts.status ?? 500,
      statusText: opts.statusText ?? '',
      headers: { get: (name: string) => headers[name] ?? null },
      json: () =>
        opts.jsonThrows ? Promise.reject(new Error('not json')) : Promise.resolve(opts.json),
    }),
  )
}

afterEach(() => {
  vi.unstubAllGlobals()
})

describe('apiFetch 429 handling', () => {
  it('surfaces the Retry-After wait time in the message', async () => {
    // httprate returns a plain-text body and Retry-After = window seconds.
    stubFetch({
      status: 429,
      statusText: 'Too Many Requests',
      headers: { 'Retry-After': '60' },
      jsonThrows: true,
    })

    const err = await apiFetch('/api/documents/x', { method: 'DELETE' }).catch((e) => e)

    expect(err).toBeInstanceOf(ApiError)
    expect(err.status).toBe(429)
    expect(err.message).toBe(
      "You're doing that too quickly. Please wait up to 60 seconds and try again.",
    )
    // Never leaks the opaque status text.
    expect(err.message).not.toContain('Too Many Requests')
  })

  it('uses the singular unit when Retry-After is 1', async () => {
    stubFetch({ status: 429, headers: { 'Retry-After': '1' }, jsonThrows: true })

    const err = await apiFetch('/api/documents/x', { method: 'DELETE' }).catch((e) => e)

    expect(err.message).toBe(
      "You're doing that too quickly. Please wait up to 1 second and try again.",
    )
  })

  it('falls back to a generic message when Retry-After is missing or invalid', async () => {
    stubFetch({ status: 429, headers: {}, jsonThrows: true })

    const err = await apiFetch('/api/documents/x', { method: 'DELETE' }).catch((e) => e)

    expect(err.message).toBe("You're doing that too quickly. Please wait a moment and try again.")
  })

  it('ignores a non-positive or non-integer Retry-After', async () => {
    stubFetch({
      status: 429,
      headers: { 'Retry-After': 'Wed, 21 Oct 2026 07:28:00 GMT' },
      jsonThrows: true,
    })

    const err = await apiFetch('/api/documents/x', { method: 'DELETE' }).catch((e) => e)

    expect(err.message).toBe("You're doing that too quickly. Please wait a moment and try again.")
  })
})

describe('apiFetch baseline behavior', () => {
  it('propagates the server message on other error statuses', async () => {
    stubFetch({ status: 404, json: { message: 'document not found' } })

    const err = await apiFetch('/api/documents/x').catch((e) => e)

    expect(err).toBeInstanceOf(ApiError)
    expect(err.status).toBe(404)
    expect(err.message).toBe('document not found')
  })

  it('returns undefined on 204 without parsing a body', async () => {
    stubFetch({ ok: true, status: 204 })

    await expect(apiFetch('/api/documents/x', { method: 'DELETE' })).resolves.toBeUndefined()
  })
})
