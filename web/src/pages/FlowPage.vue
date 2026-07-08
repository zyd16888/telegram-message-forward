<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, ref, shallowRef } from 'vue'
import { onBeforeRouteLeave, useRoute, useRouter } from 'vue-router'
import { useDialog, useMessage } from 'naive-ui'
import PageHeader from '@/components/PageHeader.vue'
import ClayIcon from '@/components/ClayIcon.vue'
import ConfigFormRenderer from '@/components/ConfigFormRenderer.vue'
import FlowCanvas from '@/components/flow/FlowCanvas.vue'
import FlowToolbar from '@/components/flow/FlowToolbar.vue'
import RouteInspector from '@/components/flow/RouteInspector.vue'
import ResourceShelf from '@/components/flow/ResourceShelf.vue'
import type {
  CanvasConnection,
  CanvasEdgeInput,
  CanvasNodeInput,
  FilterNodeData,
  ProcessorNodeData,
  RuleNodeData,
  SinkNodeData,
  SourceNodeData,
} from '@/components/flow/types'
import { filtersApi, flowsApi, sinksApi } from '@/api/client'
import { applyPatchToFlowGraph } from '@/utils/flowGraph'
import { useFlowBoard } from '@/composables/useFlowBoard'
import { useForwardingGraph } from '@/composables/useForwardingGraph'
import type {
  ConditionConfig,
  Filter,
  Flow,
  FlowEdge,
  FlowNode,
  FlowNodeType,
  LinearFlow,
  ProcessorConfig,
  RuleItemDescriptor,
  RuleMeta,
  Sink,
  SinkDescriptor,
  Source,
  RuleTarget,
} from '@/types'
import { errText } from '@/utils/error'

const router = useRouter()
const route = useRoute()
const message = useMessage()
const dialog = useDialog()

const { accounts, sources, rules, sinks, templates, loading, ruleNodes, stats, load } = useForwardingGraph()

const ruleMeta = shallowRef<RuleMeta>({ conditions: [], processors: [] })
const sinkDescriptors = shallowRef<SinkDescriptor[]>([])
const filters = shallowRef<Filter[]>([])
const flows = shallowRef<Flow[]>([])
const flowDraft = ref<Flow | null>(null)
const flowError = shallowRef('')
const flowDraftBaseline = shallowRef('null')
const canvasMode = shallowRef<'overview' | 'edit'>('overview')
const activeFlowId = shallowRef<number | null>(null)
const selectedSourceToAdd = shallowRef<number | null>(null)
const selectedFilterToAdd = shallowRef<number | null>(null)
const selectedSinkToAdd = shallowRef<number | null>(null)
const selectedProcessorToAdd = shallowRef<string>('append_source')

const conditionLabelByType = computed(() => new Map(ruleMeta.value.conditions.map((d) => [d.type, d.label])))
const processorLabelByType = computed(() => new Map(ruleMeta.value.processors.map((d) => [d.type, d.label])))
const sinkTypeLabelByType = computed(() => new Map(sinkDescriptors.value.map((d) => [d.type, d.label])))
const accountNameById = computed(() => new Map(accounts.value.map((a) => [a.id, a.name])))
const filterNameById = computed(() => new Map(filters.value.map((f) => [f.id, f.name])))
const sourceById = computed(() => new Map(sources.value.map((s) => [s.id, s])))
const sinkById = computed(() => new Map(sinks.value.map((s) => [s.id, s])))
const templateById = computed(() => new Map(templates.value.map((t) => [t.id, t])))
const flowOptions = computed(() => flows.value.map((f) => ({ label: f.name, value: f.id })))
const sourceOptions = computed(() => sources.value.map((s) => ({ label: sourceOptionLabel(s), value: s.id })))
const filterOptions = computed(() => filters.value.map((f) => ({ label: f.name, value: f.id })))
const sinkOptions = computed(() => sinks.value.map((s) => ({ label: s.name, value: s.id })))
const conditionOptions = computed(() => ruleMeta.value.conditions.map((item) => ({ label: item.label, value: item.type })))
const processorOptions = computed(() => ruleMeta.value.processors.map((p) => ({ label: p.label, value: p.type })))
const templateOptions = computed(() => [
  { label: '原文投递', value: 0 },
  ...templates.value.map((t) => ({ label: `${t.name} · ${t.format}`, value: t.id })),
])
const selectedDraftNode = computed(() => {
  if (canvasMode.value !== 'edit' || !selection.value || !flowDraft.value) return null
  return flowDraft.value.nodes.find((node) => node.id === selection.value?.id) ?? null
})
const selectedDraftNodeUsesSharedFilters = computed(() =>
  selectedDraftNode.value?.type === 'filter' && Boolean(selectedDraftNode.value.config.filter_ids?.length),
)
const flowDraftDirty = computed(() => draftSnapshot() !== flowDraftBaseline.value)

function draftStateClass(id: number): string {
  if (canvasMode.value !== 'edit' || !selection.value) return ''
  return selection.value.id === id ? 'is-selected' : ''
}

