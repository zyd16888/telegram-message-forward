<script setup lang="ts">
import { computed, h, onMounted, shallowRef } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { NAlert, NButton, NSpace, NTag, NText, useDialog, useMessage, type DataTableColumns } from 'naive-ui'
import RuleEditorModal from '@/components/rules/RuleEditorModal.vue'
import PageHeader from '@/components/PageHeader.vue'
import ClayIcon from '@/components/ClayIcon.vue'
import { filtersApi, rulesApi, sinksApi, sourcesApi, templatesApi } from '@/api/client'
import type { Filter, Rule, RuleInitialDraft, RuleMeta, Sink, Source, Template } from '@/types'
import { errText } from '@/utils/error'

const message = useMessage()
const dialog = useDialog()
const route = useRoute()
const router = useRouter()

const rules = shallowRef<Rule[]>([])
const sources = shallowRef<Source[]>([])
const sinks = shallowRef<Sink[]>([])
const templates = shallowRef<Template[]>([])
const filters = shallowRef<Filter[]>([])
const meta = shallowRef<RuleMeta>({ conditions: [], processors: [] })
const loading = shallowRef(false)
const showModal = shallowRef(false)
const editingRule = shallowRef<Rule | null>(null)
const initialDraft = shallowRef<RuleInitialDraft | null>(null)
const initialRuleOpenDone = shallowRef(false)
const initialCreateOpenDone = shallowRef(false)

function queryId(key: string): number | null {
  const raw = route.query[key]
  const id = Number(Array.isArray(raw) ? raw[0] : raw)
  return Number.isFinite(id) && id > 0 ? id : null
}

const filterSourceId = computed(() => queryId('source_id'))
const filterSinkId = computed(() => queryId('sink_id'))
const queryRuleId = computed(() => queryId('rule_id'))
const shouldCreateFromQuery = computed(() => route.query.create === '1')
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
    ;[rules.value, sources.value, sinks.value, templates.value, filters.value, meta.value] = await Promise.all([
      rulesApi.list(),
      sourcesApi.list(),
      sinksApi.list(),
      templatesApi.list(),
      filtersApi.list(),
      rulesApi.meta(),
    ])
    openQueriedRule()
    openCreateFromQuery()
  } catch (e) {
    message.error('加载失败：' + errText(e))
  } finally {
    loading.value = false
  }
}

function openCreate() {
  editingRule.value = null
  initialDraft.value = null
  showModal.value = true
}

function openCreateWithDraft(draft: RuleInitialDraft) {
  editingRule.value = null
  initialDraft.value = draft
  showModal.value = true
}

function openEdit(row: Rule) {
  editingRule.value = row
  initialDraft.value = null
  showModal.value = true
}

function openQueriedRule() {
  if (initialRuleOpenDone.value || !queryRuleId.value) return
  const rule = rules.value.find((item) => item.id === queryRuleId.value)
  if (!rule) return
  initialRuleOpenDone.value = true
  openEdit(rule)
}

function openCreateFromQuery() {
  if (initialCreateOpenDone.value || !shouldCreateFromQuery.value) return
  const draft = buildInitialDraft()
  initialCreateOpenDone.value = true
  openCreateWithDraft(draft)
}

function buildInitialDraft(): RuleInitialDraft {
  const source = filterSourceId.value ? sources.value.find((item) => item.id === filterSourceId.value) : null
  const sink = filterSinkId.value ? sinks.value.find((item) => item.id === filterSinkId.value) : null
  const parts = [source ? source.name : '', sink ? sink.name : ''].filter(Boolean)
  return {
    name: parts.length ? `转发：${parts.join(' -> ')}` : '',
    source_ids: source ? [source.id] : [],
    targets: sink ? [{ sink_id: sink.id, template_id: undefined }] : [],
  }
}

function goToFlow(query: Record<string, string>) {
  router.push({ name: 'flow', query })
}

function sourceSummary(row: Rule): string {
  if (!row.source_ids.length) return '未指定来源'
  return row.source_ids
    .map((id) => sources.value.find((source) => source.id === id)?.name ?? `#${id}`)
    .join('、')
}

function targetSummary(row: Rule): string {
  if (!row.targets.length) return '未配置目标'
  return row.targets
    .map((target) => {
      const sink = sinks.value.find((item) => item.id === target.sink_id)
      const template = target.template_id ? templates.value.find((item) => item.id === target.template_id) : null
      return `${sink?.name ?? `#${target.sink_id}`}${template ? ` / ${template.name}` : ''}`
    })
    .join('、')
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
  { title: '条件', key: 'conditions', width: 80, render: (row) => row.conditions.length },
  { title: '处理器', key: 'processors', width: 80, render: (row) => row.processors.length },
  {
    title: '来源',
    key: 'source_ids',
    width: 180,
    ellipsis: { tooltip: true },
    render: (row) => h(NText, { depth: row.source_ids.length ? 1 : 3 }, { default: () => sourceSummary(row) }),
  },
  {
    title: '目标',
    key: 'targets',
    width: 240,
    ellipsis: { tooltip: true },
    render: (row) => h(NText, { depth: row.targets.length ? 1 : 3 }, { default: () => targetSummary(row) }),
  },
  {
    title: '操作',
    key: 'actions',
    width: 210,
    render: (row) =>
      h(NSpace, {}, {
        default: () => [
          h(NButton, { size: 'small', onClick: () => openEdit(row) }, { default: () => '编辑' }),
          h(NButton, { size: 'small', onClick: () => goToFlow({ rule_id: String(row.id) }) }, { default: () => '编排' }),
          h(NButton, { size: 'small', type: 'error', onClick: () => confirmDelete(row) }, { default: () => '删除' }),
        ],
      }),
  },
]

onMounted(load)
</script>

<template>
  <NSpace vertical size="large">
    <PageHeader title="转发规则" desc="定义监听源到目标渠道的匹配、处理与投递规则" icon="rules">
      <template #actions>
        <NButton type="primary" @click="openCreate">
          <template #icon><ClayIcon name="plus" :size="16" /></template>
          新建规则
        </NButton>
        <NButton secondary @click="load">
          <template #icon><ClayIcon name="refresh" :size="16" /></template>
          刷新
        </NButton>
      </template>
    </PageHeader>

    <NAlert v-if="filterSourceId || filterSinkId" type="info" :show-icon="false">
      <NSpace align="center" justify="space-between">
        <span>
          正在按{{ filterSourceId ? `监听源「${filterSourceName}」` : `渠道「${filterSinkName}」` }}筛选，共
          {{ visibleRules.length }} 条规则
        </span>
        <NSpace>
          <NButton
            size="small"
            text
            type="primary"
            @click="goToFlow(filterSourceId ? { source_id: String(filterSourceId) } : { sink_id: String(filterSinkId) })"
          >
            查看编排
          </NButton>
          <NButton size="small" text type="primary" @click="clearFilter">清除筛选</NButton>
        </NSpace>
      </NSpace>
    </NAlert>

    <NDataTable :loading="loading" :columns="columns" :data="visibleRules" :bordered="false" :scroll-x="1120" />

    <RuleEditorModal
      v-model:show="showModal"
      :rule="editingRule"
      :sources="sources"
      :sinks="sinks"
      :templates="templates"
      :filters="filters"
      :condition-descriptors="meta.conditions"
      :processor-descriptors="meta.processors"
      :initial-draft="initialDraft"
      @saved="load"
    />
  </NSpace>
</template>
