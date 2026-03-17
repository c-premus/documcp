<script setup lang="ts">
import { computed } from 'vue'
import { SunIcon, MoonIcon, ComputerDesktopIcon } from '@heroicons/vue/24/outline'
import { useTheme } from '@/composables/useTheme'

const { mode, toggle } = useTheme()

const icon = computed(() => {
  switch (mode.value) {
    case 'light':
      return SunIcon
    case 'dark':
      return MoonIcon
    default:
      return ComputerDesktopIcon
  }
})

const nextLabel = computed(() => {
  switch (mode.value) {
    case 'light':
      return 'dark'
    case 'dark':
      return 'system'
    default:
      return 'light'
  }
})
</script>

<template>
  <button
    type="button"
    class="rounded-md p-1.5 text-text-muted hover:text-text-primary hover:bg-bg-hover transition-colors"
    :aria-label="`Switch to ${nextLabel} theme`"
    @click="toggle()"
  >
    <component :is="icon" class="h-5 w-5" />
  </button>
</template>
