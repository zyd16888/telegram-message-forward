<script setup lang="ts">
import { computed, h, onMounted, shallowRef } from 'vue'
import { useRouter } from 'vue-router'
import { NButton, NSpace, NText, useDialog, useMessage, type DataTableColumns } from 'naive-ui'
import TemplateEditorModal from '@/components/templates/TemplateEditorModal.vue'
import PageHeader from '@/components/PageHeader.vue'
import ClayIcon from '@/components/ClayIcon.vue'
import { rulesApi, templatesApi } from '@/api/client'
import type { Rule, Template } from '@/types'
import { errText } from '@/utils/error'

const message = useMessage()
const dialog = useDialog()
const router = useRouter()

const templates = shallowRef<Template[]>([])
const rules = shallowRef<Rule[]>([])
const loading = shallowRef(false)
const showModal = shallowRef(false)
const editingTemplate = shallowRef<Template | null>(null)

const ruleCountByTemplateId = computed(() => {
  const counts = new Map<number, number>()
  for (const rule of rules.value) {
    for (const target of rule.targets) {
      if (!target.template_id) continue
      counts.set(target.template_id, (counts.get(target.template_id) ?? 0) + 1)
    }
  }
  return counts
})

async function load() {
  loading.value = true
  try {
    ;[templates.value, rules.value] = await Promise.all([templatesApi.list(), rulesApi.list()])
  } catch (e) {
    message.error('加载失败：' + errText(e))
  } finally {
    loading.value = false
  }
}

function openCreate() {
  editingTemplate.value = null
  showModal.value = true
}

function openEdit(row: Template) {
  editingTemplate.value = row
  showModal.value = true
}

function goToFlow(templateId: number) {
  router.push({ name: 'flow', query: { template_id: String(templateId) } })
}

function confirmDelete(row: Template) {
  dialog.warning({
    title: '删除模板',
    content: `确定删除模板「${row.name}」？`,
    positiveText: '删除',
    negativeText: '取消',
    onPositiveClick: async () => {
      try {
        await templatesApi.remove(row.id)
        message.success('已删除')
        await load()
      } catch (e) {
        message.error('删除失败：' + errText(e))
      }
    },
  })
}

const columns: DataTableColumns<Template> = [
  { title: 'ID', key: 'id', width: 70 },
  { title: '名称', key: 'name', ellipsis: { tooltip: true } },
  { title: '格式', key: 'format', width: 110 },
  {
    title: '关联规则',
    key: 'rules',
    width: 110,
    render: (row) => {
      const count = ruleCountByTemplateId.value.get(row.id) ?? 0
      if (!count) return h(NText, { depth: 3 }, { default: () => '无' })
      return h(
        NButton,
        { text: true, type: 'primary', onClick: () => goToFlow(row.id) },
        { default: () => `${count} 条规则` },
      )
    },
  },
  { title: '内容', key: 'content', ellipsis: { tooltip: true } },
  {
    title: '操作',
    key: 'actions',
    width: 210,
    render: (row) =>
      h(NSpace, {}, {
        default: () => [
          h(NButton, { size: 'small', onClick: () => goToFlow(row.id) }, { default: () => '编排' }),
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
    <PageHeader title="模板" desc="定义消息渲染模板，供规则复用" icon="templates">
      <template #actions>
        <NButton type="primary" @click="openCreate">
          <template #icon><ClayIcon name="plus" :size="16" /></template>
          新建模板
        </NButton>
        <NButton secondary @click="load">
          <template #icon><ClayIcon name="refresh" :size="16" /></template>
          刷新
        </NButton>
      </template>
    </PageHeader>

    <NDataTable :loading="loading" :columns="columns" :data="templates" :bordered="false" :scroll-x="820" />

    <TemplateEditorModal v-model:show="showModal" :template="editingTemplate" @saved="load" />
  </NSpace>
</template>
