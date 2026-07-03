<script setup lang="ts">
import { h, onMounted, ref } from 'vue'
import { NButton, NTag, useMessage, type DataTableColumns } from 'naive-ui'
import { deliveriesApi } from '@/api/client'
import type { Delivery } from '@/types'
import { errText } from '@/utils/error'
import PageHeader from '@/components/PageHeader.vue'
import ClayIcon from '@/components/ClayIcon.vue'

const message = useMessage()

const deliveries = ref<Delivery[]>([])
const loading = ref(false)
const status = ref('')

const statusOptions = [
  { label: '全部', value: '' },
  { label: 'pending', value: 'pending' },
  { label: 'processing', value: 'processing' },
  { label: 'success', value: 'success' },
  { label: 'retrying', value: 'retrying' },
  { label: 'failed', value: 'failed' },
  { label: 'dead', value: 'dead' },
  { label: 'cancelled', value: 'cancelled' },
]

const statusType: Record<string, 'success' | 'warning' | 'error' | 'info' | 'default'> = {
  success: 'success',
  retrying: 'warning',
  pending: 'info',
  processing: 'info',
  dead: 'error',
  failed: 'error',
  cancelled: 'default',
}

async function load() {
  loading.value = true
  try {
    deliveries.value = await deliveriesApi.list(status.value, 100, 0)
  } catch (e) {
    message.error('加载失败：' + errText(e))
  } finally {
    loading.value = false
  }
}

async function retry(row: Delivery) {
  try {
    await deliveriesApi.retry(row.id)
    message.success('已重新入队')
    await load()
  } catch (e) {
    message.error('重试失败：' + errText(e))
  }
}

const columns: DataTableColumns<Delivery> = [
  { title: 'ID', key: 'id', width: 70, fixed: 'left' },
  { title: '消息', key: 'message_id', width: 90 },
  { title: '规则', key: 'rule_id', width: 80 },
  { title: '渠道', key: 'sink_id', width: 80 },
  {
    title: '状态',
    key: 'status',
    render: (r) => h(NTag, { size: 'small', type: statusType[r.status] ?? 'default' }, { default: () => r.status }),
  },
  { title: '尝试', key: 'attempt_count', width: 80, render: (r) => `${r.attempt_count}/${r.max_attempts}` },
  { title: '错误', key: 'last_error', ellipsis: { tooltip: true } },
  {
    title: '操作',
    key: 'actions',
    width: 90,
    render: (r) =>
      h(
        NButton,
        {
          size: 'small',
          disabled: !['dead', 'failed', 'cancelled'].includes(r.status),
          onClick: () => retry(r),
        },
        { default: () => '重试' },
      ),
  },
]

onMounted(load)
</script>

<template>
  <n-space vertical size="large">
    <PageHeader title="投递记录" desc="查看消息投递结果，失败可重新入队" icon="deliveries">
      <template #actions>
        <n-select
          v-model:value="status"
          class="status-filter"
          :options="statusOptions"
          @update:value="load"
        />
        <n-button secondary @click="load">
          <template #icon><ClayIcon name="refresh" :size="16" /></template>
          刷新
        </n-button>
      </template>
    </PageHeader>
    <n-data-table :loading="loading" :columns="columns" :data="deliveries" :bordered="false" :scroll-x="820" />
  </n-space>
</template>

<style scoped>
.status-filter {
  width: 170px;
}
@media (max-width: 640px) {
  .status-filter {
    flex: 1;
    width: auto;
  }
}
</style>
