<script setup lang="ts">
import { computed, onMounted, onUnmounted, ref, shallowRef } from 'vue'
import { useMessage } from 'naive-ui'
import { accountsApi, chatArchiveApi, sourcesApi } from '@/api/client'
import type {
  Account,
  ChatArchive,
  ChatArchiveMessage,
  ChatExportJob,
  SyncedPeer,
} from '@/types'
import { errText } from '@/utils/error'
import PageHeader from '@/components/PageHeader.vue'
import ClayIcon from '@/components/ClayIcon.vue'

const message = useMessage()

const archives = shallowRef<ChatArchive[]>([])
const accounts = shallowRef<Account[]>([])
const peers = shallowRef<SyncedPeer[]>([])
const selected = ref<ChatArchive | null>(null)

const messages = shallowRef<ChatArchiveMessage[]>([])
const hasMore = ref(false)
const nextCursor = ref(0)
const searching = ref(false)

const jobs = shallowRef<ChatExportJob[]>([])
const creating = ref(false)
const syncing = ref(false)
const downloading = ref(false)

// 新建任务表单。
const form = ref({
  accountId: null as number | null,
  peerKey: null as string | null,
  range: null as [number, number] | null,
  maxMessages: 50000,
  includeMedia: false,
})

// 检索条件。
const filters = ref({
  q: '',
  direction: null as 'in' | 'out' | null,
  includeService: false,
})

const exportFormat = ref<'jsonl' | 'csv' | 'md'>('jsonl')

let pollTimer: number | null = null

const accountOptions = computed(() =>
  accounts.value.map((a) => ({
    label: `${a.name}（${a.status}）`,
    value: a.id,
    disabled: a.status !== 'active',
  })),
)

const peerOptions = computed(() =>
  peers.value.map((p) => ({
    label: `${p.name}${p.username ? ' @' + p.username : ''} · ${p.display_type}`,
    value: `${p.peer_type}:${p.peer_id}`,
  })),
)

const directionOptions = [
  { label: '对方发出', value: 'in' },
  { label: '我发出', value: 'out' },
]

const formatOptions = [
  { label: 'JSONL（分析脚本 / LLM）', value: 'jsonl' },
  { label: 'CSV（Excel / pandas）', value: 'csv' },
  { label: 'Markdown（按天分节）', value: 'md' },
]

// 有任务在跑时才轮询进度。
const runningJob = computed(() =>
  jobs.value.find((j) => j.status === 'pending' || j.status === 'running'),
)

function statusType(status: ChatExportJob['status']) {
  switch (status) {
    case 'succeeded':
      return 'success'
    case 'failed':
      return 'error'
    case 'cancelled':
      return 'warning'
    default:
      return 'info'
  }
}

function statusLabel(status: ChatExportJob['status']) {
  return (
    { pending: '排队中', running: '拉取中', succeeded: '已完成', failed: '失败', cancelled: '已取消' }[
      status
    ] ?? status
  )
}

function archiveTitle(a: ChatArchive) {
  return a.peer_name || a.peer_username || `${a.peer_type}:${a.peer_id}`
}

function rangeParams() {
  if (!form.value.range) return {}
  return {
    from_date: new Date(form.value.range[0]).toISOString(),
    to_date: new Date(form.value.range[1]).toISOString(),
  }
}

async function loadArchives() {
  try {
    archives.value = await chatArchiveApi.listArchives()
    if (selected.value) {
      const fresh = archives.value.find((a) => a.id === selected.value?.id)
      if (fresh) selected.value = fresh
    }
  } catch (e) {
    message.error('加载归档列表失败：' + errText(e))
  }
}

async function selectArchive(a: ChatArchive) {
  selected.value = a
  messages.value = []
  nextCursor.value = 0
  await Promise.all([search(), loadJobs()])
}

async function loadJobs() {
  if (!selected.value) return
  try {
    jobs.value = await chatArchiveApi.listJobs(selected.value.id)
  } catch (e) {
    message.error('加载任务列表失败：' + errText(e))
  }
}

async function search(append = false) {
  if (!selected.value) return
  searching.value = true
  try {
    const page = await chatArchiveApi.searchMessages(selected.value.id, {
      q: filters.value.q || undefined,
      direction: filters.value.direction ?? undefined,
      include_service: filters.value.includeService,
      before_id: append ? nextCursor.value || undefined : undefined,
      limit: 50,
    })
    messages.value = append ? [...messages.value, ...page.data] : page.data
    hasMore.value = page.has_more
    nextCursor.value = page.next_cursor
  } catch (e) {
    message.error('检索失败：' + errText(e))
  } finally {
    searching.value = false
  }
}

async function syncPeers() {
  if (!form.value.accountId) {
    message.warning('请先选择账号')
    return
  }
  syncing.value = true
  try {
    peers.value = await sourcesApi.sync(form.value.accountId)
    message.success(`已同步 ${peers.value.length} 个会话`)
  } catch (e) {
    message.error('同步会话列表失败：' + errText(e))
  } finally {
    syncing.value = false
  }
}

