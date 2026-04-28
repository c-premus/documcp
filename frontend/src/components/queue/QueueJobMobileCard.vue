<script setup lang="ts">
import { computed } from 'vue'
import { formatDistanceToNow } from 'date-fns'
import StatusBadge from '@/components/shared/StatusBadge.vue'
import QueueJobRowActions from './QueueJobRowActions.vue'
import type { FailedJob } from '@/stores/queue'

const props = defineProps<{
  readonly job: FailedJob
}>()

const emit = defineEmits<{
  retry: [job: FailedJob]
  delete: [job: FailedJob]
}>()

const lastError = computed(() => {
  const errors = props.job.errors
  if (!errors || errors.length === 0) {
    return ''
  }
  return errors[errors.length - 1]?.error ?? ''
})

const createdDisplay = computed(() =>
  formatDistanceToNow(new Date(props.job.created_at), { addSuffix: true }),
)

function onRetry(): void {
  emit('retry', props.job)
}
function onDelete(): void {
  emit('delete', props.job)
}
</script>

<template>
  <div class="flex flex-col gap-1.5 px-4 py-3">
    <div class="flex items-start gap-3">
      <h3 class="min-w-0 flex-1 truncate text-sm font-medium text-text-primary">
        {{ job.kind }}
      </h3>
      <QueueJobRowActions :job="job" @retry="onRetry" @delete="onDelete" />
    </div>
    <div class="flex flex-wrap items-center gap-x-1.5 gap-y-0.5 text-xs text-text-muted">
      <span class="font-mono">#{{ job.id }}</span>
      <span aria-hidden="true">·</span>
      <span>queue: {{ job.queue }}</span>
      <span aria-hidden="true">·</span>
      <span>{{ job.attempt }}/{{ job.max_attempts }} attempts</span>
    </div>
    <div class="flex items-center justify-between gap-2">
      <StatusBadge :status="job.state" />
      <span class="text-xs text-text-muted">{{ createdDisplay }}</span>
    </div>
    <div
      v-if="lastError"
      role="alert"
      class="mt-1 break-words rounded-md bg-red-50 px-3 py-2 text-xs text-red-800 dark:bg-red-900/20 dark:text-red-300"
    >
      {{ lastError }}
    </div>
  </div>
</template>
