import type { Ref } from 'vue'

export async function withLoading<T>(
  loadingRef: Ref<boolean>,
  errorRef: Ref<string | null> | null,
  fn: () => Promise<T>,
  fallbackMessage: string,
): Promise<T> {
  loadingRef.value = true
  if (errorRef !== null) {
    errorRef.value = null
  }
  try {
    return await fn()
  } catch (e) {
    if (errorRef !== null) {
      errorRef.value = e instanceof Error ? e.message : fallbackMessage
    }
    throw e
  } finally {
    loadingRef.value = false
  }
}