async function createJob() {
  if (!form.value.accountId || !form.value.peerKey) {
    message.warning('请选择账号与会话')
    return
  }
  const [peerType, peerId] = form.value.peerKey.split(':')
  creating.value = true
  try {
    const created = await chatArchiveApi.createJob({
      account_id: form.value.accountId,
      peer_type: peerType,
      peer_id: Number(peerId),
      max_messages: form.value.maxMessages || undefined,
      include_media: form.value.includeMedia,
      ...rangeParams(),
    })
    message.success('归档任务已开始')
    await loadArchives()
    await selectArchive(created.archive)
  } catch (e) {
    message.error('创建归档任务失败：' + errText(e))
  } finally {
    creating.value = false
  }
}

async function cancelJob(job: ChatExportJob) {
  try {
    await chatArchiveApi.cancelJob(job.id)
    message.success('已请求取消，将在当前页拉取完成后停止')
    await loadJobs()
  } catch (e) {
    message.error('取消失败：' + errText(e))
  }
}

async function download() {
  if (!selected.value) return
  downloading.value = true
  try {
    const name = await chatArchiveApi.download(selected.value.id, {
      format: exportFormat.value,
      include_service: filters.value.includeService,
    })
    message.success(`已导出 ${name}`)
  } catch (e) {
    message.error('导出失败：' + errText(e))
  } finally {
    downloading.value = false
  }
}

async function removeArchive(a: ChatArchive) {
  try {
    await chatArchiveApi.removeArchive(a.id)
    if (selected.value?.id === a.id) {
      selected.value = null
      messages.value = []
      jobs.value = []
    }
    await loadArchives()
    message.success('已删除归档')
  } catch (e) {
    message.error('删除失败：' + errText(e))
  }
}

// 只在有任务运行时轮询，避免空转打接口。
function startPolling() {
  if (pollTimer !== null) return
  pollTimer = window.setInterval(async () => {
    if (!runningJob.value) return
    await loadJobs()
    if (!runningJob.value) {
      await loadArchives()
      await search()
    }
  }, 3000)
}

onMounted(async () => {
  await loadArchives()
  try {
    accounts.value = await accountsApi.list()
  } catch {
    /* 账号下拉为空不阻断归档浏览。 */
  }
  startPolling()
})

onUnmounted(() => {
  if (pollTimer !== null) window.clearInterval(pollTimer)
})
</script>

