<script setup lang="ts">
import { h, onMounted, ref } from 'vue'
import { NButton, NSpace, NSwitch, NTag, useDialog, useMessage, type DataTableColumns } from 'naive-ui'
import { sinksApi } from '@/api/client'
import type { Sink } from '@/types'
import { errText } from '@/utils/error'

const message = useMessage()
const dialog = useDialog()

const sinks = ref<Sink[]>([])
const types = ref<string[]>([])
const loading = ref(false)
const showCreate = ref(false)

const form = ref({ type: 'webhook', name: '', config: '{\n  "url": "https://example.com/webhook"\n}', secret: '' })

async function load() {
  loading.value = true
  try {
    sinks.value = await sinksApi.list()
    types.value = await sinksApi.types()
  } catch (e) {
    message.error('加载失败：' + errText(e))
  } finally {
    loading.value = false
  }
}

async function create() {
  let config: Record<string, unknown> = {}
  try {
    config = form.value.config.trim() ? JSON.parse(form.value.config) : {}
  } catch {
    message.error('config 不是合法 JSON')
    return
  }
  try {
    await sinksApi.create({ type: form.value.type, name: form.value.name, config, secret: form.value.secret })
    message.success('已创建')
    showCreate.value = false
    form.value = { type: 'webhook', name: '', config: '{}', secret: '' }
    await load()
  } catch (e) {
    message.error('创建失败：' + errText(e))
  }
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
  { title: '类型', key: 'type', render: (r) => h(NTag, { size: 'small' }, { default: () => r.type }) },
  { title: '名称', key: 'name' },
  { title: '含密钥', key: 'has_secret', render: (r) => (r.has_secret ? '是' : '否') },
  {
    title: '启用',
    key: 'enabled',
    render: (r) => h(NSwitch, { value: r.enabled, onUpdateValue: (v: boolean) => toggle(r, v) }),
  },
  {
    title: '操作',
    key: 'actions',
    render: (r) =>
      h(NSpace, {}, {
        default: () => [
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
      <n-button type="primary" @click="showCreate = true">新建渠道</n-button>
      <n-button @click="load">刷新</n-button>
    </n-space>

    <n-data-table :loading="loading" :columns="columns" :data="sinks" :bordered="false" />

    <n-modal v-model:show="showCreate" preset="card" title="新建渠道" style="width: 560px">
      <n-form label-placement="left" label-width="80">
        <n-form-item label="类型">
          <n-select v-model:value="form.type" :options="types.map((t) => ({ label: t, value: t }))" />
        </n-form-item>
        <n-form-item label="名称"><n-input v-model:value="form.name" /></n-form-item>
        <n-form-item label="配置">
          <n-input v-model:value="form.config" type="textarea" :autosize="{ minRows: 4 }" placeholder="JSON 配置" />
        </n-form-item>
        <n-form-item label="密钥">
          <n-input v-model:value="form.secret" type="password" show-password-on="click" placeholder="webhook key / corpsecret 等" />
        </n-form-item>
        <n-text depth="3">
          webhook: config {"url": "..."}；wecom_bot: secret=群机器人 key；wecom_app: config {"corpid","agentid"}，secret=corpsecret。
        </n-text>
      </n-form>
      <template #footer>
        <n-space justify="end">
          <n-button @click="showCreate = false">取消</n-button>
          <n-button type="primary" @click="create">创建</n-button>
        </n-space>
      </template>
    </n-modal>
  </n-space>
</template>
