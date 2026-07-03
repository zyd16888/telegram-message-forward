<script setup lang="ts">
import ClayIcon from '@/components/ClayIcon.vue'
import type { FlowSourceNode } from '@/composables/useForwardingGraph'

defineProps<{
  nodes: FlowSourceNode[]
  selectedId?: number | null
}>()

const emit = defineEmits<{
  select: [node: FlowSourceNode]
}>()

function targetLabel(target: FlowSourceNode['rules'][number]['targets'][number]): string {
  const sink = target.sink?.name ?? `渠道 #${target.sinkId}`
  const template = target.template ? ` / ${target.template.name}` : ' / 原文'
  return `${sink}${template}`
}
</script>

<template>
  <div class="flow-map">
    <button
      v-for="node in nodes"
      :key="node.source.id"
      type="button"
      class="flow-row"
      :class="{ selected: selectedId === node.source.id, warning: node.warnings.length > 0 }"
      @click="emit('select', node)"
    >
      <section class="flow-cell source-cell">
        <span class="node-icon source-icon"><ClayIcon name="sources" :size="18" /></span>
        <span class="node-main">
          <span class="node-title">{{ node.source.name }}</span>
          <span class="node-meta">{{ node.account?.name ?? `账号 #${node.source.account_id}` }}</span>
        </span>
      </section>

      <span class="flow-arrow">→</span>

      <section class="flow-cell rules-cell">
        <span class="node-icon rule-icon"><ClayIcon name="rules" :size="18" /></span>
        <span v-if="node.rules.length" class="rule-stack">
          <span v-for="item in node.rules" :key="item.rule.id" class="rule-chip" :class="{ off: !item.rule.enabled }">
            {{ item.rule.name }}
          </span>
        </span>
        <span v-else class="empty-text">未关联规则</span>
      </section>

      <span class="flow-arrow">→</span>

      <section class="flow-cell target-cell">
        <span class="node-icon sink-icon"><ClayIcon name="sinks" :size="18" /></span>
        <span v-if="node.rules.some((item) => item.targets.length)" class="target-stack">
          <template v-for="item in node.rules" :key="item.rule.id">
            <span v-for="target in item.targets" :key="`${item.rule.id}-${target.sinkId}-${target.templateId ?? 0}`" class="target-chip">
              {{ targetLabel(target) }}
            </span>
          </template>
        </span>
        <span v-else class="empty-text">未配置目标</span>
      </section>

      <span v-if="node.warnings.length" class="warn-count">{{ node.warnings.length }}</span>
    </button>
  </div>
</template>

<style scoped>
.flow-map {
  display: grid;
  gap: 10px;
}

.flow-row {
  position: relative;
  display: grid;
  grid-template-columns: minmax(220px, 1fr) auto minmax(220px, 1.1fr) auto minmax(240px, 1.2fr);
  align-items: stretch;
  gap: 12px;
  width: 100%;
  min-height: 86px;
  padding: 12px;
  border: 1px solid var(--clay-border);
  border-radius: 10px;
  color: inherit;
  background: var(--clay-surface);
  cursor: pointer;
  text-align: left;
  transition: border-color 0.18s ease, box-shadow 0.18s ease, transform 0.18s ease;
}

.flow-row:hover,
.flow-row:focus-visible,
.flow-row.selected {
  border-color: color-mix(in srgb, var(--clay-primary) 45%, var(--clay-border));
  box-shadow: var(--clay-hover);
  outline: none;
}

.flow-row.selected {
  background: color-mix(in srgb, var(--clay-primary-soft) 45%, var(--clay-surface));
}

.flow-cell {
  display: flex;
  align-items: center;
  gap: 10px;
  min-width: 0;
  padding: 10px;
  border: 1px solid var(--clay-border);
  border-radius: 8px;
  background: var(--clay-surface-2);
}

.node-icon {
  width: 34px;
  height: 34px;
  display: grid;
  place-items: center;
  flex-shrink: 0;
  border-radius: 8px;
}

.source-icon {
  color: var(--clay-primary);
  background: var(--clay-primary-soft);
}

.rule-icon {
  color: #8b5cf6;
  background: rgba(139, 92, 246, 0.1);
}

.sink-icon {
  color: #2fb896;
  background: var(--clay-success-soft);
}

.node-main,
.rule-stack,
.target-stack {
  display: flex;
  flex-direction: column;
  gap: 6px;
  min-width: 0;
}

.node-title {
  color: var(--clay-text);
  font-size: 14px;
  font-weight: 700;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.node-meta,
.empty-text {
  color: var(--clay-text-3);
  font-size: 12px;
}

.rule-chip,
.target-chip {
  max-width: 100%;
  width: fit-content;
  padding: 3px 8px;
  border: 1px solid var(--clay-border);
  border-radius: 999px;
  color: var(--clay-text-2);
  background: var(--clay-surface);
  font-size: 12px;
  font-weight: 600;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.rule-chip.off {
  color: var(--clay-text-3);
  background: var(--clay-sunken);
}

.flow-arrow {
  display: grid;
  place-items: center;
  color: var(--clay-text-3);
  font-size: 18px;
  font-weight: 700;
}

.warn-count {
  position: absolute;
  top: 8px;
  right: 8px;
  min-width: 20px;
  height: 20px;
  border-radius: 999px;
  display: grid;
  place-items: center;
  color: #b45309;
  background: var(--clay-warning-soft);
  font-size: 12px;
  font-weight: 800;
}

@media (max-width: 1100px) {
  .flow-row {
    grid-template-columns: 1fr;
  }

  .flow-arrow {
    display: none;
  }
}
</style>
