<script setup lang="ts" generic="T">
import { FlexRender, getCoreRowModel, getSortedRowModel, useVueTable } from '@tanstack/vue-table'
import type { ColumnDef, SortingState } from '@tanstack/vue-table'
import { ref } from 'vue'

const props = defineProps<{
  readonly data: T[]
  readonly columns: ColumnDef<T, unknown>[]
  readonly loading?: boolean
  readonly clickable?: boolean
}>()

defineEmits<{
  'row-click': [row: T]
}>()

const sorting = ref<SortingState>([])

const table = useVueTable({
  get data() {
    return props.data
  },
  get columns() {
    return props.columns
  },
  state: {
    get sorting() {
      return sorting.value
    },
  },
  onSortingChange: (updaterOrValue) => {
    sorting.value =
      typeof updaterOrValue === 'function' ? updaterOrValue(sorting.value) : updaterOrValue
  },
  getCoreRowModel: getCoreRowModel(),
  getSortedRowModel: getSortedRowModel(),
})
</script>

<template>
  <div
    class="overflow-hidden shadow ring-1 ring-black/5 dark:ring-white/10 rounded-lg"
    aria-live="polite"
  >
    <div v-if="loading" class="flex items-center justify-center py-12">
      <div
        class="h-8 w-8 animate-spin rounded-full border-4 border-border-input border-t-indigo-600 dark:border-t-indigo-400"
      />
    </div>
    <table v-else class="min-w-full divide-y divide-border-default">
      <thead class="bg-bg-surface-alt">
        <tr>
          <th
            v-for="header in table.getHeaderGroups()[0]?.headers"
            :key="header.id"
            :class="[
              'px-3 py-3.5 text-left text-sm font-semibold text-text-primary cursor-pointer select-none',
              header.column.columnDef.meta?.className,
            ]"
            @click="header.column.getToggleSortingHandler()?.($event)"
          >
            <div class="flex items-center gap-1">
              <FlexRender :render="header.column.columnDef.header" :props="header.getContext()" />
              <span
                v-if="header.column.getIsSorted() === 'asc'"
                class="text-indigo-600 dark:text-indigo-400"
                >↑</span
              >
              <span
                v-else-if="header.column.getIsSorted() === 'desc'"
                class="text-indigo-600 dark:text-indigo-400"
                >↓</span
              >
            </div>
          </th>
        </tr>
      </thead>
      <tbody class="divide-y divide-border-default bg-bg-surface">
        <tr
          v-for="row in table.getRowModel().rows"
          :key="row.id"
          class="hover:bg-bg-hover"
          :class="{ 'cursor-pointer': clickable }"
          @click="$emit('row-click', row.original)"
        >
          <td
            v-for="cell in row.getVisibleCells()"
            :key="cell.id"
            :class="[
              'whitespace-nowrap px-3 py-4 text-sm text-text-muted',
              cell.column.columnDef.meta?.className,
            ]"
          >
            <FlexRender :render="cell.column.columnDef.cell" :props="cell.getContext()" />
          </td>
        </tr>
        <tr v-if="table.getRowModel().rows.length === 0">
          <td :colspan="columns.length" class="px-3 py-12 text-center text-sm text-text-muted">
            <slot name="empty">No data available.</slot>
          </td>
        </tr>
      </tbody>
    </table>
  </div>
</template>
