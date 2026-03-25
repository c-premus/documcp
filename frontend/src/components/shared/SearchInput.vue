<script setup lang="ts">
import { ref, watch, onUnmounted } from 'vue'
import { MagnifyingGlassIcon, XMarkIcon } from '@heroicons/vue/24/outline'

const props = withDefaults(
  defineProps<{
    readonly modelValue: string
    readonly placeholder?: string
    readonly debounceMs?: number
  }>(),
  {
    placeholder: 'Search...',
    debounceMs: 300,
  },
)

const emit = defineEmits<{
  'update:modelValue': [value: string]
}>()

const internalValue = ref(props.modelValue)
let debounceTimer: ReturnType<typeof setTimeout> | null = null

function clearDebounce(): void {
  if (debounceTimer !== null) {
    clearTimeout(debounceTimer)
    debounceTimer = null
  }
}

watch(
  () => props.modelValue,
  (newValue) => {
    internalValue.value = newValue
  },
)

function handleInput(event: Event): void {
  const target = event.target as HTMLInputElement
  internalValue.value = target.value
  clearDebounce()
  debounceTimer = setTimeout(() => {
    emit('update:modelValue', internalValue.value)
  }, props.debounceMs)
}

function handleClear(): void {
  internalValue.value = ''
  clearDebounce()
  emit('update:modelValue', '')
}

onUnmounted(() => {
  clearDebounce()
})
</script>

<template>
  <div class="relative rounded-md shadow-sm">
    <div class="pointer-events-none absolute inset-y-0 left-0 flex items-center pl-3">
      <MagnifyingGlassIcon class="h-5 w-5 text-text-disabled" aria-hidden="true" />
    </div>
    <input
      type="text"
      :value="internalValue"
      :placeholder="placeholder"
      class="block w-full rounded-md border-0 py-1.5 pl-10 pr-10 text-text-primary bg-bg-surface ring-1 ring-inset ring-border-input placeholder:text-text-disabled focus:ring-2 focus:ring-inset focus:ring-focus sm:text-sm sm:leading-6"
      @input="handleInput"
    />
    <button
      v-if="internalValue.length > 0"
      type="button"
      class="absolute inset-y-0 right-0 flex items-center pr-3"
      aria-label="Clear search"
      @click="handleClear"
    >
      <XMarkIcon class="h-5 w-5 text-text-disabled hover:text-text-muted" aria-hidden="true" />
    </button>
  </div>
</template>
