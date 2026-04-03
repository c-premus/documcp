<script setup lang="ts">
import { ref, watch } from 'vue'
import { Dialog, DialogPanel, DialogTitle } from '@headlessui/vue'
import { CloudArrowUpIcon, DocumentIcon } from '@heroicons/vue/24/outline'
import { toast } from 'vue-sonner'
import type { Document } from '@/stores/documents'
import { useDocumentsStore } from '@/stores/documents'

const ACCEPTED_EXTENSIONS = '.pdf,.docx,.xlsx,.html,.md,.txt'
const MAX_SIZE_BYTES = 50 * 1024 * 1024

const props = defineProps<{
  readonly open: boolean
  readonly document: Document | null
}>()

const emit = defineEmits<{
  close: []
  replaced: []
}>()

const store = useDocumentsStore()

const file = ref<File | null>(null)
const uploading = ref(false)
const error = ref<string | null>(null)
const dragActive = ref(false)
const fileInputRef = ref<HTMLInputElement | null>(null)

watch(
  () => props.open,
  (isOpen) => {
    if (!isOpen) {
      file.value = null
      uploading.value = false
      error.value = null
      dragActive.value = false
    }
  },
)

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
  }
  input.value = ''
}

function onDrop(event: DragEvent): void {
  dragActive.value = false
  const dropped = event.dataTransfer?.files[0]
  if (dropped && validateFile(dropped)) {
    file.value = dropped
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

async function handleReplace(): Promise<void> {
  if (file.value === null || props.document === null) {
    return
  }

  uploading.value = true
  error.value = null

  try {
    await store.replaceContent(props.document.uuid, file.value)
    toast.success('File content replaced successfully')
    emit('replaced')
  } catch (e) {
    error.value = e instanceof Error ? e.message : 'Failed to replace file content'
    toast.error('Failed to replace file content')
  } finally {
    uploading.value = false
  }
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
            Replace File Content
          </DialogTitle>

          <p
            class="mb-4 rounded-md bg-amber-50 dark:bg-amber-900/20 p-3 text-sm text-amber-700 dark:text-amber-300"
          >
            This will replace the current file content. The document will be re-processed.
          </p>

          <input
            ref="fileInputRef"
            type="file"
            :accept="ACCEPTED_EXTENSIONS"
            class="hidden"
            @change="onFileSelect"
          />

          <div
            class="flex flex-col items-center justify-center rounded-lg border-2 border-dashed p-8 transition-colors cursor-pointer"
            :class="
              dragActive
                ? 'border-indigo-500 bg-indigo-50 dark:bg-indigo-900/20'
                : 'border-border-input hover:border-text-disabled'
            "
            @click="openFilePicker"
            @drop.prevent="onDrop"
            @dragover.prevent="onDragOver"
            @dragleave.prevent="onDragLeave"
          >
            <CloudArrowUpIcon class="h-10 w-10 text-text-disabled mb-3" aria-hidden="true" />
            <p class="text-sm text-text-muted">
              <span class="font-semibold text-indigo-600 dark:text-indigo-400"
                >Click to upload</span
              >
              or drag and drop
            </p>
            <p class="mt-1 text-xs text-text-muted">PDF, DOCX, XLSX, HTML, MD, TXT up to 50 MB</p>
          </div>

          <div v-if="file" class="mt-4 flex items-center gap-3 rounded-md bg-bg-surface-alt p-3">
            <DocumentIcon class="h-6 w-6 text-text-disabled shrink-0" aria-hidden="true" />
            <div class="min-w-0 flex-1">
              <p class="truncate text-sm font-medium text-text-primary">{{ file.name }}</p>
              <p class="text-xs text-text-muted">{{ formatFileSize(file.size) }}</p>
            </div>
          </div>

          <!-- Uploading spinner -->
          <div
            v-if="uploading"
            role="status"
            aria-live="polite"
            class="mt-4 flex items-center justify-center py-4"
          >
            <div
              class="h-8 w-8 animate-spin rounded-full border-4 border-border-default border-t-indigo-600 dark:border-t-indigo-400"
            />
            <span class="sr-only">Replacing file content...</span>
          </div>

          <p
            v-if="error"
            id="reupload-form-error"
            role="alert"
            class="mt-3 text-sm text-red-600 dark:text-red-400"
          >
            {{ error }}
          </p>

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
              :disabled="!file || uploading"
              class="inline-flex justify-center rounded-md bg-indigo-600 px-3 py-2 text-sm font-semibold text-white shadow-sm hover:bg-indigo-500 focus-visible:outline focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-focus disabled:opacity-50 disabled:cursor-not-allowed"
              @click="handleReplace"
            >
              <template v-if="uploading">Replacing...</template>
              <template v-else>Replace</template>
            </button>
          </div>
        </DialogPanel>
      </div>
    </div>
  </Dialog>
</template>
