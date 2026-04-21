<script setup lang="ts">
import { ref, onMounted } from 'vue'
import { RouterLink } from 'vue-router'
import { apiFetch, ApiError } from '@/api/helpers'
import {
  DocumentTextIcon,
  UsersIcon,
  KeyIcon,
  ServerIcon,
  BookOpenIcon,
  CodeBracketIcon,
  ClockIcon,
  CheckCircleIcon,
  ExclamationCircleIcon,
  MagnifyingGlassIcon,
} from '@heroicons/vue/24/outline'
import { useAuthStore } from '@/stores/auth'

const auth = useAuthStore()

interface QueueStats {
  readonly pending: number
  readonly completed: number
  readonly failed: number
}

interface DashboardStats {
  readonly documents: number
  readonly users: number
  readonly oauth_clients: number
  readonly external_services: number
  readonly zim_archives: number
  readonly git_templates: number
  readonly queue?: QueueStats
}

interface StatCard {
  readonly label: string
  readonly key: keyof Omit<DashboardStats, 'queue'>
  readonly icon: typeof DocumentTextIcon
  readonly color: string
  readonly route: string
}

interface QuickLink {
  readonly label: string
  readonly description: string
  readonly icon: typeof DocumentTextIcon
  readonly color: string
  readonly route: string
}

const stats = ref<DashboardStats | null>(null)
const loading = ref(true)
const error = ref<string | null>(null)

const cards: readonly StatCard[] = [
  {
    label: 'Documents',
    key: 'documents',
    icon: DocumentTextIcon,
    color: 'text-indigo-600 dark:text-indigo-400',
    route: '/documents',
  },
  {
    label: 'Users',
    key: 'users',
    icon: UsersIcon,
    color: 'text-blue-600 dark:text-blue-400',
    route: '/users',
  },
  {
    label: 'OAuth Clients',
    key: 'oauth_clients',
    icon: KeyIcon,
    color: 'text-purple-600 dark:text-purple-400',
    route: '/oauth-clients',
  },
  {
    label: 'External Services',
    key: 'external_services',
    icon: ServerIcon,
    color: 'text-teal-600 dark:text-teal-400',
    route: '/external-services',
  },
  {
    label: 'ZIM Archives',
    key: 'zim_archives',
    icon: BookOpenIcon,
    color: 'text-amber-600 dark:text-amber-400',
    route: '/zim-archives',
  },
  {
    label: 'Git Templates',
    key: 'git_templates',
    icon: CodeBracketIcon,
    color: 'text-emerald-600 dark:text-emerald-400',
    route: '/git-templates',
  },
] as const

const quickLinks: readonly QuickLink[] = [
  {
    label: 'Documents',
    description: 'Browse and search available documents',
    icon: DocumentTextIcon,
    color: 'text-indigo-600 dark:text-indigo-400',
    route: '/documents',
  },
  {
    label: 'Search',
    description: 'Search across all content sources',
    icon: MagnifyingGlassIcon,
    color: 'text-blue-600 dark:text-blue-400',
    route: '/documents',
  },
  {
    label: 'ZIM Archives',
    description: 'Browse offline knowledge bases',
    icon: BookOpenIcon,
    color: 'text-amber-600 dark:text-amber-400',
    route: '/zim-archives',
  },
  {
    label: 'Git Templates',
    description: 'Explore project templates',
    icon: CodeBracketIcon,
    color: 'text-emerald-600 dark:text-emerald-400',
    route: '/git-templates',
  },
] as const

async function fetchStats(): Promise<void> {
  loading.value = true
  error.value = null
  try {
    const body = await apiFetch<{ data: DashboardStats }>('/api/admin/dashboard/stats')
    stats.value = body.data
  } catch (e) {
    error.value = e instanceof ApiError || e instanceof Error
      ? e.message
      : 'Failed to fetch dashboard stats'
  } finally {
    loading.value = false
  }
}

onMounted(() => {
  if (auth.isAdmin) {
    fetchStats()
  } else {
    loading.value = false
  }
})
</script>

