<script setup lang="ts">
import { computed, h, onMounted, reactive, ref, shallowRef } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { NButton, NSwitch, NTag, NText, NTooltip, useDialog, useMessage, type DataTableColumns } from 'naive-ui'
import PeerSyncPanel from '@/components/sources/PeerSyncPanel.vue'
import PageHeader from '@/components/PageHeader.vue'
import ClayIcon from '@/components/ClayIcon.vue'
import { accountsApi, flowToLinearFlow, flowsApi, sourcesApi } from '@/api/client'
import type { Account, LinearFlow, Source } from '@/types'
import { errText } from '@/utils/error'

type SourceKind = 'telegram' | 'rss' | 'webhook'

const message = useMessage()
const dialog = useDialog()
const route = useRoute()
const router = useRouter()

const sources = shallowRef<Source[]>([])
const accounts = shallowRef<Account[]>([])
const rules = shallowRef<LinearFlow[]>([])
const loading = shallowRef(false)
const activeTab = shallowRef<SourceKind>('telegram')
const sourceSearch = shallowRef('')
const rssSubmitting = shallowRef(false)
const webhookSubmitting = shallowRef(false)
const rssForm = reactive({
  name: '',
  feed_url: '',
  poll_interval_seconds: 300,
  max_items: 20,
  enabled: true,
})
const webhookForm = reactive({
  name: '',
  token: '',
  enabled: true,
})

const accountNameById = computed(() => new Map(accounts.value.map((a) => [a.id, a.name])))

const ruleCountBySourceId = computed(() => {
  const counts = new Map<number, number>()
  for (const rule of rules.value) {
    for (const id of rule.source_ids) {
      counts.set(id, (counts.get(id) ?? 0) + 1)
    }
  }
  return counts
})

function sourceKind(source: Source): SourceKind {
  if (source.type === 'rss') return 'rss'
  if (source.type === 'webhook') return 'webhook'
  return 'telegram'
}

const telegramSources = computed(() => sources.value.filter((s) => sourceKind(s) === 'telegram'))
const rssSources = computed(() => sources.value.filter((s) => sourceKind(s) === 'rss'))
const webhookSources = computed(() => sources.value.filter((s) => sourceKind(s) === 'webhook'))

function filterByKeyword(list: Source[]): Source[] {
  const keyword = sourceSearch.value.trim().toLowerCase()
  if (!keyword) return list
  return list.filter((source) =>
    [String(source.id), source.name, source.username, String(source.peer_id), source.config?.feed_url]
      .filter(Boolean)
      .some((item) => String(item).toLowerCase().includes(keyword)),
  )
}

const visibleTelegramSources = computed(() => filterByKeyword(telegramSources.value))
const visibleRSSSources = computed(() => filterByKeyword(rssSources.value))
const visibleWebhookSources = computed(() => filterByKeyword(webhookSources.value))

async function load() {
  loading.value = true
  try {
    ;[sources.value, accounts.value, rules.value] = await Promise.all([
      sourcesApi.list(),
      accountsApi.list(),
      flowsApi.list().then((items) => items.map(flowToLinearFlow)),
    ])
  } catch (e) {
    message.error('加载失败：' + errText(e))
  } finally {
    loading.value = false
  }
}

function telegramPeerLabel(source: Source): string {
  const displayType = source.config?.display_type
  if (typeof displayType === 'string' && displayType) return displayType
  if (source.peer_type === 'user') return '用户'
  if (source.peer_type === 'chat') return '普通群'
  return '频道/超级群'
}

function accountName(source: Source): string {
  return accountNameById.value.get(source.account_id) ?? `#${source.account_id}`
}

function runnerStatusLabel(source: Source): string {
  if (!source.enabled) return '未启用'
  if (source.runner_status === 'running') return '运行中'
  return '未运行'
}

function runnerStatusType(source: Source): 'success' | 'warning' | 'default' {
  if (!source.enabled) return 'default'
  return source.runner_status === 'running' ? 'success' : 'warning'
}

function formatRuntimeTime(value?: string): string {
  if (!value) return ''
  return new Intl.DateTimeFormat('zh-CN', {
    month: '2-digit',
    day: '2-digit',
    hour: '2-digit',
    minute: '2-digit',
  }).format(new Date(value))
}

