<script setup lang="ts">
import RelativeTimeCell from '@/components/shared/RelativeTimeCell.vue'
import ToggleCell from '@/components/shared/ToggleCell.vue'
import TruncatedText from '@/components/shared/TruncatedText.vue'
import UserRowActions from './UserRowActions.vue'
import type { User } from '@/stores/users'

const props = defineProps<{
  readonly user: User
}>()

const emit = defineEmits<{
  toggleAdmin: [user: User]
  delete: [user: User]
  sessions: [user: User]
}>()

function onToggle(): void {
  emit('toggleAdmin', props.user)
}
function onDelete(): void {
  emit('delete', props.user)
}
function onSessions(): void {
  emit('sessions', props.user)
}
</script>

<template>
  <div class="flex items-start gap-3 px-4 py-3">
    <div class="min-w-0 flex-1 space-y-1">
      <h3 class="truncate text-sm font-medium text-text-primary">
        {{ user.name }}
      </h3>
      <p class="truncate text-xs text-text-muted">{{ user.email }}</p>
      <div class="flex items-center gap-2 text-xs text-text-muted">
        <span>Created</span>
        <RelativeTimeCell :value="user.created_at" />
      </div>
      <div v-if="user.oidc_sub" class="flex items-center gap-2 text-xs text-text-muted">
        <span class="shrink-0">OIDC</span>
        <TruncatedText :value="user.oidc_sub" />
      </div>
      <div class="flex items-center gap-2 pt-1 text-xs text-text-muted">
        <span>Admin</span>
        <ToggleCell
          :model-value="user.is_admin"
          :label="`Toggle admin for ${user.name}`"
          @update:model-value="onToggle"
        />
      </div>
    </div>
    <UserRowActions :user="user" @sessions="onSessions" @delete="onDelete" />
  </div>
</template>