const {
  selection,
  selectedEdge,
  keyword,
  onlyWarnings,
  showUnusedNodes,
  showResourceLayer,
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
  { label: '启用 Flow', done: stats.value.enabledRules > 0, route: 'flow' },
  { label: 'Flow 关联来源', done: stats.value.linkedSources > 0, route: 'flow' },
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
    hint: flowReady.value ? '来源、Flow、渠道已形成闭环' : setupIssues.value.map((item) => item.label).join(' / '),
    route: flowReady.value ? 'deliveries' : setupIssues.value[0]?.route ?? 'flow',
  },
  {
    label: '监听来源',
    value: `${stats.value.linkedSources}/${stats.value.totalSources}`,
    tone: stats.value.linkedSources > 0 ? 'good' : 'warn',
    hint: '已关联 Flow / 全部来源',
    route: 'sources',
  },
  {
    label: 'Flow',
    value: `${stats.value.enabledRules}/${stats.value.totalRules}`,
    tone: stats.value.enabledRules > 0 ? 'good' : 'warn',
    hint: '启用 / 全部 Flow',
    route: 'flow',
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
    hint: '来源与 Flow 诊断',
    route: stats.value.warningRules > 0 ? 'flow' : 'sources',
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

function filterName(id: number): string {
  return filterNameById.value.get(id) ?? `过滤器 #${id}`
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
  const name = accountNameById.value.get(source.account_id)
  return name ? `${name} (#${source.account_id})` : `账号 #${source.account_id}`
}

function sourceOptionLabel(source: Source): string {
  return `${source.name} · ${sourceTypeLabel(source)} · ${sourceAccountLabel(source)} · ${source.peer_type}/${source.peer_id}`
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

function defaultsFor(desc?: RuleItemDescriptor): Record<string, unknown> {
  const out: Record<string, unknown> = {}
  for (const field of desc?.fields ?? []) {
    if (field.default !== undefined) out[field.key] = field.default
  }
  return out
}

function conditionDescriptor(type: string): RuleItemDescriptor | undefined {
  return ruleMeta.value.conditions.find((item) => item.type === type)
}

function processorDescriptor(type: string): RuleItemDescriptor | undefined {
  return ruleMeta.value.processors.find((item) => item.type === type)
}

const filterRuleCountById = computed(() => {
  const counts = new Map<number, number>()
  for (const rule of rules.value) {
    for (const filterId of rule.filter_ids) {
      counts.set(filterId, (counts.get(filterId) ?? 0) + 1)
    }
  }
  return counts
})

const visibleFilterIds = computed(() => {
  const ids = new Set<number>()
  if (showResourceLayer.value) {
    for (const node of visibleRuleNodes.value) {
      node.rule.filter_ids.forEach((id) => ids.add(id))
    }
  }
  if (selection.value?.kind === 'rule') {
    rules.value.find((rule) => rule.id === selection.value?.id)?.filter_ids.forEach((id) => ids.add(id))
  }
  if (selection.value?.kind === 'filter') {
    ids.add(selection.value.id)
  }
  // 选中过滤器连线时选中态在 selectedEdge 上（selection 已清空），端点节点必须保持可见。
  if (selectedEdge.value?.kind === 'filter-rule') {
    ids.add(selectedEdge.value.filterId)
  }
  return ids
})

// 匹配序号只对启用 Flow 编号：引擎按 ListEnabledBySource 只评估启用 Flow，停用 Flow 不占位。
const ruleOrderBySource = computed(() => {
  const orderMap = new Map<number, Map<number, number>>()
  for (const source of sources.value) {
    const orderedRules = rules.value
      .filter((rule) => rule.enabled && rule.source_ids.includes(source.id))
      .sort((a, b) => b.priority - a.priority || a.id - b.id)
    orderMap.set(source.id, new Map(orderedRules.map((rule, index) => [rule.id, index + 1])))
  }
  return orderMap
})

function ruleOrderLabel(rule: LinearFlow): string {
  if (!rule.enabled) return `P${rule.priority}`
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

const canvasFilters = computed<CanvasNodeInput<FilterNodeData>[]>(() =>
  filters.value
    .filter((filter) => visibleFilterIds.value.has(filter.id))
    .map((filter) => ({
      id: filter.id,
      stateClass: stateClass('filter', filter.id),
      data: {
        name: filter.name,
        conditionCount: filter.conditions.length,
        ruleCount: filterRuleCountById.value.get(filter.id) ?? 0,
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
        ? node.rule.filter_ids.map((id) => filterName(id))
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
  const filterIds = new Set(canvasFilters.value.map((filter) => filter.id))
  const sinkIds = new Set(visibleSinks.value.map((s) => s.id))
  const rel = related.value
  const out: CanvasEdgeInput[] = []
  for (const node of visibleRuleNodes.value) {
    const ruleId = node.rule.id
    for (const filterId of node.rule.filter_ids) {
      if (!filterIds.has(filterId)) continue
      const hot = rel ? rel.ruleIds.has(ruleId) && rel.filterIds.has(filterId) : false
      out.push({
        key: `f${filterId}:r${ruleId}`,
        from: { kind: 'filter', id: filterId },
        to: { kind: 'rule', id: ruleId },
        label: filterName(filterId),
        warn: !filterNameById.value.has(filterId),
        hot,
        stateClass: rel ? (hot ? 'is-hot' : 'is-faded') : '',
      })
    }
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

const draftSourceNodes = computed<CanvasNodeInput<SourceNodeData>[]>(() =>
  draftNodes('source').map((node) => {
    const source = node.ref_id ? sourceById.value.get(node.ref_id) : null
    return {
      id: node.id,
      position: { x: node.pos_x, y: node.pos_y },
      stateClass: draftStateClass(node.id),
      data: {
        name: source?.name ?? `来源 #${node.ref_id ?? '?'}`,
        typeLabel: source ? sourceTypeLabel(source) : 'Source',
        accountLabel: source ? sourceAccountLabel(source) : '引用缺失',
        runtimeLabel: source ? sourceRuntimeLabel(source) : '保存时后端会校验',
        enabled: source?.enabled ?? false,
        running: source?.runner_status === 'running',
        ruleCount: 0,
      },
    }
  }),
)

const draftFilterNodes = computed<CanvasNodeInput<FilterNodeData>[]>(() =>
  draftNodes('filter').map((node) => {
    const filterIds = node.config.filter_ids ?? []
    const inlineConditions = node.config.conditions ?? []
    const firstFilter = filterIds[0] ? filterNameById.value.get(filterIds[0]) : ''
    return {
      id: node.id,
      position: { x: node.pos_x, y: node.pos_y },
      stateClass: draftStateClass(node.id),
      data: {
        name: firstFilter ?? (inlineConditions.length ? '内联过滤器' : '过滤器'),
        conditionCount: filterIds.length || inlineConditions.length,
        ruleCount: 0,
      },
    }
  }),
)

const draftProcessorNodes = computed<CanvasNodeInput<ProcessorNodeData>[]>(() =>
  draftNodes('processor').map((node) => ({
    id: node.id,
    position: { x: node.pos_x, y: node.pos_y },
    stateClass: draftStateClass(node.id),
    data: {
      name: '处理节点',
      processorChips: (node.config.processors ?? []).map((item) => processorLabel(item.type)),
    },
  })),
)

const draftTargetNodes = computed<CanvasNodeInput<SinkNodeData>[]>(() =>
  draftNodes('target').map((node) => {
    const sink = node.ref_id ? sinkById.value.get(node.ref_id) : null
    const tpl = node.template_id ? templateById.value.get(node.template_id) : null
    return {
      id: node.id,
      position: { x: node.pos_x, y: node.pos_y },
      stateClass: draftStateClass(node.id),
      data: {
        name: sink?.name ?? `渠道 #${node.ref_id ?? '?'}`,
        typeLabel: sink ? sinkTypeLabel(sink) : 'Target',
        enabled: sink?.enabled ?? false,
        deliveryLabel: tpl ? `模板：${tpl.name}` : '原文投递',
        ruleCount: 0,
      },
    }
  }),
)

const draftEdges = computed<CanvasEdgeInput[]>(() => {
  const draft = flowDraft.value
  if (!draft) return []
  const kindByNode = new Map(draft.nodes.map((node) => [node.id, canvasKindOf(node.type)]))
  const out: CanvasEdgeInput[] = []
  for (const edge of draft.edges) {
    const fromKind = kindByNode.get(edge.from_node_id)
    const toKind = kindByNode.get(edge.to_node_id)
    if (!fromKind || !toKind) continue
    out.push({
      key: `e${edge.from_node_id}:${edge.to_node_id}`,
      from: { kind: fromKind, id: edge.from_node_id },
      to: { kind: toKind, id: edge.to_node_id },
    })
  }
  return out
})

const draftHasNodes = computed(() => Boolean(flowDraft.value?.nodes.length))
const canvasHasNodes = computed(
  () => canvasSources.value.length > 0 || canvasFilters.value.length > 0 || canvasRules.value.length > 0 || canvasSinks.value.length > 0,
)

// --- 数据与操作 ---

async function refresh() {
  if (!(await confirmDiscardDraft())) return
  try {
    await Promise.all([load(), loadFlows()])
    ensureEditorSelections()
  } catch (e) {
    message.error('加载编排关系失败：' + errText(e))
  }
}

async function loadMeta() {
  try {
    ;[ruleMeta.value, sinkDescriptors.value, filters.value] = await Promise.all([flowsApi.meta(), sinksApi.meta(), filtersApi.list()])
  } catch {
    // 元信息加载失败时退回展示原始类型标识，不阻塞页面。
  }
}

function ensureEditorSelections() {
  selectedSourceToAdd.value ??= sources.value[0]?.id ?? null
  selectedFilterToAdd.value ??= filters.value[0]?.id ?? null
  selectedSinkToAdd.value ??= sinks.value[0]?.id ?? null
  selectedProcessorToAdd.value ||= ruleMeta.value.processors[0]?.type ?? 'append_source'
}

async function loadFlows() {
  flows.value = await flowsApi.list()
  if (!activeFlowId.value && flows.value.length) {
    activeFlowId.value = flows.value[0].id
  }
  if (activeFlowId.value) {
    const current = flows.value.find((item) => item.id === activeFlowId.value)
    if (current) flowDraft.value = cloneFlow(current)
  }
  if (!flowDraft.value) {
    flowDraft.value = emptyFlowDraft()
  }
  rememberFlowDraftBaseline()
}

function draftNodes(type: FlowNodeType): FlowNode[] {
  return flowDraft.value?.nodes.filter((node) => node.type === type) ?? []
}

function canvasKindOf(type: FlowNodeType) {
  if (type === 'target') return 'target' as const
  return type
}

function cloneFlow(flow: Flow): Flow {
  return JSON.parse(JSON.stringify(flow)) as Flow
}

function flowBodyOf(draft: Flow | null) {
  if (!draft) return null
  return {
    name: draft.name,
    enabled: draft.enabled,
    priority: draft.priority,
    stop_on_match: draft.stop_on_match,
    nodes: draft.nodes,
    edges: draft.edges,
  }
}

function draftSnapshot(): string {
  return JSON.stringify(flowBodyOf(flowDraft.value))
}

function rememberFlowDraftBaseline() {
  flowDraftBaseline.value = draftSnapshot()
}

function resetDraftToSavedState() {
  if (activeFlowId.value) {
    const flow = flows.value.find((item) => item.id === activeFlowId.value)
    flowDraft.value = flow ? cloneFlow(flow) : emptyFlowDraft()
  } else {
    flowDraft.value = emptyFlowDraft()
  }
  flowError.value = ''
  clearPaneSelection()
  rememberFlowDraftBaseline()
}

function confirmDiscardDraft(): Promise<boolean> {
  if (!flowDraftDirty.value) return Promise.resolve(true)
  return new Promise((resolve) => {
    dialog.warning({
      title: '放弃未保存的 Flow 草稿？',
      content: '当前 Flow 有未保存修改。继续操作会丢弃这些草稿内容。',
      positiveText: '放弃草稿',
      negativeText: '继续编辑',
      onPositiveClick: () => resolve(true),
      onNegativeClick: () => resolve(false),
      onClose: () => resolve(false),
    })
  })
}

function emptyFlowDraft(): Flow {
  return {
    id: 0,
    name: '新 Flow',
    enabled: true,
    priority: 0,
    stop_on_match: false,
    nodes: [],
    edges: [],
    created_at: '',
    updated_at: '',
  }
}

function nextTempNodeId(): number {
  const ids = flowDraft.value?.nodes.map((node) => node.id) ?? []
  return Math.min(0, ...ids) - 1
}

function addNode(node: Omit<FlowNode, 'id'>) {
  if (!flowDraft.value) flowDraft.value = emptyFlowDraft()
  flowDraft.value.nodes = [...flowDraft.value.nodes, { ...node, id: nextTempNodeId() }]
  flowError.value = ''
}

function addSourceNode() {
  if (!selectedSourceToAdd.value) return
  addNode({
    type: 'source',
    ref_id: selectedSourceToAdd.value,
    config: {},
    pos_x: 0,
    pos_y: flowDraft.value?.nodes.length ? flowDraft.value.nodes.length * 90 : 0,
  })
}

function addFilterNode() {
  addNode({
    type: 'filter',
    config: selectedFilterToAdd.value ? { filter_ids: [selectedFilterToAdd.value] } : { conditions: [] },
    pos_x: 280,
    pos_y: flowDraft.value?.nodes.length ? flowDraft.value.nodes.length * 90 : 0,
  })
}

function addProcessorNode() {
  const processorType = selectedProcessorToAdd.value || processorOptions.value[0]?.value
  if (!processorType) return
  addNode({
    type: 'processor',
    config: { processors: [{ type: processorType, config: defaultsFor(processorDescriptor(processorType)) }] },
    pos_x: 560,
    pos_y: flowDraft.value?.nodes.length ? flowDraft.value.nodes.length * 90 : 0,
  })
}

function addTargetNode() {
  if (!selectedSinkToAdd.value) return
  addNode({
    type: 'target',
    ref_id: selectedSinkToAdd.value,
    config: {},
    pos_x: 840,
    pos_y: flowDraft.value?.nodes.length ? flowDraft.value.nodes.length * 90 : 0,
  })
}

function onDraftConnect({ from, to }: CanvasConnection) {
  const draft = flowDraft.value
  if (!draft) return
  if (from.id === to.id) {
    message.warning('不能连接节点自身')
    return
  }
  if (draft.edges.some((edge) => edge.from_node_id === from.id && edge.to_node_id === to.id)) {
    message.info('这条连线已经存在')
    return
  }
  draft.edges = [...draft.edges, { id: 0, from_node_id: from.id, to_node_id: to.id }]
  flowError.value = ''
}

function onDraftNodePosition(_kind: string, id: number, position: { x: number; y: number }) {
  const node = flowDraft.value?.nodes.find((item) => item.id === id)
  if (!node) return
  node.pos_x = Math.round(position.x)
  node.pos_y = Math.round(position.y)
}

function removeDraftEdge(key: string) {
  const match = /^e(-?\d+):(-?\d+)$/.exec(key)
  if (!match || !flowDraft.value) return
  const from = Number(match[1])
  const to = Number(match[2])
  flowDraft.value.edges = flowDraft.value.edges.filter((edge) => edge.from_node_id !== from || edge.to_node_id !== to)
}

function removeSelectedDraftNode() {
  const node = selectedDraftNode.value
  if (!node || !flowDraft.value) return
  flowDraft.value.nodes = flowDraft.value.nodes.filter((item) => item.id !== node.id)
  flowDraft.value.edges = flowDraft.value.edges.filter((edge) => edge.from_node_id !== node.id && edge.to_node_id !== node.id)
  clearPaneSelection()
}

function updateSelectedTargetTemplate(value: number | null) {
  const node = selectedDraftNode.value
  if (!node || node.type !== 'target') return
  node.template_id = value && value > 0 ? value : undefined
}

function updateSelectedFilterIds(value: number[] | null) {
  const node = selectedDraftNode.value
  if (!node || node.type !== 'filter') return
  node.config.filter_ids = value ?? []
  if (node.config.filter_ids.length) node.config.conditions = []
}

function addSelectedCondition() {
  const node = selectedDraftNode.value
  const desc = ruleMeta.value.conditions[0]
  if (!node || node.type !== 'filter' || !desc) return
  node.config.filter_ids = []
  node.config.conditions = [...(node.config.conditions ?? []), { type: desc.type, config: defaultsFor(desc) }]
}

function removeSelectedCondition(index: number) {
  const node = selectedDraftNode.value
  if (!node || node.type !== 'filter') return
  node.config.conditions = (node.config.conditions ?? []).filter((_, itemIndex) => itemIndex !== index)
}

function updateSelectedConditionType(item: ConditionConfig, type: string) {
  item.type = type
  item.config = defaultsFor(conditionDescriptor(type))
}

function addSelectedProcessor() {
  const node = selectedDraftNode.value
  const desc = ruleMeta.value.processors[0]
  if (!node || node.type !== 'processor' || !desc) return
  node.config.processors = [...(node.config.processors ?? []), { type: desc.type, config: defaultsFor(desc) }]
}

function removeSelectedProcessor(index: number) {
  const node = selectedDraftNode.value
  if (!node || node.type !== 'processor') return
  node.config.processors = (node.config.processors ?? []).filter((_, itemIndex) => itemIndex !== index)
}

function updateSelectedProcessorType(item: ProcessorConfig, type: string) {
  item.type = type
  item.config = defaultsFor(processorDescriptor(type))
}

async function setCanvasMode(mode: 'overview' | 'edit') {
  if (canvasMode.value === mode) return
  if (mode === 'overview' && flowDraftDirty.value) {
    if (!(await confirmDiscardDraft())) return
    resetDraftToSavedState()
  }
  canvasMode.value = mode
}

async function newFlow() {
  if (!(await confirmDiscardDraft())) return
  activeFlowId.value = null
  flowDraft.value = emptyFlowDraft()
  flowError.value = ''
  canvasMode.value = 'edit'
  clearPaneSelection()
  rememberFlowDraftBaseline()
}

async function selectFlow(id: number) {
  if (id === activeFlowId.value) return
  if (!(await confirmDiscardDraft())) return
  activeFlowId.value = id
  const flow = flows.value.find((item) => item.id === id)
  flowDraft.value = flow ? cloneFlow(flow) : emptyFlowDraft()
  flowError.value = ''
  canvasMode.value = 'edit'
  clearPaneSelection()
  rememberFlowDraftBaseline()
}

async function onFlowSelect(value: number | null) {
  if (value) {
    await selectFlow(value)
  } else {
    await newFlow()
  }
}

async function saveFlow() {
  if (!flowDraft.value) return
  try {
    const body = flowBodyOf(flowDraft.value)
    if (!body) return
    const saved = flowDraft.value.id ? await flowsApi.update(flowDraft.value.id, body) : await flowsApi.create(body)
    message.success('Flow 已保存')
    activeFlowId.value = saved.id
    flowError.value = ''
    await loadFlows()
  } catch (e) {
    flowError.value = errText(e)
    message.error('保存 Flow 失败：' + flowError.value)
  }
}

async function deleteFlow() {
  if (!flowDraft.value?.id) return
  if (!(await confirmDiscardDraft())) return
  try {
    await flowsApi.remove(flowDraft.value.id)
    message.success('Flow 已删除')
    activeFlowId.value = null
    flowDraft.value = null
    await loadFlows()
  } catch (e) {
    message.error('删除 Flow 失败：' + errText(e))
  }
}

// 概览操作基于真实 Flow 图打增量补丁，不走线性投影往返——投影会把
// 编辑模式画出的非线性 DAG 压扁成线性链，静默丢掉分支结构和节点坐标。
async function saveRule(
  rule: LinearFlow,
  patch: Partial<{ enabled: boolean; filter_ids: number[]; source_ids: number[]; targets: RuleTarget[] }>,
  okMessage: string,
) {
  const flow = flows.value.find((item) => item.id === rule.id)
  if (!flow) {
    message.error('Flow 数据未加载，请刷新后重试')
    return
  }
  const result = applyPatchToFlowGraph(flow, patch)
  if (!result.ok) {
    message.warning(`${result.reason}，请切换到编辑模式调整`)
    return
  }
  const body = flowBodyOf(result.flow)
  if (!body) return
  try {
    await flowsApi.update(rule.id, body)
    message.success(okMessage)
    await refresh()
  } catch (e) {
    message.error('更新 Flow 失败：' + errText(e))
  }
}

async function toggleRule(ruleId: number, value: boolean) {
  const rule = rules.value.find((r) => r.id === ruleId)
  if (!rule) return
  await saveRule(rule, { enabled: value }, value ? '已启用 Flow' : '已停用 Flow')
}

async function onCanvasConnect({ from, to }: CanvasConnection) {
  if (canvasMode.value === 'edit') {
    onDraftConnect({ from, to })
    return
  }
  if (from.kind === 'source' && to.kind === 'rule') {
    const rule = rules.value.find((r) => r.id === to.id)
    if (!rule) return
    if (rule.source_ids.includes(from.id)) {
      message.info('该来源已接入此 Flow')
      return
    }
    await saveRule(rule, { source_ids: [...rule.source_ids, from.id] }, `已接入来源「${sourceName(from.id)}」`)
    return
  }
  if (from.kind === 'filter' && to.kind === 'rule') {
    const rule = rules.value.find((r) => r.id === to.id)
    if (!rule) return
    if (rule.filter_ids.includes(from.id)) {
      message.info('该过滤器已接入此 Flow')
      return
    }
    const attach = () => saveRule(rule, { filter_ids: [...rule.filter_ids, from.id] }, `已接入过滤器「${filterName(from.id)}」`)
    // 后端语义：filter_ids 非空即覆盖内联条件。连线绕过了编辑器的二选一，需要用户确认。
    if (rule.conditions.length && !rule.filter_ids.length) {
      dialog.warning({
        title: '接入共享过滤器',
        content: `Flow「${rule.name}」当前使用 ${rule.conditions.length} 个专用条件。接入共享过滤器后将以过滤器为准，专用条件不再生效。`,
        positiveText: '接入并覆盖',
        negativeText: '取消',
        onPositiveClick: () => attach(),
      })
      return
    }
    await attach()
    return
  }
  if (from.kind === 'rule' && to.kind === 'sink') {
    const rule = rules.value.find((r) => r.id === from.id)
    if (!rule) return
    if (rule.targets.some((t) => t.sink_id === to.id)) {
      message.info('该 Flow 已投递到此渠道')
      return
    }
    await saveRule(rule, { targets: [...rule.targets, { sink_id: to.id, template_id: undefined }] }, `已添加目标「${sinkName(to.id)}」`)
    return
  }
  if (from.kind === 'source' && to.kind === 'sink') {
    await newFlowFromSourceToSink(from.id, to.id)
    return
  }
  message.warning('只支持 来源→Flow、过滤器→Flow、Flow→渠道、来源→渠道 四种连线')
}

async function detachSelectedEdge() {
  const edge = selectedEdge.value
  if (!edge) return
  const rule = rules.value.find((r) => r.id === edge.ruleId)
  if (!rule) return
  if (edge.kind === 'source-rule') {
    await saveRule(rule, { source_ids: rule.source_ids.filter((id) => id !== edge.sourceId) }, '已解除来源关联')
  } else if (edge.kind === 'filter-rule') {
    await saveRule(rule, { filter_ids: rule.filter_ids.filter((id) => id !== edge.filterId) }, '已解除过滤器关联')
  } else {
    await saveRule(rule, { targets: rule.targets.filter((t) => t.sink_id !== edge.sinkId) }, '已解除目标关联')
  }
  clearSelectedEdge()
}

async function openCreateRule() {
  await newFlow()
  const sel = selection.value
  if (sel?.kind === 'source') selectedSourceToAdd.value = sel.id
  if (sel?.kind === 'sink') selectedSinkToAdd.value = sel.id
}

async function openEditRule(ruleId: number) {
  await selectFlow(ruleId)
}

async function newFlowFromSourceToSink(sourceID: number, sinkID: number) {
  if (!(await confirmDiscardDraft())) return
  activeFlowId.value = null
  flowDraft.value = emptyFlowDraft()
  flowDraft.value.name = `转发：${sourceName(sourceID)} -> ${sinkName(sinkID)}`
  flowDraft.value.nodes = [
    { id: -1, type: 'source', ref_id: sourceID, config: {}, pos_x: 0, pos_y: 0 },
    { id: -2, type: 'target', ref_id: sinkID, config: {}, pos_x: 420, pos_y: 0 },
  ]
  flowDraft.value.edges = [{ id: 0, from_node_id: -1, to_node_id: -2 }]
  flowError.value = ''
  canvasMode.value = 'edit'
  clearPaneSelection()
  rememberFlowDraftBaseline()
}

function goToDeliveries() {
  const sel = selection.value
  if (!sel || sel.kind === 'filter') return
  const query: Record<string, string> = {}
  if (sel.kind === 'source') query.source_id = String(sel.id)
  if (sel.kind === 'sink') query.sink_id = String(sel.id)
  router.push({ name: 'deliveries', query })
}

function goToSelectedConfig() {
  const sel = selection.value
  if (!sel) return
  if (sel.kind === 'source') router.push({ name: 'sources', query: { source_id: String(sel.id) } })
  if (sel.kind === 'filter') router.push({ name: 'filters', query: { filter_id: String(sel.id) } })
  if (sel.kind === 'rule') void openEditRule(sel.id)
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

function handleBeforeUnload(event: BeforeUnloadEvent) {
  if (!flowDraftDirty.value) return
  event.preventDefault()
  event.returnValue = ''
}

onBeforeRouteLeave(async () => {
  return confirmDiscardDraft()
})

onMounted(async () => {
  window.addEventListener('beforeunload', handleBeforeUnload)
  await loadMeta()
  await refresh()
  // 其它页面「查看编排」跳转过来时，直接选中对应节点进入联动高亮。
  const sourceId = queryId('source_id')
  const filterId = queryId('filter_id')
  const flowId = queryId('flow_id')
  const sinkId = queryId('sink_id')
  const templateId = queryId('template_id')
  if (flowId) selection.value = { kind: 'rule', id: flowId }
  else if (sourceId) selection.value = { kind: 'source', id: sourceId }
  else if (filterId) {
    selection.value = { kind: 'filter', id: filterId }
    showResourceLayer.value = true
  }
  else if (sinkId) selection.value = { kind: 'sink', id: sinkId }
  if (templateId) templateFilterId.value = templateId

  if (route.query.create === '1' && (sourceId || sinkId)) {
    await openCreateRule()
  }
})

onBeforeUnmount(() => {
  window.removeEventListener('beforeunload', handleBeforeUnload)
})
</script>

<template>
  <NSpace vertical size="large">
    <PageHeader
    title="转发编排"
    desc="画布上直接拖线：来源→Flow、Flow→渠道建立关联，来源→渠道快速新建 Flow；点连线可解除关联"
      icon="flow"
    >
      <template #actions>
        <NButton type="primary" @click="openCreateRule">
          <template #icon><ClayIcon name="plus" :size="16" /></template>
          {{ selection && selection.kind !== 'rule' ? '沿选中项建 Flow' : '新建 Flow' }}
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
      v-model:show-resource-layer="showResourceLayer"
      v-model:template-filter-id="templateFilterId"
      :hidden-resource-count="hiddenResourceCount"
      :template-filter-name="templateFilterName"
      :stats="stats"
    />

    <section class="mode-panel">
      <div class="mode-tabs">
        <NButton :type="canvasMode === 'overview' ? 'primary' : 'default'" secondary @click="setCanvasMode('overview')">概览模式</NButton>
        <NButton :type="canvasMode === 'edit' ? 'primary' : 'default'" secondary @click="setCanvasMode('edit')">编辑模式</NButton>
      </div>
      <div v-if="canvasMode === 'edit'" class="flow-select-row">
        <NSelect
          :value="activeFlowId"
          :options="flowOptions"
          clearable
          placeholder="选择已有 Flow"
          @update:value="onFlowSelect"
        />
        <NButton secondary @click="newFlow">新建 Flow</NButton>
        <NButton type="primary" :disabled="!flowDraftDirty" @click="saveFlow">保存 Flow</NButton>
        <NButton v-if="flowDraft?.id" type="error" secondary @click="deleteFlow">删除</NButton>
      </div>
    </section>

    <ResourceShelf
      v-if="canvasMode === 'overview' && !showUnusedNodes && hiddenResourceCount"
      :unused-sources="unusedSources"
      :unused-sinks="unusedSinks"
      :source-type-label="sourceTypeLabel"
      :source-account-label="sourceAccountLabel"
      :sink-type-label="sinkTypeLabel"
      @reveal-resource="revealResource"
      @show-unused-nodes="showUnusedNodes = true"
    />

    <div class="flow-workspace">
      <NSpin :show="loading">
        <FlowCanvas
          v-if="canvasMode === 'edit' && draftHasNodes"
          :key="`flow-edit-${flowDraft?.id ?? 'new'}-${flowDraft?.nodes.length ?? 0}`"
          editable
          :sources="draftSourceNodes"
          :filters="draftFilterNodes"
          :processors="draftProcessorNodes"
          :rules="[]"
          :sinks="[]"
          :targets="draftTargetNodes"
          :edges="draftEdges"
          @select-node="toggleSelect"
          @select-edge="removeDraftEdge"
          @clear-select="onPaneClick"
          @connect="onCanvasConnect"
          @node-position="onDraftNodePosition"
        />

        <NEmpty v-else-if="canvasMode === 'edit'" description="这个 Flow 还没有节点">
          <template #extra>
            <NButton type="primary" @click="addSourceNode">先添加一个来源节点</NButton>
          </template>
        </NEmpty>

        <FlowCanvas
          v-else-if="canvasMode === 'overview' && canvasHasNodes"
          :key="`${showUnusedNodes ? 'all-resources' : 'linked-resources'}-${showResourceLayer ? 'resource-layer' : 'route-layer'}`"
          :sources="canvasSources"
          :filters="canvasFilters"
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

        <NEmpty v-else-if="canvasMode === 'overview' && (sources.length || rules.length || sinks.length)" :description="canvasEmptyHint">
          <template #extra>
            <NSpace>
              <NButton v-if="boardFilterActive" secondary @click="clearBoardFilters">清除筛选</NButton>
              <NButton v-else secondary @click="showUnusedNodes = true">显示未接入资源</NButton>
              <NButton type="primary" @click="openCreateRule">新建 Flow</NButton>
            </NSpace>
          </template>
        </NEmpty>

        <NEmpty v-else-if="canvasMode === 'overview'" description="还没有可编排的来源、Flow 或渠道">
          <template #extra>
            <NSpace>
              <NButton @click="go('accounts')">去配置账号</NButton>
              <NButton type="primary" @click="go('sources')">去同步来源</NButton>
            </NSpace>
          </template>
        </NEmpty>
      </NSpin>

      <RouteInspector
        v-if="canvasMode === 'overview'"
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

      <aside v-else class="flow-editor-panel">
        <header class="editor-head">
          <span class="eyebrow">Flow 编辑</span>
          <strong>{{ flowDraft?.name || '新 Flow' }}</strong>
        </header>

        <section class="editor-section">
          <div class="editor-grid">
            <NInput v-if="flowDraft" v-model:value="flowDraft.name" placeholder="Flow 名称" />
            <NInputNumber v-if="flowDraft" v-model:value="flowDraft.priority" placeholder="优先级" />
          </div>
          <div class="editor-switches">
            <NSwitch v-if="flowDraft" v-model:value="flowDraft.enabled" />
            <span>启用</span>
            <NSwitch v-if="flowDraft" v-model:value="flowDraft.stop_on_match" />
            <span>命中后停止后续 Flow</span>
          </div>
        </section>

        <NAlert v-if="flowError" type="error" :show-icon="false" class="editor-alert">
          {{ flowError }}
        </NAlert>

        <section class="editor-section">
          <div class="section-title">加入资源节点</div>
          <div class="add-row">
            <NSelect v-model:value="selectedSourceToAdd" :options="sourceOptions" filterable placeholder="来源" />
            <NButton @click="addSourceNode">添加来源</NButton>
          </div>
          <div class="add-row">
            <NSelect v-model:value="selectedFilterToAdd" :options="filterOptions" filterable placeholder="共享过滤器" />
            <NButton @click="addFilterNode">添加过滤器</NButton>
          </div>
          <div class="add-row">
            <NSelect v-model:value="selectedProcessorToAdd" :options="processorOptions" filterable placeholder="处理器" />
            <NButton @click="addProcessorNode">添加处理</NButton>
          </div>
          <div class="add-row">
            <NSelect v-model:value="selectedSinkToAdd" :options="sinkOptions" filterable placeholder="目标渠道" />
            <NButton @click="addTargetNode">添加目标</NButton>
          </div>
        </section>

        <section v-if="selectedDraftNode" class="editor-section">
          <div class="section-title">选中节点</div>
          <div class="kv">
            <span>类型</span>
            <strong>{{ selectedDraftNode.type }}</strong>
          </div>
          <template v-if="selectedDraftNode.type === 'filter'">
            <NFormItem label="共享过滤器" :show-feedback="false">
              <NSelect
                :value="selectedDraftNode.config.filter_ids ?? []"
                multiple
                clearable
                filterable
                :options="filterOptions"
                placeholder="不选则使用内联条件"
                @update:value="updateSelectedFilterIds"
              />
            </NFormItem>
            <NAlert v-if="selectedDraftNodeUsesSharedFilters" type="info" :show-icon="false" class="editor-alert">
              当前节点引用共享过滤器；清空后可编辑只属于这个节点的内联条件。
            </NAlert>
            <template v-else>
              <div v-for="(condition, index) in selectedDraftNode.config.conditions ?? []" :key="index" class="node-config-block">
                <NSpace align="center" justify="space-between">
                  <NSelect
                    :value="condition.type"
                    class="type-select"
                    :options="conditionOptions"
                    @update:value="(value: string) => updateSelectedConditionType(condition, value)"
                  />
                  <NButton size="small" type="error" secondary @click="removeSelectedCondition(index)">移除</NButton>
                </NSpace>
                <ConfigFormRenderer
                  v-if="conditionDescriptor(condition.type)"
                  v-model="condition.config"
                  :fields="conditionDescriptor(condition.type)?.fields ?? []"
                />
                <NText v-if="conditionDescriptor(condition.type)?.description" depth="3">
                  {{ conditionDescriptor(condition.type)?.description }}
                </NText>
              </div>
              <NButton size="small" dashed block @click="addSelectedCondition">添加内联条件</NButton>
            </template>
          </template>
          <template v-else-if="selectedDraftNode.type === 'processor'">
            <div v-for="(processor, index) in selectedDraftNode.config.processors ?? []" :key="index" class="node-config-block">
              <NSpace align="center" justify="space-between">
                <NSelect
                  :value="processor.type"
                  class="type-select"
                  :options="processorOptions"
                  @update:value="(value: string) => updateSelectedProcessorType(processor, value)"
                />
                <NButton size="small" type="error" secondary @click="removeSelectedProcessor(index)">移除</NButton>
              </NSpace>
              <ConfigFormRenderer
                v-if="processorDescriptor(processor.type)"
                v-model="processor.config"
                :fields="processorDescriptor(processor.type)?.fields ?? []"
              />
              <NText v-if="processorDescriptor(processor.type)?.description" depth="3">
                {{ processorDescriptor(processor.type)?.description }}
              </NText>
            </div>
            <NButton size="small" dashed block @click="addSelectedProcessor">添加处理器</NButton>
          </template>
          <div v-if="selectedDraftNode.type === 'target'" class="add-row">
            <NSelect
              :value="selectedDraftNode.template_id ?? 0"
              :options="templateOptions"
              placeholder="目标模板"
              @update:value="(value: number | null) => updateSelectedTargetTemplate(value)"
            />
            <NButton secondary @click="updateSelectedTargetTemplate(0)">原文</NButton>
          </div>
          <NButton type="error" secondary @click="removeSelectedDraftNode">删除节点</NButton>
        </section>

        <section class="editor-section compact">
          <div class="section-title">保存校验</div>
          <p class="empty-text">后端会校验 DAG、source/target 方向、至少一条 source→target 通路、节点数和深度上限。点选连线可删除。</p>
        </section>
      </aside>
    </div>
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

.mode-panel {
  display: grid;
  gap: 10px;
  padding: 12px;
  border: 1px solid var(--clay-border);
  border-radius: 12px;
  background: var(--clay-surface);
  box-shadow: var(--clay-out-sm);
}

.mode-tabs,
.flow-select-row,
.editor-switches {
  display: flex;
  align-items: center;
  gap: 8px;
  flex-wrap: wrap;
}

.flow-select-row {
  display: grid;
  grid-template-columns: minmax(220px, 1fr) auto auto auto;
}

.flow-workspace {
  display: grid;
  grid-template-columns: minmax(0, 1fr) 340px;
  gap: 14px;
  align-items: start;
}

.flow-editor-panel {
  min-height: 420px;
  padding: 14px;
  border: 1px solid var(--clay-border);
  border-radius: 14px;
  background: var(--clay-surface);
  box-shadow: var(--clay-out-sm);
}

.editor-head {
  display: grid;
  gap: 4px;
  margin-bottom: 14px;
}

.editor-head strong {
  color: var(--clay-text);
  font-size: 16px;
  line-height: 1.3;
}

.editor-section {
  display: grid;
  gap: 10px;
  padding: 12px 0;
  border-top: 1px solid var(--clay-border);
}

.editor-section.compact {
  gap: 6px;
}

.editor-grid {
  display: grid;
  grid-template-columns: minmax(0, 1fr) 96px;
  gap: 8px;
}

.editor-alert {
  margin: 8px 0;
}

.add-row {
  display: grid;
  grid-template-columns: minmax(0, 1fr) auto;
  gap: 8px;
  align-items: center;
}

.node-config-block {
  display: grid;
  gap: 8px;
  padding: 10px;
  border: 1px solid var(--clay-border);
  border-radius: 8px;
  background: var(--clay-surface-2);
}

.type-select {
  width: min(240px, 100%);
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

.section-title {
  color: var(--clay-text-2);
  font-size: 12px;
  font-weight: 800;
}

.empty-text {
  margin: 0;
  color: var(--clay-text-3);
  font-size: 12px;
  line-height: 1.55;
}

@media (max-width: 1100px) {
  .overview-grid {
    grid-template-columns: repeat(3, minmax(0, 1fr));
  }

  .flow-workspace {
    grid-template-columns: 1fr;
  }

  .flow-select-row {
    grid-template-columns: 1fr 1fr;
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
