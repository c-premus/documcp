<script setup lang="ts">
import { useAuthStore } from '@/stores/auth'
import { useSSE } from '@/composables/useSSE'
import ThemeToggle from './ThemeToggle.vue'

const auth = useAuthStore()
const { connected, connect } = useSSE()

connect()
</script>

<template>
  <header class="sticky top-0 z-10 bg-bg-surface border-b border-border-default px-4 py-3">
    <div class="flex items-center justify-between">
      <div class="flex items-center gap-2" role="status" aria-live="polite">
        <span
          class="inline-block h-2 w-2 rounded-full"
          :class="connected ? 'bg-green-500' : 'bg-red-500'"
          aria-hidden="true"
        />
        <span class="text-xs text-text-muted">
          {{ connected ? 'Connected' : 'Disconnected' }}
        </span>
      </div>

      <div v-if="auth.user" class="flex items-center gap-4">
        <ThemeToggle />
        <span class="text-sm text-text-secondary">{{ auth.user.name }}</span>
        <button
          type="button"
          class="text-sm text-text-muted hover:text-text-secondary transition-colors"
          @click="auth.logout()"
        >
          Logout
        </button>
      </div>
    </div>
  </header>
</template>
