import { computed, shallowRef, type ComputedRef, type Ref } from 'vue'
import type { FlowNodeKind } from '@/components/flow/types'
import type { Sink, Source, Template } from '@/types'
import type { FlowRuleGraphNode } from '@/composables/useForwardingGraph'

export type FlowBoardNodeKind = FlowNodeKind

export type FlowBoardSelection = { kind: FlowBoardNodeKind; id: number }

export type FlowBoardEdgeRef =
  | { kind: 'source-rule'; sourceId: number; ruleId: number }
  | { kind: 'filter-rule'; filterId: number; ruleId: number }
  | { kind: 'rule-sink'; ruleId: number; sinkId: number }

interface UseFlowBoardOptions {
  sources: Ref<Source[]>
  sinks: Ref<Sink[]>
  templates: Ref<Template[]>
  ruleNodes: ComputedRef<FlowRuleGraphNode[]>
  sinkTypeLabel: (sink: Sink) => string
}

export function useFlowBoard({ sources, sinks, templates, ruleNodes, sinkTypeLabel }: UseFlowBoardOptions) {
  const selection = shallowRef<FlowBoardSelection | null>(null)
  const selectedEdge = shallowRef<FlowBoardEdgeRef | null>(null)
  const keyword = shallowRef('')
  const onlyWarnings = shallowRef(false)
  const showUnusedNodes = shallowRef(false)
  const showResourceLayer = shallowRef(false)
  const templateFilterId = shallowRef<number | null>(null)

  const templateFilterName = computed(() => {
    if (!templateFilterId.value) return ''
    return templates.value.find((t) => t.id === templateFilterId.value)?.name ?? `#${templateFilterId.value}`
  })

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

  const searchedSources = computed(() => {
    const q = keyword.value.trim().toLowerCase()
    if (!q) return sources.value
    return sources.value.filter((s) =>
      [s.name, s.username ?? '', String(s.id)].some((item) => item.toLowerCase().includes(q)),
    )
  })

  const searchedSinks = computed(() => {
    const q = keyword.value.trim().toLowerCase()
    if (!q) return sinks.value
    return sinks.value.filter((s) =>
      [s.name, s.type, sinkTypeLabel(s)].some((item) => item.toLowerCase().includes(q)),
    )
  })

  const linkedSourceIds = computed(() => {
    const ids = new Set<number>()
    for (const node of ruleNodes.value) {
      node.rule.source_ids.forEach((id) => ids.add(id))
    }
    return ids
  })

  const linkedSinkIds = computed(() => {
    const ids = new Set<number>()
    for (const node of ruleNodes.value) {
      node.targets.forEach((target) => ids.add(target.sinkId))
    }
    return ids
  })

  const visibleRuleSourceIds = computed(() => {
    const ids = new Set<number>()
    for (const node of visibleRuleNodes.value) {
      node.sources.forEach((source) => ids.add(source.sourceId))
    }
    return ids
  })

  const visibleRuleSinkIds = computed(() => {
    const ids = new Set<number>()
    for (const node of visibleRuleNodes.value) {
      node.targets.forEach((target) => ids.add(target.sinkId))
    }
    return ids
  })

  const visibleSources = computed(() =>
    searchedSources.value.filter(
      (source) =>
        showUnusedNodes.value ||
        visibleRuleSourceIds.value.has(source.id) ||
        (selection.value?.kind === 'source' && selection.value.id === source.id),
    ),
  )

  const visibleSinks = computed(() =>
    searchedSinks.value.filter(
      (sink) =>
        showUnusedNodes.value ||
        visibleRuleSinkIds.value.has(sink.id) ||
        (selection.value?.kind === 'sink' && selection.value.id === sink.id),
    ),
  )

  const unusedSources = computed(() => searchedSources.value.filter((source) => !linkedSourceIds.value.has(source.id)))
  const unusedSinks = computed(() => searchedSinks.value.filter((sink) => !linkedSinkIds.value.has(sink.id)))
  const hiddenResourceCount = computed(() => (!showUnusedNodes.value ? unusedSources.value.length + unusedSinks.value.length : 0))

  const related = computed(() => {
    const sel = selection.value
    if (!sel) return null
    const sourceIds = new Set<number>()
    const filterIds = new Set<number>()
    const ruleIds = new Set<number>()
    const sinkIds = new Set<number>()
    const collect = (node: FlowRuleGraphNode) => {
      ruleIds.add(node.rule.id)
      node.rule.source_ids.forEach((id) => sourceIds.add(id))
      node.rule.filter_ids.forEach((id) => filterIds.add(id))
      node.targets.forEach((t) => sinkIds.add(t.sinkId))
    }
    if (sel.kind === 'source') {
      sourceIds.add(sel.id)
      ruleNodes.value.filter((n) => n.rule.source_ids.includes(sel.id)).forEach(collect)
    } else if (sel.kind === 'filter') {
      filterIds.add(sel.id)
      ruleNodes.value.filter((n) => n.rule.filter_ids.includes(sel.id)).forEach(collect)
    } else if (sel.kind === 'rule') {
      const node = ruleNodes.value.find((n) => n.rule.id === sel.id)
      if (node) collect(node)
    } else {
      sinkIds.add(sel.id)
      ruleNodes.value.filter((n) => n.targets.some((t) => t.sinkId === sel.id)).forEach(collect)
    }
    return { sourceIds, filterIds, ruleIds, sinkIds }
  })

  const selectionRuleCount = computed(() => (related.value ? related.value.ruleIds.size : 0))

  const boardFilterActive = computed(
    () => Boolean(keyword.value.trim()) || onlyWarnings.value || Boolean(templateFilterId.value),
  )

  const canvasEmptyHint = computed(() => {
    if (keyword.value.trim()) return `没有匹配「${keyword.value.trim()}」的来源、Flow 或渠道`
    if (boardFilterActive.value) return '当前筛选条件下没有匹配的编排链路'
    return '当前没有已接入的编排链路'
  })

  function stateClass(kind: FlowBoardNodeKind, id: number): string {
    const sel = selection.value
    if (!sel || !related.value) return ''
    if (sel.kind === kind && sel.id === id) return 'is-selected'
    const set =
      kind === 'source'
        ? related.value.sourceIds
        : kind === 'filter'
          ? related.value.filterIds
          : kind === 'rule'
            ? related.value.ruleIds
            : related.value.sinkIds
    return set.has(id) ? 'is-linked' : 'is-dimmed'
  }

  function toggleSelect(kind: FlowBoardNodeKind, id: number) {
    selectedEdge.value = null
    if (selection.value && selection.value.kind === kind && selection.value.id === id) {
      selection.value = null
    } else {
      selection.value = { kind, id }
    }
  }

  function clearBoardSelection() {
    selection.value = null
    selectedEdge.value = null
    templateFilterId.value = null
  }

  function clearPaneSelection() {
    selection.value = null
    selectedEdge.value = null
  }

  function revealResource(kind: Exclude<FlowBoardNodeKind, 'filter' | 'rule'>, id: number) {
    showUnusedNodes.value = true
    selectedEdge.value = null
    selection.value = { kind, id }
  }

  function selectEdge(key: string) {
    selection.value = null
    selectedEdge.value = parseEdgeKey(key)
  }

  function clearSelectedEdge() {
    selectedEdge.value = null
  }

  function clearBoardFilters() {
    keyword.value = ''
    onlyWarnings.value = false
    templateFilterId.value = null
  }

  return {
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
    selectionRuleCount,
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
  }
}

function parseEdgeKey(key: string): FlowBoardEdgeRef | null {
  let match = /^s(\d+):r(\d+)$/.exec(key)
  if (match) return { kind: 'source-rule', sourceId: Number(match[1]), ruleId: Number(match[2]) }
  match = /^f(\d+):r(\d+)$/.exec(key)
  if (match) return { kind: 'filter-rule', filterId: Number(match[1]), ruleId: Number(match[2]) }
  match = /^r(\d+):k(\d+)$/.exec(key)
  if (match) return { kind: 'rule-sink', ruleId: Number(match[1]), sinkId: Number(match[2]) }
  return null
}