<template>
  <div class="archive-page">
    <PageHeader
      title="聊天归档"
      desc="把 Telegram 会话历史拉取到本地，支持中文检索与多格式导出。归档不进转发链路，不会产生投递。"
      icon="messages"
    >
      <template #actions>
        <NButton secondary @click="loadArchives">
          <template #icon><ClayIcon name="refresh" :size="16" /></template>
          刷新
        </NButton>
      </template>
    </PageHeader>

    <NCard title="新建归档" size="small">
      <NSpace vertical :size="12">
        <NSpace align="center" :size="12" wrap>
          <NSelect
            v-model:value="form.accountId"
            :options="accountOptions"
            placeholder="选择账号"
            style="width: 220px"
          />
          <NButton :loading="syncing" @click="syncPeers">同步会话列表</NButton>
          <NSelect
            v-model:value="form.peerKey"
            :options="peerOptions"
            placeholder="选择会话（需先同步）"
            filterable
            style="width: 320px"
          />
        </NSpace>
        <NSpace align="center" :size="12" wrap>
          <NDatePicker v-model:value="form.range" type="datetimerange" clearable placeholder="时间范围（可选）" />
          <NInputNumber
            v-model:value="form.maxMessages"
            :min="1"
            :step="1000"
            style="width: 180px"
          >
            <template #prefix>条数上限</template>
          </NInputNumber>
          <NCheckbox v-model:checked="form.includeMedia">下载媒体文件</NCheckbox>
          <NButton type="primary" :loading="creating" @click="createJob">开始归档</NButton>
        </NSpace>
        <NText depth="3" style="font-size: 12px">
          默认只记录媒体的元信息（类型、文件名、大小）。多年私聊的媒体可能有几十 GB，
          确有需要再勾选下载。
        </NText>
      </NSpace>
    </NCard>

    <div class="archive-body">
      <NCard title="已归档会话" size="small" class="archive-list">
        <NEmpty v-if="!archives.length" description="还没有归档，先在上方新建一个" />
        <div v-else class="archive-items">
          <button
            v-for="a in archives"
            :key="a.id"
            type="button"
            class="archive-item"
            :class="{ active: selected?.id === a.id }"
            @click="selectArchive(a)"
          >
            <div class="archive-item-name">{{ archiveTitle(a) }}</div>
            <div class="archive-item-meta">
              {{ a.message_count }} 条
              <span v-if="a.media_count"> · {{ a.media_count }} 媒体</span>
            </div>
          </button>
        </div>
      </NCard>

      <NCard size="small" class="archive-detail">
        <NEmpty v-if="!selected" description="选择左侧一个归档查看内容" />
        <template v-else>
          <NSpace vertical :size="12">
            <NSpace align="center" justify="space-between">
              <NText strong>{{ archiveTitle(selected) }}</NText>
              <NSpace :size="8">
                <NSelect
                  v-model:value="exportFormat"
                  :options="formatOptions"
                  style="width: 220px"
                  size="small"
                />
                <NButton size="small" type="primary" :loading="downloading" @click="download">
                  导出下载
                </NButton>
                <NButton size="small" quaternary @click="removeArchive(selected)">
                  <template #icon><ClayIcon name="trash" :size="14" /></template>
                </NButton>
              </NSpace>
            </NSpace>

            <div v-if="jobs.length" class="job-strip">
              <div v-for="j in jobs.slice(0, 3)" :key="j.id" class="job-row">
                <NTag :type="statusType(j.status)" size="small" round>{{ statusLabel(j.status) }}</NTag>
                <span class="job-meta">已拉取 {{ j.fetched_count }} 条</span>
                <span v-if="j.resume_from" class="job-meta">断点 #{{ j.resume_from }}（可续传）</span>
                <span v-if="j.last_error" class="job-error">{{ j.last_error }}</span>
                <NButton
                  v-if="j.status === 'pending' || j.status === 'running'"
                  size="tiny"
                  quaternary
                  @click="cancelJob(j)"
                >
                  取消
                </NButton>
              </div>
            </div>

            <NSpace align="center" :size="8" wrap>
              <NInput
                v-model:value="filters.q"
                placeholder="搜索正文（支持中文）"
                clearable
                style="width: 260px"
                @keyup.enter="search()"
              />
              <NSelect
                v-model:value="filters.direction"
                :options="directionOptions"
                placeholder="全部方向"
                clearable
                style="width: 140px"
              />
              <NCheckbox v-model:checked="filters.includeService">含系统消息</NCheckbox>
              <NButton :loading="searching" @click="search()">搜索</NButton>
            </NSpace>

            <NEmpty v-if="!messages.length && !searching" description="没有匹配的消息" />
            <div v-else class="msg-list">
              <div v-for="m in messages" :key="m.id" class="msg-row" :class="m.direction">
                <div class="msg-head">
                  <span class="msg-sender">{{ m.sender_name || m.sender_id || '未知' }}</span>
                  <span class="msg-dir">{{ m.direction === 'out' ? '我发出' : '对方' }}</span>
                  <span v-if="m.reply_to_message_id" class="msg-reply">
                    回复 #{{ m.reply_to_message_id }}
                  </span>
                  <span class="msg-date">{{ m.date }}</span>
                </div>
                <div v-if="m.service_action" class="msg-service">[系统] {{ m.service_action }}</div>
                <div v-else class="msg-text">{{ m.text || '（无正文）' }}</div>
                <div v-if="m.media?.length" class="msg-media">
                  <NTag v-for="(md, i) in m.media" :key="i" size="tiny">
                    {{ md.type }}{{ md.downloaded ? '' : '（未下载）' }}
                  </NTag>
                </div>
              </div>
            </div>

            <NButton v-if="hasMore" block secondary :loading="searching" @click="search(true)">
              加载更多
            </NButton>
          </NSpace>
        </template>
      </NCard>
    </div>
  </div>
</template>

<style scoped>
.archive-page { display: grid; gap: 16px; }
.archive-body { display: grid; grid-template-columns: 280px 1fr; gap: 16px; align-items: start; }
.archive-items { display: grid; gap: 6px; }
.archive-item { text-align: left; padding: 8px 10px; border: 1px solid var(--clay-border); border-radius: 8px; background: transparent; cursor: pointer; }
.archive-item:hover { border-color: var(--clay-primary); }
.archive-item.active { border-color: var(--clay-primary); background: var(--clay-primary-soft); }
.archive-item-name { font-weight: 700; }
.archive-item-meta { font-size: 12px; opacity: 0.7; }
.job-strip { display: grid; gap: 6px; }
.job-row { display: flex; align-items: center; gap: 8px; flex-wrap: wrap; font-size: 12px; }
.job-meta { opacity: 0.75; }
.job-error { color: var(--clay-danger, #d03050); }
.msg-list { display: grid; gap: 8px; max-height: 60vh; overflow: auto; }
.msg-row { padding: 8px 10px; border: 1px solid var(--clay-border); border-radius: 8px; }
.msg-row.out { background: var(--clay-primary-soft); }
.msg-head { display: flex; gap: 10px; flex-wrap: wrap; font-size: 12px; opacity: 0.75; }
.msg-sender { font-weight: 700; opacity: 1; }
.msg-text { white-space: pre-wrap; word-break: break-word; margin-top: 4px; }
.msg-service { margin-top: 4px; font-style: italic; opacity: 0.7; }
.msg-media { display: flex; gap: 6px; margin-top: 6px; flex-wrap: wrap; }

@media (max-width: 900px) {
  .archive-body { grid-template-columns: 1fr; }
}
</style>
