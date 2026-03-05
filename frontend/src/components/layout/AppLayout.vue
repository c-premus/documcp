<script setup lang="ts">
import { onMounted } from 'vue'
import AppSidebar from './AppSidebar.vue'
import AppHeader from './AppHeader.vue'
import AppNotifications from './AppNotifications.vue'
import { useDocumentEvents } from '@/composables/useDocumentEvents'

const { start } = useDocumentEvents()
onMounted(() => { start() })
</script>

<template>
  <div class="min-h-screen bg-gray-50">
    <AppSidebar />
    <div class="lg:pl-64">
      <AppHeader />
      <main class="px-4 py-6 sm:px-6 lg:px-8">
        <router-view />
      </main>
    </div>
    <AppNotifications />
  </div>
</template>
