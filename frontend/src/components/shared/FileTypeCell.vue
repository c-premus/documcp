<script setup lang="ts">
import { computed } from 'vue'

const props = defineProps<{
  readonly value: string
}>()

const label = computed(() => props.value.toUpperCase())
</script>

<template>
  <span>{{ label }}</span>
</template>
