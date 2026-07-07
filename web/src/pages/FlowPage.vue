<script setup lang="ts">
import { computed, onMounted, shallowRef } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { useMessage } from 'naive-ui'
import PageHeader from '@/components/PageHeader.vue'
import ClayIcon from '@/components/ClayIcon.vue'
import RuleEditorModal from '@/components/rules/RuleEditorModal.vue'
import FlowCanvas from '@/components/flow/FlowCanvas.vue'
import FlowToolbar from '@/components/flow/FlowToolbar.vue'
import RouteInspector from '@/components/flow/RouteInspector.vue'
import ResourceShelf from '@/components/flow/ResourceShelf.vue'
import type {
  CanvasConnection,
  CanvasEdgeInput,
  CanvasNodeInput,
  RuleNodeData,
  SinkNodeData,
  SourceNodeData,
} from '@/components/flow/types'
import { filtersApi, rulesApi, sinksApi } from '@/api/client'
import { useFlowBoard } from '@/composables/useFlowBoard'
import { useForwardingGraph } from '@/composables/useForwardingGraph'
import type { Filter, Rule, RuleInitialDraft, RuleMeta, Sink, SinkDescriptor, Source, RuleTarget } from '@/types'
import { errText } from '@/utils/error'

const router = useRouter()
const route = useRoute()
const message = useMessage()

const { accounts, sources, rules, sinks, templates, loading, ruleNodes, stats, load } = useForwardingGraph()

const ruleMeta = shallowRef<RuleMeta>({ conditions: [], processors: [] })
const sinkDescriptors = shallowRef<SinkDescriptor[]>([])
const filters = shallowRef<Filter[]>([])

const showRuleModal = shallowRef(false)
const editingRule = shallowRef<Rule | null>(null)
const initialDraft = shallowRef<RuleInitialDraft | null>(null)

const conditionLabelByType = computed(() => new Map(ruleMeta.value.conditions.map((d) => [d.type, d.label])))
const processorLabelByType = computed(() => new Map(ruleMeta.value.processors.map((d) => [d.type, d.label])))
const sinkTypeLabelByType = computed(() => new Map(sinkDescriptors.value.map((d) => [d.type, d.label])))
const accountNameById = computed(() => new Map(accounts.value.map((a) => [a.id, a.name])))
const filterNameById = computed(() => new Map(filters.value.map((f) => [f.id, f.name])))

const {
  selection,
  selectedEdge,
  keyword,
  onlyWarnings,
  showUnusedNodes,
  templateFilterId,
  templateFilterName,
  visibleRuleNodes,
  visibleSources,
  visibleSinks,
  unusedSources,
  unusedSinks,
  hiddenResourceCount,
  related,
  boardFilterActive,
  canvasEmptyHint,
  stateClass,
  toggleSelect,
  clearBoardSelection,
  clearPaneSelection,
  revealResource,
  selectEdge,
  clearSelectedEdge,
  clearBoardFilters,
} = useFlowBoard({ sources, sinks, templates, ruleNodes, sinkTypeLabel })

const setupChecklist = computed(() => [
  { label: '配置 Telegram 账号', done: accounts.value.length > 0, route: 'accounts' },
  { label: '同步监听源', done: stats.value.totalSources > 0, route: 'sources' },
  { label: '创建目标渠道', done: stats.value.totalSinks > 0, route: 'sinks' },
  { label: '启用转发规则', done: stats.value.enabledRules > 0, route: 'rules' },
  { label: '规则关联来源', done: stats.value.linkedSources > 0, route: 'rules' },
])
const setupIssues = computed(() => setupChecklist.value.filter((item) => !item.done))
const setupIncomplete = computed(() => setupIssues.value.length > 0)
const flowReady = computed(() => !setupIncomplete.value)
const templateModeLabel = computed(() =>
  stats.value.totalTemplates > 0 ? `${stats.value.totalTemplates} 个模板可选` : '未创建模板，未指定模板的目标将按原文投递',
)

