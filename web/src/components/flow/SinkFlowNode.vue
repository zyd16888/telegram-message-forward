<script setup lang="ts">
import { Handle, Position } from '@vue-flow/core'
import type { SinkNodeData } from './types'

defineOptions({ inheritAttrs: false })

defineProps<{
  data: SinkNodeData
}>()
</script>

<template>
  <div class="flow-node sink-node">
    <div class="node-head">
      <span class="status-dot" :class="{ on: data.enabled }" />
      <span class="node-name">{{ data.name }}</span>
    </div>
    <div class="node-meta">
      <span class="node-tag">{{ data.typeLabel }}</span>
      <span class="node-sub">{{ data.linkLabel ?? (data.ruleCount ? `${data.ruleCount} 个 Flow` : '未接 Flow') }}</span>
    </div>
    <div class="node-foot">{{ data.deliveryLabel }}</div>
    <Handle type="target" :position="Position.Left" />
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

.sink-node {
  border-right: 3px solid #f59e0b;
}

.node-head {
  display: flex;
  align-items: center;
  gap: 8px;
  min-width: 0;
}

.status-dot {
  width: 8px;
  height: 8px;
  flex-shrink: 0;
  border-radius: 999px;
  background: var(--clay-border-strong);
}

.status-dot.on {
  background: #10b981;
  box-shadow: 0 0 0 3px rgba(16, 185, 129, 0.16);
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
  margin-top: 6px;
  min-width: 0;
  overflow: hidden;
}

.node-tag {
  flex-shrink: 0;
  padding: 1px 8px;
  border: 1px solid var(--clay-border);
  border-radius: 999px;
  color: var(--clay-text-2);
  background: var(--clay-surface-2);
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

.node-foot {
  margin-top: 7px;
  color: var(--clay-text-2);
  font-size: 11px;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}
</style>
