import { describe, it, expect, vi, beforeAll, afterEach } from 'vitest'
import { mount, flushPromises } from '@vue/test-utils'
import ConfirmDialog from '@/components/shared/ConfirmDialog.vue'

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
  // Clean up portaled content from document.body
  document.body.innerHTML = ''
})

function mountDialog(props: Record<string, unknown> = {}) {
  return mount(ConfirmDialog, {
    props: {
      open: true,
      title: 'Delete Item',
      message: 'Are you sure you want to delete this?',
      ...props,
    },
    attachTo: document.body,
  })
}

describe('ConfirmDialog', () => {
  it('does not render dialog content when open is false', async () => {
    mountDialog({ open: false })
    await flushPromises()

    // HeadlessUI Dialog does not render panel content when closed
    const dialog = document.querySelector('[role="dialog"]')
    expect(dialog).toBeNull()
  })

  it('shows title and message when open is true', async () => {
    mountDialog({ open: true })
    await flushPromises()

    const body = document.body.textContent!
    expect(body).toContain('Delete Item')
    expect(body).toContain('Are you sure you want to delete this?')
  })

  it('emits confirm on confirm button click', async () => {
    const wrapper = mountDialog()
    await flushPromises()

    const buttons = Array.from(document.querySelectorAll('button'))
    const confirmBtn = buttons.find((b) => b.textContent?.trim() === 'Confirm')!
    expect(confirmBtn).toBeTruthy()
    confirmBtn.click()
    await flushPromises()

    expect(wrapper.emitted('confirm')).toBeTruthy()
  })

  it('emits cancel on cancel button click', async () => {
    const wrapper = mountDialog()
    await flushPromises()

    const buttons = Array.from(document.querySelectorAll('button'))
    const cancelBtn = buttons.find((b) => b.textContent?.trim() === 'Cancel')!
    expect(cancelBtn).toBeTruthy()
    cancelBtn.click()
    await flushPromises()

    expect(wrapper.emitted('cancel')).toBeTruthy()
  })

  it('uses custom confirm label', async () => {
    mountDialog({ confirmLabel: 'Yes, delete' })
    await flushPromises()

    const buttons = Array.from(document.querySelectorAll('button'))
    const confirmBtn = buttons.find((b) => b.textContent?.trim() === 'Yes, delete')
    expect(confirmBtn).toBeTruthy()
  })

  it('shows danger styling by default (red confirm button)', async () => {
    mountDialog()
    await flushPromises()

    const buttons = Array.from(document.querySelectorAll('button'))
    const confirmBtn = buttons.find((b) => b.textContent?.trim() === 'Confirm')!
    expect(confirmBtn).toBeTruthy()
    expect(confirmBtn.classList.contains('bg-red-600')).toBe(true)
  })

  it('shows warning styling for warning variant', async () => {
    mountDialog({ variant: 'warning' })
    await flushPromises()

    const buttons = Array.from(document.querySelectorAll('button'))
    const confirmBtn = buttons.find((b) => b.textContent?.trim() === 'Confirm')!
    expect(confirmBtn).toBeTruthy()
    expect(confirmBtn.classList.contains('bg-yellow-600')).toBe(true)
  })

  it('shows red icon background for danger variant', async () => {
    mountDialog({ variant: 'danger' })
    await flushPromises()

    const redBg = document.querySelector('.bg-red-100')
    expect(redBg).toBeTruthy()
  })

  it('shows yellow icon background for warning variant', async () => {
    mountDialog({ variant: 'warning' })
    await flushPromises()

    const yellowBg = document.querySelector('.bg-yellow-100')
    expect(yellowBg).toBeTruthy()
  })
})
