<script setup lang="ts">
import { h, onMounted, ref } from 'vue'
import { NButton, NSpace, NTag, useDialog, useMessage, type DataTableColumns } from 'naive-ui'
import { accountsApi, sourcesApi } from '@/api/client'
import type { Account, Source, SyncedPeer } from '@/types'
import { errText } from '@/utils/error'

const message = useMessage()
const dialog = useDialog()

const sources = ref<Source[]>([])
const accounts = ref<Account[]>([])
const loading = ref(false)
const syncAccountId = ref<number | null>(null)
const syncedPeers = ref<SyncedPeer[]>([])
const syncing = ref(false)

async function load() {
  loading.value = true
  try {
    sources.value = await sourcesApi.list()
    accounts.value = await accountsApi.list()
  } catch (e) {
    message.error('加载失败：' + errText(e))
  } finally {
    loading.value = false
  }
}

async function doSync() {
  if (!syncAccountId.value) {
    message.warning('请选择账号')
    return
  }
  syncing.value = true
  try {
    syncedPeers.value = await sourcesApi.sync(syncAccountId.value)
    message.success(`同步到 ${syncedPeers.value.length} 个可选 peer`)
  } catch (e) {
    message.error('同步失败：' + errText(e))
  } finally {
    syncing.value = false
  }
}

async function addPeer(peer: SyncedPeer) {
  if (!syncAccountId.value) return
  try {
    await sourcesApi.create({
      account_id: syncAccountId.value,
      peer_type: peer.peer_type,
      peer_id: peer.peer_id,
      name: peer.name,
      username: peer.username ?? '',
      enabled: true,
    })
    message.success('已添加为监听源')
    await load()
  } catch (e) {
    message.error('添加失败：' + errText(e))
  }
}

async function start(row: Source) {
  try {
    await sourcesApi.start(row.id)
    message.success('已启动监听')
  } catch (e) {
    message.error('启动失败：' + errText(e))
  }
}

async function stop(row: Source) {
  try {
    await sourcesApi.stop(row.id)
    message.success('已停止监听')
  } catch (e) {
    message.error('停止失败：' + errText(e))
  }
}

function confirmDelete(row: Source) {
  dialog.warning({
    title: '删除监听源',
    content: `确定删除「${row.name}」？`,
    positiveText: '删除',
    negativeText: '取消',
    onPositiveClick: async () => {
      try {
        await sourcesApi.remove(row.id)
        message.success('已删除')
        await load()
      } catch (e) {
        message.error('删除失败：' + errText(e))
      }
    },
  })
}

const columns: DataTableColumns<Source> = [
  { title: 'ID', key: 'id', width: 60 },
  { title: '名称', key: 'name' },
  { title: '类型', key: 'peer_type', render: (r) => h(NTag, { size: 'small' }, { default: () => r.peer_type }) },
  { title: 'Peer ID', key: 'peer_id' },
  { title: '账号', key: 'account_id' },
  { title: '启用', key: 'enabled', render: (r) => (r.enabled ? '是' : '否') },
  {
    title: '操作',
    key: 'actions',
    render: (r) =>
      h(NSpace, {}, {
        default: () => [
          h(NButton, { size: 'small', onClick: () => start(r) }, { default: () => '启动' }),
          h(NButton, { size: 'small', onClick: () => stop(r) }, { default: () => '停止' }),
          h(NButton, { size: 'small', type: 'error', onClick: () => confirmDelete(r) }, { default: () => '删除' }),
        ],
      }),
  },
]

const peerColumns: DataTableColumns<SyncedPeer> = [
  { title: '名称', key: 'name' },
  { title: '类型', key: 'peer_type' },
  { title: 'Peer ID', key: 'peer_id' },
  { title: '用户名', key: 'username' },
  {
    title: '操作',
    key: 'actions',
    render: (r) => h(NButton, { size: 'small', type: 'primary', onClick: () => addPeer(r) }, { default: () => '添加' }),
  },
]

onMounted(load)
</script>

<template>
  <n-space vertical size="large">
    <n-card title="从账号同步可监听 peer">
      <n-space>
        <n-select
          v-model:value="syncAccountId"
          style="width: 240px"
          placeholder="选择账号"
          :options="accounts.map((a) => ({ label: `${a.name} (${a.status})`, value: a.id }))"
        />
        <n-button type="primary" :loading="syncing" @click="doSync">同步</n-button>
      </n-space>
      <n-data-table
        v-if="syncedPeers.length"
        style="margin-top: 12px"
        :columns="peerColumns"
        :data="syncedPeers"
        :bordered="false"
        :max-height="260"
      />
    </n-card>

    <n-space justify="space-between">
      <n-text strong>已配置监听源</n-text>
      <n-button @click="load">刷新</n-button>
    </n-space>
    <n-data-table :loading="loading" :columns="columns" :data="sources" :bordered="false" />
  </n-space>
</template>
