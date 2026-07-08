// 概览模式的连线/启停操作对真实 Flow 图打补丁。
//
// 不能走「Flow → 线性投影 → 重建请求」的往返：编辑模式画出的非线性 DAG
// 会被投影压扁成线性链，分支结构与节点坐标全部丢失。这里直接在原图上
// 做增量修改；拓扑有歧义（多过滤节点、同渠道多目标节点等）时拒绝操作，
// 由调用方引导用户去编辑模式调整。
import type { Flow, FlowNode, RuleTarget } from '@/types'

export interface FlowGraphPatch {
  enabled?: boolean
  source_ids?: number[]
  filter_ids?: number[]
  targets?: RuleTarget[]
}

export type FlowGraphPatchResult = { ok: true; flow: Flow } | { ok: false; reason: string }

export function applyPatchToFlowGraph(flow: Flow, patch: FlowGraphPatch): FlowGraphPatchResult {
  const draft = JSON.parse(JSON.stringify(flow)) as Flow

  if (patch.enabled !== undefined) {
    draft.enabled = patch.enabled
  }

  if (patch.source_ids) {
    const reason = patchSources(draft, patch.source_ids)
    if (reason) return { ok: false, reason }
  }

  if (patch.filter_ids) {
    const reason = patchFilters(draft, patch.filter_ids)
    if (reason) return { ok: false, reason }
  }

  if (patch.targets) {
    const reason = patchTargets(draft, patch.targets)
    if (reason) return { ok: false, reason }
  }

  return { ok: true, flow: draft }
}

function nextTempNodeId(draft: Flow): number {
  return Math.min(0, ...draft.nodes.map((node) => node.id)) - 1
}

function removeNode(draft: Flow, nodeId: number) {
  draft.nodes = draft.nodes.filter((node) => node.id !== nodeId)
  draft.edges = draft.edges.filter((edge) => edge.from_node_id !== nodeId && edge.to_node_id !== nodeId)
}

function addEdge(draft: Flow, from: number, to: number) {
  if (draft.edges.some((edge) => edge.from_node_id === from && edge.to_node_id === to)) return
  draft.edges.push({ id: 0, from_node_id: from, to_node_id: to })
}

function patchSources(draft: Flow, wantIds: number[]): string | null {
  const sourceNodes = draft.nodes.filter((node) => node.type === 'source')
  const refs = sourceNodes.map((node) => node.ref_id ?? 0)
  if (new Set(refs).size !== refs.length) {
    return '该 Flow 存在引用同一来源的多个节点'
  }
  const want = new Set(wantIds)
  for (const node of sourceNodes) {
    if (!want.has(node.ref_id ?? 0)) removeNode(draft, node.id)
  }
  const remaining = draft.nodes.filter((node) => node.type === 'source')
  const have = new Set(remaining.map((node) => node.ref_id ?? 0))
  const toAdd = wantIds.filter((id) => !have.has(id))
  if (!toAdd.length) return null

  // 新来源接入现有来源的全部下游入口，与「这个 Flow 再监听一个来源」的直觉一致。
  const entryIds = new Set<number>()
  for (const node of remaining) {
    for (const edge of draft.edges) {
      if (edge.from_node_id === node.id) entryIds.add(edge.to_node_id)
    }
  }
  if (!entryIds.size) return '无法确定新来源的接入点'

  const baseX = remaining.length ? Math.min(...remaining.map((node) => node.pos_x)) : 0
  let nextY = remaining.length ? Math.max(...remaining.map((node) => node.pos_y)) + 120 : 0
  for (const sourceId of toAdd) {
    const node: FlowNode = {
      id: nextTempNodeId(draft),
      type: 'source',
      ref_id: sourceId,
      config: {},
      pos_x: baseX,
      pos_y: nextY,
    }
    nextY += 120
    draft.nodes.push(node)
    for (const to of entryIds) addEdge(draft, node.id, to)
  }
  return null
}

