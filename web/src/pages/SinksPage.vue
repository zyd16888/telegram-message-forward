<script setup lang="ts">
import { computed, h, onMounted, shallowRef } from 'vue'
import { NButton, NSpace, NSwitch, NTag, useDialog, useMessage, type DataTableColumns } from 'naive-ui'
import SinkFormModal from '@/components/sinks/SinkFormModal.vue'
import { sinksApi } from '@/api/client'
import type { Sink, SinkDescriptor } from '@/types'
import { errText } from '@/utils/error'

const message = useMessage()
const dialog = useDialog()

const sinks = shallowRef<Sink[]>([])
const descriptors = shallowRef<SinkDescriptor[]>([])
const loading = shallowRef(false)
const showForm = shallowRef(false)
const editingSink = shallowRef<Sink | null>(null)

const descriptorMap = computed(() => new Map(descriptors.value.map((item) => [item.type, item])))

async function load() {
  loading.value = true
  try {
    ;[sinks.value, descriptors.value] = await Promise.all([sinksApi.list(), sinksApi.meta()])
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

async function toggle(row: Sink, value: boolean) {
  try {
    await sinksApi.update(row.id, { enabled: value })
    row.enabled = value
  } catch (e) {
    message.error('更新失败：' + errText(e))
  }
}

function confirmDelete(row: Sink) {
  dialog.warning({
    title: '删除渠道',
    content: `确定删除渠道「${row.name}」？`,
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

const columns: DataTableColumns<Sink> = [
  { title: 'ID', key: 'id', width: 70 },
  {
    title: '类型',
    key: 'type',
    width: 160,
    render: (row) => h(NTag, { size: 'small' }, { default: () => sinkLabel(row.type) }),
  },
  { title: '名称', key: 'name', ellipsis: { tooltip: true } },
  { title: '含密钥', key: 'has_secret', width: 90, render: (row) => (row.has_secret ? '是' : '否') },
  {
    title: '启用',
    key: 'enabled',
    width: 90,
    render: (row) => h(NSwitch, { value: row.enabled, onUpdateValue: (value: boolean) => toggle(row, value) }),
  },
  {
    title: '操作',
    key: 'actions',
    width: 160,
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
      <NButton type="primary" @click="openCreate">新建渠道</NButton>
      <NButton @click="load">刷新</NButton>
    </NSpace>

    <NDataTable :loading="loading" :columns="columns" :data="sinks" :bordered="false" />

    <SinkFormModal v-model:show="showForm" :descriptors="descriptors" :sink="editingSink" @saved="load" />
  </NSpace>
</template>