const overviewCards = computed(() => [
  {
    label: '链路状态',
    value: flowReady.value ? '可运行' : `${setupIssues.value.length} 项待配置`,
    tone: flowReady.value ? 'good' : 'warn',
    hint: flowReady.value ? '来源、规则、渠道已形成闭环' : setupIssues.value.map((item) => item.label).join(' / '),
    route: flowReady.value ? 'deliveries' : setupIssues.value[0]?.route ?? 'rules',
  },
  {
    label: '监听来源',
    value: `${stats.value.linkedSources}/${stats.value.totalSources}`,
    tone: stats.value.linkedSources > 0 ? 'good' : 'warn',
    hint: '已关联规则 / 全部来源',
    route: 'sources',
  },
  {
    label: '转发规则',
    value: `${stats.value.enabledRules}/${stats.value.totalRules}`,
    tone: stats.value.enabledRules > 0 ? 'good' : 'warn',
    hint: '启用 / 全部规则',
    route: 'rules',
  },
  {
    label: '目标渠道',
    value: `${stats.value.enabledSinks}/${stats.value.totalSinks}`,
    tone: stats.value.enabledSinks > 0 ? 'good' : 'warn',
    hint: '启用 / 全部渠道',
    route: 'sinks',
  },
  {
    label: '异常项',
    value: String(stats.value.warningRules + stats.value.warningSources),
    tone: stats.value.warningRules + stats.value.warningSources > 0 ? 'danger' : 'good',
    hint: '来源与规则诊断',
    route: stats.value.warningRules > 0 ? 'rules' : 'sources',
  },
  {
    label: '模板模式',
    value: stats.value.totalTemplates > 0 ? '可选模板' : '原文投递',
    tone: 'neutral',
    hint: templateModeLabel.value,
    route: 'templates',
  },
])

function clearSelection() {
  clearBoardSelection()
  if (Object.keys(route.query).length) {
    void router.replace({ name: 'flow', query: {} })
  }
}

function onPaneClick() {
  clearPaneSelection()
}

// --- 展示辅助 ---

function sourceName(id: number): string {
  return sources.value.find((s) => s.id === id)?.name ?? `#${id}`
}

function ruleName(id: number): string {
  return rules.value.find((r) => r.id === id)?.name ?? `#${id}`
}

function sinkName(id: number): string {
  return sinks.value.find((s) => s.id === id)?.name ?? `#${id}`
}

function sourceTypeLabel(source: Source): string {
  if (source.type === 'rss') return 'RSS'
  if (source.type === 'webhook') return 'Webhook'
  const displayType = source.config?.display_type
  if (typeof displayType === 'string' && displayType) return displayType
  return 'Telegram'
}

function sourceAccountLabel(source: Source): string {
  if (source.type === 'rss') return 'RSS'
  if (source.type === 'webhook') return 'Webhook'
  return accountNameById.value.get(source.account_id) ?? `账号 #${source.account_id}`
}

function formatRuntimeTime(value?: string): string {
  if (!value) return ''
  return new Intl.DateTimeFormat('zh-CN', {
    month: '2-digit',
    day: '2-digit',
    hour: '2-digit',
    minute: '2-digit',
  }).format(new Date(value))
}

function sourceRuntimeLabel(source: Source): string {
  if (!source.enabled) return '未启用'
  if (source.runner_status === 'running') {
    return source.runner_recent_message_at ? `运行中 · 最近 ${formatRuntimeTime(source.runner_recent_message_at)}` : '运行中'
  }
  return source.runner_last_error ? `未运行 · ${source.runner_last_error}` : '未运行'
}

function sinkTypeLabel(sink: Sink): string {
  return sinkTypeLabelByType.value.get(sink.type) ?? sink.type
}

