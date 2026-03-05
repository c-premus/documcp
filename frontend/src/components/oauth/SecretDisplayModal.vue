<script setup lang="ts">
import { ref } from 'vue'
import {
  Dialog,
  DialogPanel,
  DialogTitle,
} from '@headlessui/vue'
import {
  ClipboardDocumentIcon,
  ExclamationTriangleIcon,
} from '@heroicons/vue/24/outline'
import { toast } from 'vue-sonner'

defineProps<{
  readonly open: boolean
  readonly clientId: string
  readonly clientSecret: string
}>()

const emit = defineEmits<{
  close: []
}>()

const copiedField = ref<string | null>(null)

async function copyToClipboard(value: string, fieldName: string): Promise<void> {
  try {
    await navigator.clipboard.writeText(value)
    copiedField.value = fieldName
    toast.success(`${fieldName} copied to clipboard`)
    setTimeout(() => {
      copiedField.value = null
    }, 2000)
  } catch {
    toast.error('Failed to copy to clipboard')
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
            Client Created Successfully
          </DialogTitle>

          <!-- Warning -->
          <div class="rounded-md bg-amber-50 p-4 mb-4">
            <div class="flex">
              <ExclamationTriangleIcon class="h-5 w-5 text-amber-400 shrink-0" />
              <p class="ml-3 text-sm text-amber-700">
                This secret will not be shown again. Copy it now.
              </p>
            </div>
          </div>

          <div class="space-y-4">
            <!-- Client ID -->
            <div>
              <label class="block text-sm font-medium text-gray-700 mb-1">
                Client ID
              </label>
              <div class="flex items-center gap-2">
                <input
                  :value="clientId"
                  type="text"
                  readonly
                  class="block w-full rounded-md border-gray-300 bg-gray-50 shadow-sm sm:text-sm font-mono"
                />
                <button
                  type="button"
                  class="shrink-0 rounded-md p-2 text-gray-500 hover:text-indigo-600 hover:bg-gray-100"
                  title="Copy Client ID"
                  @click="copyToClipboard(clientId, 'Client ID')"
                >
                  <ClipboardDocumentIcon class="h-5 w-5" />
                </button>
              </div>
            </div>

            <!-- Client Secret -->
            <div>
              <label class="block text-sm font-medium text-gray-700 mb-1">
                Client Secret
              </label>
              <div class="flex items-center gap-2">
                <input
                  :value="clientSecret"
                  type="text"
                  readonly
                  class="block w-full rounded-md border-gray-300 bg-gray-50 shadow-sm sm:text-sm font-mono"
                />
                <button
                  type="button"
                  class="shrink-0 rounded-md p-2 text-gray-500 hover:text-indigo-600 hover:bg-gray-100"
                  title="Copy Client Secret"
                  @click="copyToClipboard(clientSecret, 'Client Secret')"
                >
                  <ClipboardDocumentIcon class="h-5 w-5" />
                </button>
              </div>
            </div>
          </div>

          <div class="mt-5 flex justify-end">
            <button
              type="button"
              class="inline-flex justify-center rounded-md bg-indigo-600 px-3 py-2 text-sm font-semibold text-white shadow-sm hover:bg-indigo-500 focus-visible:outline focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-indigo-600"
              @click="emit('close')"
            >
              Close
            </button>
          </div>
        </DialogPanel>
      </div>
    </div>
  </Dialog>
</template>
