<script setup lang="ts">
import { computed } from 'vue'

const props = defineProps<{
  readonly status: string
}>()

interface BadgeStyle {
  readonly classes: string
}

const STATUS_STYLES: Readonly<Record<string, BadgeStyle>> = {
  uploaded: { classes: 'bg-yellow-100 text-yellow-800 dark:bg-yellow-900/30 dark:text-yellow-300' },
  extracted: { classes: 'bg-blue-100 text-blue-800 dark:bg-blue-900/30 dark:text-blue-300' },
  indexed: { classes: 'bg-green-100 text-green-800 dark:bg-green-900/30 dark:text-green-300' },
  failed: { classes: 'bg-red-100 text-red-800 dark:bg-red-900/30 dark:text-red-300' },
  index_failed: {
    classes: 'bg-orange-100 text-orange-800 dark:bg-orange-900/30 dark:text-orange-300',
  },
  active: { classes: 'bg-green-100 text-green-800 dark:bg-green-900/30 dark:text-green-300' },
  revoked: { classes: 'bg-red-100 text-red-800 dark:bg-red-900/30 dark:text-red-300' },
  public: {
    classes: 'bg-emerald-100 text-emerald-800 dark:bg-emerald-900/30 dark:text-emerald-300',
  },
  private: { classes: 'bg-gray-100 text-gray-800 dark:bg-gray-800 dark:text-gray-300' },
}

const DEFAULT_STYLE: BadgeStyle = {
  classes: 'bg-gray-100 text-gray-800 dark:bg-gray-800 dark:text-gray-300',
}

const badgeClasses = computed(() => {
  const style = STATUS_STYLES[props.status] ?? DEFAULT_STYLE
  return style.classes
})

const displayLabel = computed(() => props.status.replace(/_/g, ' '))
</script>

<template>
  <span
    role="status"
    class="inline-flex items-center rounded-full px-2.5 py-0.5 text-xs font-medium capitalize"
    :class="badgeClasses"
  >
    {{ displayLabel }}
  </span>
</template>
