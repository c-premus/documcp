<script setup lang="ts">
import { computed } from 'vue'

const DEFAULT_CLASSES = 'bg-gray-100 text-gray-800 dark:bg-gray-800 dark:text-gray-300'

const props = defineProps<{
  readonly value: string
  readonly palette: Readonly<Record<string, string>>
}>()

const classes = computed(() => props.palette[props.value] ?? DEFAULT_CLASSES)
const displayLabel = computed(() => props.value.replace(/_/g, ' '))
</script>

<template>
  <span
    class="inline-flex items-center rounded-full px-2.5 py-0.5 text-xs font-medium capitalize"
    :class="classes"
  >
    {{ displayLabel }}
  </span>
</template>
