<script setup lang="ts">
import { computed, h, onMounted, shallowRef } from 'vue'
import { useRoute } from 'vue-router'
import { NButton, NTag, NText, NTooltip, useMessage, type DataTableColumns, type PaginationProps, type SelectOption } from 'naive-ui'
import { deliveriesApi, sinksApi, sourcesApi } from '@/api/client'
import type { Delivery, Sink, SinkDescriptor, Source } from '@/types'
import { errText } from '@/utils/error'
import PageHeader from '@/components/PageHeader.vue'
import ClayIcon from '@/components/ClayIcon.vue'

const message = useMessage()
const route = useRoute()

const deliveries = shallowRef<Delivery[]>([])
const loading = shallowRef(false)
const status = shallowRef('')
const sourceId = shallowRef<number | null>(null)
const sinkId = shallowRef<number | null>(null)
const page = shallowRef(1)
const pageSize = shallowRef(20)
const total = shallowRef(0)
const detailShow = shallowRef(false)
const detailLoading = shallowRef(false)
const detail = shallowRef<Delivery | null>(null)

const sourceList = shallowRef<Source[]>([])
const sinkList = shallowRef<Sink[]>([])
const sinkDescriptors = shallowRef<SinkDescriptor[]>([])

const sourceOptions = computed<SelectOption[]>(() =>
  sourceList.value.map((s) => ({ label: s.name, value: s.id })),
)
const sinkOptions = computed<SelectOption[]>(() =>
  sinkList.value.map((s) => ({ label: s.name, value: s.id })),
)
const sinkTypeLabelByType = computed(() => new Map(sinkDescriptors.value.map((d) => [d.type, d.label])))

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
    const result = await deliveriesApi.page(status.value, pageSize.value, (page.value - 1) * pageSize.value, deliveryFilters())
    deliveries.value = result.data
    total.value = result.total
  } catch (e) {
    message.error('加载失败：' + errText(e))
  } finally {
    loading.value = false
  }
}

async function loadFilterOptions() {
  try {
    ;[sourceList.value, sinkList.value, sinkDescriptors.value] = await Promise.all([
      sourcesApi.list(),
      sinksApi.list(),
      sinksApi.meta(),
    ])
  } catch {
    // 筛选选项加载失败不阻塞记录列表，下拉框保持为空即可。
  }
}

