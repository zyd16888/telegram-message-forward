<script setup lang="ts">
import { Handle, Position } from '@vue-flow/core'
import type { FilterNodeData } from './types'

defineOptions({ inheritAttrs: false })

defineProps<{
  data: FilterNodeData
}>()
</script>

<template>
  <div class="flow-node filter-node">
    <div class="node-head">
      <span class="filter-mark">F</span>
      <span class="node-name">{{ data.name }}</span>
    </div>
    <div class="node-meta">
      <span class="node-tag">{{ data.conditionCount }} 个条件</span>
      <span class="node-sub">{{ data.ruleCount ? `${data.ruleCount} 条规则引用` : '未被引用' }}</span>
    </div>
    <Handle type="source" :position="Position.Right" />
  </div>
</template>

<style scoped>
.flow-node {
  width: 236px;
  padding: 10px 12px;
  border: 1px solid var(--clay-border);
  border-radius: 12px;
  background: var(--clay-surface);
  box-shadow: var(--clay-out-sm);
  font-size: 12px;
}

.filter-node {
  border-left: 3px solid #3b82f6;
}

.node-head {
  display: flex;
  align-items: center;
  gap: 8px;
  min-width: 0;
}

.filter-mark {
  width: 20px;
  height: 20px;
  display: grid;
  place-items: center;
  flex-shrink: 0;
  border-radius: 7px;
  color: #1d4ed8;
  background: rgba(59, 130, 246, 0.14);
  font-size: 11px;
  font-weight: 900;
}

.node-name {
  color: var(--clay-text);
  font-size: 13px;
  font-weight: 700;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.node-meta {
  display: flex;
  align-items: center;
  gap: 8px;
  margin-top: 8px;
  min-width: 0;
  overflow: hidden;
}

.node-tag {
  flex-shrink: 0;
  padding: 1px 8px;
  border: 1px solid rgba(59, 130, 246, 0.24);
  border-radius: 999px;
  color: #1d6fb8;
  background: rgba(59, 130, 246, 0.12);
  font-size: 11px;
  font-weight: 600;
}

.node-sub {
  color: var(--clay-text-3);
  font-size: 11px;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}
</style>
