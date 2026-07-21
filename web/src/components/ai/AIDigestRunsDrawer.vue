<script setup lang="ts">
import { h, shallowRef, watch } from 'vue'
import { NButton, NTag, NText, useDialog, useMessage, type DataTableColumns } from 'naive-ui'
import { aiApi, settingsApi } from '@/api/client'
import type { AIDigestProfile, AIDigestRun } from '@/types'
import { errText } from '@/utils/error'
import { formatDateTime, formatDateTimeTitle, formatDuration, runDisplayTime } from '@/utils/datetime'

const show = defineModel<boolean>('show', { required: true })
const props = defineProps<{ profile: AIDigestProfile | null }>()
const emit = defineEmits<{
  openDetail: [id: number, timeZone?: string]
  profileCloned: [profile: AIDigestProfile]
}>()

const message = useMessage()
const dialog = useDialog()
const rows = shallowRef<AIDigestRun[]>([])
const loading = shallowRef(false)
const page = shallowRef(1)
const pageSize = 20
const total = shallowRef(0)

const columns: DataTableColumns<AIDigestRun> = [
  { title: 'ID', key: 'id', width: 62 },
  {
    title: '状态', key: 'status', width: 88,
    render: (row) => h(NTag, { size: 'small', type: statusType(row.status), bordered: false }, { default: () => statusLabel(row.status) }),
  },
  { title: '触发', key: 'trigger_type', width: 72, render: (row) => triggerLabel(row.trigger_type) },
  {
    title: '消息', key: 'counts', width: 112,
    render: (row) => h('span', { title: '提交 AI / 过滤纳入 / 窗口输入' }, `${row.prompt_message_count}/${row.included_count}/${row.input_message_count}`),
  },
  { title: 'Token', key: 'token', width: 76, render: (row) => row.token_usage.total_tokens ?? 0 },
  {
    title: '执行时间', key: 'created_at', width: 170,
    render: (row) => {
      const value = runDisplayTime(row)
      return h('div', { class: 'run-time', title: formatDateTimeTitle(value, props.profile?.schedule.timezone) }, [
        h('span', formatDateTime(value, { timeZone: props.profile?.schedule.timezone, seconds: false })),
        h('small', row.finished_at ? formatDuration(row.started_at, row.finished_at) : '执行中'),
      ])
    },
  },
  {
    title: '错误', key: 'error', minWidth: 150,
    render: (row) => {
      const err = row.error_readable || row.error
      return err
        ? h(NText, { type: 'error', class: 'run-error', title: err }, { default: () => err })
        : h(NText, { depth: 3 }, { default: () => '—' })
    },
  },
  {
    title: '操作', key: 'actions', width: 246, fixed: 'right',
    render: (row) => h('div', { class: 'run-actions' }, [
      h(NButton, { size: 'small', secondary: true, onClick: () => emit('openDetail', row.id, props.profile?.schedule.timezone) }, { default: () => '详情' }),
      h(NButton, { size: 'small', secondary: true, onClick: () => cloneProfile(row) }, { default: () => '复制 Profile' }),
      h(NButton, { size: 'small', disabled: row.status !== 'success', onClick: () => deliver(row) }, { default: () => '重新投递' }),
    ]),
  },
]

watch(
  () => [show.value, props.profile?.id] as const,
  ([visible]) => {
    if (!visible) return
    page.value = 1
    void refresh()
  },
)

async function refresh(): Promise<void> {
  if (!props.profile) return
  loading.value = true
  try {
    const result = await aiApi.profiles.runs(props.profile.id, pageSize, (page.value - 1) * pageSize)
    rows.value = result.data
    total.value = result.total
  } catch (error) {
    message.error('加载运行记录失败：' + errText(error))
  } finally {
    loading.value = false
  }
}

function changePage(value: number): void {
  page.value = value
  void refresh()
}

async function deliver(run: AIDigestRun): Promise<void> {
  try {
    const result = await aiApi.runs.deliver(run.id)
    message.success(`已创建投递任务：${result.delivery_task_ids.join(', ') || '无可用渠道'}`)
    await refresh()
    emit('openDetail', run.id, props.profile?.schedule.timezone)
  } catch (error) {
    message.error('重新投递失败：' + errText(error))
  }
}

async function cloneProfile(run: AIDigestRun): Promise<void> {
  try {
    const profile = await aiApi.runs.cloneProfile(run.id)
    message.success(`已创建禁用的 Profile「${profile.name}」`)
    show.value = false
    emit('profileCloned', profile)
  } catch (error) {
    message.error('复制 Profile 失败：' + errText(error))
  }
}

function cleanup(): void {
  dialog.warning({
    title: '清理运行记录',
    content: '将按系统设置中的 AI 运行保留天数清理过期记录（默认 30 天）。是否立即执行一次？',
    positiveText: '立即清理',
    negativeText: '取消',
    onPositiveClick: async () => {
      try {
        const result = await settingsApi.dataRetention.cleanup({ targets: ['ai_runs'] })
        message.success(`已清理 ${result.deleted_ai_runs} 条运行记录`)
        page.value = 1
        await refresh()
      } catch (error) {
        message.error('清理失败：' + errText(error))
      }
    },
  })
}

function statusLabel(status: string): string {
  return { success: '成功', failed: '失败', running: '运行中', pending: '排队', cancelled: '已取消' }[status] ?? status
}

function statusType(status: string): 'success' | 'error' | 'info' | 'warning' | 'default' {
  return ({ success: 'success', failed: 'error', running: 'info', pending: 'warning', cancelled: 'default' } as const)[status] ?? 'default'
}

function triggerLabel(trigger: string): string {
  return { manual: '手动', schedule: '定时', preview: '预览' }[trigger] ?? trigger
}
</script>

<template>
  <NDrawer v-model:show="show" :width="900" placement="right">
    <NDrawerContent :title="`运行记录 · ${profile?.name ?? ''}`" closable>
      <div class="drawer-toolbar">
        <span>消息列：提交 AI / 过滤纳入 / 窗口输入</span>
        <div class="toolbar-actions">
          <NButton size="small" secondary :loading="loading" @click="refresh">刷新</NButton>
          <NButton size="small" quaternary type="error" @click="cleanup">立即清理</NButton>
        </div>
      </div>
      <NDataTable
        :loading="loading"
        :columns="columns"
        :data="rows"
        :bordered="false"
        :scroll-x="1040"
        :row-key="(row: AIDigestRun) => row.id"
      >
        <template #empty><NEmpty description="暂无运行记录" /></template>
      </NDataTable>
      <div v-if="total > pageSize" class="pagination-row">
        <NPagination :page="page" :page-size="pageSize" :item-count="total" @update:page="changePage" />
      </div>
    </NDrawerContent>
  </NDrawer>
</template>

<style scoped>
.drawer-toolbar {
  display: flex;
  justify-content: space-between;
  gap: 12px;
  align-items: center;
  margin-bottom: 12px;
  color: var(--clay-text-3);
  font-size: 12px;
}

.toolbar-actions,
:deep(.run-actions) {
  display: flex;
  gap: 6px;
}

:deep(.run-time) {
  display: grid;
  gap: 2px;
}

:deep(.run-time small) {
  color: var(--clay-text-3);
}

:deep(.run-error) {
  display: block;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.pagination-row {
  display: flex;
  justify-content: flex-end;
  padding-top: 14px;
}

@media (max-width: 640px) {
  .drawer-toolbar {
    align-items: flex-start;
    flex-direction: column;
  }
}
</style>