async function toggle(row: Source, value: boolean) {
  try {
    const updated = await sourcesApi.update(row.id, { enabled: value })
    // sources 是 shallowRef：直接改 row.enabled 不会触发表格重新渲染，
    // 必须替换数组里对应项的引用。
    sources.value = sources.value.map((s) => (s.id === row.id ? updated : s))
  } catch (e) {
    message.error('更新失败：' + errText(e))
  }
}

async function toggleDownloadFiles(row: Source, value: boolean) {
  const config = { ...(row.config ?? {}), download_files: value }
  try {
    const updated = await sourcesApi.update(row.id, { config })
    sources.value = sources.value.map((s) => (s.id === row.id ? updated : s))
  } catch (e) {
    message.error('更新失败：' + errText(e))
  }
}

async function toggleHistoryBackfill(row: Source, value: boolean) {
  const config = {
    ...(row.config ?? {}),
    history_backfill_enabled: value,
    history_backfill_limit: Number(row.config?.history_backfill_limit ?? 50) || 50,
  }
  try {
    const updated = await sourcesApi.update(row.id, { config })
    sources.value = sources.value.map((s) => (s.id === row.id ? updated : s))
    message.success(value ? '已开启历史补拉（可手动回捞；启动时仅在有游标时补漏）' : '已关闭历史补拉')
  } catch (e) {
    message.error('更新失败：' + errText(e))
  }
}

const historyBusyId = ref<number | null>(null)

async function previewHistory(row: Source) {
  historyBusyId.value = row.id
  try {
    const limit = Number(row.config?.history_backfill_limit ?? 50) || 50
    const res = await sourcesApi.previewHistory(row.id, limit)
    message.info(`预览到 ${res.fetched} 条（不投递）。最新 ID ${res.max_message_id || '-'}`)
  } catch (e) {
    message.error('预览失败：' + errText(e))
  } finally {
    historyBusyId.value = null
  }
}

async function confirmBackfill(row: Source) {
  historyBusyId.value = row.id
  try {
    const limit = Number(row.config?.history_backfill_limit ?? 50) || 50
    const res = await sourcesApi.backfillHistory(row.id, limit)
    message.success(`已回捞 ${res.ingested}/${res.fetched} 条（幂等，不会重复入队）`)
    await load()
  } catch (e) {
    message.error('回捞失败：' + errText(e))
  } finally {
    historyBusyId.value = null
  }
}

async function addRSSSource() {
  const feedURL = rssForm.feed_url.trim()
  const name = rssForm.name.trim()
  if (!feedURL || !name) {
    message.warning('请填写名称和 Feed URL')
    return
  }
  rssSubmitting.value = true
  try {
    await sourcesApi.create({
      type: 'rss',
      account_id: 0,
      peer_type: 'feed',
      peer_id: 0,
      name,
      username: '',
      enabled: rssForm.enabled,
      config: {
        feed_url: feedURL,
        poll_interval_seconds: rssForm.poll_interval_seconds,
        max_items: rssForm.max_items,
        display_type: 'RSS Feed',
      },
    })
    rssForm.name = ''
    rssForm.feed_url = ''
    rssForm.poll_interval_seconds = 300
    rssForm.max_items = 20
    rssForm.enabled = true
    message.success('RSS 订阅源已添加')
    await load()
  } catch (e) {
    message.error('添加 RSS 失败：' + errText(e))
  } finally {
    rssSubmitting.value = false
  }
}

async function addWebhookSource() {
  const name = webhookForm.name.trim()
  const token = webhookForm.token.trim()
  if (!name || !token) {
    message.warning('请填写名称和 Token')
    return
  }
  webhookSubmitting.value = true
  try {
    const created = await sourcesApi.create({
      type: 'webhook',
      account_id: 0,
      peer_type: 'webhook',
      peer_id: 0,
      name,
      username: '',
      enabled: webhookForm.enabled,
      config: {
        token,
        display_type: 'Webhook',
      },
    })
    webhookForm.name = ''
    webhookForm.token = ''
    webhookForm.enabled = true
    message.success(`Webhook Source 已添加，接收路径 /api/v1/sources/${created.id}/webhook`)
    await load()
  } catch (e) {
    message.error('添加 Webhook 失败：' + errText(e))
  } finally {
    webhookSubmitting.value = false
  }
}

function webhookPath(source: Source): string {
  return `/api/v1/sources/${source.id}/webhook`
}

function goToFlows(sourceId: number) {
  router.push({ name: 'flow', query: { source_id: String(sourceId) } })
}

