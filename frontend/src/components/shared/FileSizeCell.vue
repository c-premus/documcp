<script setup lang="ts">
import { computed } from 'vue'

const props = defineProps<{
  readonly bytes: number
}>()

const formatted = computed(() => {
  const bytes = props.bytes
  if (bytes < 1024) {
    return `${bytes} B`
  }
  if (bytes < 1024 * 1024) {
    return `${(bytes / 1024).toFixed(1)} KB`
  }
  return `${(bytes / (1024 * 1024)).toFixed(1)} MB`
})
</script>

<template>
  <span>{{ formatted }}</span>
</template>
