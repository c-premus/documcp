<script setup lang="ts">
defineProps<{
  readonly grantTypes: readonly string[]
}>()

function humanize(name: string): string {
  return name.replace(/_/g, ' ')
}
</script>

<template>
  <div class="flex flex-wrap gap-1">
    <span
      v-for="gt in grantTypes"
      :key="gt"
      class="inline-flex items-center rounded-full bg-blue-100 dark:bg-blue-900/30 px-2 py-0.5 text-xs font-medium text-blue-800 dark:text-blue-300"
    >
      {{ humanize(gt) }}
    </span>
  </div>
</template>
