<script setup lang="ts">
import { TrashIcon } from '@heroicons/vue/24/outline'
import type { ScopeGrant } from '@/stores/oauthClients'

defineProps<{
  readonly grant: ScopeGrant
}>()

const emit = defineEmits<{
  revoke: [grant: ScopeGrant]
}>()

function onRevoke(event: MouseEvent, grant: ScopeGrant): void {
  event.stopPropagation()
  emit('revoke', grant)
}
</script>

<template>
  <button
    type="button"
    class="cursor-pointer text-text-muted hover:text-red-600 dark:hover:text-red-400"
    :aria-label="`Revoke grant ${grant.scope}`"
    title="Revoke grant"
    @click="(event) => onRevoke(event, grant)"
  >
    <TrashIcon class="h-5 w-5" />
  </button>
</template>
