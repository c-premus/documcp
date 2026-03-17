<script setup lang="ts">
import { ref, watch } from 'vue'
import {
  Dialog,
  DialogPanel,
  DialogTitle,
  Switch,
} from '@headlessui/vue'
import { CloudArrowUpIcon, DocumentIcon } from '@heroicons/vue/24/outline'
import { useDocumentsStore } from '@/stores/documents'
import { toast } from 'vue-sonner'

const ACCEPTED_EXTENSIONS = '.pdf,.docx,.xlsx,.html,.md,.txt'
const MAX_SIZE_BYTES = 50 * 1024 * 1024

const props = defineProps<{ readonly open: boolean }>()
const emit = defineEmits<{ close: []; uploaded: [] }>()

const store = useDocumentsStore()
const step = ref<1 | 2 | 3>(1)
const file = ref<File | null>(null)
const title = ref('')
const description = ref('')
const tags = ref('')
const isPublic = ref(false)
const analyzing = ref(false)
const uploading = ref(false)
const error = ref<string | null>(null)
const dragActive = ref(false)
const fileInputRef = ref<HTMLInputElement | null>(null)

watch(() => props.open, (isOpen) => {
  if (!isOpen) {
    step.value = 1
    file.value = null
    title.value = ''
    description.value = ''
    tags.value = ''
    isPublic.value = false
    analyzing.value = false
    uploading.value = false
    error.value = null
    dragActive.value = false
  }
})

function formatFileSize(bytes: number): string {
  if (bytes < 1024) {
    return `${bytes} B`
  }
  if (bytes < 1024 * 1024) {
    return `${(bytes / 1024).toFixed(1)} KB`
  }
  return `${(bytes / (1024 * 1024)).toFixed(1)} MB`
}

function validateFile(candidate: File): boolean {
  const ext = candidate.name.slice(candidate.name.lastIndexOf('.')).toLowerCase()
  const allowedExts = ACCEPTED_EXTENSIONS.split(',')
  if (!allowedExts.includes(ext)) {
    error.value = `Unsupported file type: ${ext}. Accepted: ${ACCEPTED_EXTENSIONS}`
    return false
  }
  if (candidate.size > MAX_SIZE_BYTES) {
    error.value = `File exceeds 50 MB limit (${formatFileSize(candidate.size)})`
    return false
  }
  error.value = null
  return true
}

function onFileSelect(event: Event): void {
  const input = event.target as HTMLInputElement
  const selected = input.files?.[0]
  if (selected && validateFile(selected)) {
    file.value = selected
    title.value = selected.name.replace(/\.[^.]+$/, '')
  }
  input.value = ''
}

function onDrop(event: DragEvent): void {
  dragActive.value = false
  const dropped = event.dataTransfer?.files[0]
  if (dropped && validateFile(dropped)) {
    file.value = dropped
    title.value = dropped.name.replace(/\.[^.]+$/, '')
  }
}

function onDragOver(): void {
  dragActive.value = true
}

function onDragLeave(): void {
  dragActive.value = false
}

function openFilePicker(): void {
  fileInputRef.value?.click()
}

async function analyze(): Promise<void> {
  if (file.value === null) {
    return
  }
  analyzing.value = true
  error.value = null
  try {
    const formData = new FormData()
    formData.append('file', file.value)
    const result = await store.analyzeDocument(formData)
    title.value = result.title || title.value
    description.value = result.description || ''
    tags.value = result.tags.join(', ')
    step.value = 2
  } catch (e) {
    error.value = e instanceof Error ? e.message : 'Analysis failed'
  } finally {
    analyzing.value = false
  }
}

function goToMetadata(): void {
  if (file.value === null) {
    return
  }
  step.value = 2
}

function goBack(): void {
  step.value = 1
  error.value = null
}

async function upload(): Promise<void> {
  if (file.value === null) {
    return
  }
  step.value = 3
  uploading.value = true
  error.value = null
  try {
    const formData = new FormData()
    formData.append('file', file.value)
    formData.append('title', title.value)
    formData.append('description', description.value)
    formData.append('is_public', String(isPublic.value))
    const tagList = tags.value
      .split(',')
      .map((t) => t.trim())
      .filter((t) => t.length > 0)
    for (const tag of tagList) {
      formData.append('tags', tag)
    }
    await store.uploadDocument(formData)
    toast.success('Document uploaded successfully')
    emit('uploaded')
    emit('close')
  } catch (e) {
    error.value = e instanceof Error ? e.message : 'Upload failed'
    uploading.value = false
  }
}