<template>
  <div>
    <div class="mb-8">
      <h1 class="text-2xl font-bold text-text-primary">Dashboard</h1>
      <p class="mt-1 text-sm text-text-muted">
        {{ auth.isAdmin ? 'System overview' : `Welcome, ${auth.user?.name ?? 'User'}` }}
      </p>
    </div>

    <!-- Loading spinner -->
    <div v-if="loading" class="flex items-center justify-center py-20">
      <svg
        class="h-8 w-8 animate-spin text-indigo-600 dark:text-indigo-400"
        xmlns="http://www.w3.org/2000/svg"
        fill="none"
        viewBox="0 0 24 24"
      >
        <circle class="opacity-25" cx="12" cy="12" r="10" stroke="currentColor" stroke-width="4" />
        <path
          class="opacity-75"
          fill="currentColor"
          d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4z"
        />
      </svg>
    </div>

    <!-- Non-admin: quick links -->
    <template v-else-if="!auth.isAdmin">
      <div class="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-3 gap-4">
        <RouterLink
          v-for="link in quickLinks"
          :key="link.route"
          :to="link.route"
          class="rounded-lg border border-border-default bg-bg-surface p-5 shadow-sm transition hover:shadow-md"
        >
          <component :is="link.icon" class="h-6 w-6" :class="link.color" />
          <p class="mt-3 text-sm font-medium text-text-primary">{{ link.label }}</p>
          <p class="mt-1 text-xs text-text-muted">{{ link.description }}</p>
        </RouterLink>
      </div>
    </template>

    <!-- Admin: Error state -->
    <div
      v-else-if="error !== null"
      class="rounded-lg border border-red-200 dark:border-red-800 bg-red-50 dark:bg-red-900/20 p-5 text-sm text-red-700 dark:text-red-300"
    >
      {{ error }}
    </div>

    <!-- Admin: Stats grid -->
    <template v-else-if="stats !== null">
      <div class="grid grid-cols-3 gap-4">
        <RouterLink
          v-for="card in cards"
          :key="card.key"
          :to="card.route"
          class="rounded-lg border border-border-default bg-bg-surface p-5 shadow-sm transition hover:shadow-md"
        >
          <component :is="card.icon" class="h-6 w-6" :class="card.color" />
          <p class="mt-3 text-sm text-text-muted">{{ card.label }}</p>
          <p class="mt-1 text-2xl font-semibold text-text-primary">
            {{ stats[card.key] }}
          </p>
        </RouterLink>
      </div>

      <!-- Queue section -->
      <div v-if="stats.queue" class="mt-8">
        <h2 class="mb-4 text-lg font-semibold text-text-primary">Job Queue</h2>
        <div class="grid grid-cols-3 gap-4">
          <RouterLink
            to="/queue"
            class="rounded-lg border border-border-default bg-bg-surface p-5 shadow-sm transition hover:shadow-md"
          >
            <ClockIcon class="h-5 w-5 text-yellow-500" />
            <p class="mt-2 text-sm text-text-muted">Pending</p>
            <p class="mt-1 text-2xl font-semibold text-yellow-600 dark:text-yellow-400">
              {{ stats.queue.pending }}
            </p>
          </RouterLink>
          <RouterLink
            to="/queue"
            class="rounded-lg border border-border-default bg-bg-surface p-5 shadow-sm transition hover:shadow-md"
          >
            <CheckCircleIcon class="h-5 w-5 text-green-500" />
            <p class="mt-2 text-sm text-text-muted">Completed</p>
            <p class="mt-1 text-2xl font-semibold text-green-600 dark:text-green-400">
              {{ stats.queue.completed }}
            </p>
          </RouterLink>
          <RouterLink
            to="/queue"
            class="rounded-lg border border-border-default bg-bg-surface p-5 shadow-sm transition hover:shadow-md"
          >
            <ExclamationCircleIcon class="h-5 w-5 text-red-500" />
            <p class="mt-2 text-sm text-text-muted">Failed</p>
            <p class="mt-1 text-2xl font-semibold text-red-600 dark:text-red-400">
              {{ stats.queue.failed }}
            </p>
          </RouterLink>
        </div>
      </div>
    </template>
  </div>
</template>
