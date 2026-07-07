<script setup lang="ts">
import { computed } from 'vue'
import { MarkerType, VueFlow, useVueFlow, type Connection, type Edge, type EdgeMouseEvent, type Node, type NodeMouseEvent } from '@vue-flow/core'
import { NButton, NTooltip } from 'naive-ui'
import ClayIcon from '@/components/ClayIcon.vue'
import FilterFlowNode from './FilterFlowNode.vue'
import SourceFlowNode from './SourceFlowNode.vue'
import RuleFlowNode from './RuleFlowNode.vue'
import SinkFlowNode from './SinkFlowNode.vue'
import {
  flowNodeId,
  parseFlowNodeId,
  type CanvasConnection,
  type CanvasEdgeInput,
  type CanvasNodeInput,
  type FilterNodeData,
  type FlowNodeKind,
  type RuleNodeData,
  type SinkNodeData,
  type SourceNodeData,
} from './types'

import '@vue-flow/core/dist/style.css'
import '@vue-flow/core/dist/theme-default.css'

const props = defineProps<{
  sources: CanvasNodeInput<SourceNodeData>[]
  filters: CanvasNodeInput<FilterNodeData>[]
  rules: CanvasNodeInput<RuleNodeData>[]
  sinks: CanvasNodeInput<SinkNodeData>[]
  edges: CanvasEdgeInput[]
}>()

const emit = defineEmits<{
  'select-node': [kind: FlowNodeKind, id: number]
  'select-edge': [key: string]
  'clear-select': []
  connect: [connection: CanvasConnection]
  'edit-rule': [id: number]
  'toggle-rule': [id: number, value: boolean]
}>()

// 分层布局常量：没有资源层时保持三栏，有过滤器时插入资源列。
const COL_X = { source: 0, filter: 300, rule: 400, ruleWithFilter: 620, sink: 880, sinkWithFilter: 1080 }
const NODE_H = { source: 96, filter: 82, rule: 104, sink: 96 }
const GAP = 18

function columnHeight(count: number, nodeH: number): number {
  if (count <= 0) return 0
  return count * nodeH + (count - 1) * GAP
}

const nodes = computed<Node[]>(() => {
  const heights = {
    source: columnHeight(props.sources.length, NODE_H.source),
    filter: columnHeight(props.filters.length, NODE_H.filter),
    rule: columnHeight(props.rules.length, NODE_H.rule),
    sink: columnHeight(props.sinks.length, NODE_H.sink),
  }
  const maxHeight = Math.max(heights.source, heights.filter, heights.rule, heights.sink)
  const offset = {
    source: (maxHeight - heights.source) / 2,
    filter: (maxHeight - heights.filter) / 2,
    rule: (maxHeight - heights.rule) / 2,
    sink: (maxHeight - heights.sink) / 2,
  }
  const hasFilterLayer = props.filters.length > 0
  const out: Node[] = []
  props.sources.forEach((item, index) => {
    out.push({
      id: flowNodeId('source', item.id),
      type: 'source',
      position: { x: COL_X.source, y: offset.source + index * (NODE_H.source + GAP) },
      data: item.data,
      class: item.stateClass ?? '',
      draggable: false,
    })
  })
  props.filters.forEach((item, index) => {
    out.push({
      id: flowNodeId('filter', item.id),
      type: 'filter',
      position: { x: COL_X.filter, y: offset.filter + index * (NODE_H.filter + GAP) },
      data: item.data,
      class: item.stateClass ?? '',
      draggable: false,
    })
  })
  props.rules.forEach((item, index) => {
    out.push({
      id: flowNodeId('rule', item.id),
      type: 'rule',
      position: { x: hasFilterLayer ? COL_X.ruleWithFilter : COL_X.rule, y: offset.rule + index * (NODE_H.rule + GAP) },
      data: item.data,
      class: item.stateClass ?? '',
      draggable: false,
    })
  })
  props.sinks.forEach((item, index) => {
    out.push({
      id: flowNodeId('sink', item.id),
      type: 'sink',
      position: { x: hasFilterLayer ? COL_X.sinkWithFilter : COL_X.sink, y: offset.sink + index * (NODE_H.sink + GAP) },
      data: item.data,
      class: item.stateClass ?? '',
      draggable: false,
    })
  })
  return out
})

const edges = computed<Edge[]>(() =>
  props.edges.map((item) => ({
    id: item.key,
    source: flowNodeId(item.from.kind, item.from.id),
    target: flowNodeId(item.to.kind, item.to.id),
    label: item.label,
    animated: item.hot ?? false,
    class: [item.warn ? 'edge-warn' : '', item.stateClass ?? ''].filter(Boolean).join(' '),
    markerEnd: MarkerType.ArrowClosed,
  })),
)

const { fitView, zoomIn, zoomOut, onNodesInitialized } = useVueFlow()

let fitted = false
onNodesInitialized(() => {
  if (fitted) return
  fitted = true
  void fitView({ padding: 0.15, maxZoom: 1 })
})

function refit() {
  void fitView({ padding: 0.15, maxZoom: 1.2 })
}

function onConnect(connection: Connection) {
  const from = parseFlowNodeId(connection.source)
  const to = parseFlowNodeId(connection.target)
  if (!from || !to) return
  emit('connect', { from, to })
}

function onNodeClick({ node }: NodeMouseEvent) {
  const parsed = parseFlowNodeId(node.id)
  if (parsed) emit('select-node', parsed.kind, parsed.id)
}

function onEdgeClick({ edge }: EdgeMouseEvent) {
  emit('select-edge', edge.id)
}

function ruleIdOf(nodeId: string): number {
  return parseFlowNodeId(nodeId)?.id ?? 0
}
</script>

