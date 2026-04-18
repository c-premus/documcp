import { describe, it, expect } from 'vitest'
import { ref } from 'vue'
import { withLoading } from '@/composables/useAsyncAction'

describe('withLoading', () => {
  it('flips the loading ref around a successful call and returns the value', async () => {
    const loading = ref(false)
    const error = ref<string | null>('stale')

    const result = await withLoading(loading, error, async () => 42, 'failed')

    expect(result).toBe(42)
    expect(loading.value).toBe(false)
    expect(error.value).toBeNull()
  })

  it('sets loading=true during the call and clears error before it runs', async () => {
    const loading = ref(false)
    const error = ref<string | null>('stale')
    const states: Array<{ loading: boolean; error: string | null }> = []

    await withLoading(
      loading,
      error,
      async () => {
        states.push({ loading: loading.value, error: error.value })
        return 'ok'
      },
      'failed',
    )

    expect(states).toEqual([{ loading: true, error: null }])
  })

  it('records Error.message on error and rethrows', async () => {
    const loading = ref(false)
    const error = ref<string | null>(null)

    await expect(
      withLoading(
        loading,
        error,
        async () => {
          throw new Error('boom')
        },
        'fallback',
      ),
    ).rejects.toThrow('boom')

    expect(loading.value).toBe(false)
    expect(error.value).toBe('boom')
  })

  it('falls back to the provided message when a non-Error is thrown', async () => {
    const loading = ref(false)
    const error = ref<string | null>(null)

    await expect(
      withLoading(
        loading,
        error,
        async () => {
          throw 'string-thrown'
        },
        'fallback message',
      ),
    ).rejects.toBe('string-thrown')

    expect(error.value).toBe('fallback message')
  })

  it('resets loading to false even if the call rejects', async () => {
    const loading = ref(false)

    await expect(
      withLoading(
        loading,
        null,
        async () => {
          throw new Error('oops')
        },
        'fallback',
      ),
    ).rejects.toThrow('oops')

    expect(loading.value).toBe(false)
  })

  it('tolerates a null errorRef (for stores without an error ref)', async () => {
    const loading = ref(false)

    const result = await withLoading(loading, null, async () => 'ok', 'fallback')
    expect(result).toBe('ok')
    expect(loading.value).toBe(false)
  })
})
