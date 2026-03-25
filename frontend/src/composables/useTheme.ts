import { ref, computed, watch } from 'vue'

type ThemeMode = 'light' | 'dark' | 'system'
type ResolvedTheme = 'light' | 'dark'

const STORAGE_KEY = 'theme'

const mode = ref<ThemeMode>(readStoredMode())
const systemDark = ref(
  typeof window !== 'undefined' ? window.matchMedia('(prefers-color-scheme: dark)').matches : false,
)

const resolved = computed<ResolvedTheme>(() => {
  if (mode.value === 'system') {
    return systemDark.value ? 'dark' : 'light'
  }
  return mode.value
})

function readStoredMode(): ThemeMode {
  if (typeof window === 'undefined') {
    return 'system'
  }
  const stored = localStorage.getItem(STORAGE_KEY)
  if (stored === 'light' || stored === 'dark' || stored === 'system') {
    return stored
  }
  return 'system'
}

function applyTheme(theme: ResolvedTheme): void {
  if (typeof document === 'undefined') {
    return
  }
  if (theme === 'dark') {
    document.documentElement.classList.add('dark')
  } else {
    document.documentElement.classList.remove('dark')
  }
}

function setMode(m: ThemeMode): void {
  mode.value = m
  localStorage.setItem(STORAGE_KEY, m)
}

function toggle(): void {
  const order: ThemeMode[] = ['light', 'dark', 'system']
  const idx = order.indexOf(mode.value)
  setMode(order[(idx + 1) % order.length]!)
}

let initialized = false

export function useTheme() {
  if (!initialized && typeof window !== 'undefined') {
    initialized = true
    const mql = window.matchMedia('(prefers-color-scheme: dark)')
    mql.addEventListener('change', (e) => {
      systemDark.value = e.matches
    })
  }

  watch(
    resolved,
    (theme) => {
      applyTheme(theme)
    },
    { immediate: true },
  )

  return {
    mode,
    resolved,
    setMode,
    toggle,
  }
}
