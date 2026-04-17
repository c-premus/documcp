<script setup lang="ts">
import { useRoute } from 'vue-router'
import { useAuthStore } from '@/stores/auth'

interface NavItem {
  readonly name: string
  readonly to: string
  readonly icon: object
  readonly external?: boolean
}

interface NavGroup {
  readonly label?: string
  readonly items: readonly NavItem[]
  readonly adminOnly?: boolean
}

defineProps<{
  groups: readonly NavGroup[]
}>()

const route = useRoute()
const auth = useAuthStore()

const linkClasses =
  'flex items-center gap-3 rounded-md px-3 py-2 text-sm font-medium transition-colors cursor-pointer'
const idleClasses = 'text-text-muted hover:bg-bg-hover hover:text-text-primary'
const activeClasses = 'bg-bg-active text-text-primary'

function isActive(path: string): boolean {
  if (path === '/documents') {
    return (
      route.path === '/documents' ||
      (route.path.startsWith('/documents/') && !route.path.startsWith('/documents/trash'))
    )
  }
  return route.path.startsWith(path)
}
</script>

<template>
  <template v-for="(group, gi) in groups" :key="gi">
    <template v-if="!group.adminOnly || auth.isAdmin">
      <div v-if="group.label" class="my-4 mx-3 border-t border-border-default" />
      <p
        v-if="group.label"
        class="px-6 mb-1 text-xs font-semibold uppercase tracking-wider text-text-muted"
      >
        {{ group.label }}
      </p>
      <ul class="space-y-1 px-3">
        <li v-for="item in group.items" :key="item.to">
          <a
            v-if="item.external"
            :href="item.to"
            target="_blank"
            rel="noopener noreferrer"
            :class="[linkClasses, idleClasses]"
          >
            <component :is="item.icon" class="h-5 w-5 shrink-0" />
            {{ item.name }}
          </a>
          <router-link
            v-else
            :to="item.to"
            :class="[linkClasses, isActive(item.to) ? activeClasses : idleClasses]"
            :aria-current="isActive(item.to) ? 'page' : undefined"
          >
            <component :is="item.icon" class="h-5 w-5 shrink-0" />
            {{ item.name }}
          </router-link>
        </li>
      </ul>
    </template>
  </template>
</template>
