import {
  createSortedRowModel,
  metaHelper,
  rowSortingFeature,
  sortFn_alphanumeric,
  sortFn_datetime,
  sortFn_text,
  tableFeatures,
} from '@tanstack/vue-table'
import type { ColumnDef, RowData } from '@tanstack/vue-table'

export interface DataTableColumnMeta {
  readonly className?: string
}

/**
 * Feature set shared by every DataTable. TanStack Table v9 only exposes the
 * APIs of registered features, and its 'auto' sort resolves only registered
 * sort functions — these three are the ones the auto-picker chooses between
 * (numbers fall back to the built-in basic sort), matching v8 behavior.
 */
export const dataTableFeatures = tableFeatures({
  rowSortingFeature,
  sortedRowModel: createSortedRowModel(),
  sortFns: {
    alphanumeric: sortFn_alphanumeric,
    datetime: sortFn_datetime,
    text: sortFn_text,
  },
  columnMeta: metaHelper<DataTableColumnMeta>(),
})

export type DataTableFeatures = typeof dataTableFeatures

export type DataTableColumn<TData extends RowData> = ColumnDef<DataTableFeatures, TData, unknown>