<template>
  <div class="canvas-shell">
    <VueFlow
      :nodes="nodes"
      :edges="edges"
      :nodes-draggable="false"
      :edges-updatable="false"
      :delete-key-code="null"
      :min-zoom="0.25"
      :max-zoom="1.75"
      :connection-radius="36"
      @connect="onConnect"
      @node-click="onNodeClick"
      @edge-click="onEdgeClick"
      @pane-click="emit('clear-select')"
    >
      <template #node-source="nodeProps">
        <SourceFlowNode :data="nodeProps.data as SourceNodeData" />
      </template>
      <template #node-filter="nodeProps">
        <FilterFlowNode :data="nodeProps.data as FilterNodeData" />
      </template>
      <template #node-rule="nodeProps">
        <RuleFlowNode
          :data="nodeProps.data as RuleNodeData"
          @edit="emit('edit-rule', ruleIdOf(nodeProps.id))"
          @toggle="(value: boolean) => emit('toggle-rule', ruleIdOf(nodeProps.id), value)"
        />
      </template>
      <template #node-sink="nodeProps">
        <SinkFlowNode :data="nodeProps.data as SinkNodeData" />
      </template>
    </VueFlow>

    <div class="canvas-tools">
      <NTooltip trigger="hover">
        <template #trigger>
          <NButton size="small" secondary circle @click="refit">
            <template #icon><ClayIcon name="flow" :size="14" /></template>
          </NButton>
        </template>
        适应视图
      </NTooltip>
      <NButton size="small" secondary circle @click="zoomIn()">+</NButton>
      <NButton size="small" secondary circle @click="zoomOut()">−</NButton>
    </div>

    <div class="canvas-legend">
      <span class="legend-item"><span class="legend-swatch source" />来源</span>
      <span v-if="filters.length" class="legend-item"><span class="legend-swatch filter" />过滤器</span>
      <span class="legend-item"><span class="legend-swatch rule" />规则</span>
      <span class="legend-item"><span class="legend-swatch sink" />渠道</span>
      <span class="legend-hint">拖动节点右侧圆点到下一层即可连线；来源直连渠道会创建新规则</span>
    </div>
  </div>
</template>

<style scoped>
.canvas-shell {
  position: relative;
  height: 640px;
  border: 1px solid var(--clay-border);
  border-radius: 14px;
  background: var(--clay-surface);
  box-shadow: var(--clay-out-sm);
  overflow: hidden;
}

.canvas-tools {
  position: absolute;
  top: 10px;
  right: 10px;
  display: flex;
  gap: 6px;
  z-index: 5;
}

.canvas-legend {
  position: absolute;
  left: 10px;
  bottom: 10px;
  display: flex;
  align-items: center;
  gap: 12px;
  flex-wrap: wrap;
  padding: 6px 10px;
  border: 1px solid var(--clay-border);
  border-radius: 999px;
  background: color-mix(in srgb, var(--clay-surface) 88%, transparent);
  font-size: 11px;
  color: var(--clay-text-2);
  z-index: 5;
}

.legend-item {
  display: inline-flex;
  align-items: center;
  gap: 5px;
  font-weight: 600;
}

.legend-swatch {
  width: 10px;
  height: 10px;
  border-radius: 3px;
}

.legend-swatch.source {
  background: #10b981;
}

.legend-swatch.filter {
  background: #3b82f6;
}

.legend-swatch.rule {
  background: var(--clay-primary);
}

.legend-swatch.sink {
  background: #f59e0b;
}

.legend-hint {
  color: var(--clay-text-3);
}
</style>

<style>
/* Vue Flow 全局覆写：节点状态、连线配色与把手样式。 */
.canvas-shell .vue-flow__pane {
  cursor: grab;
}

.canvas-shell .vue-flow__pane.dragging {
  cursor: grabbing;
}

.canvas-shell .vue-flow__node {
  border-radius: 12px;
  transition: opacity 0.16s ease;
}

.canvas-shell .vue-flow__node.is-dimmed {
  opacity: 0.3;
}

.canvas-shell .vue-flow__node.is-selected .flow-node {
  border-color: var(--clay-primary);
  background: color-mix(in srgb, var(--clay-primary-soft) 55%, var(--clay-surface));
  box-shadow: 0 0 0 2px color-mix(in srgb, var(--clay-primary) 20%, transparent);
}

.canvas-shell .vue-flow__node.is-linked .flow-node {
  border-color: color-mix(in srgb, var(--clay-primary) 40%, var(--clay-border));
  background: color-mix(in srgb, var(--clay-primary-soft) 25%, var(--clay-surface));
}

.canvas-shell .vue-flow__handle {
  width: 10px;
  height: 10px;
  border: 2px solid var(--clay-surface);
  background: var(--clay-primary);
}

.canvas-shell .vue-flow__edge-path {
  stroke: var(--clay-border-strong);
  stroke-width: 1.6;
}

.canvas-shell .vue-flow__edge.is-hot .vue-flow__edge-path {
  stroke: var(--clay-primary);
  stroke-width: 2.2;
}

.canvas-shell .vue-flow__edge.is-faded {
  opacity: 0.2;
}

.canvas-shell .vue-flow__edge.edge-warn .vue-flow__edge-path {
  stroke: #dc2626;
  stroke-dasharray: 5 4;
}

.canvas-shell .vue-flow__edge.selected .vue-flow__edge-path {
  stroke: var(--clay-primary);
  stroke-width: 2.6;
}

.canvas-shell .vue-flow__edge-textbg {
  fill: var(--clay-surface);
}

.canvas-shell .vue-flow__edge-text {
  fill: var(--clay-text-2);
  font-size: 10px;
  font-weight: 600;
}

.canvas-shell .vue-flow__connection-path {
  stroke: var(--clay-primary);
  stroke-width: 2;
}
</style>
