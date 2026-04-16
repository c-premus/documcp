<script setup lang="ts">
import { ArrowPathIcon, TrashIcon } from '@heroicons/vue/24/outline'
import type { Document } from '@/stores/documents'

const props = defineProps<{
  readonly document: Document
  readonly canPurge: boolean
}>()

const emit = defineEmits<{
  restore: [document: Document]
  purge: [document: Document]
}>()

function onRestore(event: MouseEvent): void {
  event.stopPropagation()
  emit('restore', props.document)
}

function onPurge(event: MouseEvent): void {
  event.stopPropagation()
  emit('purge', props.document)
}
</script>

<template>
  <div class="flex items-center gap-2">
    <button
      type="button"
      class="text-text-muted hover:text-green-600 dark:hover:text-green-400"
      title="Restore document"
      aria-label="Restore document"
      @click="onRestore"
    >
      <ArrowPathIcon class="h-5 w-5" />
    </button>
    <button
      v-if="canPurge"
      type="button"
      class="text-text-muted hover:text-red-600 dark:hover:text-red-400"
      title="Permanently delete"
      aria-label="Permanently delete"
      @click="onPurge"
    >
      <TrashIcon class="h-5 w-5" />
    </button>
  </div>
</template>
