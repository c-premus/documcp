<script setup lang="ts">
import FileTypeCell from '@/components/shared/FileTypeCell.vue'
import RelativeTimeCell from '@/components/shared/RelativeTimeCell.vue'
import DocumentTrashRowActions from './DocumentTrashRowActions.vue'
import type { Document } from '@/stores/documents'

defineProps<{
  readonly document: Document
  readonly canPurge: boolean
}>()

const emit = defineEmits<{
  restore: [document: Document]
  purge: [document: Document]
}>()

function onRestore(document: Document): void {
  emit('restore', document)
}
function onPurge(document: Document): void {
  emit('purge', document)
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
        <span>Deleted</span>
        <RelativeTimeCell :value="document.updated_at" />
      </div>
    </div>
    <DocumentTrashRowActions
      :document="document"
      :can-purge="canPurge"
      @restore="onRestore"
      @purge="onPurge"
    />
  </div>
</template>
