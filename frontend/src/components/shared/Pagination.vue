<script setup lang="ts">
import { computed } from 'vue'

const props = defineProps<{
  readonly page: number
  readonly perPage: number
  readonly total: number
}>()

const emit = defineEmits<{
  'update:page': [page: number]
  'update:perPage': [perPage: number]
}>()

const PAGE_SIZE_OPTIONS = [10, 25, 50] as const

const totalPages = computed(() => Math.max(1, Math.ceil(props.total / props.perPage)))

const rangeStart = computed(() => {
  if (props.total === 0) {
    return 0
  }
  return (props.page - 1) * props.perPage + 1
})

const rangeEnd = computed(() => Math.min(props.page * props.perPage, props.total))

const isFirstPage = computed(() => props.page <= 1)
const isLastPage = computed(() => props.page >= totalPages.value)

function goToPrevious(): void {
  if (!isFirstPage.value) {
    emit('update:page', props.page - 1)
  }
}

function goToNext(): void {
  if (!isLastPage.value) {
    emit('update:page', props.page + 1)
  }
}

function handlePerPageChange(event: Event): void {
  const target = event.target as HTMLSelectElement
  const newPerPage = Number(target.value)
  emit('update:perPage', newPerPage)
  emit('update:page', 1)
}
</script>

<template>
  <div class="flex items-center justify-between border-t border-border-default bg-bg-surface px-4 py-3 sm:px-6">
    <div class="flex flex-1 items-center justify-between">
      <div class="flex items-center gap-4">
        <p class="text-sm text-text-secondary">
          Showing <span class="font-medium">{{ rangeStart }}</span> to
          <span class="font-medium">{{ rangeEnd }}</span> of
          <span class="font-medium">{{ total }}</span> results
        </p>
        <div class="flex items-center gap-2">
          <label for="page-size" class="text-sm text-text-secondary">Per page:</label>
          <select
            id="page-size"
            :value="perPage"
            class="rounded-md border border-border-input bg-bg-surface py-1 pl-2 pr-8 text-sm text-text-secondary focus:border-indigo-500 focus:outline-none focus:ring-1 focus:ring-indigo-500 dark:focus:border-indigo-400 dark:focus:ring-indigo-400"
            @change="handlePerPageChange"
          >
            <option v-for="size in PAGE_SIZE_OPTIONS" :key="size" :value="size">
              {{ size }}
            </option>
          </select>
        </div>
      </div>
      <div class="flex gap-2">
        <button
          type="button"
          :disabled="isFirstPage"
          class="relative inline-flex items-center rounded-md border border-border-input bg-bg-surface px-4 py-2 text-sm font-medium text-text-secondary hover:bg-bg-hover disabled:cursor-not-allowed disabled:opacity-50"
          @click="goToPrevious"
        >
          Previous
        </button>
        <button
          type="button"
          :disabled="isLastPage"
          class="relative inline-flex items-center rounded-md border border-border-input bg-bg-surface px-4 py-2 text-sm font-medium text-text-secondary hover:bg-bg-hover disabled:cursor-not-allowed disabled:opacity-50"
          @click="goToNext"
        >
          Next
        </button>
      </div>
    </div>
  </div>
</template>
