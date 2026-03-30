<script setup lang="ts">
import { ref, computed, onMounted } from 'vue'
import { RouterLink } from 'vue-router'
import { toast } from 'vue-sonner'
import { sanitizeHTML } from '@/utils/sanitize'
import { ArrowLeftIcon, MagnifyingGlassIcon } from '@heroicons/vue/24/outline'

import { useZimArchivesStore } from '../stores/zimArchives'
import type { ZimSearchResult } from '../stores/zimArchives'

const props = defineProps<{
  readonly archive: string
}>()

const store = useZimArchivesStore()

const currentArchive = computed(() => {
  return store.archives.find((a) => a.name === props.archive)
})

// Ensure archives are loaded so we can check has_fulltext_index.
onMounted(() => {
  if (store.archives.length === 0) {
    store.fetchArchives().catch(() => {
      // Non-critical — search still works via /search fallback.
    })
  }
})

const searchQuery = ref('')
let searchTimer: ReturnType<typeof setTimeout> | null = null

const sanitizedContent = computed(() => {
  if (store.currentArticle === null) {
    return ''
  }
  if (store.currentArticle.mime_type.startsWith('text/html')) {
    return sanitizeHTML(store.currentArticle.content)
  }
  return ''
})

const isHtmlContent = computed(() => {
  return store.currentArticle !== null && store.currentArticle.mime_type.startsWith('text/html')
})

function handleSearchInput(event: Event): void {
  const target = event.target as HTMLInputElement
  searchQuery.value = target.value

  if (searchTimer !== null) {
    clearTimeout(searchTimer)
  }

  if (searchQuery.value.trim() === '') {
    store.clearSearch()
    return
  }

  const usesSuggest = currentArchive.value != null && !currentArchive.value.has_fulltext_index

  searchTimer = setTimeout(() => {
    store.searchArticles(props.archive, searchQuery.value.trim(), 10, usesSuggest).catch(() => {
      toast.error('Search failed')
    })
  }, 300)
}

function handleResultClick(result: ZimSearchResult): void {
  store.readArticle(props.archive, result.path).catch(() => {
    toast.error('Failed to load article')
  })
}
</script>

<template>
  <div>
    <!-- Header -->
    <div class="flex items-center gap-4 mb-6">
      <RouterLink
        to="/zim-archives"
        class="inline-flex items-center gap-1 text-sm text-text-muted hover:text-text-secondary"
      >
        <ArrowLeftIcon class="h-4 w-4" />
        Back to archives
      </RouterLink>
      <h1 class="text-2xl font-bold text-text-primary">{{ archive }}</h1>
    </div>

    <!-- Two-pane layout -->
    <div class="grid grid-cols-1 gap-6 lg:grid-cols-3">
      <!-- Left pane: Search + Results -->
      <div class="lg:col-span-1">
        <!-- Search input -->
        <div class="relative rounded-md shadow-sm mb-4">
          <div class="pointer-events-none absolute inset-y-0 left-0 flex items-center pl-3">
            <MagnifyingGlassIcon class="h-5 w-5 text-text-disabled" aria-hidden="true" />
          </div>
          <input
            type="text"
            :value="searchQuery"
            :placeholder="
              currentArchive && !currentArchive.has_fulltext_index
                ? 'Search by article title...'
                : 'Search articles...'
            "
            class="block w-full rounded-md border-0 py-1.5 pl-10 pr-3 text-text-primary bg-bg-surface ring-1 ring-inset ring-border-input placeholder:text-text-disabled focus:ring-2 focus:ring-inset focus:ring-focus sm:text-sm sm:leading-6"
            @input="handleSearchInput"
          />
        </div>

        <!-- Title-only search notice -->
        <p
          v-if="currentArchive && !currentArchive.has_fulltext_index"
          class="text-xs text-amber-600 dark:text-amber-400 mb-3"
        >
          This archive supports title matching only (no full-text content search).
        </p>

        <!-- Search loading -->
        <div v-if="store.searchLoading" class="flex items-center justify-center py-8">
          <div
            class="h-6 w-6 animate-spin rounded-full border-4 border-border-input border-t-indigo-600 dark:border-t-indigo-400"
          />
        </div>

        <!-- Search results -->
        <ul
          v-else-if="store.searchResults.length > 0"
          class="divide-y divide-border-default rounded-lg border border-border-default bg-bg-surface overflow-hidden"
        >
          <li
            v-for="result in store.searchResults"
            :key="result.path"
            class="px-4 py-3 hover:bg-bg-hover cursor-pointer"
            @click="handleResultClick(result)"
          >
            <p class="text-sm font-medium text-indigo-600 dark:text-indigo-400">
              {{ result.title }}
            </p>
            <p
              v-if="result.snippet"
              class="mt-1 text-xs text-text-muted line-clamp-2"
              v-html="sanitizeHTML(result.snippet)"
            />
          </li>
        </ul>

        <!-- Empty search state -->
        <div
          v-else-if="searchQuery.trim() !== '' && !store.searchLoading"
          class="text-center py-8 text-sm text-text-muted"
        >
          No results found.
        </div>

        <div v-else class="text-center py-8 text-sm text-text-muted">
          Search for articles in this archive.
        </div>
      </div>

      <!-- Right pane: Article content -->
      <div class="lg:col-span-2">
        <!-- Article loading -->
        <div v-if="store.articleLoading" class="flex items-center justify-center py-12">
          <div
            class="h-8 w-8 animate-spin rounded-full border-4 border-border-input border-t-indigo-600 dark:border-t-indigo-400"
          />
        </div>

        <!-- Article content -->
        <div
          v-else-if="store.currentArticle !== null"
          class="rounded-lg border border-border-default bg-bg-surface p-6"
        >
          <h2 class="text-xl font-bold text-text-primary mb-4">{{ store.currentArticle.title }}</h2>

          <!-- HTML content -->
          <div
            v-if="isHtmlContent"
            class="prose prose-sm dark:prose-invert max-w-none"
            v-html="sanitizedContent"
          />

          <!-- Plain text content -->
          <pre v-else class="whitespace-pre-wrap text-sm text-text-secondary font-mono">{{
            store.currentArticle.content
          }}</pre>
        </div>

        <!-- No article selected -->
        <div
          v-else
          class="flex flex-col items-center justify-center py-16 text-center rounded-lg border border-dashed border-border-input bg-bg-surface-alt"
        >
          <p class="text-sm text-text-muted">Search and select an article to view its content.</p>
        </div>
      </div>
    </div>
  </div>
</template>
