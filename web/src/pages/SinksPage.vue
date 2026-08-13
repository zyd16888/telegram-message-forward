<script setup lang="ts">
import { computed, h, onMounted, shallowRef } from 'vue'
import { useRouter } from 'vue-router'
import { NButton, NSpace, NSwitch, NTag, NText, NTooltip, useDialog, useMessage, type DataTableColumns } from 'naive-ui'
import SinkFormModal from '@/components/sinks/SinkFormModal.vue'
import PageHeader from '@/components/PageHeader.vue'
import ClayIcon from '@/components/ClayIcon.vue'
import { flowToLinearFlow, flowsApi, sinksApi } from '@/api/client'
import type { Capabilities, LinearFlow, Sink, SinkDescriptor } from '@/types'
import { errText } from '@/utils/error'

const message = useMessage()
const dialog = useDialog()
const router = useRouter()

const sinks = shallowRef<Sink[]>([])
const descriptors = shallowRef<SinkDescriptor[]>([])
const rules = shallowRef<LinearFlow[]>([])
const loading = shallowRef(false)
const showForm = shallowRef(false)
const editingSink = shallowRef<Sink | null>(null)

const descriptorMap = computed(() => new Map(descriptors.value.map((item) => [item.type, item])))

const ruleCountBySinkId = computed(() => {
  const counts = new Map<number, number>()
  for (const rule of rules.value) {
    for (const target of rule.targets) {
      counts.set(target.sink_id, (counts.get(target.sink_id) ?? 0) + 1)
    }
  }
  return counts
})

async function load() {
  loading.value = true
  try {
    ;[sinks.value, descriptors.value, rules.value] = await Promise.all([
      sinksApi.list(),
      sinksApi.meta(),
      flowsApi.list().then((items) => items.map(flowToLinearFlow)),
    ])
  } catch (e) {
    message.error('加载失败：' + errText(e))
  } finally {
    loading.value = false
  }
}

function openCreate() {
  editingSink.value = null
  showForm.value = true
}

function openEdit(row: Sink) {
  editingSink.value = row
  showForm.value = true
}

function sinkLabel(type: string): string {
  return descriptorMap.value.get(type)?.label ?? type
}

function sinkCapabilities(row: Sink): Capabilities {
  const descriptorCaps = descriptorMap.value.get(row.type)?.capabilities
  if ((row.capabilities.media?.length ?? 0) > 0 || row.capabilities.supports_text) {
    return row.capabilities
  }
  return descriptorCaps ?? row.capabilities
}

function formatSummary(caps: Capabilities): string {
  const out: string[] = []
  if (caps.supports_text) out.push('Text')
  if (caps.supports_markdown) out.push('Markdown')
  if (caps.supports_html) out.push('HTML')
  return out.join(' / ') || '无'
}

