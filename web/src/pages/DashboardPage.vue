<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { useRouter } from 'vue-router'
import { useMessage } from 'naive-ui'
import { dashboardApi } from '@/api/client'
import { useAuthStore } from '@/stores/auth'
import type { DashboardFailureBucket, DashboardSetupStatus, DashboardSummary } from '@/types'
import { errText } from '@/utils/error'
import ClayIcon from '@/components/ClayIcon.vue'

const message = useMessage()
const auth = useAuthStore()
const router = useRouter()

const summary = ref<DashboardSummary | null>(null)
const loading = ref(false)
const windowHours = 24
const checklistCollapsed = ref(false)
const aiStats = computed(() => ({
  total: summary.value?.ai?.runs ?? 0,
  profiles: summary.value?.ai?.profiles ?? 0,
  success: summary.value?.ai?.success ?? 0,
  failed: summary.value?.ai?.failed ?? 0,
  tokens: summary.value?.ai?.tokens ?? 0,
}))

const counts = computed(() => summary.value?.resources ?? { accounts: 0, sources: 0, sinks: 0, flows: 0 })
const statusCount = computed(() => summary.value?.status ?? {})
const windowTotal = computed(() => summary.value?.window_total ?? 0)
const setup = computed<DashboardSetupStatus | null>(() => summary.value?.setup ?? null)

const successRate = computed(() => {
  const total = windowTotal.value
  if (total === 0) return '—'
  const ok = statusCount.value['success'] ?? 0
  const rate = (ok / total) * 100
  return ok === total ? '100%' : `${rate.toFixed(1)}%`
})

const tiles = computed(() => [
  { key: 'accounts', label: '账号', value: counts.value.accounts, icon: 'accounts', tone: 'blue' },
  { key: 'sources', label: '监听源', value: counts.value.sources, icon: 'sources', tone: 'mint' },
  { key: 'sinks', label: '目标渠道', value: counts.value.sinks, icon: 'sinks', tone: 'peach' },
  { key: 'flows', label: 'Flow', value: counts.value.flows, icon: 'flow', tone: 'coral' },
])

const statusItems = computed(() => [
  { label: '成功', value: statusCount.value['success'] ?? 0, tone: 'mint' },
  { label: '重试中', value: statusCount.value['retrying'] ?? 0, tone: 'peach' },
  { label: '失败 (dead)', value: statusCount.value['dead'] ?? 0, tone: 'coral' },
  { label: '待处理', value: statusCount.value['pending'] ?? 0, tone: 'blue' },
])

const queueItems = computed(() => [
  { label: '待领取', value: summary.value?.queue.pending ?? 0, tone: 'blue' },
  { label: '处理中', value: summary.value?.queue.processing ?? 0, tone: 'peach' },
  { label: '等待重试', value: summary.value?.queue.retrying ?? 0, tone: 'coral' },
])

const topFailures = computed(() => ({
  sink: mapTop(summary.value?.top_failures.sink),
  flow: mapTop(summary.value?.top_failures.flow),
  source: mapTop(summary.value?.top_failures.source),
}))

function mapTop(items?: DashboardFailureBucket[]) {
  return (items ?? []).map((item) => ({ label: item.label, value: item.count }))
}

type ChecklistItem = {
  key: string
  done: boolean
  label: string
  route: string
  optional?: boolean
}

const checklistItems = computed<ChecklistItem[]>(() => {
  const s = setup.value
  if (!s) return []
  const items: ChecklistItem[] = [
    { key: 'app', done: s.has_telegram_app, label: '已配置 Telegram App ID/Hash', route: '/telegram-config' },
    { key: 'account', done: s.has_active_account, label: '至少一个 active Telegram 账号', route: '/accounts' },
    { key: 'source', done: s.has_enabled_source, label: '至少一个启用中的监听源', route: '/sources' },
    { key: 'sink', done: s.has_enabled_sink, label: '至少一个启用中的目标渠道', route: '/sinks' },
    { key: 'flow', done: s.has_enabled_flow, label: '至少一条启用中的 Flow', route: '/flow' },
  ]
  if (s.media_url_recommended) {
    items.push({
      key: 'media',
      done: s.has_media_public_url,
      label: '媒体公网地址/S3（钉钉/Bark 等渠道需要）',
      route: '/settings',
      optional: true,
    })
  }
  return items
})

