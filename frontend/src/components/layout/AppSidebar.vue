<script setup lang="ts">
import { watch } from 'vue'
import { useRoute } from 'vue-router'
import { TransitionRoot, TransitionChild, Dialog, DialogPanel } from '@headlessui/vue'
import {
  HomeIcon,
  DocumentTextIcon,
  ArchiveBoxIcon,
  CodeBracketIcon,
  UserGroupIcon,
  KeyIcon,
  ServerIcon,
  QueueListIcon,
  TrashIcon,
  BookOpenIcon,
  XMarkIcon,
} from '@heroicons/vue/24/outline'
import { useAuthStore } from '@/stores/auth'
import { useSSEStore } from '@/stores/sse'
import { useSidebar } from '@/composables/useSidebar'
import { useAppVersion } from '@/composables/useAppVersion'
import SidebarNav from './SidebarNav.vue'

const route = useRoute()
const auth = useAuthStore()
const sse = useSSEStore()
const sidebar = useSidebar()
const logoSrc = `${import.meta.env.BASE_URL}logo-concept-1-transparent.svg`
const { version: appVersion } = useAppVersion()

const navGroups = [
  {
    items: [
      { name: 'Dashboard', to: '/dashboard', icon: HomeIcon },
      { name: 'API Docs', to: '/api-docs', icon: BookOpenIcon },
    ],
  },
  {
    label: 'Documents',
    items: [
      { name: 'Document List', to: '/documents', icon: DocumentTextIcon },
      { name: 'Trash', to: '/documents/trash', icon: TrashIcon },
    ],
  },
  {
    label: 'Content Sources',
    items: [
      { name: 'ZIM Archives', to: '/zim-archives', icon: ArchiveBoxIcon },
      { name: 'Git Templates', to: '/git-templates', icon: CodeBracketIcon },
    ],
  },
  {
    label: 'Administration',
    adminOnly: true,
    items: [
      { name: 'Users', to: '/users', icon: UserGroupIcon },
      { name: 'OAuth Clients', to: '/oauth-clients', icon: KeyIcon },
      { name: 'External Services', to: '/external-services', icon: ServerIcon },
      { name: 'Queue', to: '/queue', icon: QueueListIcon },
    ],
  },
] as const

watch(
  () => route.path,
  () => sidebar.close(),
)
</script>

<template>
  <!-- Mobile Drawer -->
  <TransitionRoot as="template" :show="sidebar.open.value">
    <Dialog as="div" class="relative z-40 lg:hidden" @close="sidebar.close()">
      <TransitionChild
        as="template"
        enter="transition-opacity ease-linear duration-300"
        enter-from="opacity-0"
        enter-to="opacity-100"
        leave="transition-opacity ease-linear duration-300"
        leave-from="opacity-100"
        leave-to="opacity-0"
      >
        <div class="fixed inset-0 bg-black/40" aria-hidden="true" />
      </TransitionChild>

      <div class="fixed inset-0 flex">
        <TransitionChild
          as="template"
          enter="transition ease-in-out duration-300 transform"
          enter-from="-translate-x-full"
          enter-to="translate-x-0"
          leave="transition ease-in-out duration-300 transform"
          leave-from="translate-x-0"
          leave-to="-translate-x-full"
        >
          <DialogPanel
            class="relative flex w-72 max-w-[85vw] flex-col bg-bg-surface-alt border-r border-border-default"
          >
            <div class="flex items-center justify-between px-4 py-4 border-b border-border-default">
              <div class="flex items-center gap-3">
                <img :src="logoSrc" alt="" aria-hidden="true" class="h-8 w-8 shrink-0" />
                <span class="text-base font-semibold text-text-primary">DocuMCP</span>
              </div>
              <button
                type="button"
                class="-m-2 p-2 rounded-md text-text-muted hover:text-text-primary hover:bg-bg-hover transition-colors"
                aria-label="Close navigation"
                @click="sidebar.close()"
              >
                <XMarkIcon class="h-6 w-6" aria-hidden="true" />
              </button>
            </div>

            <div class="flex-1 overflow-y-auto py-4">
              <SidebarNav :groups="navGroups" />
            </div>

            <div
              class="border-t border-border-default px-4 py-3 flex items-center justify-between text-xs text-text-muted"
            >
              <div class="flex items-center gap-2">
                <span
                  class="inline-block h-2 w-2 rounded-full shrink-0"
                  :class="sse.connected ? 'bg-green-500 animate-pulse' : 'bg-gray-400'"
                  aria-hidden="true"
                />
                {{ sse.connected ? 'Live' : 'Offline' }}
              </div>
              <span v-if="appVersion">v{{ appVersion }}</span>
            </div>
          </DialogPanel>
        </TransitionChild>
      </div>
    </Dialog>
  </TransitionRoot>

  <!-- Desktop Sidebar -->
  <nav
    aria-label="Main navigation"
    class="hidden lg:flex w-64 fixed top-16 bottom-0 left-0 bg-bg-surface-alt border-r border-border-default flex-col"
  >
    <div class="flex-1 overflow-y-auto py-4">
      <SidebarNav :groups="navGroups" />
    </div>

    <div
      class="border-t border-border-default px-4 py-3 flex items-center justify-between text-xs text-text-muted"
    >
      <div v-if="auth.isAdmin" class="flex items-center gap-2">
        <span
          class="inline-block h-2 w-2 rounded-full shrink-0"
          :class="sse.connected ? 'bg-green-500 animate-pulse' : 'bg-gray-400'"
          aria-hidden="true"
        />
        {{ sse.connected ? 'Live' : 'Offline' }}
      </div>
      <span v-if="appVersion">v{{ appVersion }}</span>
    </div>
  </nav>
</template>