function retry(): void {
  void upload()
}
</script>

<template>
  <Dialog :open="open" class="relative z-50" @close="emit('close')">
    <div class="fixed inset-0 bg-overlay backdrop-blur-sm transition-opacity" aria-hidden="true" />

    <div class="fixed inset-0 z-50 overflow-y-auto">
      <div class="flex min-h-full items-end justify-center p-4 text-center sm:items-center sm:p-0">
        <DialogPanel
          class="relative transform overflow-hidden rounded-lg bg-bg-surface px-4 pb-4 pt-5 text-left shadow-xl transition-all sm:my-8 sm:w-full sm:max-w-lg sm:p-6"
        >
          <DialogTitle as="h3" class="text-base font-semibold text-text-primary mb-4">
            Upload Document
          </DialogTitle>

          <!-- Step 1: File Selection -->
          <div v-if="step === 1">
            <input
              ref="fileInputRef"
              type="file"
              :accept="ACCEPTED_EXTENSIONS"
              class="hidden"
              @change="onFileSelect"
            />

            <div
              class="flex flex-col items-center justify-center rounded-lg border-2 border-dashed p-8 transition-colors cursor-pointer"
              :class="dragActive ? 'border-indigo-500 bg-indigo-50 dark:bg-indigo-900/20' : 'border-border-input hover:border-text-disabled'"
              @click="openFilePicker"
              @drop.prevent="onDrop"
              @dragover.prevent="onDragOver"
              @dragleave.prevent="onDragLeave"
            >
              <CloudArrowUpIcon class="h-10 w-10 text-text-disabled mb-3" aria-hidden="true" />
              <p class="text-sm text-text-muted">
                <span class="font-semibold text-indigo-600 dark:text-indigo-400">Click to upload</span> or drag and drop
              </p>
              <p class="mt-1 text-xs text-text-muted">
                PDF, DOCX, XLSX, HTML, MD, TXT up to 50 MB
              </p>
            </div>

            <div v-if="file" class="mt-4 flex items-center gap-3 rounded-md bg-bg-surface-alt p-3">
              <DocumentIcon class="h-6 w-6 text-text-disabled shrink-0" aria-hidden="true" />
              <div class="min-w-0 flex-1">
                <p class="truncate text-sm font-medium text-text-primary">{{ file.name }}</p>
                <p class="text-xs text-text-muted">{{ formatFileSize(file.size) }}</p>
              </div>
            </div>

            <p v-if="error" role="alert" class="mt-3 text-sm text-red-600 dark:text-red-400">{{ error }}</p>

            <div class="mt-5 flex justify-end gap-3">
              <button
                type="button"
                class="inline-flex justify-center rounded-md bg-bg-surface px-3 py-2 text-sm font-semibold text-text-primary shadow-sm ring-1 ring-inset ring-border-input hover:bg-bg-hover"
                @click="emit('close')"
              >
                Cancel
              </button>
              <button
                type="button"
                :disabled="!file || analyzing"
                class="inline-flex justify-center rounded-md bg-bg-surface px-3 py-2 text-sm font-semibold text-indigo-600 dark:text-indigo-400 shadow-sm ring-1 ring-inset ring-indigo-300 dark:ring-indigo-700 hover:bg-indigo-50 dark:hover:bg-indigo-900/20 disabled:opacity-50 disabled:cursor-not-allowed"
                @click="analyze"
              >
                <template v-if="analyzing">Analyzing...</template>
                <template v-else>Analyze</template>
              </button>
              <button
                type="button"
                :disabled="!file"
                class="inline-flex justify-center rounded-md bg-indigo-600 px-3 py-2 text-sm font-semibold text-white shadow-sm hover:bg-indigo-500 focus-visible:outline focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-focus disabled:opacity-50 disabled:cursor-not-allowed"
                @click="goToMetadata"
              >
                Next
              </button>
            </div>
          </div>

          <!-- Step 2: Metadata Form -->
          <div v-if="step === 2">
            <div class="space-y-4">
              <div>
                <label for="upload-title" class="block text-sm font-medium text-text-secondary">Title</label>
                <input
                  id="upload-title"
                  v-model="title"
                  type="text"
                  class="mt-1 block w-full rounded-md border-border-input bg-bg-surface text-text-primary shadow-sm focus:border-indigo-500 focus:ring-indigo-500 dark:focus:border-indigo-400 dark:focus:ring-indigo-400 sm:text-sm"
                />
              </div>

              <div>
                <label for="upload-description" class="block text-sm font-medium text-text-secondary">Description</label>
                <textarea
                  id="upload-description"
                  v-model="description"
                  rows="3"
                  class="mt-1 block w-full rounded-md border-border-input bg-bg-surface text-text-primary shadow-sm focus:border-indigo-500 focus:ring-indigo-500 dark:focus:border-indigo-400 dark:focus:ring-indigo-400 sm:text-sm"
                />
              </div>

              <div>
                <label for="upload-tags" class="block text-sm font-medium text-text-secondary">Tags</label>
                <input
                  id="upload-tags"
                  v-model="tags"
                  type="text"
                  placeholder="comma-separated tags"
                  class="mt-1 block w-full rounded-md border-border-input bg-bg-surface text-text-primary shadow-sm focus:border-indigo-500 focus:ring-indigo-500 dark:focus:border-indigo-400 dark:focus:ring-indigo-400 sm:text-sm"
                />
              </div>

              <div class="flex items-center justify-between">
                <span class="text-sm font-medium text-text-secondary">Public</span>
                <Switch
                  v-model="isPublic"
                  :class="isPublic ? 'bg-indigo-600' : 'bg-gray-200 dark:bg-gray-600'"
                  class="relative inline-flex h-6 w-11 shrink-0 cursor-pointer rounded-full border-2 border-transparent transition-colors duration-200 ease-in-out focus:outline-none focus-visible:ring-2 focus-visible:ring-indigo-600 focus-visible:ring-offset-2"
                >
                  <span
                    :class="isPublic ? 'translate-x-5' : 'translate-x-0'"
                    class="pointer-events-none inline-block h-5 w-5 transform rounded-full bg-white shadow ring-0 transition duration-200 ease-in-out"
                  />
                </Switch>
              </div>
            </div>

            <p v-if="error" role="alert" class="mt-3 text-sm text-red-600 dark:text-red-400">{{ error }}</p>

            <div class="mt-5 flex justify-end gap-3">
              <button
                type="button"
                class="inline-flex justify-center rounded-md bg-bg-surface px-3 py-2 text-sm font-semibold text-text-primary shadow-sm ring-1 ring-inset ring-border-input hover:bg-bg-hover"
                @click="goBack"
              >
                Back
              </button>
              <button
                type="button"
                class="inline-flex justify-center rounded-md bg-indigo-600 px-3 py-2 text-sm font-semibold text-white shadow-sm hover:bg-indigo-500 focus-visible:outline focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-focus"
                @click="upload"
              >
                Upload
              </button>
            </div>
          </div>

          <!-- Step 3: Uploading -->
          <div v-if="step === 3">
            <div v-if="uploading && !error" class="flex flex-col items-center py-8">
              <div class="h-10 w-10 animate-spin rounded-full border-4 border-border-default border-t-indigo-600 dark:border-t-indigo-400" />
              <p class="mt-4 text-sm text-text-muted">Uploading document...</p>
            </div>

            <div v-if="error" class="flex flex-col items-center py-8">
              <p role="alert" class="text-sm text-red-600 dark:text-red-400 mb-4">{{ error }}</p>
              <div class="flex gap-3">
                <button
                  type="button"
                  class="inline-flex justify-center rounded-md bg-bg-surface px-3 py-2 text-sm font-semibold text-text-primary shadow-sm ring-1 ring-inset ring-border-input hover:bg-bg-hover"
                  @click="emit('close')"
                >
                  Cancel
                </button>
                <button
                  type="button"
                  class="inline-flex justify-center rounded-md bg-indigo-600 px-3 py-2 text-sm font-semibold text-white shadow-sm hover:bg-indigo-500 focus-visible:outline focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-focus"
                  @click="retry"
                >
                  Retry
                </button>
              </div>
            </div>
          </div>
        </DialogPanel>
      </div>
    </div>
  </Dialog>
</template>