function patchFilters(draft: Flow, wantIds: number[]): string | null {
  const filterNodes = draft.nodes.filter((node) => node.type === 'filter')
  if (filterNodes.length > 1) {
    return '该 Flow 有多个过滤节点'
  }
  if (filterNodes.length === 1) {
    const node = filterNodes[0]
    if (wantIds.length) {
      // 引用即覆盖：接入共享过滤器后内联条件不再生效（调用方已弹确认框）。
      node.config = { filter_ids: [...wantIds] }
      return null
    }
    if (node.config.conditions?.length) {
      node.config = { conditions: node.config.conditions }
      return null
    }
    // 过滤节点被清空后从链路上摘除，上下游直连。
    const froms = draft.edges.filter((edge) => edge.to_node_id === node.id).map((edge) => edge.from_node_id)
    const tos = draft.edges.filter((edge) => edge.from_node_id === node.id).map((edge) => edge.to_node_id)
    removeNode(draft, node.id)
    for (const from of froms) {
      for (const to of tos) addEdge(draft, from, to)
    }
    return null
  }
  if (!wantIds.length) return null

  // 没有过滤节点时新建一个，插到全部来源与其下游之间。
  const sourceNodes = draft.nodes.filter((node) => node.type === 'source')
  if (!sourceNodes.length) return '该 Flow 没有来源节点'
  const sourceIds = new Set(sourceNodes.map((node) => node.id))
  // 插入会把所有来源出边改经过滤节点；各来源下游不一致时合并会串路，拒绝处理。
  const outSets = sourceNodes.map((node) =>
    draft.edges
      .filter((edge) => edge.from_node_id === node.id)
      .map((edge) => edge.to_node_id)
      .sort((a, b) => a - b)
      .join(','),
  )
  if (new Set(outSets).size > 1) return '该 Flow 的多个来源接往不同分支'
  const entryIds = new Set(draft.edges.filter((edge) => sourceIds.has(edge.from_node_id)).map((edge) => edge.to_node_id))
  if (!entryIds.size) return '无法确定过滤节点的插入位置'
  const node: FlowNode = {
    id: nextTempNodeId(draft),
    type: 'filter',
    config: { filter_ids: [...wantIds] },
    pos_x: Math.min(...[...entryIds].map((id) => draft.nodes.find((n) => n.id === id)?.pos_x ?? 280)) - 40,
    pos_y: Math.min(...sourceNodes.map((n) => n.pos_y)),
  }
  draft.nodes.push(node)
  draft.edges = draft.edges.filter((edge) => !sourceIds.has(edge.from_node_id))
  for (const from of sourceIds) addEdge(draft, from, node.id)
  for (const to of entryIds) addEdge(draft, node.id, to)
  return null
}

function patchTargets(draft: Flow, wantTargets: RuleTarget[]): string | null {
  const targetNodes = draft.nodes.filter((node) => node.type === 'target')
  const refs = targetNodes.map((node) => node.ref_id ?? 0)
  if (new Set(refs).size !== refs.length) {
    return '该 Flow 存在投递到同一渠道的多个目标节点'
  }
  const wantBySink = new Map(wantTargets.map((target) => [target.sink_id, target]))
  for (const node of targetNodes) {
    const want = wantBySink.get(node.ref_id ?? 0)
    if (!want) {
      removeNode(draft, node.id)
    } else {
      node.template_id = want.template_id
    }
  }
  const remaining = draft.nodes.filter((node) => node.type === 'target')
  const have = new Set(remaining.map((node) => node.ref_id ?? 0))
  const toAdd = wantTargets.filter((target) => !have.has(target.sink_id))
  if (!toAdd.length) return null

  // 新目标接到现有目标的全部上游出口。
  const remainingIds = new Set(remaining.map((node) => node.id))
  const tailIds = new Set(draft.edges.filter((edge) => remainingIds.has(edge.to_node_id)).map((edge) => edge.from_node_id))
  if (!tailIds.size) return '无法确定新目标的接入点'

  const baseX = remaining.length ? Math.max(...remaining.map((node) => node.pos_x)) : 840
  let nextY = remaining.length ? Math.max(...remaining.map((node) => node.pos_y)) + 120 : 0
  for (const target of toAdd) {
    const node: FlowNode = {
      id: nextTempNodeId(draft),
      type: 'target',
      ref_id: target.sink_id,
      template_id: target.template_id,
      config: {},
      pos_x: baseX,
      pos_y: nextY,
    }
    nextY += 120
    draft.nodes.push(node)
    for (const from of tailIds) addEdge(draft, from, node.id)
  }
  return null
}
