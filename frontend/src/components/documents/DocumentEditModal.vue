<script setup lang="ts">
import { ref, watch } from 'vue'
import { Dialog, DialogPanel, DialogTitle, Switch } from '@headlessui/vue'
import { toast } from 'vue-sonner'
import type { Document } from '@/stores/documents'
import { useDocumentsStore } from '@/stores/documents'
import TagInput from '@/components/documents/TagInput.vue'

const props = defineProps<{
  readonly open: boolean
  readonly document: Document | null
}>()

const emit = defineEmits<{
  close: []
  saved: []
}>()

const store = useDocumentsStore()

const title = ref('')
const description = ref('')
const tags = ref('')
const isPublic = ref(false)
const submitting = ref(false)
const error = ref<string | null>(null)

watch(
  () => props.open,
  (isOpen) => {
    if (isOpen) {
      title.value = props.document?.title ?? ''
      description.value = props.document?.description ?? ''
      tags.value = props.document?.tags?.join(', ') ?? ''
      isPublic.value = props.document?.is_public ?? false
      submitting.value = false
      error.value = null
    }
  },
)

async function handleSubmit(): Promise<void> {
  if (props.document === null) {
    return
  }

  submitting.value = true
  error.value = null

  try {
    const tagList = tags.value
      .split(',')
      .map((t) => t.trim())
      .filter((t) => t.length > 0)

    await store.updateDocument(props.document.uuid, {
      title: title.value,
      description: description.value,
      is_public: isPublic.value,
      tags: tagList,
    })
    toast.success('Document updated successfully')
    emit('saved')
  } catch (e) {
    error.value = e instanceof Error ? e.message : 'An unexpected error occurred'
    toast.error('Failed to update document')
  } finally {
    submitting.value = false
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
            Edit Document
          </DialogTitle>

          <form @submit.prevent="handleSubmit">
            <div class="space-y-4">
              <div>
                <label for="edit-title" class="block text-sm font-medium text-text-secondary"
                  >Title</label
                >
                <input
                  id="edit-title"
                  v-model="title"
                  type="text"
                  class="mt-1 block w-full rounded-md border-border-input bg-bg-surface text-text-primary shadow-sm focus:border-indigo-500 focus:ring-indigo-500 dark:focus:border-indigo-400 dark:focus:ring-indigo-400 sm:text-sm"
                />
              </div>

              <div>
                <label for="edit-description" class="block text-sm font-medium text-text-secondary"
                  >Description</label
                >
                <textarea
                  id="edit-description"
                  v-model="description"
                  rows="3"
                  class="mt-1 block w-full rounded-md border-border-input bg-bg-surface text-text-primary shadow-sm focus:border-indigo-500 focus:ring-indigo-500 dark:focus:border-indigo-400 dark:focus:ring-indigo-400 sm:text-sm"
                />
              </div>

              <div>
                <label for="edit-tags" class="block text-sm font-medium text-text-secondary"
                  >Tags</label
                >
                <TagInput v-model="tags" placeholder="comma-separated tags" />
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

            <p v-if="error" role="alert" class="mt-3 text-sm text-red-600 dark:text-red-400">
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
                type="submit"
                :disabled="submitting"
                class="inline-flex justify-center rounded-md bg-indigo-600 px-3 py-2 text-sm font-semibold text-white shadow-sm hover:bg-indigo-500 focus-visible:outline focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-focus disabled:opacity-50 disabled:cursor-not-allowed"
              >
                <template v-if="submitting">Saving...</template>
                <template v-else>Save</template>
              </button>
            </div>
          </form>
        </DialogPanel>
      </div>
    </div>
  </Dialog>
</template>
