<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { useMessage } from 'naive-ui'
import { accountsApi, deliveriesApi, rulesApi, sinksApi, sourcesApi } from '@/api/client'
import { useAuthStore } from '@/stores/auth'
import type { Delivery } from '@/types'
import { errText } from '@/utils/error'

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
    <n-grid :cols="4" :x-gap="16" :y-gap="16" responsive="screen">
      <n-gi><n-statistic label="账号" :value="counts.accounts" /></n-gi>
      <n-gi><n-statistic label="监听源" :value="counts.sources" /></n-gi>
      <n-gi><n-statistic label="目标渠道" :value="counts.sinks" /></n-gi>
      <n-gi><n-statistic label="规则" :value="counts.rules" /></n-gi>
    </n-grid>

    <n-card title="投递状态（最近 200 条）" style="margin-top: 16px">
      <n-space size="large" wrap>
        <n-statistic label="成功率" :value="successRate" />
        <n-statistic label="总计" :value="deliveries.length" />
        <n-statistic label="成功" :value="statusCount['success'] ?? 0" />
        <n-statistic label="重试中" :value="statusCount['retrying'] ?? 0" />
        <n-statistic label="失败(dead)" :value="statusCount['dead'] ?? 0" />
        <n-statistic label="待处理" :value="statusCount['pending'] ?? 0" />
      </n-space>
    </n-card>

    <n-card style="margin-top: 16px">
      <n-button type="primary" @click="load">刷新</n-button>
    </n-card>
  </n-spin>
</template>
