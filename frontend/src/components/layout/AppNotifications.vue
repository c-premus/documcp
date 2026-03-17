<script setup lang="ts">
import { Toaster } from 'vue-sonner'
import { useTheme } from '@/composables/useTheme'

const { resolved } = useTheme()
</script>

<template>
  <Toaster
    position="top-right"
    :theme="resolved"
    :toast-options="{
      style: { fontFamily: 'inherit' },
      class: 'text-sm',
    }"
    rich-colors
    close-button
  />
</template>
