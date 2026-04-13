<script setup lang="ts">
import { ref, watch, onMounted } from 'vue'
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

interface NavItem {
  readonly name: string
  readonly to: string
  readonly icon: typeof HomeIcon
}

const route = useRoute()
const auth = useAuthStore()
const sse = useSSEStore()
const sidebar = useSidebar()
const logoSrc = `${import.meta.env.BASE_URL}logo-concept-1-transparent.svg`
const appVersion = ref('')

onMounted(async () => {
  try {
    const res = await fetch('/health')
    if (res.ok) {
      const data = await res.json()
      if (data.version) appVersion.value = data.version
    }
  } catch {
    // Version display is non-critical
  }
})

const mainNavItems: readonly NavItem[] = [
  { name: 'Dashboard', to: '/dashboard', icon: HomeIcon },
  { name: 'API Docs', to: '/api-docs', icon: BookOpenIcon },
]

const documentNavItems: readonly NavItem[] = [
  { name: 'Document List', to: '/documents', icon: DocumentTextIcon },
  { name: 'Trash', to: '/documents/trash', icon: TrashIcon },
]

const contentNavItems: readonly NavItem[] = [
  { name: 'ZIM Archives', to: '/zim-archives', icon: ArchiveBoxIcon },
  { name: 'Git Templates', to: '/git-templates', icon: CodeBracketIcon },
]

const adminNavItems: readonly NavItem[] = [
  { name: 'Users', to: '/users', icon: UserGroupIcon },
  { name: 'OAuth Clients', to: '/oauth-clients', icon: KeyIcon },
  { name: 'External Services', to: '/external-services', icon: ServerIcon },
  { name: 'Queue', to: '/queue', icon: QueueListIcon },
]

function isActive(path: string): boolean {
  if (path === '/documents') {
    return (
      route.path === '/documents' ||
      (route.path.startsWith('/documents/') && !route.path.startsWith('/documents/trash'))
    )
  }
  return route.path.startsWith(path)
}

watch(
  () => route.path,
  () => sidebar.close(),
)
</script>

