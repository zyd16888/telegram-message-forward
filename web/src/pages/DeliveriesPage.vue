<script setup lang="ts">
import { computed, h, onMounted, shallowRef } from 'vue'
import { NButton, NTag, NText, NTooltip, useMessage, type DataTableColumns, type PaginationProps } from 'naive-ui'
import { deliveriesApi } from '@/api/client'
import type { Delivery } from '@/types'
import { errText } from '@/utils/error'
import PageHeader from '@/components/PageHeader.vue'
import ClayIcon from '@/components/ClayIcon.vue'

const message = useMessage()

const deliveries = shallowRef<Delivery[]>([])
const loading = shallowRef(false)
const status = shallowRef('')
const page = shallowRef(1)
const pageSize = shallowRef(20)
const total = shallowRef(0)

const statusOptions = [
  { label: '全部', value: '' },
  { label: '待投递', value: 'pending' },
  { label: '投递中', value: 'processing' },
  { label: '已成功', value: 'success' },
  { label: '待重试', value: 'retrying' },
  { label: '失败', value: 'failed' },
  { label: '已死亡', value: 'dead' },
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

const statusLabel: Record<string, string> = {
  pending: '待投递',
  processing: '投递中',
  success: '已成功',
  retrying: '待重试',
  failed: '失败',
  dead: '已死亡',
  cancelled: '已取消',
}

const messageTypeLabel: Record<string, string> = {
  text: '文本',
  photo: '图片',
  video: '视频',
  document: '文件',
  audio: '音频',
  voice: '语音',
}

const sinkTypeLabel: Record<string, string> = {
  webhook: 'Webhook',
  wecom_bot: '企业微信机器人',
  wecom_app: '企业微信应用',
  dingtalk: '钉钉机器人',
}

const pagination = computed<PaginationProps>(() => ({
  page: page.value,
  pageSize: pageSize.value,
  itemCount: total.value,
  showSizePicker: true,
  pageSizes: [20, 50, 100],
  prefix: ({ itemCount }) => `共 ${itemCount} 条`,
  onUpdatePage: (nextPage) => {
    page.value = nextPage
    void load()
  },
  onUpdatePageSize: (nextPageSize) => {
    pageSize.value = nextPageSize
    page.value = 1
    void load()
  },
}))

async function load() {
  loading.value = true
  try {
    const result = await deliveriesApi.page(status.value, pageSize.value, (page.value - 1) * pageSize.value)
    deliveries.value = result.data
    total.value = result.total
  } catch (e) {
    message.error('加载失败：' + errText(e))
  } finally {
    loading.value = false
  }
}

async function reloadFromFirstPage() {
  page.value = 1
  await load()
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

function formatTime(value: string): string {
  if (!value) return '-'
  return new Intl.DateTimeFormat('zh-CN', {
    month: '2-digit',
    day: '2-digit',
    hour: '2-digit',
    minute: '2-digit',
  }).format(new Date(value))
}

function sourceTitle(row: Delivery): string {
  return row.source_name || `来源 #${row.message_id}`
}

function sourceMeta(row: Delivery): string {
  const parts = [row.source_username ? `@${row.source_username}` : '', row.sender_name ? `发送人：${row.sender_name}` : '']
  return parts.filter(Boolean).join(' · ') || '未记录来源详情'
}

function targetTitle(row: Delivery): string {
  return row.sink_name || `目标渠道 #${row.sink_id}`
}

function targetMeta(row: Delivery): string {
  const sinkType = row.sink_type ? (sinkTypeLabel[row.sink_type] ?? row.sink_type) : ''
  const parts = [
    sinkType ? `类型：${sinkType}` : '',
    row.rule_name ? `规则：${row.rule_name}` : '',
    row.template_name ? `模板：${row.template_name}` : '',
  ]
  return parts.filter(Boolean).join(' · ') || '默认文本模板'
}

function messageText(row: Delivery): string {
  const text = row.message_text?.trim()
  if (text) return text
  const type = row.message_type ? (messageTypeLabel[row.message_type] ?? row.message_type) : ''
  return type ? `【${type}消息】` : '无文本内容'
}

function renderTwoLine(title: string, meta: string) {
  return h('div', { class: 'cell-stack' }, [
    h('div', { class: 'cell-primary' }, title),
    h('div', { class: 'cell-secondary' }, meta),
  ])
}

function renderMessage(row: Delivery) {
  const text = messageText(row)
  return h(
    NTooltip,
    { trigger: 'hover', placement: 'top-start', width: 460 },
    {
      trigger: () => h('div', { class: 'message-preview' }, text),
      default: () => h('div', { class: 'message-tooltip' }, text),
    },
  )
}

const columns: DataTableColumns<Delivery> = [
  {
    title: '来源',
    key: 'source',
    width: 220,
    fixed: 'left',
    render: (row) => renderTwoLine(sourceTitle(row), sourceMeta(row)),
  },
  {
    title: '内容',
    key: 'message_text',
    minWidth: 320,
    render: renderMessage,
  },
  {
    title: '目标',
    key: 'target',
    width: 260,
    render: (row) => renderTwoLine(targetTitle(row), targetMeta(row)),
  },
  {
    title: '状态',
    key: 'status',
    width: 100,
    render: (r) => h(NTag, { size: 'small', type: statusType[r.status] ?? 'default' }, { default: () => statusLabel[r.status] ?? r.status }),
  },
  { title: '尝试', key: 'attempt_count', width: 80, render: (r) => `${r.attempt_count}/${r.max_attempts}` },
  {
    title: '错误',
    key: 'last_error',
    width: 220,
    ellipsis: { tooltip: true },
    render: (r) => r.last_error || h(NText, { depth: 3 }, { default: () => '无' }),
  },
  { title: '时间', key: 'created_at', width: 130, render: (r) => formatTime(r.created_at) },
  {
    title: '操作',
    key: 'actions',
    width: 90,
    render: (r) =>
      h(
        NButton,
        {
          size: 'small',
          disabled: !['dead', 'failed'].includes(r.status),
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
          @update:value="reloadFromFirstPage"
        />
        <n-button secondary @click="load">
          <template #icon><ClayIcon name="refresh" :size="16" /></template>
          刷新
        </n-button>
      </template>
    </PageHeader>
    <n-data-table
      remote
      :loading="loading"
      :columns="columns"
      :data="deliveries"
      :bordered="false"
      :pagination="pagination"
      :scroll-x="1200"
    />
  </n-space>
</template>

<style scoped>
.status-filter {
  width: 170px;
}

.cell-stack {
  min-width: 0;
}

.cell-primary {
  overflow: hidden;
  color: var(--clay-text);
  font-weight: 700;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.cell-secondary {
  margin-top: 3px;
  overflow: hidden;
  color: var(--clay-muted);
  font-size: 12px;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.message-preview {
  display: -webkit-box;
  max-width: 100%;
  overflow: hidden;
  color: var(--clay-text);
  line-height: 1.45;
  -webkit-box-orient: vertical;
  -webkit-line-clamp: 2;
  word-break: break-word;
}

.message-tooltip {
  max-height: 360px;
  overflow: auto;
  white-space: pre-wrap;
  word-break: break-word;
}

@media (max-width: 640px) {
  .status-filter {
    flex: 1;
    width: auto;
  }
}
</style>
