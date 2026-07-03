<script setup lang="ts">
import { computed, onMounted, shallowRef } from 'vue'
import { useRouter } from 'vue-router'
import { useMessage } from 'naive-ui'
import ClayIcon from '@/components/ClayIcon.vue'
import FlowDetailDrawer from '@/components/flow/FlowDetailDrawer.vue'
import FlowMap from '@/components/flow/FlowMap.vue'
import { useForwardingGraph, type FlowSourceNode } from '@/composables/useForwardingGraph'
import { errText } from '@/utils/error'

const router = useRouter()
const message = useMessage()

const { accounts, loading, orphanRules, sourceNodes, stats, load } = useForwardingGraph()

const selectedNode = shallowRef<FlowSourceNode | null>(null)
const detailOpen = shallowRef(false)
const keyword = shallowRef('')
const onlyWarnings = shallowRef(false)

const visibleNodes = computed(() => {
  const q = keyword.value.trim().toLowerCase()
  return sourceNodes.value.filter((node) => {
    if (onlyWarnings.value && node.warnings.length === 0) return false
    if (!q) return true
    return [
      node.source.name,
      node.source.username,
      node.account?.name,
      ...node.rules.map((item) => item.rule.name),
      ...node.rules.flatMap((item) => item.targets.map((target) => target.sink?.name ?? '')),
      ...node.rules.flatMap((item) => item.targets.map((target) => target.template?.name ?? '')),
    ]
      .filter(Boolean)
      .some((item) => String(item).toLowerCase().includes(q))
  })
})

const checklist = computed(() => [
  { label: '配置 Telegram App', done: accounts.value.length > 0 },
  { label: '同步监听源', done: stats.value.totalSources > 0 },
  { label: '创建目标渠道', done: stats.value.totalSinks > 0 },
  { label: '创建模板', done: stats.value.totalTemplates > 0 },
  { label: '关联规则', done: stats.value.enabledRules > 0 && stats.value.linkedSources > 0 },
])

async function refresh() {
  try {
    await load()
  } catch (e) {
    message.error('加载编排关系失败：' + errText(e))
  }
}

function selectNode(node: FlowSourceNode) {
  selectedNode.value = node
  detailOpen.value = true
}

function go(name: string, query?: Record<string, string>) {
  router.push({ name, query })
}

function editSource(id: number) {
  detailOpen.value = false
  go('sources', { source_id: String(id) })
}

function editRule(id: number) {
  detailOpen.value = false
  go('rules', { rule_id: String(id) })
}

function editSink(id: number) {
  detailOpen.value = false
  go('rules', { sink_id: String(id) })
}

onMounted(refresh)
</script>

<template>
  <NSpace vertical size="large">
    <section class="hero-panel">
      <div class="hero-main">
        <div class="hero-icon"><ClayIcon name="flow" :size="24" /></div>
        <div class="hero-copy">
          <h1>转发编排</h1>
          <p>从监听账号和来源出发，检查消息会命中哪些规则、套用哪个模板、最终投递到哪个渠道。</p>
        </div>
      </div>
      <div class="hero-actions">
        <NButton type="primary" @click="go('rules')">
          <template #icon><ClayIcon name="plus" :size="16" /></template>
          新建规则
        </NButton>
        <NButton secondary :loading="loading" @click="refresh">
          <template #icon><ClayIcon name="refresh" :size="16" /></template>
          刷新
        </NButton>
      </div>
    </section>

    <section class="setup-panel">
      <div class="setup-title">配置进度</div>
      <div class="setup-steps">
        <div
          v-for="item in checklist"
          :key="item.label"
          class="setup-step"
          :class="{ done: item.done }"
        >
          <span class="step-dot">{{ item.done ? '✓' : '!' }}</span>
          <span>{{ item.label }}</span>
        </div>
      </div>
    </section>

    <div class="stat-grid">
      <div class="stat-card">
        <span class="stat-value">{{ stats.linkedSources }}/{{ stats.totalSources }}</span>
        <span class="stat-label">已接入规则的来源</span>
      </div>
      <div class="stat-card">
        <span class="stat-value">{{ stats.enabledRules }}/{{ stats.totalRules }}</span>
        <span class="stat-label">启用规则</span>
      </div>
      <div class="stat-card">
        <span class="stat-value">{{ stats.enabledSinks }}/{{ stats.totalSinks }}</span>
        <span class="stat-label">启用渠道</span>
      </div>
      <div class="stat-card" :class="{ warn: stats.warningSources > 0 }">
        <span class="stat-value">{{ stats.warningSources }}</span>
        <span class="stat-label">需要处理的来源</span>
      </div>
    </div>

    <NAlert v-if="orphanRules.length" type="warning" title="存在没有来源的规则">
      {{ orphanRules.map((rule) => rule.name).join('、') }}
    </NAlert>

    <section class="map-panel">
      <div class="map-toolbar">
        <div>
          <h2>转发关系</h2>
          <p>点击任意一行查看详细链路和编辑入口。</p>
        </div>
        <div class="map-filters">
          <NInput v-model:value="keyword" clearable class="search-input" placeholder="搜索来源、规则、模板、渠道" />
          <NCheckbox v-model:checked="onlyWarnings">只看异常</NCheckbox>
        </div>
      </div>

      <NSpin :show="loading">
        <FlowMap
          v-if="visibleNodes.length"
          :nodes="visibleNodes"
          :selected-id="selectedNode?.source.id"
          @select="selectNode"
        />
        <NEmpty v-else description="还没有可展示的转发关系">
          <template #extra>
            <NSpace>
              <NButton @click="go('accounts')">去配置账号</NButton>
              <NButton type="primary" @click="go('sources')">去同步来源</NButton>
            </NSpace>
          </template>
        </NEmpty>
      </NSpin>
    </section>

    <FlowDetailDrawer
      v-model:show="detailOpen"
      :node="selectedNode"
      @edit-source="editSource"
      @edit-rule="editRule"
      @edit-sink="editSink"
    />
  </NSpace>
