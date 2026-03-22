<script setup lang="ts">
import { ref, watch, onMounted } from 'vue'
import { RouterLink, useRouter } from 'vue-router'
import { toast } from 'vue-sonner'
import { format } from 'date-fns'
import { ArrowDownTrayIcon, TrashIcon } from '@heroicons/vue/24/outline'
import { useDocumentsStore } from '@/stores/documents'
import StatusBadge from '@/components/shared/StatusBadge.vue'
import ConfirmDialog from '@/components/shared/ConfirmDialog.vue'
import ContentViewer from '@/components/documents/ContentViewer.vue'

const props = defineProps<{
  readonly uuid: string
}>()

const router = useRouter()
const store = useDocumentsStore()

const showDeleteDialog = ref(false)
const deleting = ref(false)

function formatFileSize(bytes: number): string {
  if (bytes < 1024) {
    return `${bytes} B`
  }
  if (bytes < 1024 * 1024) {
    return `${(bytes / 1024).toFixed(1)} KB`
  }
  return `${(bytes / (1024 * 1024)).toFixed(1)} MB`
}

function formatDate(dateString: string): string {
  return format(new Date(dateString), 'MMM d, yyyy h:mm a')
}

function truncateHash(hash: string): string {
  if (hash.length <= 16) {
    return hash
  }
  return `${hash.slice(0, 16)}...`
}

async function loadDocument(uuid: string): Promise<void> {
  try {
    await store.fetchDocument(uuid)
  } catch {
    // error is set in store
  }
}

async function handleDelete(): Promise<void> {
  deleting.value = true
  try {
    await store.deleteDocument(props.uuid)
    toast.success('Document deleted successfully')
    router.push('/documents')
  } catch {
    toast.error('Failed to delete document')
  } finally {
    deleting.value = false
    showDeleteDialog.value = false
  }
}

onMounted(() => {
  loadDocument(props.uuid)
})

watch(
  () => props.uuid,
  (newUuid) => {
    loadDocument(newUuid)
  },
)
</script>

