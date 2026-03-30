<script setup lang="ts">
import { ref } from 'vue'
import { FolderIcon, FolderOpenIcon, DocumentIcon } from '@heroicons/vue/24/outline'

export interface TreeItem {
  name: string
  path: string
  type: 'file' | 'directory'
  children?: TreeItem[]
}

const props = withDefaults(
  defineProps<{
    readonly item: TreeItem
    readonly depth?: number
    readonly selectedPath?: string
  }>(),
  {
    depth: 0,
    selectedPath: '',
  },
)

const emit = defineEmits<{
  select: [path: string]
}>()

const expanded = ref(props.item.type === 'directory' && props.depth < 2)

function handleClick(): void {
  if (props.item.type === 'directory') {
    expanded.value = !expanded.value
  } else {
    emit('select', props.item.path)
  }
}

function handleChildSelect(path: string): void {
  emit('select', path)
}
</script>

<template>
  <div>
    <button
      type="button"
      class="flex w-full items-center gap-1.5 rounded px-1 py-0.5 text-sm hover:bg-bg-hover text-left"
      :class="[
        item.type === 'file' && selectedPath === item.path
          ? 'bg-indigo-50 text-indigo-700 dark:bg-indigo-900/30 dark:text-indigo-300'
          : 'text-text-secondary',
      ]"
      :style="{ paddingLeft: `${depth * 16 + 4}px` }"
      :aria-expanded="item.type === 'directory' ? expanded : undefined"
      :aria-label="
        item.type === 'directory'
          ? expanded
            ? `Collapse ${item.name}`
            : `Expand ${item.name}`
          : undefined
      "
      @click="handleClick"
    >
      <template v-if="item.type === 'directory'">
        <FolderOpenIcon v-if="expanded" class="h-4 w-4 shrink-0 text-amber-500" />
        <FolderIcon v-else class="h-4 w-4 shrink-0 text-amber-500" />
      </template>
      <DocumentIcon v-else class="h-4 w-4 shrink-0 text-text-disabled" />
      <span class="truncate">{{ item.name }}</span>
    </button>

    <template v-if="item.type === 'directory' && expanded && item.children">
      <TreeNode
        v-for="child in item.children"
        :key="child.path"
        :item="child"
        :depth="depth + 1"
        :selected-path="selectedPath"
        @select="handleChildSelect"
      />
    </template>
  </div>
</template>