</template>

<style scoped>
.hero-panel,
.setup-panel,
.map-panel,
.stat-card {
  border: 1px solid var(--clay-border);
  border-radius: 10px;
  background: var(--clay-surface);
}

.hero-panel {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 18px;
  padding: 20px;
}

.hero-main {
  display: flex;
  align-items: center;
  gap: 14px;
  min-width: 0;
}

.hero-icon {
  width: 46px;
  height: 46px;
  display: grid;
  place-items: center;
  flex-shrink: 0;
  border-radius: 10px;
  color: var(--clay-primary);
  background: var(--clay-primary-soft);
}

.hero-copy {
  min-width: 0;
}

.hero-copy h1,
.map-toolbar h2 {
  margin: 0;
  color: var(--clay-text);
  font-size: 20px;
  font-weight: 800;
  text-wrap: balance;
}

.hero-copy p,
.map-toolbar p {
  margin: 5px 0 0;
  color: var(--clay-text-2);
  font-size: 13px;
}

.hero-actions,
.map-filters {
  display: flex;
  align-items: center;
  gap: 10px;
  flex-wrap: wrap;
}

.setup-panel {
  padding: 14px 16px;
}

.setup-title {
  margin-bottom: 10px;
  color: var(--clay-text);
  font-size: 14px;
  font-weight: 800;
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
  border: 1px solid var(--clay-border);
  border-radius: 8px;
  color: var(--clay-text-2);
  background: var(--clay-surface-2);
  font-weight: 600;
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
  font-size: 12px;
  font-weight: 900;
}

.stat-grid {
  display: grid;
  grid-template-columns: repeat(4, 1fr);
  gap: 12px;
}

.stat-card {
  display: flex;
  flex-direction: column;
  gap: 4px;
  padding: 16px;
}

.stat-card.warn {
  background: var(--clay-warning-soft);
}

.stat-value {
  color: var(--clay-text);
  font-size: 26px;
  font-weight: 900;
  line-height: 1;
  font-variant-numeric: tabular-nums;
}

.stat-label {
  color: var(--clay-text-2);
  font-size: 13px;
  font-weight: 600;
}

.map-panel {
  padding: 16px;
}

.map-toolbar {
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  gap: 16px;
  margin-bottom: 14px;
}

.search-input {
  width: 280px;
}

@media (max-width: 980px) {
  .hero-panel,
  .map-toolbar {
    align-items: stretch;
    flex-direction: column;
  }

  .stat-grid {
    grid-template-columns: repeat(2, 1fr);
  }

  .setup-steps {
    grid-template-columns: repeat(2, minmax(120px, 1fr));
  }
}

@media (max-width: 640px) {
  .stat-grid,
  .setup-steps {
    grid-template-columns: 1fr;
  }

  .hero-actions,
  .map-filters,
  .search-input {
    width: 100%;
  }
}
</style>
