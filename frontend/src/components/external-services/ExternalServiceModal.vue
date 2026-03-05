<script setup lang="ts">
import { ref, watch, computed } from 'vue'
import {
  Dialog,
  DialogPanel,
  DialogTitle,
} from '@headlessui/vue'
import { toast } from 'vue-sonner'
import type { ExternalService } from '../../stores/externalServices'
import { useExternalServicesStore } from '../../stores/externalServices'

const props = defineProps<{
  readonly open: boolean
  readonly service?: ExternalService | null
}>()

const emit = defineEmits<{
  close: []
  saved: []
}>()

const store = useExternalServicesStore()

const name = ref('')
const serviceType = ref('kiwix')
const baseUrl = ref('')
const apiKey = ref('')
const priority = ref(0)
const submitting = ref(false)
const error = ref<string | null>(null)

const isEditMode = computed(() => props.service !== null && props.service !== undefined)
const dialogTitle = computed(() => isEditMode.value ? 'Edit Service' : 'Add Service')
const submitLabel = computed(() => isEditMode.value ? 'Save' : 'Create')
const showApiKeyHint = computed(() => serviceType.value === 'confluence')

watch(() => props.open, (isOpen) => {
  if (isOpen) {
    if (props.service !== null && props.service !== undefined) {
      name.value = props.service.name
      serviceType.value = props.service.type
      baseUrl.value = props.service.base_url
      apiKey.value = ''
      priority.value = props.service.priority
    } else {
      name.value = ''
      serviceType.value = 'kiwix'
      baseUrl.value = ''
      apiKey.value = ''
      priority.value = 0
    }
    submitting.value = false
    error.value = null
  }
})

function validate(): boolean {
  if (name.value.trim() === '') {
    error.value = 'Name is required'
    return false
  }
  if (baseUrl.value.trim() === '') {
    error.value = 'Base URL is required'
    return false
  }
  try {
    new URL(baseUrl.value.trim())
  } catch {
    error.value = 'Please enter a valid URL'
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
    if (isEditMode.value && props.service !== null && props.service !== undefined) {
      const payload: Record<string, unknown> = {
        name: name.value.trim(),
        type: serviceType.value,
        base_url: baseUrl.value.trim(),
        priority: priority.value,
      }
      if (apiKey.value.trim() !== '') {
        payload.api_key = apiKey.value.trim()
      }
      await store.updateService(props.service.uuid, payload)
      toast.success(`Service "${name.value.trim()}" updated`)
    } else {
      const payload: Record<string, unknown> = {
        name: name.value.trim(),
        type: serviceType.value,
        base_url: baseUrl.value.trim(),
        priority: priority.value,
      }
      if (apiKey.value.trim() !== '') {
        payload.api_key = apiKey.value.trim()
      }
      await store.createService(payload as { name: string; type: string; base_url: string; api_key?: string; priority?: number })
      toast.success(`Service "${name.value.trim()}" created`)
    }
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
    <div class="fixed inset-0 bg-gray-500/75 backdrop-blur-sm transition-opacity" aria-hidden="true" />

    <div class="fixed inset-0 z-50 overflow-y-auto">
      <div class="flex min-h-full items-end justify-center p-4 text-center sm:items-center sm:p-0">
        <DialogPanel
          class="relative transform overflow-hidden rounded-lg bg-white px-4 pb-4 pt-5 text-left shadow-xl transition-all sm:my-8 sm:w-full sm:max-w-lg sm:p-6"
        >
          <DialogTitle as="h3" class="text-base font-semibold text-gray-900 mb-4">
            {{ dialogTitle }}
          </DialogTitle>

          <form @submit.prevent="handleSubmit">
            <div class="space-y-4">
              <div>
                <label for="service-name" class="block text-sm font-medium text-gray-700">Name</label>
                <input
                  id="service-name"
                  v-model="name"
                  type="text"
                  required
                  class="mt-1 block w-full rounded-md border-gray-300 shadow-sm focus:border-indigo-500 focus:ring-indigo-500 sm:text-sm"
                />
              </div>

              <div>
                <label for="service-type" class="block text-sm font-medium text-gray-700">Type</label>
                <select
                  id="service-type"
                  v-model="serviceType"
                  class="mt-1 block w-full rounded-md border-gray-300 shadow-sm focus:border-indigo-500 focus:ring-indigo-500 sm:text-sm"
                >
                  <option value="kiwix">Kiwix</option>
                  <option value="confluence">Confluence</option>
                </select>
              </div>

              <div>
                <label for="service-base-url" class="block text-sm font-medium text-gray-700">Base URL</label>
                <input
                  id="service-base-url"
                  v-model="baseUrl"
                  type="url"
                  required
                  placeholder="https://example.com"
                  class="mt-1 block w-full rounded-md border-gray-300 shadow-sm focus:border-indigo-500 focus:ring-indigo-500 sm:text-sm"
                />
              </div>

              <div>
                <label for="service-api-key" class="block text-sm font-medium text-gray-700">
                  API Key
                  <span v-if="isEditMode" class="text-gray-400 font-normal">(leave blank to keep current)</span>
                </label>
                <input
                  id="service-api-key"
                  v-model="apiKey"
                  type="password"
                  :placeholder="isEditMode ? '********' : ''"
                  class="mt-1 block w-full rounded-md border-gray-300 shadow-sm focus:border-indigo-500 focus:ring-indigo-500 sm:text-sm"
                />
                <p v-if="showApiKeyHint" class="mt-1 text-xs text-gray-500">
                  Use email:token format for Confluence API key
                </p>
              </div>

              <div>
                <label for="service-priority" class="block text-sm font-medium text-gray-700">Priority</label>
                <input
                  id="service-priority"
                  v-model.number="priority"
                  type="number"
                  min="0"
                  class="mt-1 block w-full rounded-md border-gray-300 shadow-sm focus:border-indigo-500 focus:ring-indigo-500 sm:text-sm"
                />
              </div>
            </div>

            <p v-if="error" class="mt-3 text-sm text-red-600">{{ error }}</p>

            <div class="mt-5 flex justify-end gap-3">
              <button
                type="button"
                class="inline-flex justify-center rounded-md bg-white px-3 py-2 text-sm font-semibold text-gray-900 shadow-sm ring-1 ring-inset ring-gray-300 hover:bg-gray-50"
                @click="emit('close')"
              >
                Cancel
              </button>
              <button
                type="submit"
                :disabled="submitting"
                class="inline-flex justify-center rounded-md bg-indigo-600 px-3 py-2 text-sm font-semibold text-white shadow-sm hover:bg-indigo-500 focus-visible:outline focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-indigo-600 disabled:opacity-50 disabled:cursor-not-allowed"
              >
                <template v-if="submitting">Saving...</template>
                <template v-else>{{ submitLabel }}</template>
              </button>
            </div>
          </form>
        </DialogPanel>
      </div>
    </div>
  </Dialog>
</template>
