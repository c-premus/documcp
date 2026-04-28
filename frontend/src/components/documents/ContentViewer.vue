<script setup lang="ts">
import { computed, nextTick, onMounted, ref, watch } from 'vue'
import { marked } from 'marked'
import { sanitizeHTML } from '@/utils/sanitize'

const props = defineProps<{
  readonly content: string
  readonly fileType: string
}>()

const containerRef = ref<HTMLDivElement | null>(null)

const renderedContent = computed(() => {
  const ft = props.fileType.toLowerCase()
  if (ft === 'markdown' || ft === 'md') {
    return sanitizeHTML(marked.parse(props.content) as string)
  }
  if (ft === 'html' || ft === 'htm') {
    return sanitizeHTML(props.content)
  }
  return null
})

interface MermaidLike {
  initialize: (config: Record<string, unknown>) => void
  render: (id: string, source: string) => Promise<{ svg: string }>
}

let mermaidLoadPromise: Promise<MermaidLike> | null = null

async function loadMermaid(): Promise<MermaidLike> {
  if (mermaidLoadPromise === null) {
    mermaidLoadPromise = import('mermaid').then((mod) => {
      const isDark = document.documentElement.classList.contains('dark')
      mod.default.initialize({
        startOnLoad: false,
        securityLevel: 'strict',
        theme: isDark ? 'dark' : 'default',
      })
      return mod.default as unknown as MermaidLike
    })
  }
  return mermaidLoadPromise
}

let mermaidIdCounter = 0
const RENDERED_ATTR = 'data-mermaid-rendered'

async function renderMermaidBlocks(): Promise<void> {
  const container = containerRef.value
  if (!container) {
    return
  }
  const blocks = container.querySelectorAll<HTMLElement>(
    `code.language-mermaid:not([${RENDERED_ATTR}])`,
  )
  if (blocks.length === 0) {
    return
  }

  const mermaid = await loadMermaid()
  for (const block of Array.from(blocks)) {
    block.setAttribute(RENDERED_ATTR, 'true')
    const source = block.textContent ?? ''
    const id = `mermaid-${++mermaidIdCounter}`
    try {
      const { svg } = await mermaid.render(id, source)
      const wrapper = document.createElement('div')
      wrapper.className = 'mermaid-diagram my-4 overflow-x-auto'
      wrapper.innerHTML = svg
      const target = block.parentElement?.tagName === 'PRE' ? block.parentElement : block
      target.replaceWith(wrapper)
    } catch (err) {
      console.warn('Mermaid render failed:', err)
    }
  }
}

onMounted(async () => {
  await nextTick()
  void renderMermaidBlocks()
})

watch(renderedContent, async () => {
  await nextTick()
  void renderMermaidBlocks()
})
</script>

<template>
  <div
    v-if="renderedContent"
    ref="containerRef"
    class="prose prose-sm dark:prose-invert max-w-none"
    v-html="renderedContent"
  />
  <pre
    v-else
    class="whitespace-pre-wrap text-sm text-text-secondary font-mono bg-bg-surface-alt p-4 rounded-lg"
    >{{ content }}</pre
  >
</template>
