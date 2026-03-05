<script setup lang="ts">
import { useRoute } from 'vue-router'
import { useAuthStore } from '@/stores/auth'
import {
  HomeIcon,
  DocumentTextIcon,
  ArchiveBoxIcon,
  GlobeAltIcon,
  CodeBracketIcon,
  UserGroupIcon,
  KeyIcon,
  ServerIcon,
  QueueListIcon,
} from '@heroicons/vue/24/outline'

interface NavItem {
  readonly name: string
  readonly to: string
  readonly icon: typeof HomeIcon
  readonly adminOnly?: boolean
}

const route = useRoute()
const auth = useAuthStore()

const navItems: readonly NavItem[] = [
  { name: 'Dashboard', to: '/dashboard', icon: HomeIcon },
  { name: 'Documents', to: '/documents', icon: DocumentTextIcon },
  { name: 'ZIM Archives', to: '/zim-archives', icon: ArchiveBoxIcon },
  { name: 'Confluence Spaces', to: '/confluence-spaces', icon: GlobeAltIcon },
  { name: 'Git Templates', to: '/git-templates', icon: CodeBracketIcon },
]

const adminNavItems: readonly NavItem[] = [
  { name: 'Users', to: '/users', icon: UserGroupIcon, adminOnly: true },
  { name: 'OAuth Clients', to: '/oauth-clients', icon: KeyIcon, adminOnly: true },
  { name: 'External Services', to: '/external-services', icon: ServerIcon, adminOnly: true },
  { name: 'Queue', to: '/queue', icon: QueueListIcon, adminOnly: true },
]

function isActive(path: string): boolean {
  return route.path.startsWith(path)
}
</script>

<template>
  <nav class="hidden lg:flex w-64 fixed inset-y-0 left-0 bg-white border-r border-gray-200 flex-col">
    <div class="flex items-center h-16 px-6 border-b border-gray-200">
      <span class="text-lg font-semibold text-gray-900">DocuMCP</span>
    </div>

    <div class="flex-1 overflow-y-auto py-4">
      <ul class="space-y-1 px-3">
        <li v-for="item in navItems" :key="item.to">
          <router-link
            :to="item.to"
            class="flex items-center gap-3 rounded-md px-3 py-2 text-sm font-medium transition-colors"
            :class="
              isActive(item.to)
                ? 'bg-gray-100 text-gray-900'
                : 'text-gray-600 hover:bg-gray-50 hover:text-gray-900'
            "
          >
            <component :is="item.icon" class="h-5 w-5 shrink-0" />
            {{ item.name }}
          </router-link>
        </li>
      </ul>

      <template v-if="auth.isAdmin">
        <div class="my-4 mx-3 border-t border-gray-200" />
        <ul class="space-y-1 px-3">
          <li v-for="item in adminNavItems" :key="item.to">
            <router-link
              :to="item.to"
              class="flex items-center gap-3 rounded-md px-3 py-2 text-sm font-medium transition-colors"
              :class="
                isActive(item.to)
                  ? 'bg-gray-100 text-gray-900'
                  : 'text-gray-600 hover:bg-gray-50 hover:text-gray-900'
              "
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
