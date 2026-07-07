// 编排画布的节点/连线输入类型，由 FlowPage 组装、FlowCanvas 渲染。

export type FlowNodeKind = 'source' | 'filter' | 'rule' | 'sink'

export interface SourceNodeData {
  name: string
  typeLabel: string
  accountLabel: string
  runtimeLabel: string
  enabled: boolean
  running: boolean
  ruleCount: number
}

export interface RuleNodeData {
  name: string
  enabled: boolean
  priority: number
  orderLabel: string
  stopOnMatch: boolean
  conditionChips: string[]
  processorChips: string[]
  warnings: string[]
}

export interface FilterNodeData {
  name: string
  conditionCount: number
  ruleCount: number
}

export interface SinkNodeData {
  name: string
  typeLabel: string
  enabled: boolean
  deliveryLabel: string
  ruleCount: number
}

export interface CanvasNodeInput<T> {
  id: number
  stateClass?: string
  data: T
}

export interface CanvasEdgeInput {
  key: string
  from: { kind: 'source' | 'filter' | 'rule'; id: number }
  to: { kind: 'rule' | 'sink'; id: number }
  label?: string
  warn?: boolean
  hot?: boolean
  stateClass?: string
}

export interface CanvasConnection {
  from: { kind: FlowNodeKind; id: number }
  to: { kind: FlowNodeKind; id: number }
}

export function flowNodeId(kind: FlowNodeKind, id: number): string {
  return `${kind}-${id}`
}

export function parseFlowNodeId(nodeId: string): { kind: FlowNodeKind; id: number } | null {
  const match = /^(source|filter|rule|sink)-(\d+)$/.exec(nodeId)
  if (!match) return null
  return { kind: match[1] as FlowNodeKind, id: Number(match[2]) }
}
