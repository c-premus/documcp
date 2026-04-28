<script setup lang="ts">
import CategoryBadge from '@/components/shared/CategoryBadge.vue'
import PriorityReorder from '@/components/shared/PriorityReorder.vue'
import ToggleCell from '@/components/shared/ToggleCell.vue'
import TruncatedText from '@/components/shared/TruncatedText.vue'
import ExternalServiceRowActions from './ExternalServiceRowActions.vue'
import type { ExternalService } from '@/stores/externalServices'

const TYPE_PALETTE: Readonly<Record<string, string>> = {
  kiwix: 'bg-blue-100 text-blue-800 dark:bg-blue-900/30 dark:text-blue-300',
}

const HEALTH_PALETTE: Readonly<Record<string, string>> = {
  healthy: 'bg-green-100 text-green-800 dark:bg-green-900/30 dark:text-green-300',
  unhealthy: 'bg-red-100 text-red-800 dark:bg-red-900/30 dark:text-red-300',
}

const props = defineProps<{
  readonly service: ExternalService
  readonly syncing: boolean
  readonly canMoveUp: boolean
  readonly canMoveDown: boolean
}>()

const emit = defineEmits<{
  sync: [service: ExternalService]
  healthCheck: [service: ExternalService]
  edit: [service: ExternalService]
  delete: [service: ExternalService]
  toggleEnabled: [service: ExternalService]
  movePriority: [service: ExternalService, direction: 'up' | 'down']
}>()

function onSync(): void {
  emit('sync', props.service)
}
function onHealthCheck(): void {
  emit('healthCheck', props.service)
}
function onEdit(): void {
  emit('edit', props.service)
}
function onDelete(): void {
  emit('delete', props.service)
}
function onToggle(): void {
  emit('toggleEnabled', props.service)
}
function onPriorityUp(): void {
  emit('movePriority', props.service, 'up')
}
function onPriorityDown(): void {
  emit('movePriority', props.service, 'down')
}
</script>

<template>
  <div class="flex flex-col gap-2 px-4 py-3">
    <div class="flex items-start gap-3">
      <h3 class="min-w-0 flex-1 truncate text-sm font-medium text-text-primary">
        {{ service.name }}
      </h3>
      <ExternalServiceRowActions
        :service="service"
        :syncing="syncing"
        @sync="onSync"
        @health-check="onHealthCheck"
        @edit="onEdit"
        @delete="onDelete"
      />
    </div>
    <div class="flex flex-wrap items-center gap-1.5">
      <CategoryBadge :value="service.type" :palette="TYPE_PALETTE" />
      <CategoryBadge :value="service.status" :palette="HEALTH_PALETTE" />
    </div>
    <div class="flex items-center gap-2 text-xs text-text-muted">
      <TruncatedText :value="service.base_url" :mono="true" />
    </div>
    <div class="flex flex-wrap items-center gap-x-4 gap-y-2 pt-1 text-xs text-text-muted">
      <div class="flex items-center gap-2">
        <span>Enabled</span>
        <ToggleCell
          :model-value="service.is_enabled"
          :label="`Toggle ${service.name} enabled`"
          @update:model-value="onToggle"
        />
      </div>
      <div class="flex items-center gap-2">
        <span>Priority</span>
        <PriorityReorder
          :priority="service.priority"
          :can-move-up="canMoveUp"
          :can-move-down="canMoveDown"
          @up="onPriorityUp"
          @down="onPriorityDown"
        />
      </div>
    </div>
  </div>
</template>
