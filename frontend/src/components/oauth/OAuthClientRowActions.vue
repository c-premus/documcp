<script setup lang="ts">
import { TrashIcon } from '@heroicons/vue/24/outline'
import type { OAuthClient } from '@/stores/oauthClients'

const props = defineProps<{
  readonly client: OAuthClient
}>()

const emit = defineEmits<{
  delete: [client: OAuthClient]
}>()

function onDelete(event: MouseEvent): void {
  event.stopPropagation()
  emit('delete', props.client)
}
</script>

<template>
  <button
    type="button"
    class="cursor-pointer text-text-muted hover:text-red-600 dark:hover:text-red-400"
    :title="`Delete client ${client.client_name}`"
    :aria-label="`Delete client ${client.client_name}`"
    @click="onDelete"
  >
    <TrashIcon class="h-5 w-5" />
  </button>
</template>