const checklistDoneCount = computed(() => checklistItems.value.filter((i) => i.done).length)
const checklistAllDone = computed(
  () => checklistItems.value.length > 0 && checklistItems.value.every((i) => i.done || i.optional),
)
const showChecklist = computed(() => {
  if (!checklistItems.value.length) return false
  if (checklistAllDone.value && checklistCollapsed.value) return false
  return true
})

async function load() {
  if (!auth.hasToken) {
    message.warning('请先在「设置」中配置 API Token')
    return
  }
  loading.value = true
  try {
    const sum = await dashboardApi.summary(windowHours)
    summary.value = sum
    if (sum.setup && checklistAllDone.value) {
      // 全部完成后默认折叠，用户仍可展开。
      checklistCollapsed.value = true
    }
  } catch (e) {
    message.error('加载失败：' + errText(e))
  } finally {
    loading.value = false
  }
}

onMounted(load)
</script>

<template>
  <n-spin :show="loading">
    <n-card v-if="showChecklist" class="panel checklist-card" title="首次配置检查清单">
      <template #header-extra>
        <n-space size="small">
          <n-tag size="small" :bordered="false" type="info">
            {{ checklistDoneCount }}/{{ checklistItems.length }}
          </n-tag>
          <n-button
            v-if="checklistAllDone"
            size="tiny"
            quaternary
            @click="checklistCollapsed = true"
          >
            收起
          </n-button>
        </n-space>
      </template>
      <n-text depth="3" class="checklist-hint">
        非阻断引导：完成下列项即可跑通 Source → Flow → 渠道 的实时转发。可随时跳过。
      </n-text>
      <div class="checklist">
        <div
          v-for="item in checklistItems"
          :key="item.key"
          class="checklist-item"
          :class="{ done: item.done }"
          @click="router.push(item.route)"
        >
          <span class="check-mark">{{ item.done ? '✓' : '○' }}</span>
          <span class="check-label">
            {{ item.label }}
            <n-tag v-if="item.optional && !item.done" size="tiny" :bordered="false">建议</n-tag>
          </span>
          <span class="check-link">去配置 →</span>
        </div>
      </div>
    </n-card>
    <n-alert
      v-else-if="checklistAllDone && checklistCollapsed"
      type="success"
      class="panel checklist-done"
      :bordered="false"
      title="首次配置已完成"
    >
      <n-button size="tiny" text type="primary" @click="checklistCollapsed = false">展开检查清单</n-button>
    </n-alert>

    <!-- 主统计瓷砖 -->
    <div class="tiles">
      <div v-for="t in tiles" :key="t.key" class="tile" :class="`tone-${t.tone}`">
        <div class="tile-icon">
          <ClayIcon :name="t.icon" :size="24" />
        </div>
        <div class="tile-body">
          <div class="tile-value">{{ t.value }}</div>
          <div class="tile-label">{{ t.label }}</div>
        </div>
      </div>
    </div>

    <!-- 投递概览 -->
    <n-card class="panel" :title="`投递状态（近 ${windowHours} 小时）`">
      <template #header-extra>
        <n-button size="small" type="primary" @click="load">
          <template #icon><ClayIcon name="bolt" :size="15" /></template>
          刷新
        </n-button>
      </template>

      <div class="overview">
        <div class="rate-badge">
          <div class="rate-value">{{ successRate }}</div>
          <div class="rate-label">成功率</div>
        </div>

        <div class="status-grid">
          <div class="status-total">
            <div class="status-value">{{ windowTotal }}</div>
            <div class="status-label">总投递</div>
          </div>
          <div
            v-for="s in statusItems"
            :key="s.label"
            class="status-pill"
            :class="`tone-${s.tone}`"
          >
            <span class="status-dot" />
            <span class="status-body">
              <span class="status-num">{{ s.value }}</span>
              <span class="status-name">{{ s.label }}</span>
            </span>
          </div>
        </div>
      </div>
    </n-card>

    <div class="ops-grid">
      <n-card class="panel" title="队列状态">
        <div class="status-grid compact">
          <div v-for="s in queueItems" :key="s.label" class="status-pill" :class="`tone-${s.tone}`">
            <span class="status-dot" />
            <span class="status-body">
              <span class="status-num">{{ s.value }}</span>
              <span class="status-name">{{ s.label }}</span>
            </span>
          </div>
        </div>
      </n-card>
      <n-card class="panel" title="失败 Top">
        <div class="top-grid">
          <div v-for="(items, key) in topFailures" :key="key" class="top-list">
            <div class="top-title">{{ key === 'sink' ? 'Sink' : key === 'flow' ? 'Flow' : 'Source' }}</div>
            <n-empty v-if="!items.length" size="small" description="暂无失败" />
            <div v-for="item in items" :key="item.label" class="top-item">
              <span>{{ item.label }}</span>
              <strong>{{ item.value }}</strong>
            </div>
          </div>
        </div>
      </n-card>
    </div>

    <n-card class="panel" :title="`AI 整理概况（近 ${windowHours} 小时）`">
      <div class="status-grid compact-ai">
        <div class="status-pill tone-blue">
          <span class="status-dot" />
          <span class="status-body">
            <span class="status-num">{{ aiStats.profiles }}</span>
            <span class="status-name">Profile 数</span>
          </span>
        </div>
        <div class="status-pill tone-mint">
          <span class="status-dot" />
          <span class="status-body">
            <span class="status-num">{{ aiStats.success }}</span>
            <span class="status-name">成功运行</span>
          </span>
        </div>
        <div class="status-pill tone-coral">
          <span class="status-dot" />
          <span class="status-body">
            <span class="status-num">{{ aiStats.failed }}</span>
            <span class="status-name">失败</span>
          </span>
        </div>
        <div class="status-pill tone-peach">
          <span class="status-dot" />
          <span class="status-body">
            <span class="status-num">{{ aiStats.tokens }}</span>
            <span class="status-name">Token 合计</span>
          </span>
        </div>
      </div>
      <n-text depth="3" style="display: block; margin-top: 10px">
        近窗共 {{ aiStats.total }} 次运行（不含预览）。调度下次执行时间见「AI 整理」任务列表。
      </n-text>
    </n-card>
  </n-spin>
