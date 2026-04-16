<script setup lang="ts">
import { ArrowPathIcon, PencilSquareIcon, TrashIcon } from '@heroicons/vue/24/outline'
import type { GitTemplate } from '@/stores/gitTemplates'

const props = defineProps<{
  readonly template: GitTemplate
  readonly syncing: boolean
}>()

const emit = defineEmits<{
  edit: [template: GitTemplate]
  sync: [template: GitTemplate]
  delete: [template: GitTemplate]
}>()

function onEdit(event: MouseEvent): void {
  event.stopPropagation()
  emit('edit', props.template)
}

function onSync(event: MouseEvent): void {
  event.stopPropagation()
  if (props.syncing) {
    return
  }
  emit('sync', props.template)
}

function onDelete(event: MouseEvent): void {
  event.stopPropagation()
  emit('delete', props.template)
}
</script>

<template>
  <div class="flex items-center gap-2">
    <button
      type="button"
      class="text-text-muted hover:text-indigo-600 dark:hover:text-indigo-400"
      title="Edit template"
      aria-label="Edit template"
      @click="onEdit"
    >
      <PencilSquareIcon class="h-5 w-5" />
    </button>
    <button
      type="button"
      class="text-text-muted hover:text-indigo-600 dark:hover:text-indigo-400"
      title="Sync template"
      aria-label="Sync template"
      :disabled="syncing"
      @click="onSync"
    >
      <ArrowPathIcon :class="['h-5 w-5', syncing ? 'animate-spin' : '']" />
    </button>
    <button
      type="button"
      class="text-text-muted hover:text-red-600 dark:hover:text-red-400"
      title="Delete template"
      aria-label="Delete template"
      @click="onDelete"
    >
      <TrashIcon class="h-5 w-5" />
    </button>
  </div>
</template>
