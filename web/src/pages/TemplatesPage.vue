<script setup lang="ts">
import { h, onMounted, shallowRef } from 'vue'
import { NButton, NSpace, useDialog, useMessage, type DataTableColumns } from 'naive-ui'
import TemplateEditorModal from '@/components/templates/TemplateEditorModal.vue'
import PageHeader from '@/components/PageHeader.vue'
import ClayIcon from '@/components/ClayIcon.vue'
import { templatesApi } from '@/api/client'
import type { Template } from '@/types'
import { errText } from '@/utils/error'

const message = useMessage()
const dialog = useDialog()

const templates = shallowRef<Template[]>([])
const loading = shallowRef(false)
const showModal = shallowRef(false)
const editingTemplate = shallowRef<Template | null>(null)

async function load() {
  loading.value = true
  try {
    templates.value = await templatesApi.list()
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
  { title: '内容', key: 'content', ellipsis: { tooltip: true } },
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

    <NDataTable :loading="loading" :columns="columns" :data="templates" :bordered="false" :scroll-x="640" />

    <TemplateEditorModal v-model:show="showModal" :template="editingTemplate" @saved="load" />
  </NSpace>
</template>
