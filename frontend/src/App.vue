<script setup lang="ts">
import AppLayout from '@/components/layout/AppLayout.vue'
import { useAuthStore } from '@/stores/auth'

const auth = useAuthStore()
auth.fetchUser()
</script>

<template>
  <AppLayout v-if="auth.isAuthenticated">
    <router-view />
  </AppLayout>
  <div v-else-if="auth.loading" class="flex items-center justify-center min-h-screen">
    <p class="text-gray-500">Loading...</p>
  </div>
</template>
