<script setup lang="ts">
import CategoryBadge from '@/components/shared/CategoryBadge.vue'
import RelativeTimeCell from '@/components/shared/RelativeTimeCell.vue'
import TruncatedText from '@/components/shared/TruncatedText.vue'
import GitTemplateRowActions from './GitTemplateRowActions.vue'
import type { GitTemplate } from '@/stores/gitTemplates'

const CATEGORY_PALETTE: Readonly<Record<string, string>> = {
  claude: 'bg-violet-100 text-violet-800 dark:bg-violet-900/30 dark:text-violet-300',
  'memory-bank': 'bg-amber-100 text-amber-800 dark:bg-amber-900/30 dark:text-amber-300',
  project: 'bg-emerald-100 text-emerald-800 dark:bg-emerald-900/30 dark:text-emerald-300',
}

const props = defineProps<{
  readonly template: GitTemplate
  readonly syncing: boolean
  readonly isAdmin: boolean
}>()

const emit = defineEmits<{
  edit: [template: GitTemplate]
  sync: [template: GitTemplate]
  delete: [template: GitTemplate]
}>()

function onEdit(): void {
  emit('edit', props.template)
}
function onSync(): void {
  emit('sync', props.template)
}
function onDelete(): void {
  emit('delete', props.template)
}
</script>

<template>
  <div class="flex flex-col gap-1.5 px-4 py-3">
    <div class="flex items-start gap-3">
      <h3 class="min-w-0 flex-1 truncate text-sm font-medium text-text-primary">
        {{ template.name }}
      </h3>
      <GitTemplateRowActions
        v-if="isAdmin"
        :template="template"
        :syncing="syncing"
        @edit="onEdit"
        @sync="onSync"
        @delete="onDelete"
      />
    </div>
    <p v-if="template.description" class="truncate text-xs text-text-muted">
      {{ template.description }}
    </p>
    <div class="flex items-center gap-2 text-xs text-text-muted">
      <TruncatedText :value="template.repository_url" :mono="true" />
    </div>
    <div class="flex flex-wrap items-center gap-x-3 gap-y-1">
      <CategoryBadge
        v-if="template.category"
        :value="template.category"
        :palette="CATEGORY_PALETTE"
      />
      <span class="font-mono text-xs text-text-muted">{{ template.branch }}</span>
    </div>
    <div class="flex flex-wrap items-center gap-x-1.5 gap-y-0.5 text-xs text-text-muted">
      <span>{{ template.file_count }} files</span>
      <span aria-hidden="true">·</span>
      <span>Synced</span>
      <RelativeTimeCell :value="template.last_synced_at ?? null" fallback="Never" />
    </div>
  </div>
</template>
