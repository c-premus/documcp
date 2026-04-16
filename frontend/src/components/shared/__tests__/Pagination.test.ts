import { describe, it, expect } from 'vitest'
import { mount } from '@vue/test-utils'
import Pagination from '@/components/shared/Pagination.vue'

function mountPagination(props: { page: number; perPage: number; total: number }) {
  return mount(Pagination, { props })
}

describe('Pagination', () => {
  it('shows correct "Showing X to Y of Z" text', () => {
    const wrapper = mountPagination({ page: 2, perPage: 10, total: 25 })

    expect(wrapper.text()).toContain('Showing')
    expect(wrapper.text()).toContain('11')
    expect(wrapper.text()).toContain('20')
    expect(wrapper.text()).toContain('25')
  })

  it('shows 0 to 0 of 0 when total is zero', () => {
    const wrapper = mountPagination({ page: 1, perPage: 10, total: 0 })

    expect(wrapper.text()).toContain('0')
  })

  it('shows correct range on last partial page', () => {
    const wrapper = mountPagination({ page: 3, perPage: 10, total: 25 })

    expect(wrapper.text()).toContain('21')
    expect(wrapper.text()).toContain('25')
  })

  it('disables Previous button on first page', () => {
    const wrapper = mountPagination({ page: 1, perPage: 10, total: 50 })

    const prevButton = wrapper.findAll('button').find((b) => b.text() === 'Previous')!
    expect(prevButton.attributes('disabled')).toBeDefined()
  })

  it('enables Previous button on non-first page', () => {
    const wrapper = mountPagination({ page: 2, perPage: 10, total: 50 })

    const prevButton = wrapper.findAll('button').find((b) => b.text() === 'Previous')!
    expect(prevButton.attributes('disabled')).toBeUndefined()
  })

  it('disables Next button on last page', () => {
    const wrapper = mountPagination({ page: 5, perPage: 10, total: 50 })

    const nextButton = wrapper.findAll('button').find((b) => b.text() === 'Next')!
    expect(nextButton.attributes('disabled')).toBeDefined()
  })

  it('enables Next button on non-last page', () => {
    const wrapper = mountPagination({ page: 1, perPage: 10, total: 50 })

    const nextButton = wrapper.findAll('button').find((b) => b.text() === 'Next')!
    expect(nextButton.attributes('disabled')).toBeUndefined()
  })

  it('emits update:page with next page on Next click', async () => {
    const wrapper = mountPagination({ page: 1, perPage: 10, total: 50 })

    const nextButton = wrapper.findAll('button').find((b) => b.text() === 'Next')!
    await nextButton.trigger('click')

    expect(wrapper.emitted('update:page')).toBeTruthy()
    expect(wrapper.emitted('update:page')![0]).toEqual([2])
  })

  it('emits update:page with previous page on Previous click', async () => {
    const wrapper = mountPagination({ page: 3, perPage: 10, total: 50 })

    const prevButton = wrapper.findAll('button').find((b) => b.text() === 'Previous')!
    await prevButton.trigger('click')

    expect(wrapper.emitted('update:page')).toBeTruthy()
    expect(wrapper.emitted('update:page')![0]).toEqual([2])
  })

  it('does not emit update:page when clicking disabled Previous', async () => {
    const wrapper = mountPagination({ page: 1, perPage: 10, total: 50 })

    const prevButton = wrapper.findAll('button').find((b) => b.text() === 'Previous')!
    await prevButton.trigger('click')

    expect(wrapper.emitted('update:page')).toBeFalsy()
  })

  it('emits update:perPage when changing page size', async () => {
    const wrapper = mountPagination({ page: 2, perPage: 10, total: 50 })

    const select = wrapper.find('select')
    await select.setValue('25')

    expect(wrapper.emitted('update:perPage')).toBeTruthy()
    expect(wrapper.emitted('update:perPage')![0]).toEqual([25])
  })

  it('resets to page 1 when changing page size', async () => {
    const wrapper = mountPagination({ page: 2, perPage: 10, total: 50 })

    const select = wrapper.find('select')
    await select.setValue('25')

    // Should emit update:page with 1 to reset
    const pageEmits = wrapper.emitted('update:page')
    expect(pageEmits).toBeTruthy()
    expect(pageEmits![0]).toEqual([1])
  })

  it('renders 20 as a selectable page size', () => {
    const wrapper = mountPagination({ page: 1, perPage: 20, total: 100 })

    const values = wrapper.findAll('option').map((o) => o.attributes('value'))
    expect(values).toContain('20')
  })

  it('selects the current perPage value in the dropdown', () => {
    const wrapper = mountPagination({ page: 1, perPage: 20, total: 100 })

    const select = wrapper.find('select').element as HTMLSelectElement
    expect(select.value).toBe('20')
  })

  it('keeps the dropdown selected when perPage is an off-list value', () => {
    // 17 isn't in PAGE_SIZE_OPTIONS — the select must still reflect it,
    // not render blank (regression guard for the OAuth clients view).
    const wrapper = mountPagination({ page: 1, perPage: 17, total: 100 })

    const select = wrapper.find('select').element as HTMLSelectElement
    expect(select.value).toBe('17')

    const values = wrapper.findAll('option').map((o) => o.attributes('value'))
    expect(values).toContain('17')
  })

  it('keeps options sorted when a custom perPage is injected', () => {
    const wrapper = mountPagination({ page: 1, perPage: 17, total: 100 })

    const values = wrapper.findAll('option').map((o) => Number(o.attributes('value')))
    const sorted = [...values].sort((a, b) => a - b)
    expect(values).toEqual(sorted)
  })
})
