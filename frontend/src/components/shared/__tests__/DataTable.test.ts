import { describe, it, expect } from 'vitest'
import { mount, flushPromises } from '@vue/test-utils'
import { h, nextTick } from 'vue'
import DataTable from '@/components/shared/DataTable.vue'
import type { ColumnDef } from '@tanstack/vue-table'

interface TestRow {
  id: number
  name: string
  email: string
}

const columns: ColumnDef<TestRow, unknown>[] = [
  { accessorKey: 'id', header: 'ID' },
  { accessorKey: 'name', header: 'Name' },
  { accessorKey: 'email', header: 'Email' },
]

const sampleData: TestRow[] = [
  { id: 1, name: 'Alice', email: 'alice@example.com' },
  { id: 2, name: 'Bob', email: 'bob@example.com' },
]

function mountTable(
  props: Record<string, unknown> = {},
  slots: Record<string, () => unknown> = {},
) {
  return mount(DataTable as ReturnType<(typeof import('vue'))['defineComponent']>, {
    props: { data: sampleData, columns, ...props },
    slots,
  })
}

describe('DataTable', () => {
  it('renders column headers from column definitions', () => {
    const wrapper = mountTable()

    const headers = wrapper.findAll('th')
    expect(headers).toHaveLength(3)
    expect(headers[0]!.text()).toContain('ID')
    expect(headers[1]!.text()).toContain('Name')
    expect(headers[2]!.text()).toContain('Email')
  })

  it('renders data rows', () => {
    const wrapper = mountTable()

    const rows = wrapper.findAll('tbody tr')
    expect(rows).toHaveLength(2)
    expect(rows[0]!.text()).toContain('Alice')
    expect(rows[0]!.text()).toContain('alice@example.com')
    expect(rows[1]!.text()).toContain('Bob')
  })

  it('shows loading spinner when loading is true', () => {
    const wrapper = mountTable({ loading: true })

    expect(wrapper.find('.animate-spin').exists()).toBe(true)
    expect(wrapper.find('table').exists()).toBe(false)
  })

  it('shows table when loading is false', () => {
    const wrapper = mountTable({ loading: false })

    expect(wrapper.find('.animate-spin').exists()).toBe(false)
    expect(wrapper.find('table').exists()).toBe(true)
  })

  it('shows empty state when no data', () => {
    const wrapper = mountTable({ data: [] })

    expect(wrapper.text()).toContain('No data available.')
  })

  it('renders custom empty slot when no data', () => {
    const wrapper = mountTable({ data: [] }, { empty: () => h('span', 'Nothing here') })

    expect(wrapper.text()).toContain('Nothing here')
  })

  it('emits row-click with row data on click', async () => {
    const wrapper = mountTable()

    const firstRow = wrapper.findAll('tbody tr')[0]!
    await firstRow.trigger('click')

    expect(wrapper.emitted('row-click')).toBeTruthy()
    expect(wrapper.emitted('row-click')![0]).toEqual([sampleData[0]])
  })

  it('announces sort state via aria-sort after clicking a column header', async () => {
    const wrapper = mountTable()

    const firstHeader = wrapper.findAll('th')[0]!
    expect(firstHeader.attributes('aria-sort')).toBe('none')

    await firstHeader.trigger('click')
    await nextTick()
    await flushPromises()

    expect(['ascending', 'descending']).toContain(firstHeader.attributes('aria-sort'))
    expect(firstHeader.text()).toMatch(/[↑↓]/)
  })

  it('toggles sort direction on second click', async () => {
    const wrapper = mountTable()

    const firstHeader = wrapper.findAll('th')[0]!
    await firstHeader.trigger('click')
    await nextTick()
    await flushPromises()
    const firstSort = firstHeader.attributes('aria-sort')

    await firstHeader.trigger('click')
    await nextTick()
    await flushPromises()
    const secondSort = firstHeader.attributes('aria-sort')

    expect(['ascending', 'descending']).toContain(firstSort)
    expect(['ascending', 'descending']).toContain(secondSort)
    expect(secondSort).not.toBe(firstSort)
  })

  it('does not render a mobile-card list when no mobile-card slot is provided', () => {
    const wrapper = mountTable()

    expect(wrapper.find('ul[role="list"]').exists()).toBe(false)
  })

  it('renders a mobile-card list when a mobile-card slot is provided', () => {
    const wrapper = mountTable(
      {},
      {
        'mobile-card': ({ row }: { row: TestRow }) =>
          h('div', { class: 'card' }, `card-${row.name}`),
      },
    )

    const list = wrapper.find('ul[role="list"]')
    expect(list.exists()).toBe(true)
    const cards = list.findAll('.card')
    expect(cards).toHaveLength(2)
    expect(cards[0]!.text()).toBe('card-Alice')
    expect(cards[1]!.text()).toBe('card-Bob')
  })

  it('emits row-click from a mobile card when clickable is true', async () => {
    const wrapper = mountTable(
      { clickable: true },
      {
        'mobile-card': ({ row }: { row: TestRow }) =>
          h('div', { class: 'card' }, `card-${row.name}`),
      },
    )

    const cardItems = wrapper.findAll('ul[role="list"] > li')
    await cardItems[0]!.trigger('click')

    expect(wrapper.emitted('row-click')).toBeTruthy()
    expect(wrapper.emitted('row-click')![0]).toEqual([sampleData[0]])
  })

  it('does not emit row-click from a mobile card when clickable is false', async () => {
    const wrapper = mountTable(
      {},
      {
        'mobile-card': ({ row }: { row: TestRow }) =>
          h('div', { class: 'card' }, `card-${row.name}`),
      },
    )

    const cardItems = wrapper.findAll('ul[role="list"] > li')
    await cardItems[0]!.trigger('click')

    expect(wrapper.emitted('row-click')).toBeUndefined()
  })

  it('renders empty slot in the mobile-card list when data is empty', () => {
    const wrapper = mountTable(
      { data: [] },
      {
        'mobile-card': ({ row }: { row: TestRow }) =>
          h('div', { class: 'card' }, `card-${row.name}`),
        empty: () => h('span', 'Nothing on mobile'),
      },
    )

    const list = wrapper.find('ul[role="list"]')
    expect(list.exists()).toBe(true)
    expect(list.text()).toContain('Nothing on mobile')
  })
})
