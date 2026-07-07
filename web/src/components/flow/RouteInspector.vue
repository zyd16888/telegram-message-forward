<script setup lang="ts">
import { computed } from 'vue'
import type { FlowBoardEdgeRef, FlowBoardSelection } from '@/composables/useFlowBoard'
import type { FlowRuleGraphNode } from '@/composables/useForwardingGraph'
import type { Filter, Rule, Sink, Source } from '@/types'

interface MatchingRule {
  order: number
  rule: Rule
  active: boolean
}

const props = defineProps<{
  selection: FlowBoardSelection | null
  selectedEdge: FlowBoardEdgeRef | null
  ruleNodes: FlowRuleGraphNode[]
  sources: Source[]
  sinks: Sink[]
  filters: Filter[]
  sourceTypeLabel: (source: Source) => string
  sinkTypeLabel: (sink: Sink) => string
  conditionLabel: (type: string) => string
  processorLabel: (type: string) => string
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

const selectedRuleNode = computed(() => {
  if (props.selection?.kind !== 'rule') return null
  return props.ruleNodes.find((node) => node.rule.id === props.selection?.id) ?? null
})

const selectedSource = computed(() => {
  if (props.selection?.kind !== 'source') return null
  return sourceById(props.selection.id)
})

const selectedFilter = computed(() => {
  if (props.selection?.kind !== 'filter') return null
  return filterById(props.selection.id)
})

const selectedSink = computed(() => {
  if (props.selection?.kind !== 'sink') return null
  return sinkById(props.selection.id)
})

const selectedSourceRules = computed(() =>
  selectedSource.value ? matchingRulesForSource(selectedSource.value.id, props.selection?.id) : [],
)

const selectedSinkRules = computed(() => {
  if (!selectedSink.value) return []
  return props.ruleNodes
    .filter((node) => node.targets.some((target) => target.sinkId === selectedSink.value?.id))
    .map((node) => node.rule)
})

const selectedFilterRules = computed(() => {
  if (!selectedFilter.value) return []
  return props.ruleNodes
    .filter((node) => node.rule.filter_ids.includes(selectedFilter.value?.id ?? 0))
    .map((node) => node.rule)
})

const selectedEdgeRuleNode = computed(() => {
  if (!props.selectedEdge) return null
  return props.ruleNodes.find((node) => node.rule.id === props.selectedEdge?.ruleId) ?? null
})

const selectedEdgeSource = computed(() => {
  if (props.selectedEdge?.kind !== 'source-rule') return null
  return sourceById(props.selectedEdge.sourceId)
})

const selectedEdgeFilter = computed(() => {
  if (props.selectedEdge?.kind !== 'filter-rule') return null
  return filterById(props.selectedEdge.filterId)
})

const selectedEdgeSink = computed(() => {
  if (props.selectedEdge?.kind !== 'rule-sink') return null
  return sinkById(props.selectedEdge.sinkId)
})

const selectedEdgeTarget = computed(() => {
  const edge = props.selectedEdge
  if (edge?.kind !== 'rule-sink') return null
  return selectedEdgeRuleNode.value?.targets.find((target) => target.sinkId === edge.sinkId) ?? null
})

function sourceById(id: number): Source | null {
  return props.sources.find((source) => source.id === id) ?? null
}

function sinkById(id: number): Sink | null {
  return props.sinks.find((sink) => sink.id === id) ?? null
}

function filterById(id: number): Filter | null {
  return props.filters.find((filter) => filter.id === id) ?? null
}

function ruleNodeById(id: number): FlowRuleGraphNode | null {
  return props.ruleNodes.find((node) => node.rule.id === id) ?? null
}

function ruleName(id: number): string {
  return ruleNodeById(id)?.rule.name ?? `#${id}`
}

function sourceName(id: number): string {
  return sourceById(id)?.name ?? `#${id}`
}

function sinkName(id: number): string {
  return sinkById(id)?.name ?? `#${id}`
}

function filterName(id: number): string {
  return props.filters.find((filter) => filter.id === id)?.name ?? `过滤器 #${id}`
}

function conditionNames(rule: Rule): string[] {
  if (rule.filter_ids.length) return rule.filter_ids.map((id) => filterName(id))
  return rule.conditions.map((condition) => props.conditionLabel(condition.type))
}

function processorNames(rule: Rule): string[] {
  return rule.processors.map((processor) => props.processorLabel(processor.type))
}

function matchingRulesForSource(sourceId: number, activeRuleId?: number): MatchingRule[] {
  return props.ruleNodes
    .filter((node) => node.rule.source_ids.includes(sourceId))
    .map((node) => node.rule)
    .sort((a, b) => b.priority - a.priority || a.id - b.id)
    .map((rule, index) => ({
      order: index + 1,
      rule,
      active: activeRuleId === rule.id,
    }))
}

</script>

<template>
  <aside class="route-inspector">
    <template v-if="selection">
      <header class="inspector-head">
        <span class="eyebrow">
          {{
            selection.kind === 'source'
              ? '监听来源'
              : selection.kind === 'filter'
                ? '过滤器'
                : selection.kind === 'rule'
                  ? '转发规则'
                  : '目标渠道'
          }}
        </span>
        <strong class="title">
          <template v-if="selection.kind === 'source'">{{ sourceName(selection.id) }}</template>
          <template v-else-if="selection.kind === 'filter'">{{ filterName(selection.id) }}</template>
          <template v-else-if="selection.kind === 'rule'">{{ ruleName(selection.id) }}</template>
          <template v-else>{{ sinkName(selection.id) }}</template>
        </strong>
      </header>

      <section v-if="selectedRuleNode" class="section">
        <div class="section-title">路径</div>
        <div class="route-lines">
          <div v-for="source in selectedRuleNode.sources" :key="`source-${source.sourceId}`" class="route-line">
            <span class="pill source">{{ sourceName(source.sourceId) }}</span>
            <span class="arrow">→</span>
            <span class="pill rule">{{ selectedRuleNode.rule.name }}</span>
          </div>
          <div v-for="target in selectedRuleNode.targets" :key="`sink-${target.sinkId}`" class="route-line">
            <span class="pill rule">{{ selectedRuleNode.rule.name }}</span>
            <span class="arrow">→</span>
            <span class="pill sink">{{ sinkName(target.sinkId) }}</span>
            <span v-if="target.template" class="template-name">{{ target.template.name }}</span>
            <span v-else-if="target.templateId" class="template-name warn">模板 #{{ target.templateId }} 缺失</span>
          </div>
        </div>
      </section>

      <section v-if="selectedRuleNode" class="section">
        <div class="section-title">匹配顺序</div>
        <div class="order-list">
          <div
            v-for="source in selectedRuleNode.sources"
            :key="`order-${source.sourceId}`"
            class="order-group"
          >
            <span class="muted">{{ sourceName(source.sourceId) }}</span>
            <div
              v-for="item in matchingRulesForSource(source.sourceId, selectedRuleNode.rule.id)"
              :key="item.rule.id"
              class="order-row"
              :class="{ active: item.active }"
            >
              <span class="order-no">#{{ item.order }}</span>
              <span class="order-name">{{ item.rule.name }}</span>
              <span class="priority">P{{ item.rule.priority }}</span>
              <span v-if="item.rule.stop_on_match" class="stop">命中即停</span>
            </div>
          </div>
        </div>
      </section>

      <section v-if="selectedRuleNode" class="section">
        <div class="section-title">处理</div>
        <div class="chip-list">
          <span v-for="item in conditionNames(selectedRuleNode.rule)" :key="`c-${item}`" class="chip condition">{{ item }}</span>
          <span v-for="item in processorNames(selectedRuleNode.rule)" :key="`p-${item}`" class="chip processor">{{ item }}</span>
          <span v-if="!conditionNames(selectedRuleNode.rule).length && !processorNames(selectedRuleNode.rule).length" class="muted">
            全部消息 · 原样转发
          </span>
        </div>
      </section>

      <section v-if="selectedSource" class="section">
        <div class="section-title">经过它的规则路径</div>
        <div v-if="selectedSourceRules.length" class="path-list">
          <div v-for="item in selectedSourceRules" :key="item.rule.id" class="path-card">
            <div class="path-title">
              <span class="order-no">#{{ item.order }}</span>
              <strong>{{ item.rule.name }}</strong>
              <span class="priority">P{{ item.rule.priority }}</span>
              <span v-if="item.rule.stop_on_match" class="stop">命中即停</span>
            </div>
            <div class="path-line">
              {{ selectedSource.name }}
              <span>→</span>
              {{ ruleNodeById(item.rule.id)?.targets.map((target) => sinkName(target.sinkId)).join(' / ') || '未配置目标' }}
            </div>
          </div>
        </div>
        <p v-else class="empty-text">这个来源还没有接入任何规则。</p>
      </section>

      <section v-if="selectedSink" class="section">
        <div class="section-title">经过它的规则路径</div>
        <div v-if="selectedSinkRules.length" class="path-list">
          <div v-for="rule in selectedSinkRules" :key="rule.id" class="path-card">
            <div class="path-title">
              <strong>{{ rule.name }}</strong>
              <span class="priority">P{{ rule.priority }}</span>
              <span v-if="rule.stop_on_match" class="stop">命中即停</span>
            </div>
            <div class="path-line">
              {{ rule.source_ids.map((sourceId) => sourceName(sourceId)).join(' / ') || '未配置来源' }}
              <span>→</span>
              {{ selectedSink.name }}
            </div>
          </div>
        </div>
        <p v-else class="empty-text">这个渠道还没有被任何规则使用。</p>
      </section>

      <section v-if="selectedFilter" class="section">
        <div class="section-title">引用它的规则路径</div>
        <div v-if="selectedFilterRules.length" class="path-list">
          <div v-for="rule in selectedFilterRules" :key="rule.id" class="path-card">
            <div class="path-title">
              <strong>{{ rule.name }}</strong>
              <span class="priority">P{{ rule.priority }}</span>
              <span v-if="rule.stop_on_match" class="stop">命中即停</span>
            </div>
            <div class="path-line">
              {{ rule.source_ids.map((sourceId) => sourceName(sourceId)).join(' / ') || '未配置来源' }}
              <span>→</span>
              {{ rule.targets.map((target) => sinkName(target.sink_id)).join(' / ') || '未配置目标' }}
            </div>
          </div>
        </div>
        <p v-else class="empty-text">这个过滤器还没有被任何规则引用。</p>
      </section>

      <section v-if="selectedSource" class="section compact">
        <div class="section-title">状态</div>
        <div class="kv"><span>类型</span><strong>{{ sourceTypeLabel(selectedSource) }}</strong></div>
        <div class="kv"><span>启用</span><strong>{{ selectedSource.enabled ? '是' : '否' }}</strong></div>
      </section>

      <section v-if="selectedFilter" class="section compact">
        <div class="section-title">状态</div>
        <div class="kv"><span>条件数</span><strong>{{ selectedFilter.conditions.length }}</strong></div>
        <div class="kv"><span>引用规则</span><strong>{{ selectedFilterRules.length }}</strong></div>
      </section>

      <section v-if="selectedSink" class="section compact">
        <div class="section-title">状态</div>
        <div class="kv"><span>类型</span><strong>{{ sinkTypeLabel(selectedSink) }}</strong></div>
        <div class="kv"><span>启用</span><strong>{{ selectedSink.enabled ? '是' : '否' }}</strong></div>
      </section>

      <div class="actions">
        <NButton v-if="selection.kind === 'source' || selection.kind === 'sink'" size="small" @click="emit('create-rule')">沿此建规则</NButton>
        <NButton v-if="selection.kind === 'rule'" size="small" type="primary" @click="emit('edit-rule', selection.id)">编辑规则</NButton>
        <NButton size="small" @click="emit('manage-config')">管理配置</NButton>
        <NButton v-if="selection.kind !== 'filter'" size="small" @click="emit('view-deliveries')">投递记录</NButton>
        <NButton size="small" text type="primary" @click="emit('clear-selection')">清除选中</NButton>
      </div>
    </template>

    <template v-else-if="selectedEdge">
      <header class="inspector-head">
        <span class="eyebrow">连线</span>
        <strong class="title">
          <template v-if="selectedEdge.kind === 'source-rule'">
            {{ sourceName(selectedEdge.sourceId) }} → {{ ruleName(selectedEdge.ruleId) }}
          </template>
          <template v-else-if="selectedEdge.kind === 'filter-rule'">
            {{ filterName(selectedEdge.filterId) }} → {{ ruleName(selectedEdge.ruleId) }}
          </template>
          <template v-else>{{ ruleName(selectedEdge.ruleId) }} → {{ sinkName(selectedEdge.sinkId) }}</template>
        </strong>
      </header>

      <section v-if="selectedEdge.kind === 'source-rule' && selectedEdgeSource && selectedEdgeRuleNode" class="section">
        <div class="section-title">匹配顺序</div>
        <div
          v-for="item in matchingRulesForSource(selectedEdge.sourceId, selectedEdge.ruleId)"
          :key="item.rule.id"
          class="order-row"
          :class="{ active: item.active }"
        >
          <span class="order-no">#{{ item.order }}</span>
          <span class="order-name">{{ item.rule.name }}</span>
          <span class="priority">P{{ item.rule.priority }}</span>
          <span v-if="item.rule.stop_on_match" class="stop">命中即停</span>
        </div>
      </section>

      <section v-if="selectedEdge.kind === 'filter-rule' && selectedEdgeFilter && selectedEdgeRuleNode" class="section">
        <div class="section-title">过滤器绑定</div>
        <div class="kv"><span>过滤器</span><strong>{{ selectedEdgeFilter.name }}</strong></div>
        <div class="kv"><span>条件数</span><strong>{{ selectedEdgeFilter.conditions.length }}</strong></div>
        <div class="kv"><span>规则</span><strong>{{ selectedEdgeRuleNode.rule.name }}</strong></div>
      </section>

      <section v-if="selectedEdge.kind === 'rule-sink' && selectedEdgeSink && selectedEdgeRuleNode" class="section">
        <div class="section-title">目标绑定</div>
        <div class="kv"><span>渠道</span><strong>{{ selectedEdgeSink.name }}</strong></div>
        <div class="kv">
          <span>模板</span>
          <strong>
            <template v-if="selectedEdgeTarget?.template">{{ selectedEdgeTarget.template.name }}</template>
            <template v-else-if="selectedEdgeTarget?.templateId">模板 #{{ selectedEdgeTarget.templateId }} 缺失</template>
            <template v-else>原文投递</template>
          </strong>
        </div>
      </section>

      <div class="actions">
        <NButton size="small" type="error" secondary @click="emit('detach-edge')">解除关联</NButton>
        <NButton size="small" text type="primary" @click="emit('clear-edge')">取消</NButton>
      </div>
    </template>

    <template v-else>
      <header class="inspector-head">
        <span class="eyebrow">路径检查器</span>
        <strong class="title">选择节点或连线</strong>
      </header>
      <p class="empty-text">点选来源、规则、渠道或连线后，这里会显示完整路径、匹配顺序和可执行操作。</p>
    </template>
  </aside>
</template>

<style scoped>
.route-inspector {
  min-height: 420px;
  padding: 14px;
  border: 1px solid var(--clay-border);
  border-radius: 14px;
  background: var(--clay-surface);
  box-shadow: var(--clay-out-sm);
  overflow: hidden;
}

.inspector-head {
  display: grid;
  gap: 4px;
  margin-bottom: 14px;
}

.eyebrow {
  color: var(--clay-text-3);
  font-size: 12px;
  font-weight: 700;
}

.title {
  color: var(--clay-text);
  font-size: 16px;
  line-height: 1.3;
}

.section {
  padding: 12px 0;
  border-top: 1px solid var(--clay-border);
}

.section.compact {
  display: grid;
  gap: 7px;
}

.section-title {
  margin-bottom: 8px;
  color: var(--clay-text-2);
  font-size: 12px;
  font-weight: 800;
}

.route-lines,
.path-list,
.order-list,
.chip-list {
  display: grid;
  gap: 8px;
}

.route-line,
.path-line {
  display: flex;
  align-items: center;
  gap: 7px;
  min-width: 0;
  color: var(--clay-text-2);
  font-size: 12px;
  line-height: 1.4;
}

.pill,
.chip,
.priority,
.stop,
.order-no {
  display: inline-flex;
  align-items: center;
  min-width: 0;
  border-radius: 999px;
  font-size: 11px;
  font-weight: 700;
  line-height: 1.4;
}

.pill {
  max-width: 160px;
  padding: 3px 8px;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.pill.source {
  color: #047857;
  background: rgba(16, 185, 129, 0.14);
}

.pill.rule {
  color: var(--clay-primary);
  background: var(--clay-primary-soft);
}

.pill.sink {
  color: #b45309;
  background: rgba(245, 158, 11, 0.15);
}

.arrow,
.muted,
.empty-text {
  color: var(--clay-text-3);
}

.template-name {
  min-width: 0;
  color: var(--clay-text-3);
  font-size: 11px;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.template-name.warn {
  color: #b45309;
}

.order-group {
  display: grid;
  gap: 6px;
}

.order-row,
.path-card {
  min-width: 0;
  padding: 8px;
  border: 1px solid var(--clay-border);
  border-radius: 10px;
  background: var(--clay-surface-2);
}

.order-row {
  display: flex;
  align-items: center;
  gap: 7px;
}

.order-row.active {
  border-color: color-mix(in srgb, var(--clay-primary) 40%, var(--clay-border));
  background: color-mix(in srgb, var(--clay-primary-soft) 35%, var(--clay-surface));
}

.order-no {
  flex-shrink: 0;
  padding: 1px 7px;
  color: var(--clay-primary);
  background: var(--clay-primary-soft);
}

.order-name {
  flex: 1;
  min-width: 0;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
  color: var(--clay-text);
  font-weight: 700;
}

.priority,
.stop {
  flex-shrink: 0;
  padding: 1px 6px;
  color: var(--clay-text-2);
  background: var(--clay-surface);
}

.stop {
  color: #b45309;
  background: var(--clay-warning-soft);
}

.path-title {
  display: flex;
  align-items: center;
  gap: 7px;
  min-width: 0;
  margin-bottom: 5px;
}

.path-title strong {
  flex: 1;
  min-width: 0;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
  color: var(--clay-text);
}

.chip-list {
  display: flex;
  flex-wrap: wrap;
}

.chip {
  max-width: 160px;
  padding: 2px 8px;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.chip.condition {
  color: #1d6fb8;
  background: rgba(59, 130, 246, 0.12);
}

.chip.processor {
  color: #7c3aed;
  background: rgba(139, 92, 246, 0.12);
}

.kv {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 10px;
  color: var(--clay-text-3);
  font-size: 12px;
}

.kv strong {
  color: var(--clay-text-2);
  text-align: right;
}

.actions {
  display: flex;
  align-items: center;
  gap: 8px;
  flex-wrap: wrap;
  padding-top: 12px;
  border-top: 1px solid var(--clay-border);
}

.empty-text {
  margin: 0;
  font-size: 13px;
  line-height: 1.55;
}
</style>
