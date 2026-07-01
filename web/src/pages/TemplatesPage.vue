<script setup lang="ts">
import { h, onMounted, ref } from 'vue'
import { NButton, NSpace, useDialog, useMessage, type DataTableColumns } from 'naive-ui'
import { templatesApi } from '@/api/client'
import type { Template } from '@/types'
import { errText } from '@/utils/error'

const message = useMessage()
const dialog = useDialog()

const templates = ref<Template[]>([])
const loading = ref(false)
const showModal = ref(false)
const editingId = ref<number | null>(null)
const form = ref({ name: '', format: 'text', content: '{{.Text}}' })

const formatOptions = [
  { label: 'text', value: 'text' },
  { label: 'markdown', value: 'markdown' },
  { label: 'html', value: 'html' },
]

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
  editingId.value = null
  form.value = { name: '', format: 'text', content: '{{.Text}}' }
  showModal.value = true
}

function openEdit(row: Template) {
  editingId.value = row.id
  form.value = { name: row.name, format: row.format, content: row.content }
  showModal.value = true
}

async function submit() {
  try {
    if (editingId.value) {
      await templatesApi.update(editingId.value, { ...form.value })
    } else {
      await templatesApi.create({ ...form.value })
    }
    message.success('已保存')
    showModal.value = false
    await load()
  } catch (e) {
    message.error('保存失败：' + errText(e))
  }
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
  { title: '名称', key: 'name' },
  { title: '格式', key: 'format' },
  { title: '内容', key: 'content', ellipsis: { tooltip: true } },
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
      <n-button type="primary" @click="openCreate">新建模板</n-button>
      <n-button @click="load">刷新</n-button>
    </n-space>

    <n-data-table :loading="loading" :columns="columns" :data="templates" :bordered="false" />

    <n-modal v-model:show="showModal" preset="card" :title="editingId ? '编辑模板' : '新建模板'" style="width: 560px">
      <n-form label-placement="left" label-width="70">
        <n-form-item label="名称"><n-input v-model:value="form.name" /></n-form-item>
        <n-form-item label="格式"><n-select v-model:value="form.format" :options="formatOptions" /></n-form-item>
        <n-form-item label="内容">
          <n-input v-model:value="form.content" type="textarea" :autosize="{ minRows: 4 }" />
        </n-form-item>
        <n-text v-pre depth="3">支持 Go text/template，例如 {{.Text}}、{{.SenderName}}、{{.OriginalURL}}。</n-text>
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
