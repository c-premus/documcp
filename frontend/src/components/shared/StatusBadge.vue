<script setup lang="ts">
import { computed } from 'vue'

const props = defineProps<{
  readonly status: string
}>()

interface BadgeStyle {
  readonly bg: string
  readonly text: string
}

const STATUS_STYLES: Readonly<Record<string, BadgeStyle>> = {
  uploaded: { bg: 'bg-yellow-100', text: 'text-yellow-800' },
  extracted: { bg: 'bg-blue-100', text: 'text-blue-800' },
  indexed: { bg: 'bg-green-100', text: 'text-green-800' },
  failed: { bg: 'bg-red-100', text: 'text-red-800' },
  index_failed: { bg: 'bg-orange-100', text: 'text-orange-800' },
}

const DEFAULT_STYLE: BadgeStyle = { bg: 'bg-gray-100', text: 'text-gray-800' }

const badgeClasses = computed(() => {
  const style = STATUS_STYLES[props.status] ?? DEFAULT_STYLE
  return `${style.bg} ${style.text}`
})

const displayLabel = computed(() => props.status.replace(/_/g, ' '))
</script>

<template>
  <span
    class="inline-flex items-center rounded-full px-2.5 py-0.5 text-xs font-medium capitalize"
    :class="badgeClasses"
  >
    {{ displayLabel }}
  </span>
</template>
