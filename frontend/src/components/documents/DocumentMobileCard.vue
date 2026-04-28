<script setup lang="ts">
import StatusBadge from '@/components/shared/StatusBadge.vue'
import VisibilityCell from '@/components/shared/VisibilityCell.vue'
import FileTypeCell from '@/components/shared/FileTypeCell.vue'
import FileSizeCell from '@/components/shared/FileSizeCell.vue'
import RelativeTimeCell from '@/components/shared/RelativeTimeCell.vue'
import DocumentRowActions from './DocumentRowActions.vue'
import type { Document } from '@/stores/documents'

defineProps<{
  readonly document: Document
}>()

const emit = defineEmits<{
  edit: [document: Document]
  view: [document: Document]
  delete: [document: Document]
}>()

function onEdit(document: Document): void {
  emit('edit', document)
}
function onView(document: Document): void {
  emit('view', document)
}
function onDelete(document: Document): void {
  emit('delete', document)
}
</script>

<template>
  <div class="flex items-start gap-3 px-4 py-3">
    <div class="min-w-0 flex-1">
      <h3 class="truncate text-sm font-medium text-text-primary">
        {{ document.title }}
      </h3>
      <div class="mt-1 flex flex-wrap items-center gap-x-1.5 gap-y-0.5 text-xs text-text-muted">
        <FileTypeCell :value="document.file_type" />
        <span aria-hidden="true">·</span>
        <FileSizeCell :bytes="document.file_size" />
        <span aria-hidden="true">·</span>
        <RelativeTimeCell :value="document.created_at" />
      </div>
      <div class="mt-2 flex flex-wrap items-center gap-1.5">
        <StatusBadge :status="document.status" />
        <VisibilityCell :is-public="document.is_public" />
      </div>
    </div>
    <DocumentRowActions :document="document" @edit="onEdit" @view="onView" @delete="onDelete" />
  </div>
</template>
