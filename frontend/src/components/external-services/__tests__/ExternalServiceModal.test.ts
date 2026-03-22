import { describe, it, expect, vi, beforeAll, afterEach } from 'vitest'
import { mount, flushPromises } from '@vue/test-utils'
import { createPinia, setActivePinia } from 'pinia'
import ExternalServiceModal from '@/components/external-services/ExternalServiceModal.vue'
import type { ExternalService } from '@/stores/externalServices'

// HeadlessUI Dialog requires ResizeObserver which jsdom doesn't provide
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

afterEach(() => {
  document.body.innerHTML = ''
})

const sampleService: ExternalService = {
  uuid: 'abc-123',
  name: 'My Kiwix',
  slug: 'my-kiwix',
  type: 'kiwix',
  base_url: 'https://kiwix.example.com',
  priority: 1,
  status: 'healthy',
  is_enabled: true,
  is_env_managed: false,
  error_count: 0,
  consecutive_failures: 0,
}

function mountModal(props: Record<string, unknown> = {}) {
  const pinia = createPinia()
  setActivePinia(pinia)

  return mount(ExternalServiceModal, {
    props: {
      open: true,
      service: null,
      ...props,
    },
    attachTo: document.body,
    global: {
      plugins: [pinia],
    },
  })
}

describe('ExternalServiceModal', () => {
  it('renders "Add Service" title when creating', async () => {
    mountModal({ open: true, service: null })
    await flushPromises()

    const body = document.body.textContent!
    expect(body).toContain('Add Service')
  })

  it('renders "Edit Service" title when editing', async () => {
    mountModal({ open: true, service: sampleService })
    await flushPromises()

    const body = document.body.textContent!
    expect(body).toContain('Edit Service')
  })

  it('shows form fields for name, type, base URL, API key, and priority', async () => {
    mountModal({ open: true })
    await flushPromises()

    const labels = document.body.textContent!
    expect(labels).toContain('Name')
    expect(labels).toContain('Type')
    expect(labels).toContain('Base URL')
    expect(labels).toContain('API Key')
    expect(labels).toContain('Priority')
  })

  it('shows "Create" submit label for new service', async () => {
    mountModal({ open: true, service: null })
    await flushPromises()

    const buttons = Array.from(document.querySelectorAll('button'))
    const createBtn = buttons.find((b) => b.textContent?.trim() === 'Create')
    expect(createBtn).toBeTruthy()
  })

  it('shows "Save" submit label for edit mode', async () => {
    mountModal({ open: true, service: sampleService })
    await flushPromises()

    const buttons = Array.from(document.querySelectorAll('button'))
    const saveBtn = buttons.find((b) => b.textContent?.trim() === 'Save')
    expect(saveBtn).toBeTruthy()
  })

  it('shows validation error when name is empty on submit', async () => {
    mountModal({ open: true, service: null })
    await flushPromises()

    const form = document.querySelector('form')!
    form.dispatchEvent(new Event('submit', { bubbles: true, cancelable: true }))
    await flushPromises()

    expect(document.body.textContent).toContain('Name is required')
  })

  it('shows validation error when base URL is empty on submit', async () => {
    mountModal({ open: true, service: null })
    await flushPromises()

    // Fill in name but leave URL empty
    const nameInput = document.querySelector<HTMLInputElement>('#service-name')!
    nameInput.value = 'Test Service'
    nameInput.dispatchEvent(new Event('input', { bubbles: true }))
    await flushPromises()

    const form = document.querySelector('form')!
    form.dispatchEvent(new Event('submit', { bubbles: true, cancelable: true }))
    await flushPromises()

    expect(document.body.textContent).toContain('Base URL is required')
  })

  it('emits close when cancel button is clicked', async () => {
    const wrapper = mountModal({ open: true })
    await flushPromises()

    const buttons = Array.from(document.querySelectorAll('button'))
    const cancelBtn = buttons.find((b) => b.textContent?.trim() === 'Cancel')!
    expect(cancelBtn).toBeTruthy()
    cancelBtn.click()
    await flushPromises()

    expect(wrapper.emitted('close')).toBeTruthy()
  })

  it('does not render dialog content when open is false', async () => {
    mountModal({ open: false })
    await flushPromises()

    const dialog = document.querySelector('[role="dialog"]')
    expect(dialog).toBeNull()
  })

  it('populates form fields when editing an existing service', async () => {
    // Mount closed first, then open -- the watch on `open` populates form fields
    const wrapper = mountModal({ open: false, service: sampleService })
    await flushPromises()

    await wrapper.setProps({ open: true })
    await flushPromises()

    const nameInput = document.querySelector<HTMLInputElement>('#service-name')!
    const typeSelect = document.querySelector<HTMLSelectElement>('#service-type')!
    const urlInput = document.querySelector<HTMLInputElement>('#service-base-url')!

    expect(nameInput.value).toBe('My Kiwix')
    expect(typeSelect.value).toBe('kiwix')
    expect(urlInput.value).toBe('https://kiwix.example.com')
  })

  it('shows "leave blank to keep current" hint for API key in edit mode', async () => {
    mountModal({ open: true, service: sampleService })
    await flushPromises()

    expect(document.body.textContent).toContain('leave blank to keep current')
  })
})
