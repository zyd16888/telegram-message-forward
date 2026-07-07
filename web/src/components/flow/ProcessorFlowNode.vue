<script setup lang="ts">
import { Handle, Position } from '@vue-flow/core'
import type { ProcessorNodeData } from './types'

defineOptions({ inheritAttrs: false })

defineProps<{
  data: ProcessorNodeData
}>()
</script>

<template>
  <div class="flow-node processor-node">
    <div class="node-head">
      <span class="processor-mark">P</span>
      <span class="node-name">{{ data.name }}</span>
    </div>
    <div class="chip-row">
      <span v-for="chip in data.processorChips.slice(0, 4)" :key="chip" class="chip">{{ chip }}</span>
      <span v-if="data.processorChips.length > 4" class="chip">+{{ data.processorChips.length - 4 }}</span>
    </div>
    <Handle type="target" :position="Position.Left" />
    <Handle type="source" :position="Position.Right" />
  </div>
</template>

<style scoped>
.flow-node {
  width: 260px;
  padding: 10px 12px;
  border: 1px solid var(--clay-border);
  border-radius: 12px;
  background: var(--clay-surface);
  box-shadow: var(--clay-out-sm);
  font-size: 12px;
}

.processor-node {
  border-top: 3px solid #8b5cf6;
}

.node-head {
  display: flex;
  align-items: center;
  gap: 8px;
  min-width: 0;
}

.processor-mark {
  width: 20px;
  height: 20px;
  display: grid;
  place-items: center;
  flex-shrink: 0;
  border-radius: 7px;
  color: #6d28d9;
  background: rgba(139, 92, 246, 0.13);
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

.chip-row {
  display: flex;
  align-items: center;
  gap: 5px;
  margin-top: 8px;
  min-width: 0;
  overflow: hidden;
}

.chip {
  flex-shrink: 1;
  max-width: 120px;
  padding: 1px 8px;
  border: 1px solid rgba(139, 92, 246, 0.24);
  border-radius: 999px;
  color: #7c3aed;
  background: rgba(139, 92, 246, 0.12);
  font-size: 10px;
  font-weight: 600;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}
</style>
