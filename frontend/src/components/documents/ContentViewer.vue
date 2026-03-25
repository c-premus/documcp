<script setup lang="ts">
import { computed } from 'vue'
import { marked } from 'marked'
import DOMPurify from 'dompurify'

const props = defineProps<{
  readonly content: string
  readonly fileType: string
}>()

const renderedContent = computed(() => {
  const ft = props.fileType.toLowerCase()
  if (ft === 'markdown' || ft === 'md') {
    return DOMPurify.sanitize(marked.parse(props.content) as string)
  }
  if (ft === 'html' || ft === 'htm') {
    return DOMPurify.sanitize(props.content)
  }
  return null
})
</script>

<template>
  <div
    v-if="renderedContent"
    class="prose prose-sm dark:prose-invert max-w-none"
    v-html="renderedContent"
  />
  <pre
    v-else
    class="whitespace-pre-wrap text-sm text-text-secondary font-mono bg-bg-surface-alt p-4 rounded-lg"
    >{{ content }}</pre
  >
</template>