</template>

<style scoped>
/* 点缀色变量，亮暗都适用 */
.tone-blue {
  --tone: #3aa0e3;
  --tone-soft: rgba(58, 160, 227, 0.16);
}
.tone-mint {
  --tone: #2fb896;
  --tone-soft: rgba(63, 197, 160, 0.18);
}
.tone-peach {
  --tone: #ef9a4c;
  --tone-soft: rgba(245, 166, 91, 0.18);
}
.tone-coral {
  --tone: #ee7167;
  --tone-soft: rgba(240, 133, 125, 0.18);
}

/* ---------- 主瓷砖 ---------- */
.tiles {
  display: grid;
  grid-template-columns: repeat(4, 1fr);
  gap: 20px;
}
@media (max-width: 900px) {
  .tiles {
    grid-template-columns: repeat(2, 1fr);
  }
}
.tile {
  display: flex;
  align-items: center;
  gap: 16px;
  padding: 20px;
  border: 0;
  border: 1px solid var(--clay-border);
  border-radius: 8px;
  background: var(--clay-surface);
  box-shadow: var(--clay-extruded-sm);
  transition: box-shadow 0.22s ease-out, transform 0.22s ease-out;
}
.tile:hover {
  border-color: var(--clay-border-strong);
  box-shadow: var(--clay-out-sm);
}
.tile-icon {
  width: 48px;
  height: 48px;
  border-radius: 14px;
  display: flex;
  align-items: center;
  justify-content: center;
  color: var(--tone);
  background: var(--tone-soft);
  flex-shrink: 0;
}
.tile-body {
  min-width: 0;
}
.tile-value {
  font-size: 28px;
  font-weight: 700;
  line-height: 1.1;
  color: var(--clay-text);
}
.tile-label {
  margin-top: 4px;
  font-size: 13px;
  color: var(--clay-text-muted);
}

