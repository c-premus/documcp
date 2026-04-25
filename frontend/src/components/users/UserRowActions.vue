<script setup lang="ts">
import { ComputerDesktopIcon, TrashIcon } from '@heroicons/vue/24/outline'

interface UserLike {
  readonly id: number
  readonly name: string
}

const props = defineProps<{
  readonly user: UserLike
}>()

const emit = defineEmits<{
  delete: [user: UserLike]
  sessions: [user: UserLike]
}>()

function onDelete(event: MouseEvent): void {
  event.stopPropagation()
  emit('delete', props.user)
}

function onSessions(event: MouseEvent): void {
  event.stopPropagation()
  emit('sessions', props.user)
}
</script>

<template>
  <div class="flex items-center gap-2">
    <button
      type="button"
      class="text-text-muted hover:text-indigo-600 dark:hover:text-indigo-400"
      title="View active sessions"
      aria-label="View active sessions"
      @click="onSessions"
    >
      <ComputerDesktopIcon class="h-5 w-5" />
    </button>
    <button
      type="button"
      class="text-text-muted hover:text-red-600 dark:hover:text-red-400"
      title="Delete user"
      aria-label="Delete user"
      @click="onDelete"
    >
      <TrashIcon class="h-5 w-5" />
    </button>
  </div>
</template>
