<script setup lang="ts">
import type { FlowNodeKind } from './types'
import type { Sink, Source } from '@/types'

defineProps<{
  unusedSources: Source[]
  unusedSinks: Sink[]
  sourceTypeLabel: (source: Source) => string
  sinkTypeLabel: (sink: Sink) => string
}>()

const emit = defineEmits<{
  'reveal-resource': [kind: Exclude<FlowNodeKind, 'rule'>, id: number]
  'show-unused-nodes': []
}>()
</script>

<template>
  <section class="resource-shelf">
    <div class="resource-head">
      <div>
        <strong>未接入资源</strong>
        <span>来源 {{ unusedSources.length }} 个 / 渠道 {{ unusedSinks.length }} 个</span>
      </div>
      <NButton size="small" secondary @click="emit('show-unused-nodes')">全部显示到画布</NButton>
    </div>
    <div class="resource-grid">
      <div v-if="unusedSources.length" class="resource-column">
        <span class="resource-title">来源</span>
        <button
          v-for="source in unusedSources.slice(0, 8)"
          :key="source.id"
          type="button"
          class="resource-pill source"
          @click="emit('reveal-resource', 'source', source.id)"
        >
          <span>{{ source.name }}</span>
          <small>{{ sourceTypeLabel(source) }}</small>
        </button>
        <button v-if="unusedSources.length > 8" type="button" class="resource-pill more" @click="emit('show-unused-nodes')">
          <span>还有 {{ unusedSources.length - 8 }} 个…</span>
        </button>
      </div>
      <div v-if="unusedSinks.length" class="resource-column">
        <span class="resource-title">渠道</span>
        <button
          v-for="sink in unusedSinks.slice(0, 8)"
          :key="sink.id"
          type="button"
          class="resource-pill sink"
          @click="emit('reveal-resource', 'sink', sink.id)"
        >
          <span>{{ sink.name }}</span>
          <small>{{ sinkTypeLabel(sink) }}</small>
        </button>
        <button v-if="unusedSinks.length > 8" type="button" class="resource-pill more" @click="emit('show-unused-nodes')">
          <span>还有 {{ unusedSinks.length - 8 }} 个…</span>
        </button>
      </div>
    </div>
  </section>
</template>

<style scoped>
.resource-shelf {
  display: grid;
  gap: 10px;
  padding: 12px;
  border: 1px solid var(--clay-border);
  border-radius: 14px;
  background: var(--clay-surface);
  box-shadow: var(--clay-out-sm);
}

.resource-head {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
  flex-wrap: wrap;
}

.resource-head strong {
  display: block;
  color: var(--clay-text);
  font-size: 13px;
}

.resource-head span {
  display: block;
  margin-top: 2px;
  color: var(--clay-text-3);
  font-size: 12px;
}

.resource-grid {
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  gap: 12px;
}

.resource-column {
  display: flex;
  align-items: center;
  gap: 7px;
  flex-wrap: wrap;
  min-width: 0;
  padding: 10px;
  border-radius: 10px;
  background: var(--clay-surface-2);
}

.resource-title {
  width: 100%;
  color: var(--clay-text-3);
  font-size: 11px;
  font-weight: 800;
}

.resource-pill {
  display: inline-flex;
  align-items: center;
  gap: 6px;
  max-width: 220px;
  padding: 5px 9px;
  border: 1px solid var(--clay-border);
  border-radius: 999px;
  background: var(--clay-surface);
  color: var(--clay-text-2);
  cursor: pointer;
  transition: border-color 0.16s ease, box-shadow 0.16s ease, transform 0.16s ease;
}

.resource-pill:hover,
.resource-pill:focus-visible {
  border-color: var(--clay-border-strong);
  box-shadow: var(--clay-hover);
  transform: translateY(-1px);
  outline: none;
}

.resource-pill span,
.resource-pill small {
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.resource-pill span {
  min-width: 0;
  font-size: 12px;
  font-weight: 700;
}

.resource-pill small {
  flex-shrink: 0;
  color: var(--clay-text-3);
  font-size: 11px;
}

.resource-pill.source {
  border-left: 3px solid #10b981;
}

.resource-pill.sink {
  border-right: 3px solid #f59e0b;
}

.resource-pill.more {
  border-style: dashed;
  color: var(--clay-text-3);
}

@media (max-width: 680px) {
  .resource-grid {
    grid-template-columns: 1fr;
  }
}
</style>
