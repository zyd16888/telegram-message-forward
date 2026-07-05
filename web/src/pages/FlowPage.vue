<script setup lang="ts">
import { computed, onMounted, shallowRef } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { useMessage } from 'naive-ui'
import PageHeader from '@/components/PageHeader.vue'
import ClayIcon from '@/components/ClayIcon.vue'
import RuleEditorModal from '@/components/rules/RuleEditorModal.vue'
import { filtersApi, rulesApi, sinksApi } from '@/api/client'
import { useForwardingGraph, type FlowRuleGraphNode } from '@/composables/useForwardingGraph'
import type { Filter, Rule, RuleInitialDraft, RuleMeta, Sink, SinkDescriptor, Source } from '@/types'
import { errText } from '@/utils/error'

type NodeKind = 'source' | 'rule' | 'sink'

const router = useRouter()
const route = useRoute()
const message = useMessage()

const { accounts, sources, rules, sinks, templates, loading, ruleNodes, stats, load } = useForwardingGraph()

const ruleMeta = shallowRef<RuleMeta>({ conditions: [], processors: [] })
const sinkDescriptors = shallowRef<SinkDescriptor[]>([])
const filters = shallowRef<Filter[]>([])

const selection = shallowRef<{ kind: NodeKind; id: number } | null>(null)
const keyword = shallowRef('')
const onlyWarnings = shallowRef(false)
const templateFilterId = shallowRef<number | null>(null)

const showRuleModal = shallowRef(false)
const editingRule = shallowRef<Rule | null>(null)
const initialDraft = shallowRef<RuleInitialDraft | null>(null)

const conditionLabelByType = computed(() => new Map(ruleMeta.value.conditions.map((d) => [d.type, d.label])))
const processorLabelByType = computed(() => new Map(ruleMeta.value.processors.map((d) => [d.type, d.label])))
const sinkTypeLabelByType = computed(() => new Map(sinkDescriptors.value.map((d) => [d.type, d.label])))
const accountNameById = computed(() => new Map(accounts.value.map((a) => [a.id, a.name])))

const setupChecklist = computed(() => [
  { label: '配置 Telegram App', done: accounts.value.length > 0, route: 'telegram-config' },
  { label: '同步监听源', done: stats.value.totalSources > 0, route: 'sources' },
  { label: '创建目标渠道', done: stats.value.totalSinks > 0, route: 'sinks' },
  { label: '创建模板', done: stats.value.totalTemplates > 0, route: 'templates' },
  { label: '关联规则', done: stats.value.enabledRules > 0 && stats.value.linkedSources > 0, route: 'rules' },
])
const setupIncomplete = computed(() => setupChecklist.value.some((item) => !item.done))

const templateFilterName = computed(() => {
  if (!templateFilterId.value) return ''
  return templates.value.find((t) => t.id === templateFilterId.value)?.name ?? `#${templateFilterId.value}`
})

// --- 过滤 ---

const visibleRuleNodes = computed(() => {
  const q = keyword.value.trim().toLowerCase()
  return ruleNodes.value.filter((node) => {
    if (templateFilterId.value && !node.targets.some((t) => t.templateId === templateFilterId.value)) return false
    if (onlyWarnings.value && node.warnings.length === 0) return false
    if (!q) return true
    return [
      node.rule.name,
      ...node.sources.map((item) => item.source?.name ?? ''),
      ...node.rule.conditions.map((item) => item.type),
      ...node.rule.processors.map((item) => item.type),
      ...node.targets.map((target) => target.sink?.name ?? ''),
      ...node.targets.map((target) => target.template?.name ?? ''),
    ]
      .filter(Boolean)
      .some((item) => String(item).toLowerCase().includes(q))
  })
})

const visibleSources = computed(() => {
  const q = keyword.value.trim().toLowerCase()
  if (!q) return sources.value
  return sources.value.filter((s) =>
    [s.name, s.username ?? '', String(s.id)].some((item) => item.toLowerCase().includes(q)),
  )
})

const visibleSinks = computed(() => {
  const q = keyword.value.trim().toLowerCase()
  if (!q) return sinks.value
  return sinks.value.filter((s) =>
    [s.name, s.type, sinkTypeLabel(s)].some((item) => item.toLowerCase().includes(q)),
  )
})

// --- 选中与联动高亮 ---

