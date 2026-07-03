<script setup lang="ts">
import ClayIcon from '@/components/ClayIcon.vue'
import type { FlowRuleGraphNode, FlowTargetNode } from '@/composables/useForwardingGraph'
import type { ConditionConfig, ProcessorConfig } from '@/types'

defineProps<{
  nodes: FlowRuleGraphNode[]
  selectedId?: number | null
}>()

const emit = defineEmits<{
  select: [node: FlowRuleGraphNode]
}>()

function targetLabel(target: FlowTargetNode): string {
  const sink = target.sink?.name ?? `渠道 #${target.sinkId}`
  const template = target.template ? ` / ${target.template.name}` : ' / 原文'
  return `${sink}${template}`
}

function itemLabel(item: ConditionConfig | ProcessorConfig): string {
  return item.type
}
</script>

<template>
  <div class="rule-flow-list">
    <button
      v-for="node in nodes"
      :key="node.rule.id"
      type="button"
      class="rule-card"
      :class="{ selected: selectedId === node.rule.id, warning: node.warnings.length > 0 }"
      @click="emit('select', node)"
    >
      <header class="rule-head">
        <span class="rule-icon"><ClayIcon name="rules" :size="18" /></span>
        <span class="rule-title-wrap">
          <span class="rule-title">{{ node.rule.name }}</span>
          <span class="rule-meta">
            优先级 {{ node.rule.priority }} · {{ node.rule.enabled ? '启用' : '停用' }}
            <template v-if="node.rule.stop_on_match"> · 命中即停</template>
          </span>
        </span>
        <span v-if="node.warnings.length" class="warn-count">{{ node.warnings.length }}</span>
      </header>

      <div class="rule-pipeline">
        <section class="flow-cell">
          <div class="cell-label">来源</div>
          <div v-if="node.sources.length" class="chip-stack">
            <span v-for="source in node.sources" :key="source.sourceId" class="flow-chip">
              {{ source.source?.name ?? `来源 #${source.sourceId}` }}
              <span class="chip-muted">{{ source.account?.name ?? '' }}</span>
            </span>
          </div>
          <span v-else class="empty-text">未指定来源</span>
        </section>

        <section class="flow-cell">
          <div class="cell-label">匹配条件</div>
          <div v-if="node.rule.conditions.length" class="chip-stack">
            <span v-for="(condition, index) in node.rule.conditions" :key="`${condition.type}-${index}`" class="flow-chip">
              {{ itemLabel(condition) }}
            </span>
          </div>
          <span v-else class="empty-text">直接匹配</span>
        </section>

        <section class="flow-cell">
          <div class="cell-label">处理器</div>
          <div v-if="node.rule.processors.length" class="chip-stack">
            <span v-for="(processor, index) in node.rule.processors" :key="`${processor.type}-${index}`" class="flow-chip">
              {{ itemLabel(processor) }}
            </span>
          </div>
          <span v-else class="empty-text">不处理</span>
        </section>

        <section class="flow-cell target-cell">
          <div class="cell-label">目标</div>
          <div v-if="node.targets.length" class="chip-stack">
            <span v-for="target in node.targets" :key="`${target.sinkId}-${target.templateId ?? 0}`" class="flow-chip target-chip">
              {{ targetLabel(target) }}
            </span>
          </div>
          <span v-else class="empty-text">未配置目标</span>
        </section>
      </div>
    </button>
  </div>
</template>

<style scoped>
.rule-flow-list {
  display: grid;
  gap: 12px;
}

.rule-card {
  width: 100%;
  border: 1px solid var(--clay-border);
  border-radius: 10px;
  padding: 14px;
  color: inherit;
  background: var(--clay-surface);
  cursor: pointer;
  text-align: left;
  transition: border-color 0.18s ease, box-shadow 0.18s ease;
}

.rule-card:hover,
.rule-card:focus-visible,
.rule-card.selected {
  border-color: color-mix(in srgb, var(--clay-primary) 45%, var(--clay-border));
  box-shadow: var(--clay-hover);
  outline: none;
}

.rule-card.selected {
  background: color-mix(in srgb, var(--clay-primary-soft) 45%, var(--clay-surface));
}

.rule-head {
  display: flex;
  align-items: center;
  gap: 10px;
  min-width: 0;
  margin-bottom: 12px;
}

.rule-icon {
  width: 34px;
  height: 34px;
  display: grid;
  place-items: center;
  flex-shrink: 0;
  border-radius: 8px;
  color: #8b5cf6;
  background: rgba(139, 92, 246, 0.1);
}

.rule-title-wrap {
  display: flex;
  flex-direction: column;
  min-width: 0;
}

.rule-title {
  color: var(--clay-text);
  font-size: 15px;
  font-weight: 800;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.rule-meta {
  margin-top: 2px;
  color: var(--clay-text-3);
  font-size: 12px;
}

.warn-count {
  min-width: 22px;
  height: 22px;
  margin-left: auto;
  border-radius: 999px;
  display: grid;
  place-items: center;
  color: #b45309;
  background: var(--clay-warning-soft);
  font-size: 12px;
  font-weight: 800;
}

.rule-pipeline {
  display: grid;
  grid-template-columns: minmax(180px, 1fr) minmax(160px, 0.8fr) minmax(160px, 0.8fr) minmax(220px, 1.2fr);
  gap: 10px;
}

.flow-cell {
  min-width: 0;
  min-height: 78px;
  padding: 10px;
  border: 1px solid var(--clay-border);
  border-radius: 8px;
  background: var(--clay-surface-2);
}

.cell-label {
  margin-bottom: 8px;
  color: var(--clay-text-3);
  font-size: 12px;
  font-weight: 700;
}

.chip-stack {
  display: flex;
  flex-wrap: wrap;
  gap: 6px;
  min-width: 0;
}

.flow-chip {
  max-width: 100%;
  padding: 4px 8px;
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

.target-chip {
  border-radius: 8px;
}

.chip-muted {
  margin-left: 4px;
  color: var(--clay-text-3);
  font-weight: 500;
}

.empty-text {
  color: var(--clay-text-3);
  font-size: 12px;
}

@media (max-width: 1180px) {
  .rule-pipeline {
    grid-template-columns: repeat(2, minmax(0, 1fr));
  }
}

@media (max-width: 720px) {
  .rule-pipeline {
    grid-template-columns: 1fr;
  }
}
</style>
