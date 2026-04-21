<script setup lang="ts">
import { computed } from 'vue'

const props = withDefaults(
  defineProps<{
    readonly scope: string | readonly string[]
    readonly variant?: 'granted' | 'base'
  }>(),
  {
    variant: 'granted',
  },
)

const scopes = computed<readonly string[]>(() => {
  if (Array.isArray(props.scope)) {
    return props.scope
  }
  const raw = props.scope as string
  return raw ? raw.split(/\s+/).filter(Boolean) : []
})

const chipClass = computed(() => {
  return props.variant === 'base'
    ? 'inline-flex items-center rounded-full bg-indigo-50 dark:bg-indigo-900/30 px-2 py-0.5 text-xs font-medium text-indigo-700 dark:text-indigo-300'
    : 'inline-flex items-center rounded-full bg-green-100 dark:bg-green-900/30 px-2 py-0.5 text-xs font-medium text-green-800 dark:text-green-300'
})
</script>

<template>
  <div class="flex flex-wrap gap-1">
    <span v-for="s in scopes" :key="s" :class="chipClass">{{ s }}</span>
  </div>
</template>