const related = computed(() => {
  const sel = selection.value
  if (!sel) return null
  const sourceIds = new Set<number>()
  const ruleIds = new Set<number>()
  const sinkIds = new Set<number>()
  const collect = (node: FlowRuleGraphNode) => {
    ruleIds.add(node.rule.id)
    node.rule.source_ids.forEach((id) => sourceIds.add(id))
    node.targets.forEach((t) => sinkIds.add(t.sinkId))
  }
  if (sel.kind === 'source') {
    sourceIds.add(sel.id)
    ruleNodes.value.filter((n) => n.rule.source_ids.includes(sel.id)).forEach(collect)
  } else if (sel.kind === 'rule') {
    const node = ruleNodes.value.find((n) => n.rule.id === sel.id)
    if (node) collect(node)
  } else {
    sinkIds.add(sel.id)
    ruleNodes.value.filter((n) => n.targets.some((t) => t.sinkId === sel.id)).forEach(collect)
  }
  return { sourceIds, ruleIds, sinkIds }
})

function cardClass(kind: NodeKind, id: number): string {
  const sel = selection.value
  if (!sel || !related.value) return ''
  if (sel.kind === kind && sel.id === id) return 'selected'
  const set = kind === 'source' ? related.value.sourceIds : kind === 'rule' ? related.value.ruleIds : related.value.sinkIds
  return set.has(id) ? 'linked' : 'dimmed'
}

function toggleSelect(kind: NodeKind, id: number) {
  if (selection.value && selection.value.kind === kind && selection.value.id === id) {
    selection.value = null
  } else {
    selection.value = { kind, id }
  }
}

function clearSelection() {
  selection.value = null
  templateFilterId.value = null
  if (Object.keys(route.query).length) {
    void router.replace({ name: 'flow', query: {} })
  }
}

const selectionLabel = computed(() => {
  const sel = selection.value
  if (!sel) return ''
  if (sel.kind === 'source') return `来源「${sources.value.find((s) => s.id === sel.id)?.name ?? `#${sel.id}`}」`
  if (sel.kind === 'rule') return `规则「${rules.value.find((r) => r.id === sel.id)?.name ?? `#${sel.id}`}」`
  return `渠道「${sinks.value.find((s) => s.id === sel.id)?.name ?? `#${sel.id}`}」`
})

const selectionRuleCount = computed(() => (related.value ? related.value.ruleIds.size : 0))

// --- 展示辅助 ---

function sourceTypeLabel(source: Source): string {
  if (source.type === 'rss') return 'RSS'
  if (source.type === 'webhook') return 'Webhook'
  const displayType = source.config?.display_type
  if (typeof displayType === 'string' && displayType) return displayType
  return 'Telegram'
}

function sinkTypeLabel(sink: Sink): string {
  return sinkTypeLabelByType.value.get(sink.type) ?? sink.type
}

function sourceRuleCount(sourceId: number): number {
  return rules.value.filter((r) => r.source_ids.includes(sourceId)).length
}

function sinkRuleCount(sinkId: number): number {
  return rules.value.filter((r) => r.targets.some((t) => t.sink_id === sinkId)).length
}

function conditionLabel(type: string): string {
  return conditionLabelByType.value.get(type) ?? type
}

function processorLabel(type: string): string {
  return processorLabelByType.value.get(type) ?? type
}

// --- 数据与操作 ---

async function refresh() {
  try {
    await load()
  } catch (e) {
    message.error('加载编排关系失败：' + errText(e))
  }
}

async function loadMeta() {
  try {
    ;[ruleMeta.value, sinkDescriptors.value, filters.value] = await Promise.all([rulesApi.meta(), sinksApi.meta(), filtersApi.list()])
  } catch {
    // 元信息加载失败时退回展示原始类型标识，不阻塞页面。
  }
}

async function toggleRule(rule: Rule, value: boolean) {
  try {
    // 规则更新是全量 PUT，必须回传完整规则体，只改 enabled。
    await rulesApi.update(rule.id, {
      name: rule.name,
      enabled: value,
      priority: rule.priority,
      conditions: rule.conditions,
      processors: rule.processors,
      stop_on_match: rule.stop_on_match,
      source_ids: rule.source_ids,
      targets: rule.targets,
    })
    await refresh()
  } catch (e) {
    message.error('更新规则失败：' + errText(e))
  }
}

function openCreateRule() {
  editingRule.value = null
  initialDraft.value = buildDraftFromSelection()
  showRuleModal.value = true
}

