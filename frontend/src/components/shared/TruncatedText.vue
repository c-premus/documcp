<script setup lang="ts">
withDefaults(
  defineProps<{
    readonly value: string
    readonly mono?: boolean
    readonly maxWidth?: string
  }>(),
  {
    mono: false,
    maxWidth: 'max-w-xs',
  },
)
</script>

<template>
  <span class="block truncate" :class="[maxWidth, mono ? 'font-mono text-xs' : '']" :title="value">
    {{ value }}
  </span>
</template>