function sinkDeliveryLabel(sink: Sink): string {
  const total = sink.observability?.delivery_total_24h ?? 0
  if (!total) return '24h 无投递'
  const success = sink.observability?.delivery_success_24h ?? 0
  const rate = Math.round((sink.observability?.success_rate_24h ?? 0) * 100)
  return `24h ${rate}% · ${success}/${total}`
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

const ruleOrderBySource = computed(() => {
  const orderMap = new Map<number, Map<number, number>>()
  for (const source of sources.value) {
    const orderedRules = rules.value
      .filter((rule) => rule.source_ids.includes(source.id))
      .sort((a, b) => b.priority - a.priority || a.id - b.id)
    orderMap.set(source.id, new Map(orderedRules.map((rule, index) => [rule.id, index + 1])))
  }
  return orderMap
})

function ruleOrderLabel(rule: Rule): string {
  const selectedSourceId = selection.value?.kind === 'source' ? selection.value.id : null
  if (selectedSourceId && rule.source_ids.includes(selectedSourceId)) {
    return `#${ruleOrderBySource.value.get(selectedSourceId)?.get(rule.id) ?? 1}`
  }
  if (rule.source_ids.length === 1) {
    return `#${ruleOrderBySource.value.get(rule.source_ids[0])?.get(rule.id) ?? 1}`
  }
  return `P${rule.priority}`
}

// --- 画布数据组装 ---

const canvasSources = computed<CanvasNodeInput<SourceNodeData>[]>(() =>
  visibleSources.value.map((source) => ({
    id: source.id,
    stateClass: stateClass('source', source.id),
    data: {
      name: source.name,
      typeLabel: sourceTypeLabel(source),
      accountLabel: sourceAccountLabel(source),
      runtimeLabel: sourceRuntimeLabel(source),
      enabled: source.enabled,
      running: source.runner_status === 'running',
      ruleCount: sourceRuleCount(source.id),
    },
  })),
)

const canvasRules = computed<CanvasNodeInput<RuleNodeData>[]>(() =>
  visibleRuleNodes.value.map((node) => ({
    id: node.rule.id,
    stateClass: stateClass('rule', node.rule.id),
    data: {
      name: node.rule.name,
      enabled: node.rule.enabled,
      priority: node.rule.priority,
      orderLabel: ruleOrderLabel(node.rule),
      stopOnMatch: node.rule.stop_on_match,
      conditionChips: node.rule.filter_ids.length
        ? node.rule.filter_ids.map((id) => filterNameById.value.get(id) ?? `过滤器 #${id}`)
        : node.rule.conditions.map((item) => conditionLabel(item.type)),
      processorChips: node.rule.processors.map((item) => processorLabel(item.type)),
      warnings: node.warnings,
    },
  })),
)

const canvasSinks = computed<CanvasNodeInput<SinkNodeData>[]>(() =>
  visibleSinks.value.map((sink) => ({
    id: sink.id,
    stateClass: stateClass('sink', sink.id),
    data: {
      name: sink.name,
      typeLabel: sinkTypeLabel(sink),
      enabled: sink.enabled,
      deliveryLabel: sinkDeliveryLabel(sink),
      ruleCount: sinkRuleCount(sink.id),
    },
  })),
)

const canvasEdges = computed<CanvasEdgeInput[]>(() => {
  const sourceIds = new Set(visibleSources.value.map((s) => s.id))
  const sinkIds = new Set(visibleSinks.value.map((s) => s.id))
  const rel = related.value
  const out: CanvasEdgeInput[] = []
  for (const node of visibleRuleNodes.value) {
    const ruleId = node.rule.id
    for (const src of node.sources) {
      if (!sourceIds.has(src.sourceId)) continue
      const hot = rel ? rel.ruleIds.has(ruleId) && rel.sourceIds.has(src.sourceId) : false
      out.push({
        key: `s${src.sourceId}:r${ruleId}`,
        from: { kind: 'source', id: src.sourceId },
        to: { kind: 'rule', id: ruleId },
        warn: Boolean(src.source && !src.source.enabled),
        hot,
        stateClass: rel ? (hot ? 'is-hot' : 'is-faded') : '',
      })
    }
    for (const target of node.targets) {
      if (!sinkIds.has(target.sinkId)) continue
      const hot = rel ? rel.ruleIds.has(ruleId) && rel.sinkIds.has(target.sinkId) : false
      const templateMissing = Boolean(target.templateId && !target.template)
      out.push({
        key: `r${ruleId}:k${target.sinkId}`,
        from: { kind: 'rule', id: ruleId },
        to: { kind: 'sink', id: target.sinkId },
        label: target.template?.name ?? (templateMissing ? `模板 #${target.templateId} 缺失` : undefined),
        warn: templateMissing || Boolean(target.sink && !target.sink.enabled),
        hot,
        stateClass: rel ? (hot ? 'is-hot' : 'is-faded') : '',
      })
    }
  }
  return out
})

const canvasHasNodes = computed(() => canvasSources.value.length > 0 || canvasRules.value.length > 0 || canvasSinks.value.length > 0)

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

// 规则更新是全量 PUT：必须回传含 filter_ids 在内的完整规则体，只覆盖 patch 字段。
function ruleBodyOf(rule: Rule) {
  return {
    name: rule.name,
    enabled: rule.enabled,
    priority: rule.priority,
    filter_ids: rule.filter_ids,
    conditions: rule.filter_ids.length ? [] : rule.conditions,
    processors: rule.processors,
    stop_on_match: rule.stop_on_match,
    source_ids: rule.source_ids,
    targets: rule.targets,
  }
}

async function saveRule(
  rule: Rule,
  patch: Partial<{ enabled: boolean; source_ids: number[]; targets: RuleTarget[] }>,
  okMessage: string,
) {
  try {
    await rulesApi.update(rule.id, { ...ruleBodyOf(rule), ...patch })
    message.success(okMessage)
    await refresh()
  } catch (e) {
    message.error('更新规则失败：' + errText(e))
  }
}

async function toggleRule(ruleId: number, value: boolean) {
  const rule = rules.value.find((r) => r.id === ruleId)
  if (!rule) return
  await saveRule(rule, { enabled: value }, value ? '已启用规则' : '已停用规则')
}

async function onCanvasConnect({ from, to }: CanvasConnection) {
  if (from.kind === 'source' && to.kind === 'rule') {
    const rule = rules.value.find((r) => r.id === to.id)
    if (!rule) return
    if (rule.source_ids.includes(from.id)) {
      message.info('该来源已接入此规则')
      return
    }
    await saveRule(rule, { source_ids: [...rule.source_ids, from.id] }, `已接入来源「${sourceName(from.id)}」`)
    return
  }
  if (from.kind === 'rule' && to.kind === 'sink') {
    const rule = rules.value.find((r) => r.id === from.id)
    if (!rule) return
    if (rule.targets.some((t) => t.sink_id === to.id)) {
      message.info('该规则已投递到此渠道')
      return
    }
    await saveRule(rule, { targets: [...rule.targets, { sink_id: to.id, template_id: undefined }] }, `已添加目标「${sinkName(to.id)}」`)
    return
  }
  if (from.kind === 'source' && to.kind === 'sink') {
    editingRule.value = null
    initialDraft.value = {
      name: `转发：${sourceName(from.id)} -> ${sinkName(to.id)}`,
      source_ids: [from.id],
      targets: [{ sink_id: to.id, template_id: undefined }],
    }
    showRuleModal.value = true
    return
  }
  message.warning('只支持 来源→规则、规则→渠道、来源→渠道 三种连线')
}

async function detachSelectedEdge() {
  const edge = selectedEdge.value
  if (!edge) return
  const rule = rules.value.find((r) => r.id === edge.ruleId)
  if (!rule) return
  if (edge.kind === 'source-rule') {
    await saveRule(rule, { source_ids: rule.source_ids.filter((id) => id !== edge.sourceId) }, '已解除来源关联')
  } else {
    await saveRule(rule, { targets: rule.targets.filter((t) => t.sink_id !== edge.sinkId) }, '已解除目标关联')
  }
  clearSelectedEdge()
}

function openCreateRule() {
  editingRule.value = null
  initialDraft.value = buildDraftFromSelection()
  showRuleModal.value = true
}

function openEditRule(ruleId: number) {
  const rule = rules.value.find((r) => r.id === ruleId)
  if (!rule) return
  editingRule.value = rule
  initialDraft.value = null
  showRuleModal.value = true
}

function buildDraftFromSelection(): RuleInitialDraft | null {
  const sel = selection.value
  if (!sel) return null
  if (sel.kind === 'source') {
    return { name: `转发：${sourceName(sel.id)}`, source_ids: [sel.id], targets: [] }
  }
  if (sel.kind === 'sink') {
    return { name: `转发 -> ${sinkName(sel.id)}`, source_ids: [], targets: [{ sink_id: sel.id, template_id: undefined }] }
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

function goToSelectedConfig() {
  const sel = selection.value
  if (!sel) return
  if (sel.kind === 'source') router.push({ name: 'sources', query: { source_id: String(sel.id) } })
  if (sel.kind === 'rule') router.push({ name: 'rules', query: { rule_id: String(sel.id) } })
  if (sel.kind === 'sink') router.push({ name: 'sinks', query: { sink_id: String(sel.id) } })
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
    <PageHeader
      title="转发编排"
      desc="画布上直接拖线：来源→规则、规则→渠道建立关联，来源→渠道快速新建规则；点连线可解除关联"
      icon="flow"
    >
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

    <section class="overview-grid">
      <button
        v-for="item in overviewCards"
        :key="item.label"
        type="button"
        class="overview-card"
        :class="item.tone"
        @click="go(item.route)"
      >
        <span class="overview-label">{{ item.label }}</span>
        <strong class="overview-value">{{ item.value }}</strong>
        <span class="overview-hint">{{ item.hint }}</span>
      </button>
    </section>

    <section v-if="setupIncomplete" class="setup-panel">
      <div class="setup-copy">
        <div class="setup-title">链路还差 {{ setupIssues.length }} 项才能自动转发</div>
        <div class="setup-desc">模板不是必需配置；没有模板时会按原文或默认文本投递。</div>
      </div>
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
    <section v-else-if="stats.totalTemplates === 0" class="template-note">
      <div>
        <strong>当前使用原文投递</strong>
        <span>未创建渲染模板不会阻塞转发；需要统一格式时再补模板即可。</span>
      </div>
      <NButton size="small" secondary @click="go('templates')">管理模板</NButton>
    </section>

    <FlowToolbar
      v-model:keyword="keyword"
      v-model:only-warnings="onlyWarnings"
      v-model:show-unused-nodes="showUnusedNodes"
      v-model:template-filter-id="templateFilterId"
      :hidden-resource-count="hiddenResourceCount"
      :template-filter-name="templateFilterName"
      :stats="stats"
    />

    <ResourceShelf
      v-if="!showUnusedNodes && hiddenResourceCount"
      :unused-sources="unusedSources"
      :unused-sinks="unusedSinks"
      :source-type-label="sourceTypeLabel"
      :sink-type-label="sinkTypeLabel"
      @reveal-resource="revealResource"
      @show-unused-nodes="showUnusedNodes = true"
    />

    <div class="flow-workspace">
      <NSpin :show="loading">
        <FlowCanvas
          v-if="canvasHasNodes"
          :key="showUnusedNodes ? 'all-resources' : 'linked-resources'"
          :sources="canvasSources"
          :rules="canvasRules"
          :sinks="canvasSinks"
          :edges="canvasEdges"
          @select-node="toggleSelect"
          @select-edge="selectEdge"
          @clear-select="onPaneClick"
          @connect="onCanvasConnect"
          @edit-rule="openEditRule"
          @toggle-rule="toggleRule"
        />

        <NEmpty v-else-if="sources.length || rules.length || sinks.length" :description="canvasEmptyHint">
          <template #extra>
            <NSpace>
              <NButton v-if="boardFilterActive" secondary @click="clearBoardFilters">清除筛选</NButton>
              <NButton v-else secondary @click="showUnusedNodes = true">显示未接入资源</NButton>
              <NButton type="primary" @click="openCreateRule">新建规则</NButton>
            </NSpace>
          </template>
        </NEmpty>

        <NEmpty v-else description="还没有可编排的来源、规则或渠道">
          <template #extra>
            <NSpace>
              <NButton @click="go('accounts')">去配置账号</NButton>
              <NButton type="primary" @click="go('sources')">去同步来源</NButton>
            </NSpace>
          </template>
        </NEmpty>
      </NSpin>

      <RouteInspector
        :selection="selection"
        :selected-edge="selectedEdge"
        :rule-nodes="ruleNodes"
        :sources="sources"
        :sinks="sinks"
        :filters="filters"
        :source-type-label="sourceTypeLabel"
        :sink-type-label="sinkTypeLabel"
        :condition-label="conditionLabel"
        :processor-label="processorLabel"
        @create-rule="openCreateRule"
        @edit-rule="openEditRule"
        @manage-config="goToSelectedConfig"
        @view-deliveries="goToDeliveries"
        @clear-selection="clearSelection"
        @detach-edge="detachSelectedEdge"
        @clear-edge="clearSelectedEdge"
      />
    </div>

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
.overview-grid {
  display: grid;
  grid-template-columns: repeat(6, minmax(130px, 1fr));
  gap: 10px;
}

.overview-card {
  min-width: 0;
  min-height: 92px;
  padding: 12px;
  border: 1px solid var(--clay-border);
  border-radius: 12px;
  background: var(--clay-surface);
  box-shadow: var(--clay-out-sm);
  color: inherit;
  text-align: left;
  cursor: pointer;
  transition: border-color 0.16s ease, box-shadow 0.16s ease, transform 0.16s ease;
}

.overview-card:hover,
.overview-card:focus-visible {
  border-color: var(--clay-border-strong);
  box-shadow: var(--clay-hover);
  transform: translateY(-1px);
  outline: none;
}

.overview-label,
.overview-hint {
  display: block;
  color: var(--clay-text-3);
  font-size: 12px;
  line-height: 1.35;
}

.overview-value {
  display: block;
  margin: 4px 0;
  color: var(--clay-text);
  font-size: 22px;
  line-height: 1.15;
  font-weight: 900;
}

.overview-card.good .overview-value {
  color: #14755f;
}

.overview-card.warn .overview-value {
  color: #b45309;
}

.overview-card.danger .overview-value {
  color: #b91c1c;
}

.setup-panel {
  display: grid;
  grid-template-columns: minmax(220px, 0.7fr) minmax(0, 1.3fr);
  gap: 14px;
  align-items: center;
  padding: 14px;
  border: 0;
  border-radius: 12px;
  background: var(--clay-surface);
  box-shadow: var(--clay-extruded-sm);
}

.setup-title {
  color: var(--clay-text);
  font-size: 14px;
  font-weight: 800;
}

.setup-desc {
  margin-top: 5px;
  color: var(--clay-text-3);
  font-size: 12px;
  line-height: 1.45;
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
  box-shadow: var(--clay-inset-deep);
  transform: scale(0.96);
  transition-duration: 0.08s;
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

.template-note {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
  flex-wrap: wrap;
  padding: 12px 14px;
  border: 1px solid color-mix(in srgb, var(--clay-primary) 22%, var(--clay-border));
  border-radius: 12px;
  background: color-mix(in srgb, var(--clay-primary-soft) 38%, var(--clay-surface));
}

.template-note strong {
  display: block;
  color: var(--clay-text);
  font-size: 13px;
}

.template-note span {
  display: block;
  margin-top: 3px;
  color: var(--clay-text-3);
  font-size: 12px;
}

.flow-workspace {
  display: grid;
  grid-template-columns: minmax(0, 1fr) 340px;
  gap: 14px;
  align-items: start;
}

@media (max-width: 1100px) {
  .overview-grid {
    grid-template-columns: repeat(3, minmax(0, 1fr));
  }

  .flow-workspace {
    grid-template-columns: 1fr;
  }
}

@media (max-width: 680px) {
  .overview-grid {
    grid-template-columns: repeat(2, minmax(0, 1fr));
  }

  .setup-panel {
    grid-template-columns: 1fr;
  }

  .setup-steps {
    grid-template-columns: repeat(2, minmax(120px, 1fr));
  }

}
</style>
