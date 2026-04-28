<script setup lang="ts">
import { computed } from 'vue'
import { formatDistanceToNow } from 'date-fns'
import StatusBadge from '@/components/shared/StatusBadge.vue'
import type { ZimArchive } from '@/stores/zimArchives'

const props = defineProps<{
  readonly archive: ZimArchive
}>()

const lastSyncedDisplay = computed(() => {
  if (!props.archive.last_synced_at) {
    return 'Never synced'
  }
  return `synced ${formatDistanceToNow(new Date(props.archive.last_synced_at), { addSuffix: true })}`
})

const articleCountDisplay = computed(() => props.archive.article_count.toLocaleString())
</script>

<template>
  <div class="flex flex-col gap-1.5 px-4 py-3">
    <h3 class="truncate text-sm font-medium text-indigo-600 dark:text-indigo-400">
      {{ archive.name }}
    </h3>
    <p v-if="archive.title" class="truncate text-xs text-text-muted">{{ archive.title }}</p>
    <div class="flex flex-wrap items-center gap-1.5">
      <StatusBadge v-if="archive.category" :status="archive.category" />
      <StatusBadge :status="archive.has_fulltext_index ? 'fulltext' : 'title only'" />
    </div>
    <div class="flex flex-wrap items-center gap-x-1.5 gap-y-0.5 text-xs text-text-muted">
      <span>{{ articleCountDisplay }} articles</span>
      <span aria-hidden="true">·</span>
      <span>{{ archive.file_size_human }}</span>
      <span aria-hidden="true">·</span>
      <span>{{ archive.language.toUpperCase() }}</span>
      <span aria-hidden="true">·</span>
      <span>{{ lastSyncedDisplay }}</span>
    </div>
  </div>
</template>
