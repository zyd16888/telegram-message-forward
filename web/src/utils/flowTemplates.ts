// Flow 新建模板与线性默认骨架（纯前端预置 graph，不改变 AI/Flow 双线）。
import type { Flow, FlowEdge, FlowNode, FlowNodeConfig } from '@/types'

export type FlowTemplateId = 'linear' | 'keyword' | 'template_delivery' | 'multi_source'

export interface FlowTemplateMeta {
  id: FlowTemplateId
  name: string
  description: string
}

export const FLOW_TEMPLATES: FlowTemplateMeta[] = [
  {
    id: 'linear',
    name: '线性骨架',
    description: '来源 → 过滤器 → 目标，适合最简单的实时转发',
  },
  {
    id: 'keyword',
    name: '单源关键词',
    description: '单来源 + 关键词包含过滤 + 单渠道',
  },
  {
    id: 'template_delivery',
    name: '单源模板投递',
    description: '单来源 + 文本处理 + 目标（可再绑模板）',
  },
  {
    id: 'multi_source',
    name: '多源合并',
    description: '两个来源占位合并到同一过滤器与渠道',
  },
]

function node(
  id: number,
  type: FlowNode['type'],
  pos: { x: number; y: number },
  extra: Partial<FlowNode> = {},
): FlowNode {
  return {
    id,
    type,
    config: {} as FlowNodeConfig,
    pos_x: pos.x,
    pos_y: pos.y,
    ...extra,
  }
}

function edge(from: number, to: number): FlowEdge {
  return { id: 0, from_node_id: from, to_node_id: to }
}

/** 默认线性骨架：Source 占位 → Filter → Target 占位。 */
export function buildLinearSkeleton(opts?: {
  name?: string
  sourceId?: number | null
  sinkId?: number | null
}): Flow {
  const sourceId = opts?.sourceId && opts.sourceId > 0 ? opts.sourceId : undefined
  const sinkId = opts?.sinkId && opts.sinkId > 0 ? opts.sinkId : undefined
  const src = node(-1, 'source', { x: 40, y: 120 }, sourceId ? { ref_id: sourceId } : {})
  const filter = node(-2, 'filter', { x: 320, y: 120 }, {
    config: {
      conditions: [{ type: 'keyword_contains', config: { keywords: [], case_sensitive: false } }],
    },
  })
  const target = node(-3, 'target', { x: 600, y: 120 }, sinkId ? { ref_id: sinkId } : {})
  return {
    id: 0,
    name: opts?.name ?? '新 Flow',
    enabled: true,
    priority: 0,
    stop_on_match: false,
    nodes: [src, filter, target],
    edges: [edge(-1, -2), edge(-2, -3)],
    created_at: '',
    updated_at: '',
  }
}

/** 单源关键词过滤 → 单渠道。 */
export function buildKeywordTemplate(opts?: { sourceId?: number | null; sinkId?: number | null }): Flow {
  const flow = buildLinearSkeleton({
    name: '单源关键词转发',
    sourceId: opts?.sourceId,
    sinkId: opts?.sinkId,
  })
  const filter = flow.nodes.find((n) => n.type === 'filter')
  if (filter) {
    filter.config = {
      conditions: [
        {
          type: 'keyword_contains',
          config: { keywords: ['关键词'], case_sensitive: false },
        },
      ],
    }
  }
  return flow
}

/** 单源 → 处理器（追加来源）→ 目标。 */
export function buildTemplateDeliveryTemplate(opts?: {
  sourceId?: number | null
  sinkId?: number | null
}): Flow {
  const sourceId = opts?.sourceId && opts.sourceId > 0 ? opts.sourceId : undefined
  const sinkId = opts?.sinkId && opts.sinkId > 0 ? opts.sinkId : undefined
  const src = node(-1, 'source', { x: 40, y: 120 }, sourceId ? { ref_id: sourceId } : {})
  const proc = node(-2, 'processor', { x: 320, y: 120 }, {
    config: {
      processors: [{ type: 'append_source', config: { label: 'Telegram', prefix: false } }],
    },
  })
  const target = node(-3, 'target', { x: 600, y: 120 }, sinkId ? { ref_id: sinkId } : {})
  return {
    id: 0,
    name: '单源模板投递',
    enabled: true,
    priority: 0,
    stop_on_match: false,
    nodes: [src, proc, target],
    edges: [edge(-1, -2), edge(-2, -3)],
    created_at: '',
    updated_at: '',
  }
}

/** 多源合并到同一过滤与渠道。 */
export function buildMultiSourceTemplate(opts?: { sinkId?: number | null }): Flow {
  const sinkId = opts?.sinkId && opts.sinkId > 0 ? opts.sinkId : undefined
  const src1 = node(-1, 'source', { x: 40, y: 40 })
  const src2 = node(-2, 'source', { x: 40, y: 200 })
  const filter = node(-3, 'filter', { x: 320, y: 120 }, {
    config: {
      conditions: [{ type: 'keyword_contains', config: { keywords: [], case_sensitive: false } }],
    },
  })
  const target = node(-4, 'target', { x: 600, y: 120 }, sinkId ? { ref_id: sinkId } : {})
  return {
    id: 0,
    name: '多源合并转发',
    enabled: true,
    priority: 0,
    stop_on_match: false,
    nodes: [src1, src2, filter, target],
    edges: [edge(-1, -3), edge(-2, -3), edge(-3, -4)],
    created_at: '',
    updated_at: '',
  }
}

export function buildFlowFromTemplate(
  id: FlowTemplateId,
  opts?: { sourceId?: number | null; sinkId?: number | null },
): Flow {
  switch (id) {
    case 'keyword':
      return buildKeywordTemplate(opts)
    case 'template_delivery':
      return buildTemplateDeliveryTemplate(opts)
    case 'multi_source':
      return buildMultiSourceTemplate(opts)
    case 'linear':
    default:
      return buildLinearSkeleton(opts)
  }
}

/** 节点是否仍为未绑定占位（需用户选择真实 source/sink）。 */
export function isPlaceholderNode(node: FlowNode): boolean {
  if (node.type === 'source' || node.type === 'target') {
    return !node.ref_id || node.ref_id <= 0
  }
  return false
}
