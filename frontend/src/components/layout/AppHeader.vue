<script setup lang="ts">
import { Menu, MenuButton, MenuItems, MenuItem } from '@headlessui/vue'
import {
  Bars3Icon,
  ChevronDownIcon,
  SunIcon,
  MoonIcon,
  ComputerDesktopIcon,
} from '@heroicons/vue/24/outline'
import { useAuthStore } from '@/stores/auth'
import { useSSEStore } from '@/stores/sse'
import { useTheme } from '@/composables/useTheme'
import { useSidebar } from '@/composables/useSidebar'

const auth = useAuthStore()
const sse = useSSEStore()
const { mode, setMode } = useTheme()
const sidebar = useSidebar()
const logoSrc = `${import.meta.env.BASE_URL}logo-concept-1-transparent.svg`
</script>

<template>
  <header
    class="fixed top-0 left-0 right-0 z-20 h-16 bg-bg-surface border-b border-border-default px-4"
  >
    <div class="flex h-full items-center justify-between">
      <div class="flex items-center gap-3">
        <button
          type="button"
          class="lg:hidden -m-2 p-2 rounded-md text-text-muted hover:text-text-primary hover:bg-bg-hover transition-colors"
          aria-label="Open navigation"
          @click="sidebar.toggle()"
        >
          <Bars3Icon class="h-6 w-6" aria-hidden="true" />
        </button>
        <img :src="logoSrc" alt="" aria-hidden="true" class="h-8 w-8 shrink-0" />
        <span class="hidden sm:inline text-lg font-semibold text-text-primary">DocuMCP</span>
      </div>

      <Menu v-if="auth.user" as="div" class="relative">
        <MenuButton
          class="flex items-center gap-2 rounded-md px-3 py-2 text-sm text-text-muted hover:text-text-primary hover:bg-bg-hover transition-colors cursor-pointer"
        >
          <span
            class="inline-block h-2 w-2 rounded-full shrink-0"
            :class="sse.connected ? 'bg-green-500 animate-pulse' : 'bg-gray-400'"
            role="status"
            :aria-label="
              sse.connected ? 'Connected to live updates' : 'Disconnected from live updates'
            "
          />
          <span aria-live="polite" class="sr-only">
            {{ sse.connected ? 'Connected to live updates' : 'Disconnected from live updates' }}
          </span>
          <span class="hidden sm:inline-block">{{ auth.user.name }}</span>
          <ChevronDownIcon class="h-4 w-4" aria-hidden="true" />
        </MenuButton>

        <MenuItems
          class="absolute right-0 mt-2 w-56 origin-top-right z-50 rounded-lg shadow-lg ring-1 ring-black/5 dark:ring-white/10 bg-bg-surface border border-border-default focus:outline-none"
        >
          <!-- User info -->
          <div class="px-4 py-3 border-b border-border-default">
            <p class="text-sm font-semibold text-text-primary truncate">{{ auth.user.name }}</p>
            <p class="text-xs text-text-muted truncate">{{ auth.user.email }}</p>
          </div>

          <!-- Theme selector -->
          <div class="px-3 py-2 border-b border-border-default">
            <p class="text-xs font-semibold uppercase tracking-wider text-text-disabled mb-1">
              Theme
            </p>
            <div class="flex flex-col">
              <button
                type="button"
                class="w-full rounded-md px-3 py-1.5 text-xs flex items-center gap-2 transition-colors cursor-pointer"
                :class="
                  mode === 'light'
                    ? 'bg-bg-active text-text-primary'
                    : 'text-text-muted hover:bg-bg-hover hover:text-text-primary'
                "
                @click="setMode('light')"
              >
                <SunIcon class="h-4 w-4 shrink-0" aria-hidden="true" />
                Light
              </button>
              <button
                type="button"
                class="w-full rounded-md px-3 py-1.5 text-xs flex items-center gap-2 transition-colors cursor-pointer"
                :class="
                  mode === 'dark'
                    ? 'bg-bg-active text-text-primary'
                    : 'text-text-muted hover:bg-bg-hover hover:text-text-primary'
                "
                @click="setMode('dark')"
              >
                <MoonIcon class="h-4 w-4 shrink-0" aria-hidden="true" />
                Dark
              </button>
              <button
                type="button"
                class="w-full rounded-md px-3 py-1.5 text-xs flex items-center gap-2 transition-colors cursor-pointer"
                :class="
                  mode === 'system'
                    ? 'bg-bg-active text-text-primary'
                    : 'text-text-muted hover:bg-bg-hover hover:text-text-primary'
                "
                @click="setMode('system')"
              >
                <ComputerDesktopIcon class="h-4 w-4 shrink-0" aria-hidden="true" />
                System
              </button>
            </div>
          </div>

          <!-- Logout -->
          <div class="py-1">
            <MenuItem v-slot="{ active }">
              <button
                type="button"
                class="w-full text-left px-4 py-2 text-sm transition-colors cursor-pointer"
                :class="active ? 'bg-bg-hover text-text-primary' : 'text-text-muted'"
                @click="auth.logout()"
              >
                Log out
              </button>
            </MenuItem>
          </div>
        </MenuItems>
      </Menu>
    </div>
  </header>
</template>
