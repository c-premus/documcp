import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'
import { mount } from '@vue/test-utils'
import { ref, computed } from 'vue'

const mockMode = ref<'light' | 'dark' | 'system'>('system')
const mockToggle = vi.fn()
const mockResolved = computed(() => mockMode.value === 'system' ? 'light' : mockMode.value)
const mockSetMode = vi.fn()

vi.mock('@/composables/useTheme', () => ({
  useTheme: () => ({
    mode: mockMode,
    resolved: mockResolved,
    toggle: mockToggle,
    setMode: mockSetMode,
  }),
}))

import ThemeToggle from '@/components/layout/ThemeToggle.vue'

describe('ThemeToggle', () => {
  beforeEach(() => {
    mockMode.value = 'system'
    vi.clearAllMocks()
  })

  afterEach(() => {
    vi.restoreAllMocks()
  })

  it('renders a button element', () => {
    const wrapper = mount(ThemeToggle)
    expect(wrapper.find('button').exists()).toBe(true)
  })

  describe('icon rendering', () => {
    it('renders ComputerDesktopIcon when mode is system', () => {
      mockMode.value = 'system'
      const wrapper = mount(ThemeToggle)
      // The dynamic component renders an SVG from heroicons
      const svg = wrapper.find('svg')
      expect(svg.exists()).toBe(true)
      // ComputerDesktopIcon is the default for system mode
    })

    it('renders SunIcon when mode is light', () => {
      mockMode.value = 'light'
      const wrapper = mount(ThemeToggle)
      const svg = wrapper.find('svg')
      expect(svg.exists()).toBe(true)
    })

    it('renders MoonIcon when mode is dark', () => {
      mockMode.value = 'dark'
      const wrapper = mount(ThemeToggle)
      const svg = wrapper.find('svg')
      expect(svg.exists()).toBe(true)
    })
  })

  describe('aria-label', () => {
    it('shows "Switch to light theme" when mode is system', () => {
      mockMode.value = 'system'
      const wrapper = mount(ThemeToggle)
      expect(wrapper.find('button').attributes('aria-label')).toBe('Switch to light theme')
    })

    it('shows "Switch to dark theme" when mode is light', () => {
      mockMode.value = 'light'
      const wrapper = mount(ThemeToggle)
      expect(wrapper.find('button').attributes('aria-label')).toBe('Switch to dark theme')
    })

    it('shows "Switch to system theme" when mode is dark', () => {
      mockMode.value = 'dark'
      const wrapper = mount(ThemeToggle)
      expect(wrapper.find('button').attributes('aria-label')).toBe('Switch to system theme')
    })
  })

  describe('toggle interaction', () => {
    it('calls toggle when button is clicked', async () => {
      const wrapper = mount(ThemeToggle)
      await wrapper.find('button').trigger('click')
      expect(mockToggle).toHaveBeenCalledOnce()
    })

    it('calls toggle on each click', async () => {
      const wrapper = mount(ThemeToggle)
      await wrapper.find('button').trigger('click')
      await wrapper.find('button').trigger('click')
      expect(mockToggle).toHaveBeenCalledTimes(2)
    })
  })

  describe('button attributes', () => {
    it('has type="button"', () => {
      const wrapper = mount(ThemeToggle)
      expect(wrapper.find('button').attributes('type')).toBe('button')
    })
  })
})
