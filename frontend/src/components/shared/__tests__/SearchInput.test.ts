import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'
import { mount } from '@vue/test-utils'
import SearchInput from '@/components/shared/SearchInput.vue'

describe('SearchInput', () => {
  beforeEach(() => {
    vi.useFakeTimers()
  })

  afterEach(() => {
    vi.useRealTimers()
  })

  function mountInput(props = {}) {
    return mount(SearchInput, {
      props: { modelValue: '', ...props },
      global: {
        stubs: {
          MagnifyingGlassIcon: true,
          XMarkIcon: true,
        },
      },
    })
  }

  it('renders with default placeholder', () => {
    const wrapper = mountInput()
    expect(wrapper.find('input').attributes('placeholder')).toBe('Search...')
  })

  it('renders with custom placeholder', () => {
    const wrapper = mountInput({ placeholder: 'Find documents...' })
    expect(wrapper.find('input').attributes('placeholder')).toBe('Find documents...')
  })

  it('displays the modelValue', () => {
    const wrapper = mountInput({ modelValue: 'hello' })
    expect(wrapper.find('input').element.value).toBe('hello')
  })

  it('emits update:modelValue after debounce on input', async () => {
    const wrapper = mountInput({ debounceMs: 200 })

    await wrapper.find('input').setValue('test')
    await wrapper.find('input').trigger('input')

    expect(wrapper.emitted('update:modelValue')).toBeUndefined()

    vi.advanceTimersByTime(200)

    expect(wrapper.emitted('update:modelValue')).toBeTruthy()
  })

  it('shows clear button when input has value', async () => {
    const wrapper = mountInput({ modelValue: 'hello' })
    expect(wrapper.find('button[aria-label="Clear search"]').exists()).toBe(true)
  })

  it('hides clear button when input is empty', () => {
    const wrapper = mountInput({ modelValue: '' })
    expect(wrapper.find('button[aria-label="Clear search"]').exists()).toBe(false)
  })

  it('emits empty string immediately on clear', async () => {
    const wrapper = mountInput({ modelValue: 'hello' })

    await wrapper.find('button[aria-label="Clear search"]').trigger('click')

    expect(wrapper.emitted('update:modelValue')?.[0]).toEqual([''])
  })
})
