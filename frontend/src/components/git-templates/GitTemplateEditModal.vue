<script setup lang="ts">
import { ref, watch } from 'vue'
import { Dialog, DialogPanel, DialogTitle } from '@headlessui/vue'
import { toast } from 'vue-sonner'
import type { GitTemplate } from '../../stores/gitTemplates'
import { useGitTemplatesStore } from '../../stores/gitTemplates'

const props = defineProps<{
  readonly open: boolean
  readonly template: GitTemplate | null
}>()

const emit = defineEmits<{
  close: []
  saved: []
}>()

const store = useGitTemplatesStore()

const name = ref('')
const branch = ref('')
const category = ref('project')
const description = ref('')
const gitToken = ref('')
const submitting = ref(false)
const error = ref<string | null>(null)

const CATEGORIES = [
  { value: 'claude', label: 'Claude' },
  { value: 'memory-bank', label: 'Memory Bank' },
  { value: 'project', label: 'Project' },
] as const

watch(
  () => props.open,
  (isOpen) => {
    if (isOpen && props.template !== null) {
      name.value = props.template.name
      branch.value = props.template.branch
      category.value = props.template.category ?? 'project'
      description.value = props.template.description ?? ''
      gitToken.value = ''
      submitting.value = false
      error.value = null
    }
  },
)

function validate(): boolean {
  if (name.value.trim() === '') {
    error.value = 'Name is required'
    return false
  }
  if (branch.value.trim() === '') {
    error.value = 'Branch is required'
    return false
  }
  error.value = null
  return true
}

async function handleSubmit(): Promise<void> {
  if (!validate() || props.template === null) return

  submitting.value = true
  error.value = null

  try {
    const payload: Record<string, unknown> = {
      name: name.value.trim(),
      branch: branch.value.trim(),
      category: category.value,
      description: description.value.trim() || undefined,
    }
    if (gitToken.value.trim() !== '') {
      payload.git_token = gitToken.value.trim()
    }
    await store.updateTemplate(props.template.uuid, payload)
    toast.success(`Template "${name.value.trim()}" updated`)
    emit('saved')
  } catch (e) {
    error.value = e instanceof Error ? e.message : 'An unexpected error occurred'
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
            Edit Git Template
          </DialogTitle>

          <form @submit.prevent="handleSubmit">
            <div class="space-y-4">
              <div>
                <label
                  for="edit-template-name"
                  class="block text-sm font-medium text-text-secondary"
                  >Name</label
                >
                <input
                  id="edit-template-name"
                  v-model="name"
                  type="text"
                  required
                  class="mt-1 block w-full rounded-md border-border-input bg-bg-surface text-text-primary shadow-sm focus:border-indigo-500 focus:ring-indigo-500 dark:focus:border-indigo-400 dark:focus:ring-indigo-400 sm:text-sm"
                />
              </div>

              <div>
                <label
                  for="edit-template-branch"
                  class="block text-sm font-medium text-text-secondary"
                  >Branch</label
                >
                <input
                  id="edit-template-branch"
                  v-model="branch"
                  type="text"
                  required
                  class="mt-1 block w-full rounded-md border-border-input bg-bg-surface text-text-primary shadow-sm focus:border-indigo-500 focus:ring-indigo-500 dark:focus:border-indigo-400 dark:focus:ring-indigo-400 sm:text-sm"
                />
              </div>

              <div>
                <label
                  for="edit-template-category"
                  class="block text-sm font-medium text-text-secondary"
                  >Category</label
                >
                <select
                  id="edit-template-category"
                  v-model="category"
                  class="mt-1 block w-full rounded-md border-border-input bg-bg-surface text-text-primary shadow-sm focus:border-indigo-500 focus:ring-indigo-500 dark:focus:border-indigo-400 dark:focus:ring-indigo-400 sm:text-sm"
                >
                  <option v-for="cat in CATEGORIES" :key="cat.value" :value="cat.value">
                    {{ cat.label }}
                  </option>
                </select>
              </div>

              <div>
                <label
                  for="edit-template-description"
                  class="block text-sm font-medium text-text-secondary"
                  >Description</label
                >
                <textarea
                  id="edit-template-description"
                  v-model="description"
                  rows="3"
                  class="mt-1 block w-full rounded-md border-border-input bg-bg-surface text-text-primary shadow-sm focus:border-indigo-500 focus:ring-indigo-500 dark:focus:border-indigo-400 dark:focus:ring-indigo-400 sm:text-sm"
                />
              </div>

              <div>
                <label
                  for="edit-template-git-token"
                  class="block text-sm font-medium text-text-secondary"
                >
                  Access Token
                  <span class="text-text-disabled font-normal">(leave blank to keep current)</span>
                </label>
                <input
                  id="edit-template-git-token"
                  v-model="gitToken"
                  type="password"
                  placeholder="********"
                  class="mt-1 block w-full rounded-md border-border-input bg-bg-surface text-text-primary shadow-sm focus:border-indigo-500 focus:ring-indigo-500 dark:focus:border-indigo-400 dark:focus:ring-indigo-400 sm:text-sm"
                />
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
