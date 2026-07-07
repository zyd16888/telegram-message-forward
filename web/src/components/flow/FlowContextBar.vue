<script setup lang="ts">
import type { FlowBoardEdgeRef, FlowBoardSelection } from '@/composables/useFlowBoard'

defineProps<{
  selection: FlowBoardSelection | null
  selectedEdge: FlowBoardEdgeRef | null
  selectionLabel: string
  selectionRuleCount: number
  selectedEdgeLabel: string
}>()

const emit = defineEmits<{
  'create-rule': []
  'edit-rule': [id: number]
  'manage-config': []
  'view-deliveries': []
  'clear-selection': []
  'detach-edge': []
  'clear-edge': []
}>()
</script>

<template>
  <div v-if="selection" class="context-bar">
    <span class="context-label">
      已选中 {{ selectionLabel }}
      <template v-if="selection.kind !== 'rule'">，关联 {{ selectionRuleCount }} 条规则</template>
    </span>
    <div class="context-actions">
      <NButton v-if="selection.kind !== 'rule'" size="small" @click="emit('create-rule')">沿此建规则</NButton>
      <NButton v-if="selection.kind === 'rule'" size="small" @click="emit('edit-rule', selection.id)">编辑规则</NButton>
      <NButton size="small" @click="emit('manage-config')">管理配置</NButton>
      <NButton size="small" @click="emit('view-deliveries')">查看投递记录</NButton>
      <NButton size="small" text type="primary" @click="emit('clear-selection')">清除选中</NButton>
    </div>
  </div>

  <div v-else-if="selectedEdge" class="context-bar">
    <span class="context-label">已选中 {{ selectedEdgeLabel }}</span>
    <div class="context-actions">
      <NButton size="small" type="error" secondary @click="emit('detach-edge')">解除关联</NButton>
      <NButton size="small" text type="primary" @click="emit('clear-edge')">取消</NButton>
    </div>
  </div>
</template>

<style scoped>
.context-bar {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
  flex-wrap: wrap;
  padding: 10px 14px;
  border: 1px solid color-mix(in srgb, var(--clay-primary) 32%, var(--clay-border));
  border-radius: 14px;
  background: var(--clay-primary-soft);
  box-shadow: none;
}

.context-label {
  color: var(--clay-text);
  font-size: 13px;
  font-weight: 600;
}

.context-actions {
  display: flex;
  align-items: center;
  gap: 8px;
  flex-wrap: wrap;
}
</style>
