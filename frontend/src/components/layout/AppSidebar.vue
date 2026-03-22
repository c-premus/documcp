<script setup lang="ts">
import { useRoute } from 'vue-router'
import { useAuthStore } from '@/stores/auth'
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
} from '@heroicons/vue/24/outline'

interface NavItem {
  readonly name: string
  readonly to: string
  readonly icon: typeof HomeIcon
}

const route = useRoute()
const auth = useAuthStore()

const mainNavItems: readonly NavItem[] = [
  { name: 'Dashboard', to: '/dashboard', icon: HomeIcon },
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
    return route.path === '/documents' || route.path.startsWith('/documents/') && !route.path.startsWith('/documents/trash')
  }
  return route.path.startsWith(path)
}
</script>

<template>
  <nav aria-label="Main navigation" class="hidden lg:flex w-64 fixed top-16 bottom-0 left-0 bg-bg-surface border-r border-border-default flex-col">
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
      <p class="px-6 mb-1 text-xs font-semibold uppercase tracking-wider text-text-disabled">Documents</p>
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
      <p class="px-6 mb-1 text-xs font-semibold uppercase tracking-wider text-text-disabled">Content Sources</p>
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
        <p class="px-6 mb-1 text-xs font-semibold uppercase tracking-wider text-text-disabled">Administration</p>
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
  </nav>
</template>
