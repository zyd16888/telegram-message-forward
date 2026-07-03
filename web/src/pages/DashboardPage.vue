<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { useMessage } from 'naive-ui'
import { accountsApi, deliveriesApi, rulesApi, sinksApi, sourcesApi } from '@/api/client'
import { useAuthStore } from '@/stores/auth'
import type { Delivery } from '@/types'
import { errText } from '@/utils/error'
import ClayIcon from '@/components/ClayIcon.vue'

const message = useMessage()
const auth = useAuthStore()

const counts = ref({ accounts: 0, sources: 0, sinks: 0, rules: 0 })
const deliveries = ref<Delivery[]>([])
const loading = ref(false)

const statusCount = computed(() => {
  const m: Record<string, number> = {}
  for (const d of deliveries.value) {
    m[d.status] = (m[d.status] ?? 0) + 1
  }
  return m
})

const successRate = computed(() => {
  const total = deliveries.value.length
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

async function load() {
  if (!auth.hasToken) {
    message.warning('请先在「设置」中配置 API Token')
    return
  }
  loading.value = true
  try {
    const [accs, srcs, snks, rls, dels] = await Promise.all([
      accountsApi.list(),
      sourcesApi.list(),
      sinksApi.list(),
      rulesApi.list(),
      deliveriesApi.list('', 200, 0),
    ])
    counts.value = { accounts: accs.length, sources: srcs.length, sinks: snks.length, rules: rls.length }
    deliveries.value = dels
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
    <n-card class="panel" title="投递状态（最近 200 条）">
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
            <div class="status-value">{{ deliveries.length }}</div>
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
  gap: 18px;
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
  padding: 22px;
  border-radius: 24px;
  background: var(--clay-surface);
  box-shadow: var(--clay-out), var(--clay-inset-hi);
  transition: transform 0.25s ease, box-shadow 0.25s ease;
}
.tile:hover {
  transform: translateY(-3px);
  box-shadow: var(--clay-hover), var(--clay-inset-hi);
}
.tile-icon {
  width: 56px;
  height: 56px;
  border-radius: 18px;
  display: grid;
  place-items: center;
  color: var(--tone);
  background: var(--tone-soft);
  box-shadow:
    inset 2px 2px 5px rgba(255, 255, 255, 0.55),
    inset -3px -3px 6px rgba(56, 104, 150, 0.12);
  flex-shrink: 0;
}
.tile-value {
  font-size: 34px;
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
.overview {
  display: flex;
  align-items: stretch;
  gap: 20px;
  flex-wrap: wrap;
}
.rate-badge {
  flex-shrink: 0;
  width: 150px;
  border-radius: 22px;
  padding: 22px;
  display: flex;
  flex-direction: column;
  justify-content: center;
  align-items: center;
  color: #fff;
  background: linear-gradient(150deg, #56b0ea, #2f8fd6);
  box-shadow:
    6px 6px 16px rgba(24, 108, 170, 0.4),
    -3px -3px 10px rgba(255, 255, 255, 0.4),
    inset 2px 2px 5px rgba(255, 255, 255, 0.4),
    inset -4px -4px 8px rgba(18, 90, 150, 0.35);
}
.rate-value {
  font-size: 40px;
  font-weight: 900;
  line-height: 1;
  font-variant-numeric: tabular-nums;
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
  border-radius: 18px;
  background: var(--clay-surface-2);
  box-shadow: var(--clay-out-sm), var(--clay-inset-hi);
}
.status-dot {
  width: 12px;
  height: 12px;
  border-radius: 50%;
  background: var(--tone);
  box-shadow: 0 2px 5px var(--tone-soft);
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
</style>
