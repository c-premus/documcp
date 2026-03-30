<script setup lang="ts">
import { FolderOpenIcon } from '@heroicons/vue/24/outline'

defineProps<{
  readonly title: string
  readonly description?: string
}>()
</script>

<template>
  <div class="flex flex-col items-center justify-center py-12 text-center">
    <slot name="icon">
      <FolderOpenIcon class="mx-auto h-12 w-12 text-text-disabled" />
    </slot>
    <h3 class="mt-4 text-sm font-semibold text-text-primary">{{ title }}</h3>
    <p v-if="description" class="mt-1 text-sm text-text-muted">{{ description }}</p>
    <div v-if="$slots.action" class="mt-6">
      <slot name="action" />
    </div>
  </div>
</template>
