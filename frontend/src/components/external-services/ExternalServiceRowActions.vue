<script setup lang="ts">
import { computed } from 'vue'
import { ArrowPathIcon, HeartIcon, PencilSquareIcon, TrashIcon } from '@heroicons/vue/24/outline'
import type { ExternalService } from '@/stores/externalServices'

const props = defineProps<{
  readonly service: ExternalService
  readonly syncing: boolean
}>()

const emit = defineEmits<{
  sync: [service: ExternalService]
  healthCheck: [service: ExternalService]
  edit: [service: ExternalService]
  delete: [service: ExternalService]
}>()

const canSync = computed(() => props.service.type === 'kiwix')

function onSync(event: MouseEvent): void {
  event.stopPropagation()
  if (props.syncing) {
    return
  }
  emit('sync', props.service)
}

function onHealthCheck(event: MouseEvent): void {
  event.stopPropagation()
  emit('healthCheck', props.service)
}

function onEdit(event: MouseEvent): void {
  event.stopPropagation()
  emit('edit', props.service)
}

function onDelete(event: MouseEvent): void {
  event.stopPropagation()
  emit('delete', props.service)
}
</script>

<template>
  <div class="flex items-center gap-2">
    <button
      v-if="canSync"
      type="button"
      class="text-text-muted hover:text-indigo-600 dark:hover:text-indigo-400"
      :class="{ 'opacity-50 cursor-not-allowed': syncing }"
      title="Sync now"
      aria-label="Sync now"
      :disabled="syncing"
      @click="onSync"
    >
      <ArrowPathIcon :class="['h-5 w-5', syncing ? 'animate-spin' : '']" />
    </button>
    <button
      type="button"
      class="text-text-muted hover:text-green-600 dark:hover:text-green-400"
      title="Health check"
      aria-label="Health check"
      @click="onHealthCheck"
    >
      <HeartIcon class="h-5 w-5" />
    </button>
    <button
      type="button"
      class="text-text-muted hover:text-indigo-600 dark:hover:text-indigo-400"
      title="Edit service"
      aria-label="Edit service"
      @click="onEdit"
    >
      <PencilSquareIcon class="h-5 w-5" />
    </button>
    <button
      type="button"
      class="text-text-muted hover:text-red-600 dark:hover:text-red-400"
      title="Delete service"
      aria-label="Delete service"
      @click="onDelete"
    >
      <TrashIcon class="h-5 w-5" />
    </button>
  </div>
</template>