function goToFlow(sourceId: number) {
  router.push({ name: 'flow', query: { source_id: String(sourceId) } })
}

function createRuleForSource(sourceId: number) {
  router.push({ name: 'flow', query: { create: '1', source_id: String(sourceId) } })
}

function confirmDelete(row: Source) {
  dialog.warning({
    title: '删除监听源',
    content: `确定删除「${row.name}」？`,
    positiveText: '删除',
    negativeText: '取消',
    onPositiveClick: async () => {
      try {
        await sourcesApi.remove(row.id)
        message.success('已删除')
        await load()
      } catch (e) {
        message.error('删除失败：' + errText(e))
      }
    },
  })
}

function actionButton(label: string, icon: string, onClick: () => void, type?: 'error') {
  return h(
    NTooltip,
    { trigger: 'hover' },
    {
      trigger: () =>
        h(
          NButton,
          { size: 'small', quaternary: true, circle: true, type, onClick },
          { icon: () => h(ClayIcon, { name: icon, size: 16 }) },
        ),
      default: () => label,
    },
  )
}

// 各类型表格共用的尾部列：运行状态、关联 Flow、启用、操作。
function commonTailColumns(): DataTableColumns<Source> {
  return [
    {
      title: '运行',
      key: 'runner_status',
      width: 180,
      render: (row) =>
        h('div', { class: 'runtime-cell' }, [
          h(NTag, { size: 'small', type: runnerStatusType(row), bordered: false }, { default: () => runnerStatusLabel(row) }),
          h(
            NText,
            { depth: row.runner_last_error ? 1 : 3 },
            {
              default: () =>
                row.runner_last_error ||
                (row.runner_recent_message_at
                  ? `最近 ${formatRuntimeTime(row.runner_recent_message_at)}`
                  : row.runner_subscriptions
                    ? `${row.runner_subscriptions} 个订阅`
                    : '无运行信息'),
            },
          ),
        ]),
    },
    {
      title: '关联 Flow',
      key: 'rules',
      width: 100,
      render: (row) => {
        const count = ruleCountBySourceId.value.get(row.id) ?? 0
        if (!count) return h(NText, { depth: 3 }, { default: () => '无' })
        return h(
          NButton,
          { text: true, type: 'primary', onClick: () => goToFlows(row.id) },
          { default: () => `${count} 个 Flow` },
        )
      },
    },
    {
      title: '启用',
      key: 'enabled',
      width: 80,
      render: (row) => h(NSwitch, { value: row.enabled, onUpdateValue: (value: boolean) => toggle(row, value) }),
    },
    {
      title: '操作',
      key: 'actions',
      width: 120,
      render: (row) =>
        h('div', { class: 'action-row' }, [
          actionButton('查看编排', 'flow', () => goToFlow(row.id)),
          actionButton('创建 Flow', 'plus', () => createRuleForSource(row.id)),
          actionButton('删除', 'trash', () => confirmDelete(row), 'error'),
        ]),
    },
  ]
}

const telegramColumns: DataTableColumns<Source> = [
  { title: 'ID', key: 'id', width: 70 },
  { title: '名称', key: 'name', ellipsis: { tooltip: true } },
  {
    title: '类型',
    key: 'peer_type',
    width: 120,
    render: (row) => h(NTag, { size: 'small' }, { default: () => telegramPeerLabel(row) }),
  },
  { title: 'Peer ID', key: 'peer_id', width: 140 },
  { title: '账号', key: 'account_id', width: 130, render: (row) => accountName(row) },
  {
    title: '文件下载',
    key: 'download_files',
    width: 100,
    render: (row) =>
      h(
        NTooltip,
        { trigger: 'hover' },
        {
          trigger: () =>
            h(NSwitch, {
              size: 'small',
              value: row.config?.download_files === true,
              onUpdateValue: (value: boolean) => toggleDownloadFiles(row, value),
            }),
          default: () => '开启后下载该来源的 PDF 等文件（大小与类型限制见「设置 → 媒体存储」）；图片始终下载',
        },
      ),
  },
  {
    title: '历史补拉',
    key: 'history_backfill',
    width: 100,
    render: (row) =>
      h(
        NTooltip,
        { trigger: 'hover' },
        {
          trigger: () =>
            h(NSwitch, {
              size: 'small',
              value: row.config?.history_backfill_enabled === true,
              onUpdateValue: (value: boolean) => toggleHistoryBackfill(row, value),
            }),
          default: () =>
            '默认关闭：只收实时消息，断线/重启丢弃历史。开启后可手动回捞，且 last_message_id>0 时启动/恢复会增量补漏',
        },
      ),
  },
  {
    title: '回捞',
    key: 'history_actions',
    width: 160,
    render: (row) => {
      if (row.config?.history_backfill_enabled !== true) {
        return h(NText, { depth: 3 }, { default: () => '—' })
      }
      const busy = historyBusyId.value === row.id
      return h('div', { class: 'action-row' }, [
        h(
          NButton,
          { size: 'tiny', secondary: true, loading: busy, onClick: () => previewHistory(row) },
          { default: () => '预览' },
        ),
        h(
          NButton,
          { size: 'tiny', type: 'primary', secondary: true, loading: busy, onClick: () => confirmBackfill(row) },
          { default: () => '确认回捞' },
        ),
      ])
    },
  },
  ...commonTailColumns(),
]

