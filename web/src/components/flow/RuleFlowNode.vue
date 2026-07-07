<script setup lang="ts">
import { Handle, Position } from '@vue-flow/core'
import { NButton, NSwitch, NTooltip } from 'naive-ui'
import ClayIcon from '@/components/ClayIcon.vue'
import type { RuleNodeData } from './types'

defineOptions({ inheritAttrs: false })

defineProps<{
  data: RuleNodeData
}>()

const emit = defineEmits<{
  edit: []
  toggle: [value: boolean]
}>()
</script>

<template>
  <div class="flow-node rule-node">
    <span class="order-badge">{{ data.orderLabel }}</span>
    <div class="node-head">
      <span class="node-name">{{ data.name }}</span>
      <NTooltip v-if="data.warnings.length" trigger="hover">
        <template #trigger>
          <span class="warn-badge">{{ data.warnings.length }}</span>
        </template>
        <ul class="warn-list">
          <li v-for="warning in data.warnings" :key="warning">{{ warning }}</li>
        </ul>
      </NTooltip>
      <span class="rule-actions" @click.stop @mousedown.stop>
        <NTooltip trigger="hover">
          <template #trigger>
            <NButton size="tiny" quaternary circle @click="emit('edit')">
              <template #icon><ClayIcon name="edit" :size="14" /></template>
            </NButton>
          </template>
          编辑 Flow
        </NTooltip>
        <NSwitch size="small" :value="data.enabled" @update:value="(value: boolean) => emit('toggle', value)" />
      </span>
    </div>
    <div class="rule-meta">
      优先级 {{ data.priority }}
      <template v-if="data.stopOnMatch"> · 命中即停</template>
    </div>
    <div class="chip-row">
      <span v-for="chip in data.conditionChips.slice(0, 3)" :key="`c-${chip}`" class="chip condition">{{ chip }}</span>
      <span v-if="data.conditionChips.length > 3" class="chip condition">+{{ data.conditionChips.length - 3 }}</span>
      <span v-for="chip in data.processorChips.slice(0, 2)" :key="`p-${chip}`" class="chip processor">{{ chip }}</span>
      <span v-if="data.processorChips.length > 2" class="chip processor">+{{ data.processorChips.length - 2 }}</span>
      <span v-if="!data.conditionChips.length && !data.processorChips.length" class="chip plain">全部消息 · 原样转发</span>
    </div>
    <Handle type="target" :position="Position.Left" />
    <Handle type="source" :position="Position.Right" />
  </div>
</template>

<style scoped>
.flow-node {
  width: 300px;
  padding: 10px 12px;
  border: 1px solid var(--clay-border);
  border-radius: 12px;
  background: var(--clay-surface);
  box-shadow: var(--clay-out-sm);
  font-size: 12px;
}

.rule-node {
  position: relative;
  border-top: 3px solid var(--clay-primary);
}

.order-badge {
  position: absolute;
  top: -11px;
  left: 10px;
  min-width: 28px;
  height: 22px;
  padding: 0 8px;
  display: inline-flex;
  align-items: center;
  justify-content: center;
  border: 1px solid color-mix(in srgb, var(--clay-primary) 35%, var(--clay-border));
  border-radius: 999px;
  color: var(--clay-primary);
  background: var(--clay-surface);
  box-shadow: var(--clay-out-sm);
  font-size: 11px;
  font-weight: 900;
  line-height: 1;
}

.node-head {
  display: flex;
  align-items: center;
  gap: 8px;
  min-width: 0;
}

.node-name {
  flex: 1;
  color: var(--clay-text);
  font-size: 13px;
  font-weight: 700;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.warn-badge {
  min-width: 20px;
  height: 20px;
  padding: 0 6px;
  border-radius: 999px;
  display: grid;
  place-items: center;
  flex-shrink: 0;
  color: #b45309;
  background: var(--clay-warning-soft);
  font-size: 12px;
  font-weight: 800;
}

.warn-list {
  margin: 0;
  padding-left: 16px;
  max-width: 320px;
}

.rule-actions {
  display: flex;
  align-items: center;
  gap: 6px;
  flex-shrink: 0;
}

.rule-meta {
  margin-top: 4px;
  color: var(--clay-text-3);
  font-size: 11px;
}

.chip-row {
  display: flex;
  align-items: center;
  gap: 5px;
  margin-top: 7px;
  min-width: 0;
  overflow: hidden;
}

.chip {
  flex-shrink: 1;
  max-width: 110px;
  padding: 1px 8px;
  border-radius: 999px;
  border: 1px solid var(--clay-border);
  color: var(--clay-text-2);
  background: var(--clay-surface-2);
  font-size: 10px;
  font-weight: 600;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.chip.condition {
  color: #1d6fb8;
  background: rgba(59, 130, 246, 0.12);
  border-color: rgba(59, 130, 246, 0.24);
}

.chip.processor {
  color: #7c3aed;
  background: rgba(139, 92, 246, 0.12);
  border-color: rgba(139, 92, 246, 0.24);
}

.chip.plain {
  color: var(--clay-text-3);
}
</style>
