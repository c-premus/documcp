<script setup lang="ts">
import { ref, onMounted } from 'vue'
import { RouterLink } from 'vue-router'
import {
  DocumentTextIcon,
  UsersIcon,
  KeyIcon,
  ServerIcon,
  BookOpenIcon,
  CloudIcon,
  CodeBracketIcon,
  ClockIcon,
  CheckCircleIcon,
  ExclamationCircleIcon,
} from '@heroicons/vue/24/outline'

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
  readonly confluence_spaces: number
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

const stats = ref<DashboardStats | null>(null)
const loading = ref(true)
const error = ref<string | null>(null)

const cards: readonly StatCard[] = [
  { label: 'Documents', key: 'documents', icon: DocumentTextIcon, color: 'text-indigo-600', route: '/documents' },
  { label: 'Users', key: 'users', icon: UsersIcon, color: 'text-blue-600', route: '/users' },
  { label: 'OAuth Clients', key: 'oauth_clients', icon: KeyIcon, color: 'text-purple-600', route: '/oauth-clients' },
  { label: 'External Services', key: 'external_services', icon: ServerIcon, color: 'text-teal-600', route: '/external-services' },
  { label: 'ZIM Archives', key: 'zim_archives', icon: BookOpenIcon, color: 'text-amber-600', route: '/zim-archives' },
  { label: 'Confluence Spaces', key: 'confluence_spaces', icon: CloudIcon, color: 'text-cyan-600', route: '/confluence-spaces' },
  { label: 'Git Templates', key: 'git_templates', icon: CodeBracketIcon, color: 'text-emerald-600', route: '/git-templates' },
] as const

async function fetchStats(): Promise<void> {
  loading.value = true
  error.value = null
  try {
    const res = await fetch('/api/admin/dashboard/stats')
    if (!res.ok) {
      const body = await res.json().catch(() => ({ message: res.statusText }))
      throw new Error(body.message || res.statusText)
    }
    const body = await res.json()
    stats.value = body.data as DashboardStats
  } catch (e) {
    error.value = e instanceof Error ? e.message : 'Failed to fetch dashboard stats'
  } finally {
    loading.value = false
  }
}

onMounted(fetchStats)
</script>

<template>
  <div>
    <div class="mb-8">
      <h1 class="text-2xl font-bold text-gray-900">Dashboard</h1>
      <p class="mt-1 text-sm text-gray-500">System overview</p>
    </div>

    <!-- Loading spinner -->
    <div
      v-if="loading"
      class="flex items-center justify-center py-20"
    >
      <svg
        class="h-8 w-8 animate-spin text-indigo-600"
        xmlns="http://www.w3.org/2000/svg"
        fill="none"
        viewBox="0 0 24 24"
      >
        <circle
          class="opacity-25"
          cx="12"
          cy="12"
          r="10"
          stroke="currentColor"
          stroke-width="4"
        />
        <path
          class="opacity-75"
          fill="currentColor"
          d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4z"
        />
      </svg>
    </div>

    <!-- Error state -->
    <div
      v-else-if="error !== null"
      class="rounded-lg border border-red-200 bg-red-50 p-5 text-sm text-red-700"
    >
      {{ error }}
    </div>

    <!-- Stats grid -->
    <template v-else-if="stats !== null">
      <div class="grid grid-cols-2 gap-4 sm:grid-cols-3 lg:grid-cols-4">
        <RouterLink
          v-for="card in cards"
          :key="card.key"
          :to="card.route"
          class="rounded-lg border border-gray-200 bg-white p-5 shadow-sm transition hover:shadow-md"
        >
          <component
            :is="card.icon"
            class="h-6 w-6"
            :class="card.color"
          />
          <p class="mt-3 text-sm text-gray-500">{{ card.label }}</p>
          <p class="mt-1 text-2xl font-semibold text-gray-900">
            {{ stats[card.key] }}
          </p>
        </RouterLink>
      </div>

      <!-- Queue section -->
      <div
        v-if="stats.queue"
        class="mt-8"
      >
        <h2 class="mb-4 text-lg font-semibold text-gray-900">Job Queue</h2>
        <div class="grid grid-cols-3 gap-4">
          <RouterLink
            to="/queue"
            class="rounded-lg border border-gray-200 bg-white p-5 shadow-sm transition hover:shadow-md"
          >
            <ClockIcon class="h-5 w-5 text-yellow-500" />
            <p class="mt-2 text-sm text-gray-500">Pending</p>
            <p class="mt-1 text-2xl font-semibold text-yellow-600">
              {{ stats.queue.pending }}
            </p>
          </RouterLink>
          <RouterLink
            to="/queue"
            class="rounded-lg border border-gray-200 bg-white p-5 shadow-sm transition hover:shadow-md"
          >
            <CheckCircleIcon class="h-5 w-5 text-green-500" />
            <p class="mt-2 text-sm text-gray-500">Completed</p>
            <p class="mt-1 text-2xl font-semibold text-green-600">
              {{ stats.queue.completed }}
            </p>
          </RouterLink>
          <RouterLink
            to="/queue"
            class="rounded-lg border border-gray-200 bg-white p-5 shadow-sm transition hover:shadow-md"
          >
            <ExclamationCircleIcon class="h-5 w-5 text-red-500" />
            <p class="mt-2 text-sm text-gray-500">Failed</p>
            <p class="mt-1 text-2xl font-semibold text-red-600">
              {{ stats.queue.failed }}
            </p>
          </RouterLink>
        </div>
      </div>
    </template>
  </div>
</template>
