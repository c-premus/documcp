<script setup lang="ts">
import { computed } from 'vue'
import {
  Dialog,
  DialogPanel,
  DialogTitle,
} from '@headlessui/vue'
import { ExclamationTriangleIcon } from '@heroicons/vue/24/outline'

const props = withDefaults(
  defineProps<{
    readonly open: boolean
    readonly title: string
    readonly message: string
    readonly confirmLabel?: string
    readonly variant?: 'danger' | 'warning'
  }>(),
  {
    confirmLabel: 'Confirm',
    variant: 'danger',
  },
)

const emit = defineEmits<{
  confirm: []
  cancel: []
}>()

const confirmButtonClasses = computed(() => {
  if (props.variant === 'warning') {
    return 'bg-yellow-600 hover:bg-yellow-500 focus-visible:outline-yellow-600'
  }
  return 'bg-red-600 hover:bg-red-500 focus-visible:outline-red-600'
})

const iconBgClass = computed(() => {
  if (props.variant === 'warning') {
    return 'bg-yellow-100 dark:bg-yellow-900/30'
  }
  return 'bg-red-100 dark:bg-red-900/30'
})

const iconColorClass = computed(() => {
  if (props.variant === 'warning') {
    return 'text-yellow-600 dark:text-yellow-400'
  }
  return 'text-red-600 dark:text-red-400'
})
</script>

<template>
  <Dialog :open="open" class="relative z-50" @close="emit('cancel')">
    <div class="fixed inset-0 bg-overlay backdrop-blur-sm transition-opacity" aria-hidden="true" />

    <div class="fixed inset-0 z-50 overflow-y-auto">
      <div class="flex min-h-full items-end justify-center p-4 text-center sm:items-center sm:p-0">
        <DialogPanel
          class="relative transform overflow-hidden rounded-lg bg-bg-surface px-4 pb-4 pt-5 text-left shadow-xl transition-all sm:my-8 sm:w-full sm:max-w-lg sm:p-6"
        >
          <div class="sm:flex sm:items-start">
            <div
              class="mx-auto flex h-12 w-12 shrink-0 items-center justify-center rounded-full sm:mx-0 sm:h-10 sm:w-10"
              :class="iconBgClass"
            >
              <ExclamationTriangleIcon class="h-6 w-6" :class="iconColorClass" aria-hidden="true" />
            </div>
            <div class="mt-3 text-center sm:ml-4 sm:mt-0 sm:text-left">
              <DialogTitle as="h3" class="text-base font-semibold text-text-primary">
                {{ title }}
              </DialogTitle>
              <div class="mt-2">
                <p class="text-sm text-text-muted">{{ message }}</p>
                <slot />
              </div>
            </div>
          </div>
          <div class="mt-5 sm:mt-4 sm:flex sm:flex-row-reverse">
            <button
              type="button"
              class="inline-flex w-full justify-center rounded-md px-3 py-2 text-sm font-semibold text-white shadow-sm focus-visible:outline focus-visible:outline-2 focus-visible:outline-offset-2 sm:ml-3 sm:w-auto"
              :class="confirmButtonClasses"
              @click="emit('confirm')"
            >
              {{ confirmLabel }}
            </button>
            <button
              type="button"
              class="mt-3 inline-flex w-full justify-center rounded-md bg-bg-surface px-3 py-2 text-sm font-semibold text-text-primary shadow-sm ring-1 ring-inset ring-border-input hover:bg-bg-hover sm:mt-0 sm:w-auto"
              @click="emit('cancel')"
            >
              Cancel
            </button>
          </div>
        </DialogPanel>
      </div>
    </div>
  </Dialog>
</template>