function openEditRule(rule: Rule) {
  editingRule.value = rule
  initialDraft.value = null
  showRuleModal.value = true
}

function buildDraftFromSelection(): RuleInitialDraft | null {
  const sel = selection.value
  if (!sel) return null
  if (sel.kind === 'source') {
    const source = sources.value.find((s) => s.id === sel.id)
    return { name: source ? `转发：${source.name}` : '', source_ids: [sel.id], targets: [] }
  }
  if (sel.kind === 'sink') {
    const sink = sinks.value.find((s) => s.id === sel.id)
    return { name: sink ? `转发 -> ${sink.name}` : '', source_ids: [], targets: [{ sink_id: sel.id, template_id: undefined }] }
  }
  return null
}

function goToDeliveries() {
  const sel = selection.value
  if (!sel) return
  const query: Record<string, string> = {}
  if (sel.kind === 'source') query.source_id = String(sel.id)
  if (sel.kind === 'rule') query.rule_id = String(sel.id)
  if (sel.kind === 'sink') query.sink_id = String(sel.id)
  router.push({ name: 'deliveries', query })
}

function go(name: string) {
  router.push({ name })
}

function queryId(key: string): number | null {
  const raw = route.query[key]
  const id = Number(Array.isArray(raw) ? raw[0] : raw)
  return Number.isFinite(id) && id > 0 ? id : null
}

onMounted(async () => {
  void loadMeta()
  await refresh()
  // 其它页面「查看编排」跳转过来时，直接选中对应节点进入联动高亮。
  const sourceId = queryId('source_id')
  const ruleId = queryId('rule_id')
  const sinkId = queryId('sink_id')
  const templateId = queryId('template_id')
  if (ruleId) selection.value = { kind: 'rule', id: ruleId }
  else if (sourceId) selection.value = { kind: 'source', id: sourceId }
  else if (sinkId) selection.value = { kind: 'sink', id: sinkId }
  if (templateId) templateFilterId.value = templateId
})
</script>

