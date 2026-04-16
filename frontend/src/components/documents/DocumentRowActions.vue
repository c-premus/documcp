<script setup lang="ts">
import { EyeIcon, TrashIcon, PencilSquareIcon } from '@heroicons/vue/24/outline'
import type { Document } from '@/stores/documents'

const props = defineProps<{
  readonly document: Document
}>()

const emit = defineEmits<{
  edit: [document: Document]
  view: [document: Document]
  delete: [document: Document]
}>()

function onEdit(event: MouseEvent): void {
  event.stopPropagation()
  emit('edit', props.document)
}

function onView(event: MouseEvent): void {
  event.stopPropagation()
  emit('view', props.document)
}

function onDelete(event: MouseEvent): void {
  event.stopPropagation()
  emit('delete', props.document)
}
</script>

<template>
  <div class="flex items-center gap-2">
    <button
      type="button"
      class="text-text-muted hover:text-indigo-600 dark:hover:text-indigo-400"
      title="Edit document"
      aria-label="Edit document"
      @click="onEdit"
    >
      <PencilSquareIcon class="h-5 w-5" />
    </button>
    <button
      type="button"
      class="text-text-muted hover:text-indigo-600 dark:hover:text-indigo-400"
      title="View document"
      aria-label="View document"
      @click="onView"
    >
      <EyeIcon class="h-5 w-5" />
    </button>
    <button
      type="button"
      class="text-text-muted hover:text-red-600 dark:hover:text-red-400"
      title="Delete document"
      aria-label="Delete document"
      @click="onDelete"
    >
      <TrashIcon class="h-5 w-5" />
    </button>
  </div>
</template>
