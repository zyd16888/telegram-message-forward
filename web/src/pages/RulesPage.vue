<script setup lang="ts">
import { computed, h, onMounted, shallowRef } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { NAlert, NButton, NSpace, NTag, useDialog, useMessage, type DataTableColumns } from 'naive-ui'
import RuleEditorModal from '@/components/rules/RuleEditorModal.vue'
import { rulesApi, sinksApi, sourcesApi, templatesApi } from '@/api/client'
import type { Rule, RuleMeta, Sink, Source, Template } from '@/types'
import { errText } from '@/utils/error'

const message = useMessage()
const dialog = useDialog()
const route = useRoute()
const router = useRouter()

const rules = shallowRef<Rule[]>([])
const sources = shallowRef<Source[]>([])
const sinks = shallowRef<Sink[]>([])
const templates = shallowRef<Template[]>([])
const meta = shallowRef<RuleMeta>({ conditions: [], processors: [] })
const loading = shallowRef(false)
const showModal = shallowRef(false)
const editingRule = shallowRef<Rule | null>(null)

function queryId(key: string): number | null {
  const raw = route.query[key]
  const id = Number(Array.isArray(raw) ? raw[0] : raw)
  return Number.isFinite(id) && id > 0 ? id : null
}

const filterSourceId = computed(() => queryId('source_id'))
const filterSinkId = computed(() => queryId('sink_id'))
const filterSourceName = computed(
  () => sources.value.find((s) => s.id === filterSourceId.value)?.name ?? `#${filterSourceId.value}`,
)
const filterSinkName = computed(
  () => sinks.value.find((s) => s.id === filterSinkId.value)?.name ?? `#${filterSinkId.value}`,
)

const visibleRules = computed(() =>
  rules.value.filter((rule) => {
    if (filterSourceId.value && !rule.source_ids.includes(filterSourceId.value)) return false
    if (filterSinkId.value && !rule.targets.some((target) => target.sink_id === filterSinkId.value)) return false
    return true
  }),
)

function clearFilter() {
  router.replace({ name: 'rules', query: {} })
}

async function load() {
  loading.value = true
  try {
    ;[rules.value, sources.value, sinks.value, templates.value, meta.value] = await Promise.all([
      rulesApi.list(),
      sourcesApi.list(),
      sinksApi.list(),
      templatesApi.list(),
      rulesApi.meta(),
    ])
  } catch (e) {
    message.error('加载失败：' + errText(e))
  } finally {
    loading.value = false
  }
}

function openCreate() {
  editingRule.value = null
  showModal.value = true
}

function openEdit(row: Rule) {
  editingRule.value = row
  showModal.value = true
}

function confirmDelete(row: Rule) {
  dialog.warning({
    title: '删除规则',
    content: `确定删除规则「${row.name}」？`,
    positiveText: '删除',
    negativeText: '取消',
    onPositiveClick: async () => {
      try {
        await rulesApi.remove(row.id)
        message.success('已删除')
        await load()
      } catch (e) {
        message.error('删除失败：' + errText(e))
      }
    },
  })
}

const columns: DataTableColumns<Rule> = [
  { title: 'ID', key: 'id', width: 70 },
  { title: '名称', key: 'name', ellipsis: { tooltip: true } },
  { title: '优先级', key: 'priority', width: 90 },
  {
    title: '启用',
    key: 'enabled',
    width: 90,
    render: (row) => h(NTag, { size: 'small', type: row.enabled ? 'success' : 'default' }, { default: () => (row.enabled ? '是' : '否') }),
  },
  { title: '条件', key: 'conditions', width: 90, render: (row) => row.conditions.length },
  { title: '处理器', key: 'processors', width: 90, render: (row) => row.processors.length },
  { title: '来源数', key: 'source_ids', width: 90, render: (row) => row.source_ids.length },
  { title: '目标数', key: 'targets', width: 90, render: (row) => row.targets.length },
  {
    title: '操作',
    key: 'actions',
    width: 150,
    render: (row) =>
      h(NSpace, {}, {
        default: () => [
          h(NButton, { size: 'small', onClick: () => openEdit(row) }, { default: () => '编辑' }),
          h(NButton, { size: 'small', type: 'error', onClick: () => confirmDelete(row) }, { default: () => '删除' }),
        ],
      }),
  },
]

onMounted(load)
</script>

<template>
  <NSpace vertical size="large">
    <NSpace justify="space-between">
      <NButton type="primary" @click="openCreate">新建规则</NButton>
      <NButton @click="load">刷新</NButton>
    </NSpace>

    <NAlert v-if="filterSourceId || filterSinkId" type="info" :show-icon="false">
      <NSpace align="center" justify="space-between">
        <span>
          正在按{{ filterSourceId ? `监听源「${filterSourceName}」` : `渠道「${filterSinkName}」` }}筛选，共
          {{ visibleRules.length }} 条规则
        </span>
        <NButton size="small" text type="primary" @click="clearFilter">清除筛选</NButton>
      </NSpace>
    </NAlert>

    <NDataTable :loading="loading" :columns="columns" :data="visibleRules" :bordered="false" />

    <RuleEditorModal
      v-model:show="showModal"
      :rule="editingRule"
      :sources="sources"
      :sinks="sinks"
      :templates="templates"
      :condition-descriptors="meta.conditions"
      :processor-descriptors="meta.processors"
      @saved="load"
    />
  </NSpace>
</template>