<template>
  <div class="space-y-6">
    <!-- Back link -->
    <RouterLink
      to="/documents"
      class="inline-flex items-center text-sm font-medium text-text-muted hover:text-text-secondary"
    >
      &larr; Documents
    </RouterLink>

    <!-- Loading state -->
    <div v-if="store.loading && !store.currentDocument" class="flex items-center justify-center py-20">
      <div class="h-8 w-8 animate-spin rounded-full border-4 border-border-input border-t-indigo-600 dark:border-t-indigo-400" />
    </div>

    <!-- Error state -->
    <div v-else-if="store.error && !store.currentDocument" class="rounded-lg bg-red-50 dark:bg-red-900/20 p-6 text-center">
      <p class="text-sm text-red-800 dark:text-red-300">{{ store.error }}</p>
      <button
        type="button"
        class="mt-4 rounded-md bg-red-100 dark:bg-red-900/30 px-3 py-2 text-sm font-medium text-red-800 dark:text-red-300 hover:bg-red-200 dark:hover:bg-red-900/50"
        @click="loadDocument(uuid)"
      >
        Retry
      </button>
    </div>

    <!-- Document detail -->
    <template v-else-if="store.currentDocument">
      <!-- Page title with status -->
      <div class="flex items-center gap-3">
        <h1 class="text-2xl font-bold text-text-primary">{{ store.currentDocument.title }}</h1>
        <StatusBadge :status="store.currentDocument.status" />
      </div>

      <!-- Two-column layout -->
      <div class="grid grid-cols-1 gap-6 lg:grid-cols-3">
        <!-- Metadata sidebar -->
        <div class="rounded-lg bg-bg-surface p-6 shadow-sm lg:col-span-1">
          <h2 class="text-sm font-semibold uppercase tracking-wider text-text-muted">Details</h2>
          <dl class="mt-4 space-y-4">
            <!-- Description -->
            <div v-if="store.currentDocument.description">
              <dt class="text-sm font-medium text-text-muted">Description</dt>
              <dd class="mt-1 text-sm text-text-primary">{{ store.currentDocument.description }}</dd>
            </div>

            <!-- File Type -->
            <div>
              <dt class="text-sm font-medium text-text-muted">File Type</dt>
              <dd class="mt-1">
                <span class="inline-flex items-center rounded bg-bg-active px-2 py-0.5 text-xs font-medium uppercase text-text-primary">
                  {{ store.currentDocument.file_type }}
                </span>
              </dd>
            </div>

            <!-- Status -->
            <div>
              <dt class="text-sm font-medium text-text-muted">Status</dt>
              <dd class="mt-1">
                <StatusBadge :status="store.currentDocument.status" />
              </dd>
            </div>

            <!-- File Size -->
            <div>
              <dt class="text-sm font-medium text-text-muted">File Size</dt>
              <dd class="mt-1 text-sm text-text-primary">{{ formatFileSize(store.currentDocument.file_size) }}</dd>
            </div>

            <!-- Word Count -->
            <div>
              <dt class="text-sm font-medium text-text-muted">Word Count</dt>
              <dd class="mt-1 text-sm text-text-primary">{{ store.currentDocument.word_count.toLocaleString() }}</dd>
            </div>

            <!-- Content Hash -->
            <div>
              <dt class="text-sm font-medium text-text-muted">Content Hash</dt>
              <dd class="mt-1 font-mono text-sm text-text-primary" :title="store.currentDocument.content_hash">
                {{ truncateHash(store.currentDocument.content_hash) }}
              </dd>
            </div>

            <!-- Tags -->
            <div v-if="store.currentDocument.tags?.length">
              <dt class="text-sm font-medium text-text-muted">Tags</dt>
              <dd class="mt-1 flex flex-wrap gap-1">
                <span
                  v-for="tag in store.currentDocument.tags"
                  :key="tag"
                  class="inline-flex items-center rounded-full bg-indigo-50 dark:bg-indigo-900/30 px-2 py-0.5 text-xs font-medium text-indigo-700 dark:text-indigo-300"
                >
                  {{ tag }}
                </span>
              </dd>
            </div>

            <!-- Created at -->
            <div>
              <dt class="text-sm font-medium text-text-muted">Created</dt>
              <dd class="mt-1 text-sm text-text-primary">{{ formatDate(store.currentDocument.created_at) }}</dd>
            </div>

            <!-- Updated at -->
            <div>
              <dt class="text-sm font-medium text-text-muted">Updated</dt>
              <dd class="mt-1 text-sm text-text-primary">{{ formatDate(store.currentDocument.updated_at) }}</dd>
            </div>
          </dl>

          <!-- Actions -->
          <div class="mt-6 space-y-3">
            <a
              v-if="store.currentDocument?.has_file || store.currentDocument?.content"
              :href="`/api/documents/${uuid}/download`"
              class="inline-flex w-full items-center justify-center gap-2 rounded-md bg-indigo-600 px-3 py-2 text-sm font-semibold text-white shadow-sm hover:bg-indigo-500 focus-visible:outline focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-focus"
            >
              <ArrowDownTrayIcon class="h-4 w-4" aria-hidden="true" />
              Download
            </a>
            <button
              type="button"
              class="inline-flex w-full items-center justify-center gap-2 rounded-md bg-red-50 dark:bg-red-900/20 px-3 py-2 text-sm font-semibold text-red-700 dark:text-red-300 shadow-sm hover:bg-red-100 dark:hover:bg-red-900/30 focus-visible:outline focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-red-600"
              @click="showDeleteDialog = true"
            >
              <TrashIcon class="h-4 w-4" aria-hidden="true" />
              Delete
            </button>
          </div>
        </div>

        <!-- Content area -->
        <div class="rounded-lg bg-bg-surface p-6 shadow-sm lg:col-span-2">
          <h2 class="text-sm font-semibold uppercase tracking-wider text-text-muted">Content</h2>
          <div class="mt-4">
            <ContentViewer
              v-if="store.currentDocument?.content"
              :content="store.currentDocument.content"
              :file-type="store.currentDocument.file_type"
            />
            <p v-else class="text-sm text-text-muted">
              Content not available for preview.
            </p>
          </div>
        </div>
      </div>
    </template>

    <!-- Delete confirmation dialog -->
    <ConfirmDialog
      :open="showDeleteDialog"
      title="Delete Document"
      :message="`Are you sure you want to delete &quot;${store.currentDocument?.title ?? ''}&quot;? This action will move the document to trash.`"
      confirm-label="Delete"
      variant="danger"
      @confirm="handleDelete"
      @cancel="showDeleteDialog = false"
    />
  </div>
</template>
