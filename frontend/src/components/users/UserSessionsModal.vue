<script setup lang="ts">
import { storeToRefs } from 'pinia'
import { watch } from 'vue'
import { Dialog, DialogPanel, DialogTitle } from '@headlessui/vue'
import { TrashIcon, XMarkIcon } from '@heroicons/vue/24/outline'
import { toast } from 'vue-sonner'

import { useUsersStore, type User } from '@/stores/users'

interface Props {
  readonly open: boolean
  readonly user: User | null
}
const props = defineProps<Props>()

const emit = defineEmits<{
  close: []
}>()

const store = useUsersStore()
const { sessionIDs, sessionsLoading } = storeToRefs(store)

watch(
  () => [props.open, props.user?.id] as const,
  async ([open, id]) => {
    if (open && typeof id === 'number') {
      try {
        await store.fetchUserSessions(id)
      } catch {
        toast.error('Failed to load sessions')
      }
    }
  },
  { immediate: true },
)

async function handleRevokeOne(sessionID: string): Promise<void> {
  if (props.user === null) {
    return
  }
  try {
    await store.revokeUserSession(props.user.id, sessionID)
    toast.success('Session revoked')
  } catch {
    toast.error('Failed to revoke session')
  }
}

async function handleRevokeAll(): Promise<void> {
  if (props.user === null) {
    return
  }
  try {
    const count = await store.revokeAllUserSessions(props.user.id)
    toast.success(count === 1 ? 'Revoked 1 session' : `Revoked ${count} sessions`)
  } catch {
    toast.error('Failed to revoke sessions')
  }
}

function shortID(id: string): string {
  return id.length > 16 ? `${id.slice(0, 12)}…${id.slice(-4)}` : id
}
</script>

<template>
  <Dialog :open="open" class="relative z-50" @close="emit('close')">
    <div class="fixed inset-0 bg-overlay backdrop-blur-sm transition-opacity" aria-hidden="true" />

    <div class="fixed inset-0 z-50 overflow-y-auto">
      <div class="flex min-h-full items-end justify-center p-4 text-center sm:items-center sm:p-0">
        <DialogPanel
          class="relative transform overflow-hidden rounded-lg bg-bg-surface px-4 pb-4 pt-5 text-left shadow-xl transition-all sm:my-8 sm:w-full sm:max-w-lg sm:p-6"
        >
          <div class="flex items-start justify-between mb-4">
            <DialogTitle as="h3" class="text-base font-semibold text-text-primary">
              Active sessions
              <span v-if="user" class="block text-sm font-normal text-text-muted mt-1">
                {{ user.name }} &mdash; {{ user.email }}
              </span>
            </DialogTitle>
            <button
              type="button"
              class="rounded-md p-1 text-text-muted hover:text-text-primary hover:bg-bg-hover focus-visible:outline focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-focus"
              aria-label="Close"
              @click="emit('close')"
            >
              <XMarkIcon class="h-5 w-5" />
            </button>
          </div>

          <p class="text-xs text-text-muted mb-4">
            Sessions appear here once a user signs in through OIDC. Revoking a session forces the
            user to re-authenticate on their next request &mdash; the cookie itself becomes invalid
            because the server-side payload is gone.
          </p>

          <div
            v-if="sessionsLoading"
            class="text-sm text-text-muted py-6 text-center"
            role="status"
            aria-live="polite"
          >
            Loading sessions&hellip;
          </div>

          <div
            v-else-if="sessionIDs.length === 0"
            class="text-sm text-text-muted py-6 text-center"
            role="status"
          >
            No active sessions.
          </div>

          <ul v-else class="divide-y divide-border-input border border-border-input rounded-md">
            <li
              v-for="sid in sessionIDs"
              :key="sid"
              class="flex items-center justify-between gap-3 px-3 py-2"
            >
              <code
                class="text-xs text-text-secondary font-mono truncate"
                :title="sid"
                :aria-label="`Session ${sid}`"
              >
                {{ shortID(sid) }}
              </code>
              <button
                type="button"
                class="shrink-0 rounded-md p-1 text-text-muted hover:text-red-600 dark:hover:text-red-400 hover:bg-bg-hover focus-visible:outline focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-focus"
                :aria-label="`Revoke session ${sid}`"
                @click="handleRevokeOne(sid)"
              >
                <TrashIcon class="h-4 w-4" />
              </button>
            </li>
          </ul>

          <div class="mt-5 flex flex-wrap justify-end gap-2">
            <button
              type="button"
              class="inline-flex justify-center rounded-md bg-bg-surface-alt px-3 py-2 text-sm font-semibold text-text-primary shadow-sm hover:bg-bg-hover focus-visible:outline focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-focus"
              @click="emit('close')"
            >
              Close
            </button>
            <button
              type="button"
              class="inline-flex justify-center rounded-md bg-red-600 px-3 py-2 text-sm font-semibold text-white shadow-sm hover:bg-red-500 focus-visible:outline focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-focus disabled:opacity-50 disabled:cursor-not-allowed"
              :disabled="sessionIDs.length === 0 || sessionsLoading"
              @click="handleRevokeAll"
            >
              Revoke all
            </button>
          </div>
        </DialogPanel>
      </div>
    </div>
  </Dialog>
</template>