<template>
  <!-- Mobile Drawer -->
  <TransitionRoot as="template" :show="sidebar.open.value">
    <Dialog as="div" class="relative z-40 lg:hidden" @close="sidebar.close()">
      <!-- Backdrop -->
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

      <!-- Sliding panel -->
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
            <!-- Drawer header row -->
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

            <!-- Nav content -->
            <div class="flex-1 overflow-y-auto py-4">
              <!-- Main -->
              <ul class="space-y-1 px-3">
                <li v-for="item in mainNavItems" :key="item.to">
                  <router-link
                    :to="item.to"
                    class="flex items-center gap-3 rounded-md px-3 py-2 text-sm font-medium transition-colors cursor-pointer"
                    :class="
                      isActive(item.to)
                        ? 'bg-bg-active text-text-primary'
                        : 'text-text-muted hover:bg-bg-hover hover:text-text-primary'
                    "
                    :aria-current="isActive(item.to) ? 'page' : undefined"
                  >
                    <component :is="item.icon" class="h-5 w-5 shrink-0" />
                    {{ item.name }}
                  </router-link>
                </li>
              </ul>

              <!-- Documents -->
              <div class="my-4 mx-3 border-t border-border-default" />
              <p class="px-6 mb-1 text-xs font-semibold uppercase tracking-wider text-text-muted">
                Documents
              </p>
              <ul class="space-y-1 px-3">
                <li v-for="item in documentNavItems" :key="item.to">
                  <router-link
                    :to="item.to"
                    class="flex items-center gap-3 rounded-md px-3 py-2 text-sm font-medium transition-colors cursor-pointer"
                    :class="
                      isActive(item.to)
                        ? 'bg-bg-active text-text-primary'
                        : 'text-text-muted hover:bg-bg-hover hover:text-text-primary'
                    "
                    :aria-current="isActive(item.to) ? 'page' : undefined"
                  >
                    <component :is="item.icon" class="h-5 w-5 shrink-0" />
                    {{ item.name }}
                  </router-link>
                </li>
              </ul>

              <!-- Content Sources -->
              <div class="my-4 mx-3 border-t border-border-default" />
              <p class="px-6 mb-1 text-xs font-semibold uppercase tracking-wider text-text-muted">
                Content Sources
              </p>
              <ul class="space-y-1 px-3">
                <li v-for="item in contentNavItems" :key="item.to">
                  <router-link
                    :to="item.to"
                    class="flex items-center gap-3 rounded-md px-3 py-2 text-sm font-medium transition-colors cursor-pointer"
                    :class="
                      isActive(item.to)
                        ? 'bg-bg-active text-text-primary'
                        : 'text-text-muted hover:bg-bg-hover hover:text-text-primary'
                    "
                    :aria-current="isActive(item.to) ? 'page' : undefined"
                  >
                    <component :is="item.icon" class="h-5 w-5 shrink-0" />
                    {{ item.name }}
                  </router-link>
                </li>
              </ul>

              <!-- Administration -->
              <template v-if="auth.isAdmin">
                <div class="my-4 mx-3 border-t border-border-default" />
                <p class="px-6 mb-1 text-xs font-semibold uppercase tracking-wider text-text-muted">
                  Administration
                </p>
                <ul class="space-y-1 px-3">
                  <li v-for="item in adminNavItems" :key="item.to">
                    <router-link
                      :to="item.to"
                      class="flex items-center gap-3 rounded-md px-3 py-2 text-sm font-medium transition-colors cursor-pointer"
                      :class="
                        isActive(item.to)
                          ? 'bg-bg-active text-text-primary'
                          : 'text-text-muted hover:bg-bg-hover hover:text-text-primary'
                      "
                      :aria-current="isActive(item.to) ? 'page' : undefined"
                    >
                      <component :is="item.icon" class="h-5 w-5 shrink-0" />
                      {{ item.name }}
                    </router-link>
                  </li>
                </ul>
              </template>
            </div>

            <!-- Status footer -->
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
      <!-- Main -->
      <ul class="space-y-1 px-3">
        <li v-for="item in mainNavItems" :key="item.to">
          <router-link
            :to="item.to"
            class="flex items-center gap-3 rounded-md px-3 py-2 text-sm font-medium transition-colors cursor-pointer"
            :class="
              isActive(item.to)
                ? 'bg-bg-active text-text-primary'
                : 'text-text-muted hover:bg-bg-hover hover:text-text-primary'
            "
            :aria-current="isActive(item.to) ? 'page' : undefined"
          >
            <component :is="item.icon" class="h-5 w-5 shrink-0" />
            {{ item.name }}
          </router-link>
        </li>
      </ul>

      <!-- Documents -->
      <div class="my-4 mx-3 border-t border-border-default" />
      <p class="px-6 mb-1 text-xs font-semibold uppercase tracking-wider text-text-muted">
        Documents
      </p>
      <ul class="space-y-1 px-3">
        <li v-for="item in documentNavItems" :key="item.to">
          <router-link
            :to="item.to"
            class="flex items-center gap-3 rounded-md px-3 py-2 text-sm font-medium transition-colors cursor-pointer"
            :class="
              isActive(item.to)
                ? 'bg-bg-active text-text-primary'
                : 'text-text-muted hover:bg-bg-hover hover:text-text-primary'
            "
            :aria-current="isActive(item.to) ? 'page' : undefined"
          >
            <component :is="item.icon" class="h-5 w-5 shrink-0" />
            {{ item.name }}
          </router-link>
        </li>
      </ul>

      <!-- Content Sources -->
      <div class="my-4 mx-3 border-t border-border-default" />
      <p class="px-6 mb-1 text-xs font-semibold uppercase tracking-wider text-text-muted">
        Content Sources
      </p>
      <ul class="space-y-1 px-3">
        <li v-for="item in contentNavItems" :key="item.to">
          <router-link
            :to="item.to"
            class="flex items-center gap-3 rounded-md px-3 py-2 text-sm font-medium transition-colors cursor-pointer"
            :class="
              isActive(item.to)
                ? 'bg-bg-active text-text-primary'
                : 'text-text-muted hover:bg-bg-hover hover:text-text-primary'
            "
            :aria-current="isActive(item.to) ? 'page' : undefined"
          >
            <component :is="item.icon" class="h-5 w-5 shrink-0" />
            {{ item.name }}
          </router-link>
        </li>
      </ul>

      <!-- Administration -->
      <template v-if="auth.isAdmin">
        <div class="my-4 mx-3 border-t border-border-default" />
        <p class="px-6 mb-1 text-xs font-semibold uppercase tracking-wider text-text-muted">
          Administration
        </p>
        <ul class="space-y-1 px-3">
          <li v-for="item in adminNavItems" :key="item.to">
            <router-link
              :to="item.to"
              class="flex items-center gap-3 rounded-md px-3 py-2 text-sm font-medium transition-colors cursor-pointer"
              :class="
                isActive(item.to)
                  ? 'bg-bg-active text-text-primary'
                  : 'text-text-muted hover:bg-bg-hover hover:text-text-primary'
              "
              :aria-current="isActive(item.to) ? 'page' : undefined"
            >
              <component :is="item.icon" class="h-5 w-5 shrink-0" />
              {{ item.name }}
            </router-link>
          </li>
        </ul>
      </template>
    </div>

    <!-- Status footer -->
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
