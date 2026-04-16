<script setup lang="ts">
import { ArrowUpIcon, ArrowDownIcon } from '@heroicons/vue/24/outline'

defineProps<{
  readonly priority: number
  readonly canMoveUp: boolean
  readonly canMoveDown: boolean
}>()

const emit = defineEmits<{
  up: []
  down: []
}>()

function onUp(event: MouseEvent): void {
  event.stopPropagation()
  emit('up')
}

function onDown(event: MouseEvent): void {
  event.stopPropagation()
  emit('down')
}
</script>

<template>
  <div class="flex items-center gap-1">
    <span class="text-sm tabular-nums">{{ priority }}</span>
    <button
      type="button"
      class="text-text-disabled hover:text-indigo-600 dark:hover:text-indigo-400 p-0.5 disabled:opacity-30 disabled:cursor-not-allowed"
      title="Move up"
      aria-label="Move up"
      :disabled="!canMoveUp"
      @click="onUp"
    >
      <ArrowUpIcon class="h-4 w-4" />
    </button>
    <button
      type="button"
      class="text-text-disabled hover:text-indigo-600 dark:hover:text-indigo-400 p-0.5 disabled:opacity-30 disabled:cursor-not-allowed"
      title="Move down"
      aria-label="Move down"
      :disabled="!canMoveDown"
      @click="onDown"
    >
      <ArrowDownIcon class="h-4 w-4" />
    </button>
  </div>
</template>