function deliveryFilters(): Record<string, number> {
  const out: Record<string, number> = {}
  if (sourceId.value) out.source_id = sourceId.value
  if (sinkId.value) out.sink_id = sinkId.value
  return out
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

async function retryDeadBatch() {
  try {
    const result = await deliveriesApi.retryDead()
    message.success(`已重新入队 ${result.requeued} 条 dead 任务`)
    await reloadFromFirstPage()
  } catch (e) {
    message.error('批量重试失败：' + errText(e))
  }
}

async function openDetail(row: Delivery) {
  detailShow.value = true
  detailLoading.value = true
  detail.value = null
  try {
    detail.value = await deliveriesApi.get(row.id)
  } catch (e) {
    message.error('加载详情失败：' + errText(e))
  } finally {
    detailLoading.value = false
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

function sinkTypeLabel(row: Delivery): string {
  if (!row.sink_type) return ''
  return sinkTypeLabelByType.value.get(row.sink_type) ?? row.sink_type
}

function targetMeta(row: Delivery): string {
  const parts = [
    deliveryOriginLabel(row),
    row.template_name ? `模板：${row.template_name}` : '',
  ]
  return parts.filter(Boolean).join(' · ') || '默认文本模板'
}

function engineLabel(row: Delivery): string {
  if (row.engine_name) return row.engine_name
  if (row.origin_type === 'flow') return 'Flow 引擎'
  if (row.origin_type === 'ai_digest') return 'AI 整理'
  return 'Flow 引擎'
}

function engineTagType(row: Delivery): 'success' | 'warning' | 'error' | 'info' | 'default' {
  if (row.origin_type === 'flow') return 'success'
  if (row.origin_type === 'ai_digest') return 'warning'
  return 'info'
}

function deliveryOriginLabel(row: Delivery): string {
  if (row.origin_type === 'flow') {
    const name = row.flow_name || (row.origin_id ? `Flow #${row.origin_id}` : '未知 Flow')
    const node = row.origin_node_id ? ` / 节点 #${row.origin_node_id}` : ''
    return `Flow：${name}${node}`
  }
  if (row.origin_type === 'ai_digest') {
    return row.origin_id ? `AI整理：#${row.origin_id}` : 'AI整理'
  }
  return row.flow_name || (row.origin_id ? `Flow：#${row.origin_id}` : '')
}

function renderTarget(row: Delivery) {
  const typeLabel = sinkTypeLabel(row)
  return h('div', { class: 'cell-stack' }, [
    h('div', { class: 'cell-primary target-line' }, [
      h('span', { class: 'target-name' }, targetTitle(row)),
      typeLabel
        ? h(NTag, { size: 'tiny', round: true, bordered: false, type: 'info' }, { default: () => typeLabel })
        : null,
      h(NTag, { size: 'tiny', round: true, bordered: false, type: engineTagType(row) }, { default: () => engineLabel(row) }),
    ]),
    h('div', { class: 'cell-secondary' }, targetMeta(row)),
  ])
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

function jsonText(value: unknown): string {
  if (value === undefined || value === null || value === '') return ''
  return JSON.stringify(value, null, 2)
}

const columns: DataTableColumns<Delivery> = [
  {
    title: '来源',
    key: 'source',
    width: 190,
    fixed: 'left',
    render: (row) => renderTwoLine(sourceTitle(row), sourceMeta(row)),
  },
  {
    title: '内容',
    key: 'message_text',
    minWidth: 240,
    render: renderMessage,
  },
  {
    title: '目标',
    key: 'target',
    width: 280,
    render: renderTarget,
  },
  {
    title: '状态',
    key: 'status',
    width: 88,
    render: (r) => h(NTag, { size: 'small', type: statusType[r.status] ?? 'default' }, { default: () => statusLabel[r.status] ?? r.status }),
  },
  { title: '尝试', key: 'attempt_count', width: 70, render: (r) => `${r.attempt_count}/${r.max_attempts}` },
  {
    title: '错误',
    key: 'last_error_readable',
    width: 180,
    ellipsis: { tooltip: true },
    render: (r) =>
      r.last_error_readable || r.last_error || h(NText, { depth: 3 }, { default: () => '无' }),
  },
  { title: '时间', key: 'created_at', width: 110, render: (r) => formatTime(r.created_at) },
  {
    title: '操作',
    key: 'actions',
    width: 140,
    render: (r) =>
      h('div', { class: 'action-row' }, [
        h(NButton, { size: 'small', secondary: true, onClick: () => openDetail(r) }, { default: () => '详情' }),
        h(
          NButton,
          {
            size: 'small',
            disabled: !['dead', 'failed'].includes(r.status),
            onClick: () => retry(r),
          },
          { default: () => '重试' },
        ),
      ]),
  },
]

function queryId(key: string): number | null {
  const raw = route.query[key]
  const id = Number(Array.isArray(raw) ? raw[0] : raw)
  return Number.isFinite(id) && id > 0 ? id : null
}

onMounted(() => {
  // 支持从编排页等入口带筛选条件跳转。
  sourceId.value = queryId('source_id')
  sinkId.value = queryId('sink_id')
  void load()
  void loadFilterOptions()
  const taskID = queryId('task_id')
  if (taskID) void openDetail({ id: taskID } as Delivery)
})
</script>

<template>
  <n-space vertical size="large">
    <PageHeader title="投递记录" desc="查看消息投递结果，失败可重新入队" icon="deliveries">
      <template #actions>
        <n-select
          v-model:value="sourceId"
          class="entity-filter"
          clearable
          filterable
          placeholder="全部来源"
          :options="sourceOptions"
          @update:value="reloadFromFirstPage"
        />
        <n-select
          v-model:value="sinkId"
          class="entity-filter"
          clearable
          filterable
          placeholder="全部渠道"
          :options="sinkOptions"
          @update:value="reloadFromFirstPage"
        />
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
        <n-button secondary type="warning" @click="retryDeadBatch">批量重试 dead</n-button>
      </template>
    </PageHeader>
    <n-data-table
      remote
      class="deliveries-table"
      :loading="loading"
      :columns="columns"
      :data="deliveries"
      :bordered="false"
      :pagination="pagination"
      :scroll-x="1160"
    />
    <n-modal v-model:show="detailShow" preset="card" title="投递详情" class="delivery-modal" :style="{ width: 'min(760px, calc(100vw - 32px))' }">
      <n-spin :show="detailLoading">
        <n-empty v-if="!detail" description="暂无详情" />
        <div v-else class="detail-stack">
          <div class="detail-grid">
            <div><span>状态</span><strong>{{ statusLabel[detail.status] ?? detail.status }}</strong></div>
            <div><span>任务</span><strong>#{{ detail.id }}</strong></div>
            <div><span>来源</span><strong>{{ detail.source_name || `#${detail.message_id}` }}</strong></div>
            <div><span>目标</span><strong>{{ detail.sink_name || `#${detail.sink_id}` }}</strong></div>
            <div><span>引擎</span><strong>{{ engineLabel(detail) }}</strong></div>
            <div><span>触发对象</span><strong>{{ deliveryOriginLabel(detail) || '-' }}</strong></div>
          </div>
          <section>
            <div class="section-title">消息快照</div>
            <div class="detail-text">{{ messageText(detail) }}</div>
          </section>
          <section>
            <div class="section-title">Attempts</div>
            <n-empty v-if="!detail.attempts?.length" size="small" description="暂无尝试记录" />
            <div v-for="attempt in detail.attempts" :key="attempt.id" class="attempt-item">
              <div class="attempt-head">
                <n-tag size="small" :type="attempt.status === 'success' ? 'success' : 'error'">#{{ attempt.attempt_no }} {{ attempt.status }}</n-tag>
                <n-text depth="3">{{ formatTime(attempt.finished_at || attempt.created_at) }}</n-text>
              </div>
              <n-text v-if="attempt.error_readable || attempt.error" type="error">
                {{ attempt.error_readable || attempt.error }}
              </n-text>
              <n-code v-if="jsonText(attempt.response_summary)" :code="jsonText(attempt.response_summary)" language="json" />
            </div>
          </section>
        </div>
      </n-spin>
    </n-modal>
  </n-space>
</template>

<style scoped>
.status-filter {
  width: 170px;
}

.entity-filter {
  width: 160px;
}

/* 表格单元格由 NDataTable 的 render 回调生成，拿不到本组件的 scopeId，须用 :deep() 下穿。 */
.deliveries-table :deep(.target-line) {
  display: flex;
  align-items: center;
  gap: 6px;
  min-width: 0;
}

.deliveries-table :deep(.target-name) {
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.deliveries-table :deep(.cell-stack) {
  min-width: 0;
}

.deliveries-table :deep(.cell-primary) {
  overflow: hidden;
  color: var(--clay-text);
  font-weight: 700;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.deliveries-table :deep(.cell-secondary) {
  margin-top: 3px;
  overflow: hidden;
  color: var(--clay-muted);
  font-size: 12px;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.deliveries-table :deep(.message-preview) {
  display: -webkit-box;
  max-width: 100%;
  overflow: hidden;
  color: var(--clay-text);
  line-height: 1.45;
  -webkit-box-orient: vertical;
  -webkit-line-clamp: 2;
  word-break: break-word;
}

.deliveries-table :deep(.action-row) {
  display: flex;
  gap: 8px;
}

.detail-stack {
  display: grid;
  gap: 16px;
}

.detail-grid {
  display: grid;
  grid-template-columns: repeat(4, minmax(0, 1fr));
  gap: 10px;
}

.detail-grid > div {
  display: grid;
  gap: 4px;
  padding: 10px;
  border: 1px solid var(--clay-border);
  border-radius: 8px;
  background: var(--clay-surface-2);
}

.detail-grid span {
  color: var(--clay-muted);
  font-size: 12px;
}

.section-title {
  margin-bottom: 8px;
  color: var(--clay-text);
  font-size: 13px;
  font-weight: 800;
}

.detail-text {
  max-height: 180px;
  overflow: auto;
  padding: 10px;
  border: 1px solid var(--clay-border);
  border-radius: 8px;
  white-space: pre-wrap;
}

.attempt-item {
  display: grid;
  gap: 8px;
  padding: 10px 0;
  border-top: 1px solid var(--clay-border);
}

.attempt-head {
  display: flex;
  justify-content: space-between;
  gap: 10px;
}

@media (max-width: 640px) {
  .status-filter {
    flex: 1;
    width: auto;
  }

  .entity-filter {
    flex: 1;
    width: auto;
  }

  .detail-grid {
    grid-template-columns: 1fr;
  }
}
</style>

<style>
/* NTooltip 内容挂载在 body 下，scoped 样式作用不到。 */
.message-tooltip {
  max-height: 360px;
  overflow: auto;
  white-space: pre-wrap;
  word-break: break-word;
}
</style>
