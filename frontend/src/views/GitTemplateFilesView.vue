<script setup lang="ts">
import { ref, onMounted } from 'vue'
import { useRouter } from 'vue-router'
import { toast } from 'vue-sonner'
import { ArrowLeftIcon } from '@heroicons/vue/24/outline'

import TreeNode from '../components/shared/TreeNode.vue'
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
      <div class="h-8 w-8 animate-spin rounded-full border-4 border-border-input border-t-indigo-600 dark:border-t-indigo-400" />
    </div>

    <!-- Two-pane layout -->
    <div v-else class="flex gap-4 h-[calc(100vh-12rem)]">
      <!-- Left sidebar: file tree -->
      <div class="w-72 shrink-0 overflow-y-auto rounded-lg border border-border-default bg-bg-surface p-2">
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
      <div class="flex-1 overflow-hidden rounded-lg border border-border-default bg-bg-surface flex flex-col">
        <template v-if="selectedPath === ''">
          <div class="flex items-center justify-center h-full text-text-disabled text-sm">
            Select a file to view its contents.
          </div>
        </template>
        <template v-else>
          <!-- File header -->
          <div class="flex items-center gap-2 border-b border-border-default px-4 py-2 bg-bg-surface-alt">
            <span class="font-mono text-sm text-text-secondary">{{ selectedPath }}</span>
          </div>

          <!-- File content -->
          <div v-if="fileLoading" class="flex items-center justify-center py-12">
            <div class="h-6 w-6 animate-spin rounded-full border-2 border-border-input border-t-indigo-600 dark:border-t-indigo-400" />
          </div>
          <pre
            v-else
            class="flex-1 overflow-auto p-4 text-sm leading-relaxed text-text-primary font-mono whitespace-pre-wrap break-words"
          >{{ fileContent }}</pre>
        </template>
      </div>
    </div>
  </div>
</template>
