<script setup lang="ts">
import { Toaster } from 'vue-sonner'
</script>

<template>
  <Toaster
    position="top-right"
    :toast-options="{
      style: { fontFamily: 'inherit' },
      class: 'text-sm',
    }"
    rich-colors
    close-button
  />
</template>
