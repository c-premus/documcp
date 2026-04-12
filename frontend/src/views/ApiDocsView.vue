<script setup lang="ts">
import { computed } from 'vue'
import { ApiReference } from '@scalar/api-reference'
import '@scalar/api-reference/style.css'
import { useTheme } from '@/composables/useTheme'
import spec from '@docs/contracts/openapi.yaml?raw'

const { resolved } = useTheme()

const configuration = computed(() => ({
  content: spec,
  darkMode: resolved.value === 'dark',
  hideDownloadButton: false,
  hideModels: false,
  theme: 'kepler' as const,
}))
</script>

<template>
  <div class="-m-6 lg:-m-8">
    <ApiReference :configuration="configuration" />
  </div>
</template>
