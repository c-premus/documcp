<script setup lang="ts">
import { ref, h } from 'vue'
import { toast } from 'vue-sonner'
import type { ColumnDef } from '@tanstack/vue-table'

import DataTable from '../components/shared/DataTable.vue'
import EmptyState from '../components/shared/EmptyState.vue'
import { useConfluenceSpacesStore } from '../stores/confluenceSpaces'
import type { ConfluenceSpace } from '../stores/confluenceSpaces'

const store = useConfluenceSpacesStore()
const loading = ref(false)
const spaces = ref<ConfluenceSpace[]>([])

async function fetchSpaces(): Promise<void> {
  loading.value = true
  try {
    const response = await store.fetchSpaces()
    spaces.value = response.data
  } catch {
    toast.error('Failed to load Confluence spaces')
  } finally {
    loading.value = false
  }
}

fetchSpaces()

function typeBadgeClasses(type: string): string {
  if (type === 'personal') {
    return 'bg-purple-100 text-purple-800'
  }
  return 'bg-blue-100 text-blue-800'
}

const columns: ColumnDef<ConfluenceSpace, unknown>[] = [
  {
    accessorKey: 'name',
    header: 'Name',
    enableSorting: true,
  },
  {
    accessorKey: 'key',
    header: 'Key',
    enableSorting: true,
    cell: ({ getValue }) => {
      const value = getValue<string>()
      return h(
        'span',
        { class: 'font-mono text-xs text-gray-600' },
        value,
      )
    },
  },
  {
    accessorKey: 'type',
    header: 'Type',
    enableSorting: true,
    cell: ({ getValue }) => {
      const value = getValue<string>()
      return h(
        'span',
        {
          class: [
            'inline-flex items-center rounded-full px-2.5 py-0.5 text-xs font-medium capitalize',
            typeBadgeClasses(value),
          ],
        },
        value,
      )
    },
  },
  {
    accessorKey: 'description',
    header: 'Description',
    enableSorting: false,
    cell: ({ getValue }) => {
      const value = getValue<string | undefined>()
      return value ?? '-'
    },
  },
]
</script>

<template>
  <div>
    <!-- Toolbar -->
    <div class="flex items-center gap-4 mb-4">
      <h1 class="text-2xl font-bold text-gray-900">Confluence Spaces</h1>
    </div>

    <!-- Empty State -->
    <EmptyState
      v-if="!loading && spaces.length === 0"
      title="No Confluence spaces"
      description="No Confluence spaces are available. Check your Confluence integration settings."
    />

    <!-- Data Table -->
    <DataTable
      v-else
      :data="spaces"
      :columns="columns"
      :loading="loading"
    />
  </div>
</template>
