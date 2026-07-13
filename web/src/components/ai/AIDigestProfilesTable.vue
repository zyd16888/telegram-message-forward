<script setup lang="ts">
import { computed, h } from 'vue'
import { NButton, NTag, NText, type DataTableColumns } from 'naive-ui'
import type { AIDigestOutputTemplate, AIDigestProfile } from '@/types'
import { formatDateTime, formatDateTimeTitle, formatDuration, runDisplayTime } from '@/utils/datetime'
import AIDigestScheduleStatus from '@/components/ai/AIDigestScheduleStatus.vue'

const props = defineProps<{
  profiles: AIDigestProfile[]
  templates: AIDigestOutputTemplate[]
  loading?: boolean
  now: number
}>()
const emit = defineEmits<{
  create: []
  run: [profile: AIDigestProfile]
  preview: [profile: AIDigestProfile]
  records: [profile: AIDigestProfile]
  edit: [profile: AIDigestProfile]
  remove: [profile: AIDigestProfile]
}>()

const templateNames = computed(() => new Map(props.templates.map((item) => [item.id, item.name])))
const columns: DataTableColumns<AIDigestProfile> = [
  {
    title: '名称', key: 'name',
    render: (row) => h('div', { class: 'profile-name' }, [
      h('strong', row.name),
      h(NTag, { size: 'small', type: row.enabled ? 'success' : 'default', bordered: false }, { default: () => (row.enabled ? '定时启用' : '仅手动') }),
    ]),
  },
  { title: '输入 / 输出', key: 'io', render: (row) => `${row.source_ids.length} 来源 · ${row.target_sink_ids.length} 渠道` },
  {
    title: '输出模板', key: 'template',
    render: (row) => row.output_template_id
      ? h(NTag, { size: 'small', type: 'info', bordered: false }, { default: () => templateNames.value.get(row.output_template_id) ?? `#${row.output_template_id}` })
      : h(NText, { depth: 3 }, { default: () => '自定义' }),
  },
  { title: '调度', key: 'schedule', width: 240, render: (row) => h(AIDigestScheduleStatus, { schedule: row.schedule, enabled: row.enabled, now: props.now }) },
  {
    title: '近 7 天', key: 'stats', width: 130,
    render: (row) => {
      const st = row.stats
      if (!st || !st.runs) return h(NText, { depth: 3 }, { default: () => '—' })
      return h('div', { class: 'stats-cell', title: `成功 ${st.success} / 失败 ${st.failed}` }, [
        h('span', `${st.runs} 次`),
        h('small', `${st.tokens || 0} tok`),
        st.failed ? h('small', { class: 'stats-fail' }, `${st.failed} 失败`) : null,
      ])
    },
  },
  {
    title: '最近运行', key: 'recent_run', minWidth: 230,
    render: (row) => {
      const run = row.recent_run
      if (!run) return h(NText, { depth: 3 }, { default: () => '—' })
      const value = runDisplayTime(run)
      const err = run.error_readable || run.error
      return h('div', { class: 'recent-run', title: formatDateTimeTitle(value, row.schedule.timezone) }, [
        h(NTag, { size: 'small', type: statusType(run.status), bordered: false }, { default: () => statusLabel(run.status) }),
        h('span', formatDateTime(value, { timeZone: row.schedule.timezone, seconds: false })),
        run.finished_at
          ? h('small', `耗时 ${formatDuration(run.started_at, run.finished_at)} · ${run.token_usage?.total_tokens ?? 0} tok`)
          : null,
        err ? h('small', { class: 'recent-error', title: err }, err) : null,
      ])
    },
  },
  {
    title: '操作', key: 'actions', width: 300, fixed: 'right',
    render: (row) => h('div', { class: 'profile-actions' }, [
      h(NButton, { size: 'small', type: 'primary', secondary: true, onClick: () => emit('run', row) }, { default: () => '执行' }),
      h(NButton, { size: 'small', onClick: () => emit('preview', row) }, { default: () => '预览' }),
      h(NButton, { size: 'small', onClick: () => emit('records', row) }, { default: () => '记录' }),
      h(NButton, { size: 'small', secondary: true, onClick: () => emit('edit', row) }, { default: () => '编辑' }),
      h(NButton, { size: 'small', type: 'error', quaternary: true, onClick: () => emit('remove', row) }, { default: () => '删除' }),
    ]),
  },
]

function statusLabel(status: string): string {
  return { success: '成功', failed: '失败', running: '运行中', pending: '排队', cancelled: '已取消' }[status] ?? status
}

function statusType(status: string): 'success' | 'error' | 'info' | 'warning' | 'default' {
  return ({ success: 'success', failed: 'error', running: 'info', pending: 'warning', cancelled: 'default' } as const)[status] ?? 'default'
}
</script>

<template>
  <NDataTable :loading="loading" :columns="columns" :data="profiles" :bordered="false" :scroll-x="1280" :row-key="(row: AIDigestProfile) => row.id">
    <template #empty>
      <div class="empty-state">
        <p>还没有 AI 整理任务。</p>
        <NButton type="primary" size="small" @click="emit('create')">新建第一个 Profile</NButton>
      </div>
    </template>
  </NDataTable>
</template>

<style scoped>
:deep(.profile-name),
:deep(.profile-actions) { display: flex; flex-wrap: wrap; align-items: center; gap: 8px; }
:deep(.recent-run) { display: grid; grid-template-columns: max-content minmax(0, 1fr); gap: 3px 8px; align-items: center; min-width: 190px; }
:deep(.recent-run small) { grid-column: 2; color: var(--clay-text-3); overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
:deep(.recent-run .recent-error) { color: var(--clay-error); }
:deep(.stats-cell) { display: flex; flex-direction: column; gap: 2px; font-size: 13px; }
:deep(.stats-cell small) { color: var(--clay-text-3); }
:deep(.stats-cell .stats-fail) { color: var(--clay-error); }
.empty-state { display: flex; flex-direction: column; align-items: center; gap: 10px; padding: 28px 0; color: var(--clay-text-3); }
</style>