<template>
  <NSpace vertical size="large">
    <PageHeader title="转发编排" desc="点击任意来源、规则或渠道，查看它的上下游链路；规则可直接启停和编辑" icon="flow">
      <template #actions>
        <NButton type="primary" @click="openCreateRule">
          <template #icon><ClayIcon name="plus" :size="16" /></template>
          {{ selection && selection.kind !== 'rule' ? '沿选中项建规则' : '新建规则' }}
        </NButton>
        <NButton secondary :loading="loading" @click="refresh">
          <template #icon><ClayIcon name="refresh" :size="16" /></template>
          刷新
        </NButton>
      </template>
    </PageHeader>

    <section v-if="setupIncomplete" class="setup-panel">
      <div class="setup-title">尚未完成基础配置</div>
      <div class="setup-steps">
        <button
          v-for="item in setupChecklist"
          :key="item.label"
          type="button"
          class="setup-step"
          :class="{ done: item.done }"
          @click="go(item.route)"
        >
          <span class="step-dot">{{ item.done ? '✓' : '!' }}</span>
          <span>{{ item.label }}</span>
        </button>
      </div>
    </section>

    <div class="board-toolbar">
      <div class="toolbar-filters">
        <NInput v-model:value="keyword" clearable class="search-input" placeholder="搜索来源、规则、渠道" />
        <NCheckbox v-model:checked="onlyWarnings">只看异常规则</NCheckbox>
        <NTag v-if="templateFilterId" closable size="small" type="info" @close="templateFilterId = null">
          模板：{{ templateFilterName }}
        </NTag>
      </div>
      <div class="toolbar-stats">
        <span>来源 {{ stats.linkedSources }}/{{ stats.totalSources }} 已接入</span>
        <span>规则 {{ stats.enabledRules }}/{{ stats.totalRules }} 启用</span>
        <span>渠道 {{ stats.enabledSinks }}/{{ stats.totalSinks }} 启用</span>
        <span :class="{ 'stat-warn': stats.warningRules > 0 }">异常 {{ stats.warningRules }}</span>
      </div>
    </div>

    <div v-if="selection" class="context-bar">
      <span class="context-label">
        已选中 {{ selectionLabel }}
        <template v-if="selection.kind !== 'rule'">，关联 {{ selectionRuleCount }} 条规则</template>
      </span>
      <div class="context-actions">
        <NButton v-if="selection.kind !== 'rule'" size="small" @click="openCreateRule">沿此建规则</NButton>
        <NButton size="small" @click="goToDeliveries">查看投递记录</NButton>
        <NButton size="small" text type="primary" @click="clearSelection">清除选中</NButton>
      </div>
    </div>

    <NSpin :show="loading">
      <div v-if="sources.length || rules.length || sinks.length" class="board">
        <!-- 来源列 -->
        <section class="board-column">
          <header class="column-head">
            <span class="column-title">来源</span>
            <span class="column-count">{{ visibleSources.length }}</span>
          </header>
          <div class="card-stack">
            <button
              v-for="source in visibleSources"
              :key="source.id"
              type="button"
              class="node-card"
              :class="cardClass('source', source.id)"
              @click="toggleSelect('source', source.id)"
            >
              <div class="node-head">
                <span class="status-dot" :class="{ on: source.enabled }" />
                <span class="node-name">{{ source.name }}</span>
              </div>
              <div class="node-meta">
                <span class="node-tag">{{ sourceTypeLabel(source) }}</span>
                <span v-if="source.type !== 'rss' && source.type !== 'webhook'" class="node-sub">
                  {{ accountNameById.get(source.account_id) ?? '' }}
                </span>
                <span class="node-sub">{{ sourceRuleCount(source.id) ? `${sourceRuleCount(source.id)} 条规则` : '未接规则' }}</span>
              </div>
            </button>
            <NEmpty v-if="!visibleSources.length" size="small" description="没有匹配的来源" />
          </div>
        </section>

        <!-- 规则列 -->
        <section class="board-column rules-column">
          <header class="column-head">
            <span class="column-title">规则</span>
            <span class="column-count">{{ visibleRuleNodes.length }}</span>
          </header>
          <div class="card-stack">
            <div
              v-for="node in visibleRuleNodes"
              :key="node.rule.id"
              class="node-card rule-card"
              :class="cardClass('rule', node.rule.id)"
              role="button"
              tabindex="0"
              @click="toggleSelect('rule', node.rule.id)"
              @keydown.enter="toggleSelect('rule', node.rule.id)"
            >
              <div class="node-head">
                <span class="node-name rule-name">{{ node.rule.name }}</span>
                <NTooltip v-if="node.warnings.length" trigger="hover">
                  <template #trigger>
                    <span class="warn-badge">{{ node.warnings.length }}</span>
                  </template>
                  <ul class="warn-list">
                    <li v-for="warning in node.warnings" :key="warning">{{ warning }}</li>
                  </ul>
                </NTooltip>
                <span class="rule-actions" @click.stop>
                  <NTooltip trigger="hover">
                    <template #trigger>
                      <NButton size="tiny" quaternary circle @click="openEditRule(node.rule)">
                        <template #icon><ClayIcon name="edit" :size="14" /></template>
                      </NButton>
                    </template>
                    编辑规则
                  </NTooltip>
                  <NSwitch
                    size="small"
                    :value="node.rule.enabled"
                    @update:value="(value: boolean) => toggleRule(node.rule, value)"
                  />
                </span>
              </div>
              <div class="rule-meta">
                优先级 {{ node.rule.priority }}
                <template v-if="node.rule.stop_on_match"> · 命中即停</template>
                · {{ node.sources.length }} 来源 → {{ node.targets.length }} 目标
              </div>
              <div v-if="node.rule.conditions.length || node.rule.processors.length" class="chip-row">
                <span
                  v-for="(condition, index) in node.rule.conditions"
                  :key="`c-${condition.type}-${index}`"
                  class="pipe-chip condition"
                >
                  {{ conditionLabel(condition.type) }}
                </span>
                <span
                  v-for="(processor, index) in node.rule.processors"
                  :key="`p-${processor.type}-${index}`"
                  class="pipe-chip processor"
                >
                  {{ processorLabel(processor.type) }}
                </span>
              </div>
              <div class="target-rows">
                <div v-for="target in node.targets" :key="`${target.sinkId}-${target.templateId ?? 0}`" class="target-row">
                  <span class="target-sink">{{ target.sink?.name ?? `渠道 #${target.sinkId}` }}</span>
                  <span class="target-template">{{ target.template?.name ?? '原文' }}</span>
                </div>
                <div v-if="!node.targets.length" class="target-row empty">未配置目标渠道</div>
              </div>
            </div>
            <NEmpty v-if="!visibleRuleNodes.length" size="small" description="没有匹配的规则">
              <template #extra>
                <NButton size="small" type="primary" @click="openCreateRule">新建规则</NButton>
              </template>
            </NEmpty>
          </div>
        </section>

        <!-- 渠道列 -->
        <section class="board-column">
          <header class="column-head">
            <span class="column-title">渠道</span>
            <span class="column-count">{{ visibleSinks.length }}</span>
          </header>
          <div class="card-stack">
            <button
              v-for="sink in visibleSinks"
              :key="sink.id"
              type="button"
              class="node-card"
              :class="cardClass('sink', sink.id)"
              @click="toggleSelect('sink', sink.id)"
            >
              <div class="node-head">
                <span class="status-dot" :class="{ on: sink.enabled }" />
                <span class="node-name">{{ sink.name }}</span>
              </div>
              <div class="node-meta">
                <span class="node-tag">{{ sinkTypeLabel(sink) }}</span>
                <span class="node-sub">{{ sinkRuleCount(sink.id) ? `${sinkRuleCount(sink.id)} 条规则` : '未接规则' }}</span>
              </div>
            </button>
            <NEmpty v-if="!visibleSinks.length" size="small" description="没有匹配的渠道" />
          </div>
        </section>
      </div>

      <NEmpty v-else description="还没有可编排的来源、规则或渠道">
        <template #extra>
          <NSpace>
            <NButton @click="go('accounts')">去配置账号</NButton>
            <NButton type="primary" @click="go('sources')">去同步来源</NButton>
          </NSpace>
        </template>
      </NEmpty>
    </NSpin>

    <RuleEditorModal
      v-model:show="showRuleModal"
      :rule="editingRule"
      :sources="sources"
      :sinks="sinks"
      :templates="templates"
      :filters="filters"
      :condition-descriptors="ruleMeta.conditions"
      :processor-descriptors="ruleMeta.processors"
      :initial-draft="initialDraft"
      @saved="refresh"
    />
  </NSpace>
