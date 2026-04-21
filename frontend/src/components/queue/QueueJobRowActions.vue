<script setup lang="ts">
import { ArrowPathIcon, TrashIcon } from '@heroicons/vue/24/outline'
import type { FailedJob } from '@/stores/queue'

defineProps<{
  readonly job: FailedJob
}>()

const emit = defineEmits<{
  retry: [job: FailedJob]
  delete: [job: FailedJob]
}>()

function onRetry(event: MouseEvent, job: FailedJob): void {
  event.stopPropagation()
  emit('retry', job)
}

function onDelete(event: MouseEvent, job: FailedJob): void {
  event.stopPropagation()
  emit('delete', job)
}
</script>

<template>
  <div class="flex items-center gap-2">
    <button
      type="button"
      class="text-text-muted hover:text-indigo-600 dark:hover:text-indigo-400"
      :aria-label="`Retry job ${job.kind} ${job.id}`"
      title="Retry job"
      @click="(event) => onRetry(event, job)"
    >
      <ArrowPathIcon class="h-5 w-5" />
    </button>
    <button
      type="button"
      class="text-text-muted hover:text-red-600 dark:hover:text-red-400"
      :aria-label="`Delete job ${job.kind} ${job.id}`"
      title="Delete job"
      @click="(event) => onDelete(event, job)"
    >
      <TrashIcon class="h-5 w-5" />
    </button>
  </div>
</template>
