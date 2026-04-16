<script setup lang="ts">
import { computed } from 'vue'
import { formatDistanceToNow } from 'date-fns'

const props = withDefaults(
  defineProps<{
    readonly value: string | null | undefined
    readonly fallback?: string
  }>(),
  {
    fallback: '',
  },
)

const isEmpty = computed(
  () => props.value === null || props.value === undefined || props.value === '',
)

const parsed = computed(() => (isEmpty.value ? null : new Date(props.value as string)))
const label = computed(() =>
  parsed.value === null ? '' : formatDistanceToNow(parsed.value, { addSuffix: true }),
)
</script>

<template>
  <span v-if="isEmpty">{{ fallback }}</span>
  <time v-else :datetime="value ?? undefined">{{ label }}</time>
</template>