const rssColumns: DataTableColumns<Source> = [
  { title: 'ID', key: 'id', width: 70 },
  { title: '名称', key: 'name', ellipsis: { tooltip: true } },
  {
    title: 'Feed URL',
    key: 'feed_url',
    minWidth: 220,
    ellipsis: { tooltip: true },
    render: (row) => String(row.config?.feed_url ?? '-'),
  },
  {
    title: '轮询',
    key: 'poll',
    width: 140,
    render: (row) => {
      const interval = Number(row.config?.poll_interval_seconds ?? 0)
      const maxItems = Number(row.config?.max_items ?? 0)
      const parts = [interval ? `${interval}s` : '', maxItems ? `每次 ${maxItems} 条` : '']
      return parts.filter(Boolean).join(' / ') || '-'
    },
  },
  ...commonTailColumns(),
]

const webhookColumns: DataTableColumns<Source> = [
  { title: 'ID', key: 'id', width: 70 },
  { title: '名称', key: 'name', ellipsis: { tooltip: true } },
  {
    title: '接收路径',
    key: 'webhook_path',
    minWidth: 240,
    render: (row) => h(NText, { code: true }, { default: () => webhookPath(row) }),
  },
  ...commonTailColumns(),
]

onMounted(async () => {
  const raw = route.query.source_id
  const id = Number(Array.isArray(raw) ? raw[0] : raw)
  await load()
  // 从其它页面带 source_id 跳转过来时，切到对应类型的 Tab 并定位该来源。
  if (Number.isFinite(id) && id > 0) {
    const target = sources.value.find((s) => s.id === id)
    if (target) {
      activeTab.value = sourceKind(target)
      sourceSearch.value = String(id)
    }
  }
})
</script>

