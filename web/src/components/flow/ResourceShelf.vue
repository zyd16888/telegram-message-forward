<script setup lang="ts">
import { computed } from 'vue'
import type { FlowNodeKind } from './types'
import type { Sink, Source } from '@/types'

const MAX_PILLS_PER_KIND = 5

const props = defineProps<{
  unusedSources: Source[]
  unusedSinks: Sink[]
  sourceTypeLabel: (source: Source) => string
  sourceAccountLabel: (source: Source) => string
  sinkTypeLabel: (sink: Sink) => string
}>()

const emit = defineEmits<{
  'reveal-resource': [kind: Exclude<FlowNodeKind, 'filter' | 'rule'>, id: number]
  'show-unused-nodes': []
}>()

const shownSources = computed(() => props.unusedSources.slice(0, MAX_PILLS_PER_KIND))
const shownSinks = computed(() => props.unusedSinks.slice(0, MAX_PILLS_PER_KIND))
const overflowCount = computed(
  () => props.unusedSources.length + props.unusedSinks.length - shownSources.value.length - shownSinks.value.length,
)
</script>

<template>
  <section class="resource-shelf">
    <span class="shelf-label">未接入</span>
    <button
      v-for="source in shownSources"
      :key="`source-${source.id}`"
      type="button"
      class="resource-pill source"
      :title="`${sourceTypeLabel(source)} · ${sourceAccountLabel(source)}，点击显示到画布`"
      @click="emit('reveal-resource', 'source', source.id)"
    >
      <span>{{ source.name }}</span>
      <small>{{ sourceTypeLabel(source) }}</small>
    </button>
    <button
      v-for="sink in shownSinks"
      :key="`sink-${sink.id}`"
      type="button"
      class="resource-pill sink"
      :title="`${sinkTypeLabel(sink)}，点击显示到画布`"
      @click="emit('reveal-resource', 'sink', sink.id)"
    >
      <span>{{ sink.name }}</span>
      <small>{{ sinkTypeLabel(sink) }}</small>
    </button>
    <button v-if="overflowCount > 0" type="button" class="resource-pill more" @click="emit('show-unused-nodes')">
      还有 {{ overflowCount }} 个…
    </button>
    <NButton size="tiny" quaternary class="shelf-action" @click="emit('show-unused-nodes')">全部显示到画布</NButton>
  </section>
</template>

<style scoped>
.resource-shelf {
  display: flex;
  align-items: center;
  gap: 7px;
  flex-wrap: wrap;
  padding: 7px 10px;
  border: 1px dashed var(--clay-border);
  border-radius: 12px;
  background: var(--clay-surface-2);
}

.shelf-label {
  flex-shrink: 0;
  color: var(--clay-text-3);
  font-size: 12px;
  font-weight: 800;
}

.shelf-action {
  margin-left: auto;
}

.resource-pill {
  display: inline-flex;
  align-items: center;
  gap: 6px;
  max-width: 200px;
  padding: 3px 9px;
  border: 1px solid var(--clay-border);
  border-radius: 999px;
  background: var(--clay-surface);
  color: var(--clay-text-2);
  font-size: 12px;
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
</style>