</template>

<style scoped>
.setup-panel {
  padding: 16px;
  border: 0;
  border-radius: 14px;
  background: var(--clay-surface);
  box-shadow: var(--clay-extruded-sm);
}

.setup-title {
  margin-bottom: 10px;
  color: var(--clay-text);
  font-size: 14px;
  font-weight: 800;
}

.setup-steps {
  display: grid;
  grid-template-columns: repeat(5, minmax(120px, 1fr));
  gap: 8px;
}

.setup-step {
  display: inline-flex;
  align-items: center;
  gap: 8px;
  min-width: 0;
  padding: 9px 10px;
  border: 0;
  border-radius: 11px;
  color: var(--clay-text-2);
  background: var(--clay-surface-2);
  box-shadow: var(--clay-extruded-sm);
  font-weight: 600;
  cursor: pointer;
  text-align: left;
  transition: box-shadow 0.16s ease, transform 0.16s ease, color 0.16s ease;
}

.setup-step:hover,
.setup-step:focus-visible {
  box-shadow: var(--clay-extruded-hover);
  transform: translateY(-1px);
  outline: none;
}

.setup-step:active {
  box-shadow: var(--clay-inset-sm);
  transform: translateY(1px);
}

.setup-step.done {
  color: #14755f;
  background: var(--clay-success-soft);
}

.step-dot {
  width: 20px;
  height: 20px;
  border-radius: 999px;
  display: grid;
  place-items: center;
  flex-shrink: 0;
  background: var(--clay-surface);
  box-shadow: var(--clay-inset-sm);
  font-size: 12px;
  font-weight: 900;
}

.board-toolbar {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 14px;
  flex-wrap: wrap;
  padding: 10px 12px;
  border-radius: 14px;
  background: var(--clay-surface);
  box-shadow: var(--clay-out-sm);
}

.toolbar-filters {
  display: flex;
  align-items: center;
  gap: 12px;
  flex-wrap: wrap;
}

.search-input {
  width: 260px;
}

.toolbar-stats {
  display: flex;
  align-items: center;
  gap: 14px;
  color: var(--clay-text-2);
  font-size: 13px;
  flex-wrap: wrap;
}

.toolbar-stats span {
  padding: 4px 9px;
  border: 1px solid var(--clay-border);
  border-radius: 999px;
  background: var(--clay-surface-2);
}

.stat-warn {
  color: #b45309;
  font-weight: 700;
}

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

