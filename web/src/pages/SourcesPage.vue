<script setup lang="ts">
import { computed, h, onMounted, shallowRef } from 'vue'
import { NButton, NSpace, NSwitch, NTag, useDialog, useMessage, type DataTableColumns } from 'naive-ui'
import PeerSyncPanel from '@/components/sources/PeerSyncPanel.vue'
import { accountsApi, sourcesApi } from '@/api/client'
import type { Account, Source } from '@/types'
import { errText } from '@/utils/error'

const message = useMessage()
const dialog = useDialog()

const sources = shallowRef<Source[]>([])
const accounts = shallowRef<Account[]>([])
const loading = shallowRef(false)
const sourceSearch = shallowRef('')
const sourceKindFilter = shallowRef<string | null>(null)

const visibleSources = computed(() => {
  const keyword = sourceSearch.value.trim().toLowerCase()
  return sources.value.filter((source) => {
    const kind = sourceDisplayType(source)
    if (sourceKindFilter.value && kind !== sourceKindFilter.value) return false
    if (!keyword) return true
    return [source.name, source.username, String(source.peer_id), kind, source.peer_type]
      .filter(Boolean)
      .some((item) => String(item).toLowerCase().includes(keyword))
  })
})

const sourceKindOptions = computed(() => {
  const kinds = new Set<string>()
  for (const source of sources.value) {
    kinds.add(sourceDisplayType(source))
  }
  return [...kinds].map((kind) => ({ label: kind, value: kind }))
})

async function load() {
  loading.value = true
  try {
    ;[sources.value, accounts.value] = await Promise.all([sourcesApi.list(), accountsApi.list()])
  } catch (e) {
    message.error('加载失败：' + errText(e))
  } finally {
    loading.value = false
  }
}

function sourceDisplayType(source: Source): string {
  const displayType = source.config?.display_type
  if (typeof displayType === 'string' && displayType) return displayType
  if (source.peer_type === 'user') return '用户'
  if (source.peer_type === 'chat') return '普通群'
  return '频道/超级群'
}

async function toggle(row: Source, value: boolean) {
  try {
    await sourcesApi.update(row.id, { enabled: value })
    row.enabled = value
  } catch (e) {
    message.error('更新失败：' + errText(e))
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
  { title: 'ID', key: 'id', width: 70 },
  { title: '名称', key: 'name', ellipsis: { tooltip: true } },
  {
    title: '类型',
    key: 'peer_type',
    width: 130,
    render: (row) => h(NTag, { size: 'small' }, { default: () => sourceDisplayType(row) }),
  },
  { title: 'Peer ID', key: 'peer_id', width: 140 },
  { title: '账号', key: 'account_id', width: 90 },
  {
    title: '启用',
    key: 'enabled',
    width: 90,
    render: (row) => h(NSwitch, { value: row.enabled, onUpdateValue: (value: boolean) => toggle(row, value) }),
  },
  {
    title: '操作',
    key: 'actions',
    width: 220,
    render: (row) =>
      h(NSpace, {}, {
        default: () => [
          h(NButton, { size: 'small', onClick: () => start(row) }, { default: () => '启动' }),
          h(NButton, { size: 'small', onClick: () => stop(row) }, { default: () => '停止' }),
          h(NButton, { size: 'small', type: 'error', onClick: () => confirmDelete(row) }, { default: () => '删除' }),
        ],
      }),
  },
]

onMounted(load)
</script>

<template>
  <NSpace vertical size="large">
    <PeerSyncPanel :accounts="accounts" :sources="sources" @added="load" />

    <NSpace justify="space-between" align="center">
      <NText strong>已配置监听源</NText>
      <NSpace>
        <NInput v-model:value="sourceSearch" clearable class="source-search" placeholder="搜索名称、用户名、Peer ID" />
        <NSelect
          v-model:value="sourceKindFilter"
          clearable
          class="source-kind"
          placeholder="全部类型"
          :options="sourceKindOptions"
        />
        <NButton @click="load">刷新</NButton>
      </NSpace>
    </NSpace>
    <NDataTable :loading="loading" :columns="columns" :data="visibleSources" :bordered="false" />
  </NSpace>
</template>

<style scoped>
.source-search {
  width: 260px;
}

.source-kind {
  width: 150px;
}
</style>
