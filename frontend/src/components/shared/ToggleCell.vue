<script setup lang="ts">
import { Switch } from '@headlessui/vue'

defineProps<{
  readonly modelValue: boolean
  readonly label: string
}>()

const emit = defineEmits<{
  'update:modelValue': [value: boolean]
}>()

function onClick(event: MouseEvent): void {
  event.stopPropagation()
}

function onUpdate(value: boolean): void {
  emit('update:modelValue', value)
}
</script>

<template>
  <Switch
    :model-value="modelValue"
    :aria-label="label"
    :class="[
      'relative inline-flex h-6 w-11 shrink-0 cursor-pointer rounded-full border-2 border-transparent transition-colors duration-200 ease-in-out focus:outline-none focus-visible:ring-2 focus-visible:ring-indigo-600 focus-visible:ring-offset-2',
      modelValue ? 'bg-indigo-600' : 'bg-gray-200 dark:bg-gray-600',
    ]"
    @click="onClick"
    @update:model-value="onUpdate"
  >
    <span
      :class="[
        'pointer-events-none inline-block h-5 w-5 transform rounded-full bg-white shadow ring-0 transition duration-200 ease-in-out',
        modelValue ? 'translate-x-5' : 'translate-x-0',
      ]"
    />
  </Switch>
</template>
