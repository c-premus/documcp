<script setup lang="ts">
import { RouterLink } from 'vue-router'
import RelativeTimeCell from '@/components/shared/RelativeTimeCell.vue'
import TruncatedText from '@/components/shared/TruncatedText.vue'
import OAuthGrantTypeChips from './OAuthGrantTypeChips.vue'
import OAuthClientRowActions from './OAuthClientRowActions.vue'
import type { OAuthClient } from '@/stores/oauthClients'

const props = defineProps<{
  readonly client: OAuthClient
}>()

const emit = defineEmits<{
  delete: [client: OAuthClient]
}>()

function onDelete(): void {
  emit('delete', props.client)
}
</script>

<template>
  <div class="flex items-start gap-3 px-4 py-3">
    <div class="min-w-0 flex-1 space-y-1.5">
      <RouterLink
        :to="`/oauth-clients/${client.id}`"
        class="block truncate text-sm font-medium text-indigo-600 hover:underline dark:text-indigo-400"
      >
        {{ client.client_name }}
      </RouterLink>
      <div class="flex items-center gap-2 text-xs text-text-muted">
        <span class="shrink-0">ID</span>
        <TruncatedText :value="client.client_id" :mono="true" />
      </div>
      <div class="flex flex-wrap items-center gap-1.5">
        <OAuthGrantTypeChips :grant-types="client.grant_types" />
      </div>
      <div class="text-xs text-text-muted">
        Auth: {{ client.token_endpoint_auth_method.replace(/_/g, ' ') }}
      </div>
      <div class="flex flex-wrap items-center gap-x-1.5 gap-y-0.5 text-xs text-text-muted">
        <span>Created</span>
        <RelativeTimeCell :value="client.created_at" />
        <span aria-hidden="true">·</span>
        <span>Last used</span>
        <RelativeTimeCell :value="client.last_used_at ?? null" fallback="—" />
      </div>
    </div>
    <OAuthClientRowActions :client="client" @delete="onDelete" />
  </div>
</template>