<template>
  <NSpace vertical size="large">
    <PageHeader title="监听源" desc="按类型管理 Telegram、RSS、Webhook 监听来源" icon="sources" />

    <NTabs v-model:value="activeTab" type="line" animated>
      <NTabPane name="telegram">
        <template #tab>
          <span class="tab-label">
            <ClayIcon name="telegram" :size="15" />
            Telegram（{{ telegramSources.length }}）
          </span>
        </template>
        <NSpace vertical size="large" class="tab-body">
          <PeerSyncPanel :accounts="accounts" :sources="sources" @added="load" />
          <div class="list-toolbar">
            <NText strong class="list-title">已配置 Telegram 监听源</NText>
            <div class="list-filters">
              <NInput v-model:value="sourceSearch" clearable class="source-search" placeholder="搜索名称、用户名、Peer ID" />
              <NButton secondary @click="load">
                <template #icon><ClayIcon name="refresh" :size="16" /></template>
                刷新
              </NButton>
            </div>
          </div>
          <NDataTable
            :loading="loading"
            :columns="telegramColumns"
            :data="visibleTelegramSources"
            :bordered="false"
            :scroll-x="1220"
          />
        </NSpace>
      </NTabPane>

      <NTabPane name="rss">
        <template #tab>
          <span class="tab-label">
            <ClayIcon name="rss" :size="15" />
            RSS（{{ rssSources.length }}）
          </span>
        </template>
        <NSpace vertical size="large" class="tab-body">
          <NCard title="添加 RSS 订阅源">
            <NForm label-placement="left" label-width="96" class="rss-form">
              <NFormItem label="名称">
                <NInput v-model:value="rssForm.name" placeholder="例如：项目发布订阅" />
              </NFormItem>
              <NFormItem label="Feed URL">
                <NInput v-model:value="rssForm.feed_url" placeholder="https://example.com/feed.xml" />
              </NFormItem>
              <NFormItem label="轮询秒数">
                <NInputNumber v-model:value="rssForm.poll_interval_seconds" :min="30" :step="60" />
              </NFormItem>
              <NFormItem label="每次条数">
                <NInputNumber v-model:value="rssForm.max_items" :min="1" :max="100" />
              </NFormItem>
              <NFormItem label="启用">
                <NSwitch v-model:value="rssForm.enabled" />
              </NFormItem>
              <NFormItem label=" ">
                <NButton type="primary" :loading="rssSubmitting" @click="addRSSSource">
                  <template #icon><ClayIcon name="plus" :size="16" /></template>
                  添加 RSS
                </NButton>
              </NFormItem>
            </NForm>
          </NCard>
          <div class="list-toolbar">
            <NText strong class="list-title">已配置 RSS 订阅源</NText>
            <div class="list-filters">
              <NInput v-model:value="sourceSearch" clearable class="source-search" placeholder="搜索名称、Feed URL" />
              <NButton secondary @click="load">
                <template #icon><ClayIcon name="refresh" :size="16" /></template>
                刷新
              </NButton>
            </div>
          </div>
          <NDataTable
            :loading="loading"
            :columns="rssColumns"
            :data="visibleRSSSources"
            :bordered="false"
            :scroll-x="1020"
          />
        </NSpace>
      </NTabPane>

      <NTabPane name="webhook">
        <template #tab>
          <span class="tab-label">
            <ClayIcon name="link" :size="15" />
            Webhook（{{ webhookSources.length }}）
          </span>
        </template>
        <NSpace vertical size="large" class="tab-body">
          <NCard title="添加 Webhook Source">
            <NForm label-placement="left" label-width="96" class="webhook-form">
              <NFormItem label="名称">
                <NInput v-model:value="webhookForm.name" placeholder="例如：CI 事件入口" />
              </NFormItem>
              <NFormItem label="Token">
                <NInput v-model:value="webhookForm.token" type="password" show-password-on="click" placeholder="外部请求鉴权 token" />
              </NFormItem>
              <NFormItem label="启用">
                <NSwitch v-model:value="webhookForm.enabled" />
              </NFormItem>
              <NFormItem label=" ">
                <NButton type="primary" :loading="webhookSubmitting" @click="addWebhookSource">
                  <template #icon><ClayIcon name="plus" :size="16" /></template>
                  添加 Webhook
                </NButton>
              </NFormItem>
            </NForm>
          </NCard>
          <div class="list-toolbar">
            <NText strong class="list-title">已配置 Webhook Source</NText>
            <div class="list-filters">
              <NInput v-model:value="sourceSearch" clearable class="source-search" placeholder="搜索名称" />
              <NButton secondary @click="load">
                <template #icon><ClayIcon name="refresh" :size="16" /></template>
                刷新
              </NButton>
            </div>
          </div>
          <NDataTable
            :loading="loading"
            :columns="webhookColumns"
            :data="visibleWebhookSources"
            :bordered="false"
            :scroll-x="960"
          />
        </NSpace>
      </NTabPane>
    </NTabs>
  </NSpace>
</template>

<style scoped>
.tab-label {
  display: inline-flex;
  align-items: center;
  gap: 6px;
}

.tab-body {
  padding-top: 4px;
}

.list-toolbar {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 14px;
  flex-wrap: wrap;
}
.list-title {
  font-size: 15px;
}
.list-filters {
  display: flex;
  align-items: center;
  gap: 10px;
  flex-wrap: wrap;
}
.source-search {
  width: 260px;
}

.rss-form,
.webhook-form {
  display: grid;
  grid-template-columns: repeat(2, minmax(260px, 1fr));
  gap: 2px 18px;
}

.rss-form :deep(.n-form-item:last-child),
.webhook-form :deep(.n-form-item:last-child) {
  grid-column: 1 / -1;
}

.runtime-cell {
  display: grid;
  gap: 4px;
}

:deep(.action-row) {
  display: flex;
  align-items: center;
  gap: 2px;
  flex-wrap: nowrap;
}

@media (max-width: 640px) {
  .list-filters {
    width: 100%;
  }
  .source-search {
    flex: 1 1 100%;
    width: auto;
  }
  .rss-form,
  .webhook-form {
    grid-template-columns: 1fr;
  }
}
</style>