function mediaSummary(caps: Capabilities): string {
  const supported = (caps.media ?? [])
    .filter((item) => item.supported)
    .map((item) => item.type)
  if (supported.length) return supported.join(' / ')
  const legacy: string[] = []
  if (caps.supports_image) legacy.push('image')
  if (caps.supports_file) legacy.push('file')
  if (caps.supports_audio) legacy.push('audio')
  if (caps.supports_video) legacy.push('video')
  return legacy.join(' / ') || '文本降级'
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

function successRate(row: Sink): string {
  const total = row.observability?.delivery_total_24h ?? 0
  if (!total) return '24h 无投递'
  return `${Math.round((row.observability.success_rate_24h ?? 0) * 100)}% (${row.observability.delivery_success_24h}/${total})`
}

function lastTestLabel(row: Sink): string {
  const obs = row.observability
  if (!obs?.last_test_at) return '未测试'
  return `${obs.last_test_success ? '测试成功' : '测试失败'} ${formatRuntimeTime(obs.last_test_at)}`
}

async function toggle(row: Sink, value: boolean) {
  try {
    await sinksApi.update(row.id, { enabled: value })
    // sinks 是 shallowRef：直接改 row.enabled 不会触发表格重新渲染，
    // 必须替换数组里对应项的引用。
    sinks.value = sinks.value.map((s) => (s.id === row.id ? { ...s, enabled: value } : s))
  } catch (e) {
    message.error('更新失败：' + errText(e))
  }
}

function goToFlows(sinkId: number) {
  router.push({ name: 'flow', query: { sink_id: String(sinkId) } })
}

function goToFlow(sinkId: number) {
  router.push({ name: 'flow', query: { sink_id: String(sinkId) } })
}

function createRuleForSink(sinkId: number) {
  router.push({ name: 'flow', query: { create: '1', sink_id: String(sinkId) } })
}

function confirmDelete(row: Sink) {
  dialog.warning({
    title: '删除渠道',
    content: `确定删除渠道「${row.name}」？存在 Flow、AI 整理或历史投递引用时，系统会阻止删除并保留审计记录。`,
    positiveText: '删除',
    negativeText: '取消',
    onPositiveClick: async () => {
      try {
        await sinksApi.remove(row.id)
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

const columns: DataTableColumns<Sink> = [
  { title: 'ID', key: 'id', width: 70 },
  {
    title: '类型',
    key: 'type',
    width: 160,
    render: (row) => h(NTag, { size: 'small' }, { default: () => sinkLabel(row.type) }),
  },
  { title: '名称', key: 'name', ellipsis: { tooltip: true } },
  {
    title: '能力',
    key: 'capabilities',
    width: 220,
    render: (row) => {
      const caps = sinkCapabilities(row)
      return h(NSpace, { vertical: true, size: 2 }, {
        default: () => [
          h(NText, { depth: 2 }, { default: () => formatSummary(caps) }),
          h(NText, { depth: 3 }, { default: () => `媒体：${mediaSummary(caps)}` }),
        ],
      })
    },
  },
  {
    title: '观测',
    key: 'observability',
    width: 230,
    render: (row) =>
      h('div', { class: 'observability-cell' }, [
        h(
          NTag,
          {
            size: 'small',
            type: row.observability?.last_test_at
              ? row.observability.last_test_success
                ? 'success'
                : 'error'
              : 'default',
            bordered: false,
          },
          { default: () => lastTestLabel(row) },
        ),
        h(NText, { depth: 2 }, { default: () => `成功率：${successRate(row)}` }),
        h(
          NText,
          { depth: row.observability?.recent_failure ? 1 : 3 },
          { default: () => row.observability?.recent_failure || row.observability?.last_test_error || '无最近失败' },
        ),
      ]),
  },
  { title: '含密钥', key: 'has_secret', width: 90, render: (row) => (row.has_secret ? '是' : '否') },
  {
    title: '关联 Flow',
    key: 'rules',
    width: 110,
    render: (row) => {
      const count = ruleCountBySinkId.value.get(row.id) ?? 0
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
    width: 90,
    render: (row) => h(NSwitch, { value: row.enabled, onUpdateValue: (value: boolean) => toggle(row, value) }),
  },
  {
    title: '操作',
    key: 'actions',
    width: 150,
    render: (row) =>
      h('div', { class: 'action-row' }, [
        actionButton('查看编排', 'flow', () => goToFlow(row.id)),
        actionButton('创建 Flow', 'plus', () => createRuleForSink(row.id)),
        actionButton('编辑', 'edit', () => openEdit(row)),
        actionButton('删除', 'trash', () => confirmDelete(row), 'error'),
      ]),
  },
]

onMounted(load)
</script>

<template>
  <NSpace vertical size="large">
    <PageHeader title="目标渠道" desc="配置消息投递的下游渠道（Webhook、机器人等）" icon="sinks">
      <template #actions>
        <NButton type="primary" @click="openCreate">
          <template #icon><ClayIcon name="plus" :size="16" /></template>
          新建渠道
        </NButton>
        <NButton secondary @click="load">
          <template #icon><ClayIcon name="refresh" :size="16" /></template>
          刷新
        </NButton>
      </template>
    </PageHeader>

    <NDataTable :loading="loading" :columns="columns" :data="sinks" :bordered="false" :scroll-x="1060" />

    <SinkFormModal v-model:show="showForm" :descriptors="descriptors" :sink="editingSink" @saved="load" />
  </NSpace>
</template>

<style scoped>
.observability-cell {
  display: grid;
  gap: 4px;
}

:deep(.action-row) {
  display: flex;
  align-items: center;
  gap: 2px;
  flex-wrap: nowrap;
}
</style>
