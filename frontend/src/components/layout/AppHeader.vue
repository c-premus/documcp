<script setup lang="ts">
import { useAuthStore } from '@/stores/auth'
import { useSSEStore } from '@/stores/sse'
import ThemeToggle from './ThemeToggle.vue'

const auth = useAuthStore()
const sse = useSSEStore()
const logoSrc = `${import.meta.env.BASE_URL}logo-concept-1-transparent.svg`
</script>

<template>
  <header class="fixed top-0 left-0 right-0 z-20 h-16 bg-bg-surface border-b border-border-default px-4">
    <div class="flex h-full items-center justify-between">
      <div class="flex items-center gap-3">
        <img :src="logoSrc" alt="" aria-hidden="true" class="h-8 w-8 shrink-0" />
        <span class="text-lg font-semibold text-text-primary">DocuMCP</span>
      </div>

      <div class="flex items-center gap-4">
        <div
          class="flex items-center gap-2"
          role="status"
          aria-live="polite"
          title="Real-time queue event stream"
        >
          <span
            class="inline-block h-2 w-2 rounded-full"
            :class="sse.connected ? 'bg-green-500' : 'bg-red-500'"
            aria-hidden="true"
          />
          <span class="text-xs text-text-muted">
            {{ sse.connected ? 'Live' : 'Offline' }}
          </span>
        </div>

        <template v-if="auth.user">
          <ThemeToggle />
          <span class="text-sm text-text-secondary">{{ auth.user.name }}</span>
          <button
            type="button"
            class="cursor-pointer text-sm text-text-muted hover:text-text-secondary transition-colors"
            @click="auth.logout()"
          >
            Logout
          </button>
        </template>
      </div>
    </div>
  </header>
</template>
