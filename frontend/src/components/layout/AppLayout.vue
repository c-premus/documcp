<script setup lang="ts">
import { onMounted } from 'vue'
import AppSidebar from './AppSidebar.vue'
import AppHeader from './AppHeader.vue'
import AppNotifications from './AppNotifications.vue'
import { useSSEStore } from '@/stores/sse'
import { useDocumentEvents } from '@/composables/useDocumentEvents'

const sseStore = useSSEStore()
const { start } = useDocumentEvents()

onMounted(() => {
  sseStore.connect()
  start()
})
</script>

<template>
  <div class="min-h-screen bg-bg-page">
    <a
      href="#main-content"
      class="sr-only focus:not-sr-only focus:absolute focus:z-50 focus:rounded-md focus:bg-bg-surface focus:px-4 focus:py-2 focus:text-text-primary focus:shadow-lg focus:outline-none focus:ring-2 focus:ring-focus"
    >
      Skip to content
    </a>
    <AppHeader />
    <AppSidebar />
    <div class="lg:pl-64 pt-16">
      <main id="main-content" class="px-4 py-6 sm:px-6 lg:px-8">
        <router-view />
      </main>
    </div>
    <AppNotifications />
  </div>
</template>
