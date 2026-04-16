<script setup lang="ts">
import { computed } from 'vue'
import StatusBadge from './StatusBadge.vue'

const props = defineProps<{
  readonly isPublic: boolean
}>()

const status = computed(() => (props.isPublic ? 'public' : 'private'))
</script>

<template>
  <StatusBadge :status="status" />
</template>
