<script setup lang="ts">
import { h, onMounted, ref } from 'vue'
import { NButton, NSpace, NTag, useDialog, useMessage, type DataTableColumns } from 'naive-ui'
import { rulesApi, sinksApi, sourcesApi, templatesApi } from '@/api/client'
import type { Rule, RuleTarget, Sink, Source, Template } from '@/types'
import { errText } from '@/utils/error'

const message = useMessage()
const dialog = useDialog()

const rules = ref<Rule[]>([])
const sources = ref<Source[]>([])
const sinks = ref<Sink[]>([])
const templates = ref<Template[]>([])
const loading = ref(false)
const showModal = ref(false)
const editingId = ref<number | null>(null)

const form = ref({
  name: '',
  enabled: true,
  priority: 0,
  stop_on_match: false,
  conditions: '[\n  { "type": "keyword_contains", "config": { "keywords": ["golang"] } }\n]',
  processors: '[]',
  source_ids: [] as number[],
  targets: [] as RuleTarget[],
})

async function load() {
  loading.value = true
  try {
    ;[rules.value, sources.value, sinks.value, templates.value] = await Promise.all([
      rulesApi.list(),
      sourcesApi.list(),
      sinksApi.list(),
      templatesApi.list(),
    ])
  } catch (e) {
    message.error('加载失败：' + errText(e))
  } finally {
    loading.value = false
  }
}

function openCreate() {
  editingId.value = null
  form.value = {
    name: '',
    enabled: true,
    priority: 0,
    stop_on_match: false,
    conditions: '[]',
    processors: '[]',
    source_ids: [],
    targets: [],
  }
  showModal.value = true
}

function openEdit(row: Rule) {
  editingId.value = row.id
  form.value = {
    name: row.name,
    enabled: row.enabled,
    priority: row.priority,
    stop_on_match: row.stop_on_match,
    conditions: JSON.stringify(row.conditions, null, 2),
    processors: JSON.stringify(row.processors, null, 2),
    source_ids: [...row.source_ids],
    targets: row.targets.map((t) => ({ ...t })),
  }
  showModal.value = true
}

function addTarget() {
  form.value.targets.push({ sink_id: sinks.value[0]?.id ?? 0, template_id: undefined })
}

function removeTarget(idx: number) {
  form.value.targets.splice(idx, 1)
}

async function submit() {
  let conditions: unknown
  let processors: unknown
  try {
    conditions = JSON.parse(form.value.conditions || '[]')
    processors = JSON.parse(form.value.processors || '[]')
  } catch {
    message.error('conditions / processors 不是合法 JSON')
    return
  }
  const body = {
    name: form.value.name,
    enabled: form.value.enabled,
    priority: form.value.priority,
    stop_on_match: form.value.stop_on_match,
    conditions,
    processors,
    source_ids: form.value.source_ids,
    targets: form.value.targets,
  }
  try {
    if (editingId.value) {
      await rulesApi.update(editingId.value, body)
    } else {
      await rulesApi.create(body)
    }
    message.success('已保存')
    showModal.value = false
    await load()
  } catch (e) {
    message.error('保存失败：' + errText(e))
  }
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
  { title: 'ID', key: 'id', width: 60 },
  { title: '名称', key: 'name' },
  { title: '优先级', key: 'priority', width: 80 },
  {
    title: '启用',
    key: 'enabled',
    render: (r) => h(NTag, { size: 'small', type: r.enabled ? 'success' : 'default' }, { default: () => (r.enabled ? '是' : '否') }),
  },
  { title: '来源数', key: 'source_ids', render: (r) => r.source_ids.length },
  { title: '目标数', key: 'targets', render: (r) => r.targets.length },
  {
    title: '操作',
    key: 'actions',
    render: (r) =>
      h(NSpace, {}, {
        default: () => [
          h(NButton, { size: 'small', onClick: () => openEdit(r) }, { default: () => '编辑' }),
          h(NButton, { size: 'small', type: 'error', onClick: () => confirmDelete(r) }, { default: () => '删除' }),
        ],
      }),
  },
]

onMounted(load)
</script>

<template>
  <n-space vertical size="large">
    <n-space justify="space-between">
      <n-button type="primary" @click="openCreate">新建规则</n-button>
      <n-button @click="load">刷新</n-button>
    </n-space>

    <n-data-table :loading="loading" :columns="columns" :data="rules" :bordered="false" />

    <n-modal v-model:show="showModal" preset="card" :title="editingId ? '编辑规则' : '新建规则'" style="width: 680px">
      <n-form label-placement="left" label-width="90">
        <n-form-item label="名称"><n-input v-model:value="form.name" /></n-form-item>
        <n-space>
          <n-form-item label="优先级"><n-input-number v-model:value="form.priority" /></n-form-item>
          <n-form-item label="启用"><n-switch v-model:value="form.enabled" /></n-form-item>
          <n-form-item label="命中即停"><n-switch v-model:value="form.stop_on_match" /></n-form-item>
        </n-space>
        <n-form-item label="来源">
          <n-select
            v-model:value="form.source_ids"
            multiple
            :options="sources.map((s) => ({ label: `${s.name} (#${s.id})`, value: s.id }))"
          />
        </n-form-item>
        <n-form-item label="条件(JSON)">
          <n-input v-model:value="form.conditions" type="textarea" :autosize="{ minRows: 3 }" />
        </n-form-item>
        <n-form-item label="处理器(JSON)">
          <n-input v-model:value="form.processors" type="textarea" :autosize="{ minRows: 2 }" />
        </n-form-item>
        <n-divider>目标渠道</n-divider>
        <div v-for="(t, idx) in form.targets" :key="idx" style="margin-bottom: 8px">
          <n-space align="center">
            <n-select
              v-model:value="t.sink_id"
              style="width: 220px"
              placeholder="渠道"
              :options="sinks.map((s) => ({ label: `${s.name} (${s.type})`, value: s.id }))"
            />
            <n-select
              v-model:value="t.template_id"
              style="width: 220px"
              clearable
              placeholder="模板（可空=纯文本）"
              :options="templates.map((tp) => ({ label: tp.name, value: tp.id }))"
            />
            <n-button size="small" type="error" @click="removeTarget(idx)">移除</n-button>
          </n-space>
        </div>
        <n-button size="small" dashed @click="addTarget">+ 添加目标</n-button>
      </n-form>
      <template #footer>
        <n-space justify="end">
          <n-button @click="showModal = false">取消</n-button>
          <n-button type="primary" @click="submit">保存</n-button>
        </n-space>
      </template>
    </n-modal>
  </n-space>
</template>
