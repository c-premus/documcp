import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'

describe('useTheme', () => {
  let storageMap: Record<string, string>

  beforeEach(() => {
    // Reset module state by clearing the module cache before each test
    vi.resetModules()

    // Fresh localStorage mock
    storageMap = {}
    vi.stubGlobal('localStorage', {
      getItem: vi.fn((key: string) => storageMap[key] ?? null),
      setItem: vi.fn((key: string, value: string) => {
        storageMap[key] = value
      }),
      removeItem: vi.fn((key: string) => {
        delete storageMap[key]
      }),
    })

    // Default matchMedia stub: system prefers light
    vi.stubGlobal('matchMedia', vi.fn().mockReturnValue({
      matches: false,
      addEventListener: vi.fn(),
      removeEventListener: vi.fn(),
    }))

    // Ensure document.documentElement.classList is available
    document.documentElement.classList.remove('dark')
  })

  afterEach(() => {
    vi.unstubAllGlobals()
    document.documentElement.classList.remove('dark')
  })

  async function loadTheme() {
    const mod = await import('@/composables/useTheme')
    return mod.useTheme()
  }

  describe('readStoredMode', () => {
    it('defaults to system when localStorage has no theme value', async () => {
      const { mode } = await loadTheme()
      expect(mode.value).toBe('system')
    })

    it('reads light from localStorage', async () => {
      storageMap['theme'] = 'light'
      const { mode } = await loadTheme()
      expect(mode.value).toBe('light')
    })

    it('reads dark from localStorage', async () => {
      storageMap['theme'] = 'dark'
      const { mode } = await loadTheme()
      expect(mode.value).toBe('dark')
    })

    it('reads system from localStorage', async () => {
      storageMap['theme'] = 'system'
      const { mode } = await loadTheme()
      expect(mode.value).toBe('system')
    })

    it('defaults to system for invalid localStorage value', async () => {
      storageMap['theme'] = 'invalid-value'
      const { mode } = await loadTheme()
      expect(mode.value).toBe('system')
    })
  })

  describe('setMode', () => {
    it('updates mode ref to light', async () => {
      const { mode, setMode } = await loadTheme()
      setMode('light')
      expect(mode.value).toBe('light')
    })

    it('updates mode ref to dark', async () => {
      const { mode, setMode } = await loadTheme()
      setMode('dark')
      expect(mode.value).toBe('dark')
    })

    it('persists value to localStorage', async () => {
      const { setMode } = await loadTheme()
      setMode('dark')
      expect(localStorage.setItem).toHaveBeenCalledWith('theme', 'dark')
    })

    it('updates localStorage on each call', async () => {
      const { setMode } = await loadTheme()
      setMode('light')
      setMode('dark')
      expect(localStorage.setItem).toHaveBeenCalledWith('theme', 'light')
      expect(localStorage.setItem).toHaveBeenCalledWith('theme', 'dark')
    })
  })

  describe('toggle', () => {
    it('cycles from light to dark', async () => {
      storageMap['theme'] = 'light'
      const { mode, toggle } = await loadTheme()
      toggle()
      expect(mode.value).toBe('dark')
    })

    it('cycles from dark to system', async () => {
      storageMap['theme'] = 'dark'
      const { mode, toggle } = await loadTheme()
      toggle()
      expect(mode.value).toBe('system')
    })

    it('cycles from system to light', async () => {
      storageMap['theme'] = 'system'
      const { mode, toggle } = await loadTheme()
      toggle()
      expect(mode.value).toBe('light')
    })

    it('completes full cycle light -> dark -> system -> light', async () => {
      storageMap['theme'] = 'light'
      const { mode, toggle } = await loadTheme()

      toggle()
      expect(mode.value).toBe('dark')

      toggle()
      expect(mode.value).toBe('system')

      toggle()
      expect(mode.value).toBe('light')
    })
  })

  describe('resolved', () => {
    it('returns light when mode is light', async () => {
      storageMap['theme'] = 'light'
      const { resolved } = await loadTheme()
      expect(resolved.value).toBe('light')
    })

    it('returns dark when mode is dark', async () => {
      storageMap['theme'] = 'dark'
      const { resolved } = await loadTheme()
      expect(resolved.value).toBe('dark')
    })

    it('returns light when mode is system and system prefers light', async () => {
      vi.stubGlobal('matchMedia', vi.fn().mockReturnValue({
        matches: false,
        addEventListener: vi.fn(),
        removeEventListener: vi.fn(),
      }))

      storageMap['theme'] = 'system'
      const { resolved } = await loadTheme()
      expect(resolved.value).toBe('light')
    })

    it('returns dark when mode is system and system prefers dark', async () => {
      vi.stubGlobal('matchMedia', vi.fn().mockReturnValue({
        matches: true,
        addEventListener: vi.fn(),
        removeEventListener: vi.fn(),
      }))

      storageMap['theme'] = 'system'
      const { resolved } = await loadTheme()
      expect(resolved.value).toBe('dark')
    })

    it('updates when mode changes via setMode', async () => {
      storageMap['theme'] = 'light'
      const { resolved, setMode } = await loadTheme()
      expect(resolved.value).toBe('light')

      setMode('dark')
      expect(resolved.value).toBe('dark')
    })
  })

  describe('applyTheme (dark class on documentElement)', () => {
    it('adds dark class when resolved is dark', async () => {
      storageMap['theme'] = 'dark'
      await loadTheme()
      // The watch with immediate: true applies the theme on init
      expect(document.documentElement.classList.contains('dark')).toBe(true)
    })

    it('does not add dark class when resolved is light', async () => {
      storageMap['theme'] = 'light'
      await loadTheme()
      expect(document.documentElement.classList.contains('dark')).toBe(false)
    })

    it('removes dark class when switching from dark to light', async () => {
      storageMap['theme'] = 'dark'
      const { setMode } = await loadTheme()
      expect(document.documentElement.classList.contains('dark')).toBe(true)

      setMode('light')
      // Vue watch is synchronous in test environment for immediate watchers
      // but reactive updates may need a tick
      await new Promise((r) => setTimeout(r, 0))
      expect(document.documentElement.classList.contains('dark')).toBe(false)
    })

    it('adds dark class when switching from light to dark', async () => {
      storageMap['theme'] = 'light'
      const { setMode } = await loadTheme()
      expect(document.documentElement.classList.contains('dark')).toBe(false)

      setMode('dark')
      await new Promise((r) => setTimeout(r, 0))
      expect(document.documentElement.classList.contains('dark')).toBe(true)
    })

    it('applies dark class when system mode resolves to dark', async () => {
      vi.stubGlobal('matchMedia', vi.fn().mockReturnValue({
        matches: true,
        addEventListener: vi.fn(),
        removeEventListener: vi.fn(),
      }))

      storageMap['theme'] = 'system'
      await loadTheme()
      expect(document.documentElement.classList.contains('dark')).toBe(true)
    })
  })

  describe('matchMedia listener', () => {
    it('registers a change listener on matchMedia', async () => {
      const addEventListenerSpy = vi.fn()
      vi.stubGlobal('matchMedia', vi.fn().mockReturnValue({
        matches: false,
        addEventListener: addEventListenerSpy,
        removeEventListener: vi.fn(),
      }))

      await loadTheme()
      expect(addEventListenerSpy).toHaveBeenCalledWith('change', expect.any(Function))
    })
  })
})
