<script setup lang="ts">
import { ref, watch } from 'vue'
import { Dialog, DialogPanel, DialogTitle } from '@headlessui/vue'
import { CheckCircleIcon, XCircleIcon } from '@heroicons/vue/24/outline'
import { toast } from 'vue-sonner'
import { useGitTemplatesStore, type CreateTemplateParams } from '../../stores/gitTemplates'

const props = defineProps<{
  readonly open: boolean
}>()

const emit = defineEmits<{
  close: []
  saved: []
}>()

const store = useGitTemplatesStore()

const name = ref('')
const repositoryUrl = ref('')
const branch = ref('main')
const category = ref('project')
const description = ref('')
const gitToken = ref('')

const submitting = ref(false)
const error = ref<string | null>(null)

const urlValidating = ref(false)
const urlValid = ref<boolean | null>(null)
const urlError = ref<string | null>(null)

const CATEGORIES = [
  { value: 'claude', label: 'Claude' },
  { value: 'memory-bank', label: 'Memory Bank' },
  { value: 'project', label: 'Project' },
] as const

watch(
  () => props.open,
  (isOpen) => {
    if (isOpen) {
      name.value = ''
      repositoryUrl.value = ''
      branch.value = 'main'
      category.value = 'project'
      description.value = ''
      gitToken.value = ''
      submitting.value = false
      error.value = null
      urlValid.value = null
      urlError.value = null
    }
  },
)

watch(repositoryUrl, () => {
  urlValid.value = null
  urlError.value = null
})

async function handleUrlBlur(): Promise<void> {
  const url = repositoryUrl.value.trim()
  if (url === '') {
    urlValid.value = null
    urlError.value = null
    return
  }

  urlValidating.value = true
  urlValid.value = null
  urlError.value = null

  try {
    const result = await store.validateUrl(url)
    urlValid.value = result.valid
    if (!result.valid) {
      urlError.value = result.error ?? 'Invalid repository URL'
    }
  } catch {
    urlValid.value = false
    urlError.value = 'Failed to validate URL'
  } finally {
    urlValidating.value = false
  }
}

function validate(): boolean {
  if (name.value.trim() === '') {
    error.value = 'Name is required'
    return false
  }
  if (repositoryUrl.value.trim() === '') {
    error.value = 'Repository URL is required'
    return false
  }
  if (branch.value.trim() === '') {
    error.value = 'Branch is required'
    return false
  }
  if (urlValid.value === false) {
    error.value = 'Repository URL is not valid'
    return false
  }
  error.value = null
  return true
}

