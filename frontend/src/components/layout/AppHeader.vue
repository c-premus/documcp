<script setup lang="ts">
import { useAuthStore } from '@/stores/auth'
import { useSSE } from '@/composables/useSSE'

const auth = useAuthStore()
const { connected, connect } = useSSE()

connect()
</script>

<template>
  <header class="sticky top-0 z-10 bg-white border-b border-gray-200 px-4 py-3">
    <div class="flex items-center justify-between">
      <div class="flex items-center gap-2">
        <span
          class="inline-block h-2 w-2 rounded-full"
          :class="connected ? 'bg-green-500' : 'bg-red-500'"
          :title="connected ? 'Connected' : 'Disconnected'"
        />
        <span class="text-xs text-gray-500">
          {{ connected ? 'Connected' : 'Disconnected' }}
        </span>
      </div>

      <div v-if="auth.user" class="flex items-center gap-4">
        <span class="text-sm text-gray-700">{{ auth.user.name }}</span>
        <button
          type="button"
          class="text-sm text-gray-500 hover:text-gray-700 transition-colors"
          @click="auth.logout()"
        >
          Logout
        </button>
      </div>
    </div>
  </header>
</template>
