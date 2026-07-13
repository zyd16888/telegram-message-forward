import { describe, expect, it } from 'vitest'
import {
  buildFlowFromTemplate,
  buildLinearSkeleton,
  isPlaceholderNode,
} from './flowTemplates'

describe('flowTemplates', () => {
  it('linear skeleton has source → filter → target chain', () => {
    const flow = buildLinearSkeleton()
    expect(flow.nodes.map((n) => n.type)).toEqual(['source', 'filter', 'target'])
    expect(flow.edges).toHaveLength(2)
    expect(flow.edges[0].from_node_id).toBe(flow.nodes[0].id)
    expect(flow.edges[0].to_node_id).toBe(flow.nodes[1].id)
    expect(isPlaceholderNode(flow.nodes[0])).toBe(true)
    expect(isPlaceholderNode(flow.nodes[2])).toBe(true)
  })

  it('keyword template uses keyword_contains', () => {
    const flow = buildFlowFromTemplate('keyword', { sourceId: 3, sinkId: 9 })
    const filter = flow.nodes.find((n) => n.type === 'filter')
    expect(filter?.config.conditions?.[0]?.type).toBe('keyword_contains')
    expect(flow.nodes.find((n) => n.type === 'source')?.ref_id).toBe(3)
    expect(flow.nodes.find((n) => n.type === 'target')?.ref_id).toBe(9)
  })

  it('multi source has two sources into one filter', () => {
    const flow = buildFlowFromTemplate('multi_source')
    const sources = flow.nodes.filter((n) => n.type === 'source')
    expect(sources).toHaveLength(2)
    const filterId = flow.nodes.find((n) => n.type === 'filter')?.id
    const intoFilter = flow.edges.filter((e) => e.to_node_id === filterId)
    expect(intoFilter).toHaveLength(2)
  })

  it('template delivery uses processor node', () => {
    const flow = buildFlowFromTemplate('template_delivery')
    expect(flow.nodes.some((n) => n.type === 'processor')).toBe(true)
  })
})
