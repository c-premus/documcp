<script setup lang="ts">
import { ref, watch, computed, onBeforeUnmount } from 'vue'
import { useDocumentsStore } from '@/stores/documents'

const props = withDefaults(
  defineProps<{
    readonly modelValue: string
    readonly placeholder?: string
  }>(),
  {
    placeholder: 'comma-separated tags',
  },
)

const emit = defineEmits<{
  'update:modelValue': [value: string]
}>()

const store = useDocumentsStore()

const inputRef = ref<HTMLInputElement | null>(null)
const suggestions = ref<string[]>([])
const showDropdown = ref(false)
const activeIndex = ref(-1)
let debounceTimer: ReturnType<typeof setTimeout> | null = null

const currentWord = computed(() => {
  const parts = props.modelValue.split(',')
  return parts[parts.length - 1]?.trim() ?? ''
})

function scheduleSearch(query: string): void {
  if (debounceTimer !== null) {
    clearTimeout(debounceTimer)
  }
  debounceTimer = setTimeout(async () => {
    if (query.length === 0) {
      suggestions.value = []
      showDropdown.value = false
      return
    }
    const results = await store.fetchTags(query)
    suggestions.value = results
    showDropdown.value = results.length > 0
    activeIndex.value = -1
  }, 300)
}

watch(currentWord, (word) => {
  scheduleSearch(word)
})

function selectSuggestion(tag: string): void {
  const parts = props.modelValue.split(',')
  parts[parts.length - 1] = ` ${tag}`
  const updated = parts.join(',').replace(/^[\s,]+/, '') + ', '
  emit('update:modelValue', updated)
  suggestions.value = []
  showDropdown.value = false
  activeIndex.value = -1
  inputRef.value?.focus()
}

function handleKeydown(event: KeyboardEvent): void {
  if (!showDropdown.value) {
    return
  }

  if (event.key === 'ArrowDown') {
    event.preventDefault()
    activeIndex.value = Math.min(activeIndex.value + 1, suggestions.value.length - 1)
  } else if (event.key === 'ArrowUp') {
    event.preventDefault()
    activeIndex.value = Math.max(activeIndex.value - 1, 0)
  } else if (event.key === 'Enter' && activeIndex.value >= 0) {
    event.preventDefault()
    const selected = suggestions.value[activeIndex.value]
    if (selected !== undefined) selectSuggestion(selected)
  } else if (event.key === 'Escape') {
    showDropdown.value = false
    activeIndex.value = -1
  }
}

function handleInput(event: Event): void {
  const target = event.target as HTMLInputElement
  emit('update:modelValue', target.value)
}

function handleBlur(): void {
  // Delay to allow click events on suggestions to fire
  setTimeout(() => {
    showDropdown.value = false
    activeIndex.value = -1
  }, 200)
}

onBeforeUnmount(() => {
  if (debounceTimer !== null) {
    clearTimeout(debounceTimer)
  }
})
</script>

<template>
  <div class="relative">
    <input
      ref="inputRef"
      type="text"
      :value="modelValue"
      :placeholder="placeholder"
      role="combobox"
      aria-autocomplete="list"
      aria-controls="tag-suggestions"
      :aria-expanded="showDropdown && suggestions.length > 0"
      :aria-activedescendant="activeIndex >= 0 ? `tag-option-${activeIndex}` : undefined"
      class="mt-1 block w-full rounded-md border-border-input bg-bg-surface text-text-primary shadow-sm focus:border-indigo-500 focus:ring-indigo-500 dark:focus:border-indigo-400 dark:focus:ring-indigo-400 sm:text-sm"
      @input="handleInput"
      @keydown="handleKeydown"
      @blur="handleBlur"
    />
    <ul
      v-if="showDropdown && suggestions.length > 0"
      id="tag-suggestions"
      role="listbox"
      class="absolute z-10 mt-1 max-h-48 w-full overflow-y-auto rounded-md border border-border-input bg-bg-surface shadow-lg"
    >
      <li
        v-for="(tag, index) in suggestions"
        :id="`tag-option-${index}`"
        :key="tag"
        role="option"
        :aria-selected="index === activeIndex"
        class="cursor-pointer px-3 py-2 text-sm text-text-primary"
        :class="
          index === activeIndex
            ? 'bg-indigo-50 dark:bg-indigo-900/30 text-indigo-700 dark:text-indigo-300'
            : 'hover:bg-bg-hover'
        "
        @mousedown.prevent="selectSuggestion(tag)"
      >
        {{ tag }}
      </li>
    </ul>
  </div>
</template>
