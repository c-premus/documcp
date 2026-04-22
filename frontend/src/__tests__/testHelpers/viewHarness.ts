import { afterEach, beforeEach, vi } from 'vitest'
import { createPinia, setActivePinia } from 'pinia'
import { useAuthStore, type User } from '@/stores/auth'

// Router-stub canonical pattern for view tests. Navigation-chrome components
// (AppSidebar) use the real `createRouter` to exercise RouterLink resolution;
// view tests only need `useRouter().push` to be spyable, so they mock the
// module at the top of each test file:
//
//   vi.mock('vue-router', () => ({
//     useRouter: () => ({ push: vi.fn() }),
//     RouterLink: { template: '<a><slot/></a>' },
//   }))
//
// This must stay in the test file (vi.mock is hoisted before imports).

export function stubFetch(response: unknown, ok = true): void {
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

// Registers the shared before/afterEach hooks for a view test suite.
// Call once at the top of a `describe` block.
// `defaultFetch`: seeds fetch with this response body before every test.
// Omit for suites that stub fetch per-test.
export function setupViewTest(options: { defaultFetch?: unknown } = {}): void {
  beforeEach(() => {
    setActivePinia(createPinia())
    if (options.defaultFetch !== undefined) {
      stubFetch(options.defaultFetch)
    } else {
      vi.stubGlobal('fetch', vi.fn())
    }
  })
  afterEach(() => {
    vi.restoreAllMocks()
    vi.unstubAllGlobals()
  })
}

export function setAdmin(overrides: Partial<User> = {}): User {
  const auth = useAuthStore()
  const user: User = {
    id: 1,
    email: 'admin@test.com',
    name: 'Admin',
    is_admin: true,
    ...overrides,
  }
  auth.user = user
  return user
}

export function setNonAdmin(overrides: Partial<User> = {}): User {
  const auth = useAuthStore()
  const user: User = {
    id: 2,
    email: 'user@test.com',
    name: 'Regular User',
    is_admin: false,
    ...overrides,
  }
  auth.user = user
  return user
}
