<script setup lang="ts">
import { computed } from 'vue'
import type { FailedJob } from '@/stores/queue'

const props = defineProps<{
  readonly job: FailedJob
}>()

const message = computed(() => {
  const errors = props.job.errors
  if (!errors || errors.length === 0) {
    return ''
  }
  const raw = errors[errors.length - 1]?.error ?? ''
  return raw.length > 80 ? `${raw.slice(0, 80)}...` : raw
})
</script>

<template>
  <span class="text-red-600 dark:text-red-400 text-xs" :title="message">{{ message }}</span>
</template>