.panel {
  margin-top: 20px;
  border-radius: 8px;
}

.checklist-card {
  margin-top: 0;
  margin-bottom: 4px;
}
.checklist-hint {
  display: block;
  margin-bottom: 12px;
}
.checklist {
  display: flex;
  flex-direction: column;
  gap: 8px;
}
.checklist-item {
  display: flex;
  align-items: center;
  gap: 10px;
  padding: 10px 12px;
  border-radius: 8px;
  border: 1px solid var(--clay-border);
  background: var(--clay-surface);
  cursor: pointer;
  transition: border-color 0.15s ease, transform 0.15s ease;
}
.checklist-item:hover {
  border-color: var(--clay-primary);
  transform: translateY(-1px);
}
.checklist-item.done {
  opacity: 0.72;
}
.check-mark {
  width: 22px;
  text-align: center;
  font-weight: 700;
  color: var(--clay-primary);
}
.check-label {
  flex: 1;
  display: flex;
  align-items: center;
  gap: 8px;
}
.check-link {
  font-size: 12px;
  color: var(--clay-primary);
}
.checklist-done {
  margin-bottom: 12px;
}

.overview {
  display: flex;
  gap: 24px;
  align-items: stretch;
}
@media (max-width: 720px) {
  .overview {
    flex-direction: column;
  }
}
.rate-badge {
  min-width: 120px;
  padding: 16px;
  border-radius: 8px;
  background: var(--clay-primary-soft);
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
}
.rate-value {
  font-size: 28px;
  font-weight: 700;
  color: var(--clay-primary);
}
.rate-label {
  margin-top: 4px;
  font-size: 12px;
  color: var(--clay-text-muted);
}
.status-grid {
  flex: 1;
  display: grid;
  grid-template-columns: repeat(3, minmax(0, 1fr));
  gap: 12px;
}
.status-grid.compact {
  grid-template-columns: 1fr;
}
.status-grid.compact-ai {
  grid-template-columns: repeat(4, minmax(0, 1fr));
}
@media (max-width: 900px) {
  .status-grid.compact-ai {
    grid-template-columns: repeat(2, minmax(0, 1fr));
  }
}
.status-total {
  grid-column: 1 / -1;
  padding: 12px 14px;
  border-radius: 8px;
  background: var(--clay-inset-bg, rgba(15, 23, 42, 0.03));
}
.status-value {
  font-size: 22px;
  font-weight: 700;
}
.status-label {
  font-size: 12px;
  color: var(--clay-text-muted);
}
.status-pill {
  display: flex;
  align-items: center;
  gap: 10px;
  padding: 10px 12px;
  border-radius: 8px;
  border: 1px solid var(--clay-border);
  background: var(--clay-surface);
}
.status-dot {
  width: 8px;
  height: 8px;
  border-radius: 50%;
  background: var(--tone);
  flex-shrink: 0;
}
.status-body {
  display: flex;
  flex-direction: column;
  min-width: 0;
}
.status-num {
  font-weight: 700;
  font-size: 16px;
}
.status-name {
  font-size: 12px;
  color: var(--clay-text-muted);
}
.ops-grid {
  display: grid;
  grid-template-columns: 1fr 2fr;
  gap: 20px;
}
@media (max-width: 900px) {
  .ops-grid {
    grid-template-columns: 1fr;
  }
}
.top-grid {
  display: grid;
  grid-template-columns: repeat(3, minmax(0, 1fr));
  gap: 16px;
}
@media (max-width: 900px) {
  .top-grid {
    grid-template-columns: 1fr;
  }
}
.top-title {
  font-weight: 600;
  margin-bottom: 8px;
}
.top-item {
  display: flex;
  justify-content: space-between;
  gap: 12px;
  padding: 6px 0;
  border-bottom: 1px dashed var(--clay-border);
  font-size: 13px;
}
.top-item strong {
  color: var(--clay-primary);
}
</style>
