<script setup lang="ts">
import { ref, computed, onMounted } from 'vue'
import { useRouter } from 'vue-router'
import { toast } from 'vue-sonner'
import { ArrowLeftIcon } from '@heroicons/vue/24/outline'

import TreeNode from '../components/shared/TreeNode.vue'
import ContentViewer from '../components/documents/ContentViewer.vue'
import { useGitTemplatesStore, buildTree } from '../stores/gitTemplates'
import type { TreeItem } from '../stores/gitTemplates'

const props = defineProps<{
  readonly uuid: string
}>()

const router = useRouter()
const store = useGitTemplatesStore()

const templateName = ref('')
const tree = ref<TreeItem[]>([])
const loading = ref(false)
const selectedPath = ref('')
const fileContent = ref('')
const fileLoading = ref(false)
const fileName = ref('')

const contentFileType = computed(() => {
  const lower = fileName.value.toLowerCase()
  if (lower.endsWith('.md') || lower.endsWith('.markdown')) {
    return 'markdown'
  }
  if (lower.endsWith('.html') || lower.endsWith('.htm')) {
    return 'html'
  }
  return 'text'
})

const isProseFile = computed(
  () => contentFileType.value === 'markdown' || contentFileType.value === 'html',
)

onMounted(async () => {
  loading.value = true
  try {
    const structure = await store.fetchStructure(props.uuid)
    templateName.value = structure.name
    tree.value = buildTree(structure.files)
  } catch {
    toast.error('Failed to load template structure')
  } finally {
    loading.value = false
  }
})

async function handleFileSelect(path: string): Promise<void> {
  selectedPath.value = path
  fileLoading.value = true
  fileContent.value = ''
  fileName.value = ''

  try {
    const file = await store.readFile(props.uuid, path)
    fileContent.value = file.content
    fileName.value = file.filename
  } catch {
    toast.error('Failed to load file content')
    fileContent.value = ''
  } finally {
    fileLoading.value = false
  }
}

function backToTree(): void {
  selectedPath.value = ''
  fileContent.value = ''
  fileName.value = ''
}

function goBack(): void {
  router.push('/git-templates')
}
</script>

<template>
  <div>
    <!-- Header -->
    <div class="flex items-center gap-3 mb-4">
      <button
        type="button"
        class="text-text-muted hover:text-text-secondary"
        title="Back to templates"
        aria-label="Back to templates"
        @click="goBack"
      >
        <ArrowLeftIcon class="h-5 w-5" />
      </button>
      <h1 class="text-2xl font-bold text-text-primary">{{ templateName || 'Template Files' }}</h1>
    </div>

    <!-- Loading -->
    <div v-if="loading" class="flex items-center justify-center py-12">
      <div
        class="h-8 w-8 animate-spin rounded-full border-4 border-border-input border-t-indigo-600 dark:border-t-indigo-400"
      />
    </div>

    <!-- Two-pane on md+, state-machine pane swap on <md -->
    <div v-else class="flex flex-col gap-4 md:h-[calc(100vh-12rem)] md:flex-row">
      <!-- Left sidebar: file tree -->
      <div
        :class="[
          'overflow-y-auto rounded-lg border border-border-default bg-bg-surface p-2',
          'md:w-72 md:shrink-0',
          selectedPath ? 'hidden md:block' : '',
        ]"
      >
        <template v-if="tree.length > 0">
          <TreeNode
            v-for="item in tree"
            :key="item.path"
            :item="item"
            :selected-path="selectedPath"
            @select="handleFileSelect"
          />
        </template>
        <p v-else class="px-2 py-4 text-sm text-text-muted text-center">No files found.</p>
      </div>

      <!-- Right main: file content viewer -->
      <div
        :class="[
          'overflow-hidden rounded-lg border border-border-default bg-bg-surface flex flex-col',
          'md:flex-1',
          !selectedPath ? 'hidden md:flex' : '',
        ]"
      >
        <template v-if="selectedPath === ''">
          <div class="flex items-center justify-center h-full text-text-disabled text-sm py-12">
            Select a file to view its contents.
          </div>
        </template>
        <template v-else>
          <!-- File header (mobile-only back button + path) -->
          <div
            class="flex items-center gap-2 border-b border-border-default px-4 py-2 bg-bg-surface-alt"
          >
            <button
              type="button"
              class="md:hidden text-text-muted hover:text-text-secondary"
              title="Back to files"
              aria-label="Back to files"
              @click="backToTree"
            >
              <ArrowLeftIcon class="h-5 w-5" />
            </button>
            <span class="truncate font-mono text-sm text-text-secondary">{{ selectedPath }}</span>
          </div>

          <!-- File content -->
          <div v-if="fileLoading" class="flex items-center justify-center py-12">
            <div
              class="h-6 w-6 animate-spin rounded-full border-2 border-border-input border-t-indigo-600 dark:border-t-indigo-400"
            />
          </div>
          <div v-else class="flex-1 overflow-auto">
            <div v-if="isProseFile" class="p-4">
              <ContentViewer :content="fileContent" :file-type="contentFileType" />
            </div>
            <pre
              v-else
              class="p-4 text-sm leading-relaxed text-text-primary font-mono whitespace-pre-wrap break-words"
              >{{ fileContent }}</pre
            >
          </div>
        </template>
      </div>
    </div>
  </div>
</template>
