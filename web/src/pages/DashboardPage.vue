<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { useMessage } from 'naive-ui'
import { accountsApi, aiApi, deliveriesApi, rulesApi, sinksApi, sourcesApi } from '@/api/client'
import { useAuthStore } from '@/stores/auth'
import type { Delivery } from '@/types'
import { errText } from '@/utils/error'
import ClayIcon from '@/components/ClayIcon.vue'

const message = useMessage()
const auth = useAuthStore()

const counts = ref({ accounts: 0, sources: 0, sinks: 0, rules: 0 })
const deliveries = ref<Delivery[]>([])
const aiStats = ref({ total: 0, success: 0, failed: 0, tokens: 0 })
const statusTotals = ref<Record<string, number>>({})
const windowTotal = ref(0)
const loading = ref(false)
const windowHours = 24

const statusCount = computed(() => {
  return statusTotals.value
})

const successRate = computed(() => {
  const total = windowTotal.value
  if (total === 0) return '—'
  const ok = statusCount.value['success'] ?? 0
  return `${Math.round((ok / total) * 100)}%`
})

// 顶部四张主统计瓷砖，各配一个点缀色。
const tiles = computed(() => [
  { key: 'accounts', label: '账号', value: counts.value.accounts, icon: 'accounts', tone: 'blue' },
  { key: 'sources', label: '监听源', value: counts.value.sources, icon: 'sources', tone: 'mint' },
  { key: 'sinks', label: '目标渠道', value: counts.value.sinks, icon: 'sinks', tone: 'peach' },
  { key: 'rules', label: '规则', value: counts.value.rules, icon: 'rules', tone: 'coral' },
])

// 投递状态点缀。
const statusItems = computed(() => [
  { label: '成功', value: statusCount.value['success'] ?? 0, tone: 'mint' },
  { label: '重试中', value: statusCount.value['retrying'] ?? 0, tone: 'peach' },
  { label: '失败 (dead)', value: statusCount.value['dead'] ?? 0, tone: 'coral' },
  { label: '待处理', value: statusCount.value['pending'] ?? 0, tone: 'blue' },
])

const queueItems = computed(() => [
  { label: '待领取', value: statusCount.value['pending'] ?? 0, tone: 'blue' },
  { label: '处理中', value: statusCount.value['processing'] ?? 0, tone: 'peach' },
  { label: '等待重试', value: statusCount.value['retrying'] ?? 0, tone: 'coral' },
])

const topFailures = computed(() => ({
  sink: topBy(deliveries.value, (item) => item.sink_name || item.sink_type || `Sink #${item.sink_id}`),
  rule: topBy(deliveries.value, (item) => item.rule_name || `Rule #${item.rule_id}`),
  source: topBy(deliveries.value, (item) => item.source_name || `Source #${item.message_id}`),
}))

function topBy(items: Delivery[], keyFn: (item: Delivery) => string) {
  const counts = new Map<string, number>()
  for (const item of items) {
    const key = keyFn(item)
    counts.set(key, (counts.get(key) ?? 0) + 1)
  }
  return [...counts.entries()]
    .map(([label, value]) => ({ label, value }))
    .sort((a, b) => b.value - a.value)
    .slice(0, 5)
}