.board {
  display: grid;
  grid-template-columns: minmax(220px, 0.8fr) minmax(320px, 1.6fr) minmax(220px, 0.8fr);
  gap: 14px;
  align-items: start;
}

.board-column {
  border: 1px solid var(--clay-border);
  border-radius: 14px;
  background: var(--clay-surface);
  box-shadow: var(--clay-out-sm);
  overflow: hidden;
}

.column-head {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 10px 14px;
  border-bottom: 1px solid var(--clay-border);
  background: var(--clay-surface-2);
  box-shadow: none;
}

.column-title {
  color: var(--clay-text);
  font-size: 13px;
  font-weight: 800;
}

.column-count {
  min-width: 22px;
  padding: 0 6px;
  border-radius: 999px;
  color: var(--clay-text-2);
  background: var(--clay-surface);
  border: 1px solid var(--clay-border);
  box-shadow: none;
  font-size: 12px;
  font-weight: 700;
  text-align: center;
}

.card-stack {
  display: grid;
  gap: 8px;
  max-height: 640px;
  overflow: auto;
  padding: 10px;
}

.node-card {
  width: 100%;
  min-width: 0;
  padding: 11px 12px;
  border: 1px solid var(--clay-border);
  border-radius: 12px;
  color: inherit;
  background: var(--clay-surface);
  box-shadow: var(--clay-out-sm);
  cursor: pointer;
  text-align: left;
  transition: opacity 0.16s ease, box-shadow 0.16s ease, transform 0.16s ease, background-color 0.16s ease;
}

.node-card:hover,
.node-card:focus-visible {
  border-color: var(--clay-border-strong);
  box-shadow: var(--clay-hover);
  transform: translateY(-1px);
  outline: none;
}

.node-card.selected {
  border-color: var(--clay-primary);
  background: color-mix(in srgb, var(--clay-primary-soft) 55%, var(--clay-surface));
  box-shadow: 0 0 0 2px color-mix(in srgb, var(--clay-primary) 18%, transparent);
  transform: none;
}

.node-card.linked {
  border-color: color-mix(in srgb, var(--clay-primary) 40%, var(--clay-border));
  background: color-mix(in srgb, var(--clay-primary-soft) 25%, var(--clay-surface));
  box-shadow: var(--clay-out-sm);
}

.node-card.dimmed {
  opacity: 0.38;
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
  box-shadow: 0 0 0 4px rgba(16, 185, 129, 0.14);
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
  flex-wrap: wrap;
}

.node-tag {
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
  font-size: 12px;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

/* 规则卡片 */

.rule-card {
  cursor: pointer;
}

.rule-name {
  flex: 1;
  font-size: 14px;
}

.rule-actions {
  display: flex;
  align-items: center;
  gap: 8px;
  flex-shrink: 0;
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

.rule-meta {
  margin-top: 4px;
  color: var(--clay-text-3);
  font-size: 12px;
}

.chip-row {
  display: flex;
  flex-wrap: wrap;
  gap: 6px;
  margin-top: 8px;
}

.pipe-chip {
  max-width: 100%;
  padding: 2px 8px;
  border-radius: 999px;
  font-size: 11px;
  font-weight: 600;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.pipe-chip.condition {
  color: #1d6fb8;
  background: rgba(59, 130, 246, 0.12);
}

.pipe-chip.processor {
  color: #7c3aed;
  background: rgba(139, 92, 246, 0.12);
}

.target-rows {
  display: grid;
  gap: 4px;
  margin-top: 8px;
}

.target-row {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 10px;
  min-width: 0;
  padding: 5px 8px;
  border: 1px solid var(--clay-border);
  border-radius: 10px;
  background: var(--clay-surface-2);
  font-size: 12px;
}

.target-row.empty {
  justify-content: flex-start;
  color: var(--clay-text-3);
  border-style: dashed;
}

.target-sink {
  color: var(--clay-text);
  font-weight: 600;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.target-template {
  flex-shrink: 0;
  color: var(--clay-text-3);
}

@media (max-width: 1100px) {
  .board {
    grid-template-columns: 1fr 1fr;
  }

  .rules-column {
    grid-column: 1 / -1;
    order: -1;
  }
}

@media (max-width: 680px) {
  .board {
    grid-template-columns: 1fr;
  }

  .setup-steps {
    grid-template-columns: repeat(2, minmax(120px, 1fr));
  }

  .search-input {
    width: 100%;
  }
}
</style>
