/**
 * ApiError carries the HTTP status code alongside the error message,
 * allowing callers to distinguish 403 (insufficient scope) from other errors.
 */
export class ApiError extends Error {
  readonly status: number

  constructor(status: number, message: string) {
    super(message)
    this.name = 'ApiError'
    this.status = status
  }
}

/**
 * Shared fetch wrapper used by all Pinia stores. Throws {@link ApiError}
 * on non-2xx responses so callers can inspect `error.status`.
 *
 * On 401, redirects to the OIDC login page before throwing — handles
 * mid-session token/session expiry for any store call. The auth store's
 * `fetchUser` probe uses raw `fetch` directly (a 401 there is expected
 * and means "not yet authenticated", handled by the router guard).
 */
export async function apiFetch<T>(url: string, options?: RequestInit): Promise<T> {
  const res = await fetch(url, options)
  if (!res.ok) {
    if (res.status === 401) {
      window.location.href = '/auth/login?redirect=' + encodeURIComponent(window.location.pathname)
    }
    const body = await res.json().catch(() => ({ message: res.statusText }))
    const msg = (body?.message || res.statusText).slice(0, 200)
    throw new ApiError(res.status, msg)
  }
  if (res.status === 204) return undefined as T
  return res.json() as Promise<T>
}

/**
 * Build a query string from an object, omitting undefined/empty values.
 */
export function buildQuery(params: Record<string, string | number | undefined>): string {
  const search = new URLSearchParams()
  for (const [key, value] of Object.entries(params)) {
    if (value !== undefined && value !== '') {
      search.set(key, String(value))
    }
  }
  const qs = search.toString()
  return qs ? `?${qs}` : ''
}