async function load() {
  if (!auth.hasToken) {
    message.warning('请先在「设置」中配置 API Token')
    return
  }
  loading.value = true
  try {
    const [accs, srcs, snks, rls, all, success, retrying, pending, processing, dead, failed] = await Promise.all([
      accountsApi.list(),
      sourcesApi.list(),
      sinksApi.list(),
      rulesApi.list(),
      deliveriesApi.page('', 1, 0, { since_hours: windowHours }),
      deliveriesApi.page('success', 1, 0, { since_hours: windowHours }),
      deliveriesApi.page('retrying', 1, 0, { since_hours: windowHours }),
      deliveriesApi.page('pending', 1, 0, { since_hours: windowHours }),
      deliveriesApi.page('processing', 1, 0, { since_hours: windowHours }),
      deliveriesApi.page('dead', 250, 0, { since_hours: windowHours }),
      deliveriesApi.page('failed', 250, 0, { since_hours: windowHours }),
    ])
    const aiProfiles = await aiApi.profiles.list()
    counts.value = { accounts: accs.length, sources: srcs.length, sinks: snks.length, rules: rls.length }
    windowTotal.value = all.total
    statusTotals.value = {
      success: success.total,
      retrying: retrying.total,
      pending: pending.total,
      processing: processing.total,
      dead: dead.total,
      failed: failed.total,
    }
    deliveries.value = [...dead.data, ...failed.data]
    const recentRuns = aiProfiles.map((item) => item.recent_run).filter(Boolean)
    aiStats.value = {
      total: recentRuns.length,
      success: recentRuns.filter((item) => item?.status === 'success').length,
      failed: recentRuns.filter((item) => item?.status === 'failed').length,
      tokens: recentRuns.reduce((sum, item) => sum + (item?.token_usage.total_tokens ?? 0), 0),
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
            <span class="status-num">{{ s.value }}</span>
            <span class="status-name">{{ s.label }}</span>
          </div>
        </div>
      </div>
    </n-card>

    <div class="ops-grid">
      <n-card class="panel" title="队列状态">
        <div class="status-grid compact">
          <div v-for="s in queueItems" :key="s.label" class="status-pill" :class="`tone-${s.tone}`">
            <span class="status-dot" />
            <span class="status-num">{{ s.value }}</span>
            <span class="status-name">{{ s.label }}</span>
          </div>
        </div>
      </n-card>
      <n-card class="panel" title="失败 Top">
        <div class="top-grid">
          <div v-for="(items, key) in topFailures" :key="key" class="top-list">
            <div class="top-title">{{ key === 'sink' ? 'Sink' : key === 'rule' ? 'Rule' : 'Source' }}</div>
            <n-empty v-if="!items.length" size="small" description="暂无失败" />
            <div v-for="item in items" :key="item.label" class="top-item">
              <span>{{ item.label }}</span>
              <strong>{{ item.value }}</strong>
            </div>
          </div>
        </div>
      </n-card>
    </div>

    <n-card class="panel" title="AI 整理概况">
      <div class="status-grid compact-ai">
        <div class="status-pill tone-blue">
          <span class="status-dot" />
          <span class="status-num">{{ aiStats.total }}</span>
          <span class="status-name">最近运行 Profile</span>
        </div>
        <div class="status-pill tone-mint">
          <span class="status-dot" />
          <span class="status-num">{{ aiStats.success }}</span>
          <span class="status-name">成功</span>
        </div>
        <div class="status-pill tone-coral">
          <span class="status-dot" />
          <span class="status-num">{{ aiStats.failed }}</span>
          <span class="status-name">失败</span>
        </div>
        <div class="status-pill tone-peach">
          <span class="status-dot" />
          <span class="status-num">{{ aiStats.tokens }}</span>
          <span class="status-name">Token</span>
        </div>
      </div>
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
  border-radius: 14px;
  background: var(--clay-surface);
  box-shadow: var(--clay-extruded);
  transition: box-shadow 0.22s ease-out, transform 0.22s ease-out;
}
.tile:hover {
  box-shadow: var(--clay-extruded-hover);
  transform: translateY(-2px);
}
.tile-icon {
  width: 48px;
  height: 48px;
  border-radius: 14px;
  display: grid;
  place-items: center;
  color: var(--tone);
  background: var(--tone-soft);
  box-shadow: var(--clay-inset-sm);
  flex-shrink: 0;
  transition: transform 0.3s ease-out;
}
.tile:hover .tile-icon {
  animation: clay-float-scale 1.4s ease-in-out infinite;
}
.tile-value {
  font-size: 30px;
  font-weight: 900;
  line-height: 1;
  font-variant-numeric: tabular-nums;
  color: var(--n-text-color, #22364a);
}
.tile-label {
  margin-top: 6px;
  font-size: 14px;
  font-weight: 700;
  color: #8399ad;
}

/* ---------- 概览面板 ---------- */
.panel {
  margin-top: 20px;
}

.ops-grid {
  display: grid;
  grid-template-columns: minmax(280px, 0.8fr) minmax(320px, 1.2fr);
  gap: 20px;
}
.overview {
  display: flex;
  align-items: stretch;
  gap: 20px;
  flex-wrap: wrap;
}
.rate-badge {
  flex-shrink: 0;
  width: 150px;
  border-radius: 18px;
  padding: 22px;
  display: flex;
  flex-direction: column;
  justify-content: center;
  align-items: center;
  color: #fff;
  background: linear-gradient(150deg, #56b0ea, #2f8fd6);
  box-shadow:
    7px 7px 16px rgba(32, 117, 179, 0.26),
    -5px -5px 14px rgba(255, 255, 255, 0.7),
    inset 2px 2px 6px rgba(255, 255, 255, 0.28),
    inset -4px -4px 10px rgba(22, 100, 160, 0.24);
  animation: clay-breathe 4.6s ease-in-out infinite;
}
.rate-value {
  font-size: 34px;
  font-weight: 900;
  line-height: 1;
  font-variant-numeric: tabular-nums;
  white-space: nowrap;
}
.rate-label {
  margin-top: 8px;
  font-size: 13px;
  font-weight: 700;
  opacity: 0.9;
}
.status-grid {
  flex: 1;
  min-width: 260px;
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(130px, 1fr));
  gap: 14px;
}

.status-grid.compact {
  grid-template-columns: 1fr;
}

.status-grid.compact-ai {
  grid-template-columns: repeat(4, minmax(140px, 1fr));
}
.status-total {
  grid-column: 1 / -1;
  display: flex;
  align-items: baseline;
  gap: 10px;
  padding: 4px 4px 8px;
}
.status-value {
  font-size: 28px;
  font-weight: 900;
  font-variant-numeric: tabular-nums;
}
.status-label {
  font-size: 13px;
  font-weight: 700;
  color: #8399ad;
}
.status-pill {
  display: flex;
  align-items: center;
  gap: 10px;
  padding: 14px 16px;
  border: 0;
  border-radius: 12px;
  background: var(--clay-surface-2);
  box-shadow: var(--clay-inset-sm);
  transition: box-shadow 0.18s ease, transform 0.18s ease;
}

.status-pill:hover {
  box-shadow: var(--clay-inset-deep);
  transform: translateY(-1px);
}
.status-dot {
  width: 12px;
  height: 12px;
  border-radius: 50%;
  background: var(--tone);
  box-shadow: 0 0 0 4px var(--tone-soft), 0 2px 5px var(--tone-soft);
  flex-shrink: 0;
}
.status-num {
  font-size: 22px;
  font-weight: 900;
  font-variant-numeric: tabular-nums;
}
.status-name {
  font-size: 13px;
  font-weight: 600;
  color: #8399ad;
}

.top-grid {
  display: grid;
  grid-template-columns: repeat(3, minmax(0, 1fr));
  gap: 14px;
}

.top-title {
  margin-bottom: 8px;
  color: var(--clay-text);
  font-size: 13px;
  font-weight: 800;
}

.top-item {
  display: flex;
  justify-content: space-between;
  gap: 10px;
  padding: 9px 10px;
  border-top: 0;
  border-radius: 10px;
  background: var(--clay-surface-2);
  box-shadow: var(--clay-inset-sm);
}

.top-item span {
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

@media (max-width: 900px) {
  .ops-grid,
  .top-grid,
  .status-grid.compact-ai {
    grid-template-columns: 1fr;
  }
}
</style>