async function handleSubmit(): Promise<void> {
  if (!validate()) {
    return
  }

  submitting.value = true
  error.value = null

  try {
    const params: CreateTemplateParams = {
      name: name.value.trim(),
      repository_url: repositoryUrl.value.trim(),
      branch: branch.value.trim(),
      category: category.value,
      description: description.value.trim() || undefined,
      git_token: gitToken.value.trim() || undefined,
    }
    await store.createTemplate(params)
    toast.success(`Template "${name.value.trim()}" created`)
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
            Add Git Template
          </DialogTitle>

          <p class="text-sm text-text-secondary mb-4">
            Only text files are synced from the repository (e.g. Markdown, YAML,
            code). Binary files such as PDFs and images are excluded. Per-file
            limit: 1 MB, total limit: 10 MB.
          </p>

          <form @submit.prevent="handleSubmit">
            <div class="space-y-4">
              <div>
                <label for="template-name" class="block text-sm font-medium text-text-secondary"
                  >Name</label
                >
                <input
                  id="template-name"
                  v-model="name"
                  type="text"
                  required
                  :aria-describedby="error ? 'template-create-form-error' : undefined"
                  class="mt-1 block w-full rounded-md border-border-input bg-bg-surface text-text-primary shadow-sm focus:border-indigo-500 focus:ring-indigo-500 dark:focus:border-indigo-400 dark:focus:ring-indigo-400 sm:text-sm"
                />
              </div>

              <div>
                <label for="template-url" class="block text-sm font-medium text-text-secondary"
                  >Repository URL</label
                >
                <div class="relative mt-1">
                  <input
                    id="template-url"
                    v-model="repositoryUrl"
                    type="url"
                    required
                    placeholder="https://github.com/org/repo.git"
                    :aria-invalid="urlValid === false ? true : undefined"
                    :aria-describedby="[urlError ? 'template-url-error' : '', error ? 'template-create-form-error' : ''].filter(Boolean).join(' ') || undefined"
                    class="block w-full rounded-md border-border-input bg-bg-surface text-text-primary shadow-sm focus:border-indigo-500 focus:ring-indigo-500 dark:focus:border-indigo-400 dark:focus:ring-indigo-400 sm:text-sm pr-10"
                    @blur="handleUrlBlur"
                  />
                  <div class="absolute inset-y-0 right-0 flex items-center pr-3">
                    <div
                      v-if="urlValidating"
                      class="h-4 w-4 animate-spin rounded-full border-2 border-border-input border-t-indigo-600 dark:border-t-indigo-400"
                    />
                    <CheckCircleIcon v-else-if="urlValid === true" class="h-5 w-5 text-green-500" />
                    <XCircleIcon v-else-if="urlValid === false" class="h-5 w-5 text-red-500" />
                  </div>
                </div>
                <p
                  v-if="urlError"
                  id="template-url-error"
                  class="mt-1 text-sm text-red-600 dark:text-red-400"
                  role="alert"
                >
                  {{ urlError }}
                </p>
              </div>

              <div>
                <label
                  for="template-git-token"
                  class="block text-sm font-medium text-text-secondary"
                >
                  Access Token
                  <span class="text-text-disabled font-normal">(optional, for private repos)</span>
                </label>
                <input
                  id="template-git-token"
                  v-model="gitToken"
                  type="password"
                  placeholder="ghp_xxxx or glpat-xxxx"
                  :aria-describedby="error ? 'template-create-form-error' : undefined"
                  class="mt-1 block w-full rounded-md border-border-input bg-bg-surface text-text-primary shadow-sm focus:border-indigo-500 focus:ring-indigo-500 dark:focus:border-indigo-400 dark:focus:ring-indigo-400 sm:text-sm"
                />
              </div>

              <div>
                <label for="template-branch" class="block text-sm font-medium text-text-secondary"
                  >Branch</label
                >
                <input
                  id="template-branch"
                  v-model="branch"
                  type="text"
                  required
                  :aria-describedby="error ? 'template-create-form-error' : undefined"
                  class="mt-1 block w-full rounded-md border-border-input bg-bg-surface text-text-primary shadow-sm focus:border-indigo-500 focus:ring-indigo-500 dark:focus:border-indigo-400 dark:focus:ring-indigo-400 sm:text-sm"
                />
              </div>

              <div>
                <label for="template-category" class="block text-sm font-medium text-text-secondary"
                  >Category</label
                >
                <select
                  id="template-category"
                  v-model="category"
                  :aria-describedby="error ? 'template-create-form-error' : undefined"
                  class="mt-1 block w-full rounded-md border-border-input bg-bg-surface text-text-primary shadow-sm focus:border-indigo-500 focus:ring-indigo-500 dark:focus:border-indigo-400 dark:focus:ring-indigo-400 sm:text-sm"
                >
                  <option v-for="cat in CATEGORIES" :key="cat.value" :value="cat.value">
                    {{ cat.label }}
                  </option>
                </select>
              </div>

              <div>
                <label
                  for="template-description"
                  class="block text-sm font-medium text-text-secondary"
                  >Description</label
                >
                <textarea
                  id="template-description"
                  v-model="description"
                  rows="3"
                  :aria-describedby="error ? 'template-create-form-error' : undefined"
                  class="mt-1 block w-full rounded-md border-border-input bg-bg-surface text-text-primary shadow-sm focus:border-indigo-500 focus:ring-indigo-500 dark:focus:border-indigo-400 dark:focus:ring-indigo-400 sm:text-sm"
                />
              </div>
            </div>

            <p
              v-if="error"
              id="template-create-form-error"
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
                type="submit"
                :disabled="submitting"
                class="inline-flex justify-center rounded-md bg-indigo-600 px-3 py-2 text-sm font-semibold text-white shadow-sm hover:bg-indigo-500 focus-visible:outline focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-focus disabled:opacity-50 disabled:cursor-not-allowed"
              >
                <template v-if="submitting">Creating...</template>
                <template v-else>Create</template>
              </button>
            </div>
          </form>
        </DialogPanel>
      </div>
    </div>
  </Dialog>
</template>
