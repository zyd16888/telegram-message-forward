<script setup lang="ts">
import { computed, h, shallowRef } from 'vue'
import { NButton, NCheckbox, NSpace, NTag, useMessage, type DataTableColumns, type DataTableRowKey } from 'naive-ui'
import { sourcesApi } from '@/api/client'
import type { Account, Source, SyncedPeer } from '@/types'
import { errText } from '@/utils/error'

const props = defineProps<{
  accounts: Account[]
  sources: Source[]
}>()

const emit = defineEmits<{
  added: []
}>()

const message = useMessage()

const syncAccountId = shallowRef<number | null>(null)
const syncedPeers = shallowRef<SyncedPeer[]>([])
const syncing = shallowRef(false)
const forceSync = shallowRef(false)
const query = shallowRef('')
const peerKindFilter = shallowRef<string | null>(null)
const addedFilter = shallowRef<'all' | 'new' | 'added'>('new')
const checkedRowKeys = shallowRef<DataTableRowKey[]>([])
const localAddedKeys = shallowRef<Set<string>>(new Set())
const syncController = shallowRef<AbortController | null>(null)

const accountOptions = computed(() =>
  props.accounts.map((account) => ({
    label: `${account.name} (${account.status})`,
    value: account.id,
    disabled: account.status !== 'active',
  })),
)

const existingKeys = computed(() => {
  const keys = new Set<string>()
  for (const source of props.sources) {
    keys.add(peerKey(source.account_id, source.peer_type, source.peer_id))
  }
  for (const key of localAddedKeys.value) {
    keys.add(key)
  }
  return keys
})

const peerKindOptions = computed(() => {
  const values = new Map<string, string>()
  for (const peer of syncedPeers.value) {
    values.set(peer.peer_kind || peer.peer_type, peer.display_type || peer.peer_kind || peer.peer_type)
  }
  return [...values.entries()].map(([value, label]) => ({ value, label }))
})

const visiblePeers = computed(() => {
  const keyword = query.value.trim().toLowerCase()
  return syncedPeers.value.filter((peer) => {
    if (peerKindFilter.value && (peer.peer_kind || peer.peer_type) !== peerKindFilter.value) return false
    const key = peerRowKey(peer)
    const added = existingKeys.value.has(key)
    if (addedFilter.value === 'new' && added) return false
    if (addedFilter.value === 'added' && !added) return false
    if (!keyword) return true
    return [peer.name, peer.username, String(peer.peer_id), peer.display_type, peer.peer_kind]
      .filter(Boolean)
      .some((item) => String(item).toLowerCase().includes(keyword))
  })
})

const selectedAddablePeers = computed(() => {
  const selected = new Set(checkedRowKeys.value.map(String))
  return visiblePeers.value.filter((peer) => selected.has(peerRowKey(peer)) && !existingKeys.value.has(peerRowKey(peer)))
})

const cachedCount = computed(() => syncedPeers.value.filter((peer) => peer.cached).length)

function peerKey(accountId: number, peerType: string, peerId: number): string {
  return `${accountId}:${peerType}:${peerId}`
}

function peerRowKey(peer: SyncedPeer): string {
  return peerKey(syncAccountId.value ?? 0, peer.peer_type, peer.peer_id)
}

function isAdded(peer: SyncedPeer): boolean {
  return existingKeys.value.has(peerRowKey(peer))
}

function mergePeers(existing: SyncedPeer[], incoming: SyncedPeer[]): SyncedPeer[] {
  const byKey = new Map<string, SyncedPeer>()
  const order: string[] = []
  for (const peer of existing) {
    const key = peerRowKey(peer)
    byKey.set(key, peer)
    order.push(key)
  }
  for (const peer of incoming) {
    const key = peerRowKey(peer)
    if (!byKey.has(key)) {
      order.push(key)
    }
    byKey.set(key, peer)
  }
  return order.map((key) => byKey.get(key)).filter(Boolean) as SyncedPeer[]
}

async function doSync() {
  if (!syncAccountId.value) {
    message.warning('请选择已登录账号')
    return
  }
  syncController.value?.abort()
  const controller = new AbortController()
  syncController.value = controller
  syncedPeers.value = []
  checkedRowKeys.value = []
  syncing.value = true
  try {
    const result = await sourcesApi.syncStream(
      syncAccountId.value,
      (peers) => {
        syncedPeers.value = mergePeers(syncedPeers.value, peers)
      },
      controller.signal,
      forceSync.value,
    )
    if (result?.usedCache) {
      message.info(`距上次全量同步不足 15 分钟，已使用缓存数据（共 ${syncedPeers.value.length} 个）；如需最新数据请勾选「强制刷新」`)
    } else {
      message.success(`同步完成，共 ${syncedPeers.value.length} 个可监听 peer`)
    }
  } catch (e) {
    if (controller.signal.aborted) {
      message.warning('已停止同步')
    } else {
      message.error('同步失败：' + errText(e))
    }
  } finally {
    if (syncController.value === controller) {
      syncController.value = null
      syncing.value = false
    }
  }
}

function stopSync() {
  syncController.value?.abort()
}

async function addPeer(peer: SyncedPeer) {
  if (!syncAccountId.value || isAdded(peer)) return
  try {
    await sourcesApi.create({
      account_id: syncAccountId.value,
      peer_type: peer.peer_type,
      peer_id: peer.peer_id,
      name: peer.name,
      username: peer.username ?? '',
      enabled: true,
      config: {
        peer_kind: peer.peer_kind,
        display_type: peer.display_type,
        flags: peer.flags ?? [],
      },
    })
    localAddedKeys.value = new Set([...localAddedKeys.value, peerRowKey(peer)])
    message.success(`已添加「${peer.name}」`)
    emit('added')
  } catch (e) {
    message.error('添加失败：' + errText(e))
  }
}

async function addSelected() {
  const peers = selectedAddablePeers.value
  if (!peers.length) {
    message.warning('没有可添加的已选 peer')
    return
  }
  for (const peer of peers) {
    await addPeer(peer)
  }
  checkedRowKeys.value = []
}

const columns: DataTableColumns<SyncedPeer> = [
  { type: 'selection' },
  { title: '名称', key: 'name', ellipsis: { tooltip: true } },
  {
    title: '类型',
    key: 'display_type',
    width: 130,
    render: (row) =>
      h(NTag, { size: 'small', type: row.is_bot ? 'warning' : row.is_channel ? 'info' : 'default' }, {
        default: () => row.display_type || row.peer_kind || row.peer_type,
      }),
  },
  { title: 'Peer ID', key: 'peer_id', width: 130 },
  { title: '用户名', key: 'username', width: 150, ellipsis: { tooltip: true }, render: (row) => row.username ? `@${row.username}` : '-' },
  {
    title: '状态',
    key: 'status',
    width: 90,
    render: (row) => {
      if (isAdded(row)) return h(NTag, { size: 'small', type: 'success' }, { default: () => '已添加' })
      if (row.cached) return h(NTag, { size: 'small', type: 'warning' }, { default: () => '缓存' })
      return h(NTag, { size: 'small' }, { default: () => '未添加' })
    },
  },
  {
    title: '操作',
    key: 'actions',
    width: 90,
    render: (row) =>
      h(NButton, { size: 'small', type: 'primary', disabled: isAdded(row), onClick: () => addPeer(row) }, { default: () => '添加' }),
  },
]
</script>

<template>
  <NCard title="从账号同步可监听 peer">
    <NSpace vertical size="medium">
      <NSpace align="center">
        <NSelect
          v-model:value="syncAccountId"
          class="account-select"
          placeholder="选择已登录账号"
          :options="accountOptions"
        />
        <NButton type="primary" :loading="syncing" @click="doSync">同步</NButton>
        <NButton v-if="syncing" @click="stopSync">停止</NButton>
        <NCheckbox v-model:checked="forceSync" :disabled="syncing">
          强制刷新（忽略 15 分钟缓存冷却，重新拉取全量会话列表）
        </NCheckbox>
        <NText v-if="syncing || syncedPeers.length" depth="3">
          已加载 {{ syncedPeers.length }} 个，当前显示 {{ visiblePeers.length }} 个
          <template v-if="cachedCount">，缓存 {{ cachedCount }} 个</template>
          <template v-if="syncing">，刷新中</template>
        </NText>
      </NSpace>

      <NSpace v-if="syncedPeers.length" align="center">
        <NInput v-model:value="query" clearable class="peer-search" placeholder="搜索名称、用户名、Peer ID" />
        <NSelect
          v-model:value="peerKindFilter"
          clearable
          class="kind-select"
          placeholder="全部类型"
          :options="peerKindOptions"
        />
        <NSelect
          v-model:value="addedFilter"
          class="state-select"
          :options="[
            { label: '仅未添加', value: 'new' },
            { label: '全部', value: 'all' },
            { label: '仅已添加', value: 'added' },
          ]"
        />
        <NButton :disabled="!selectedAddablePeers.length" @click="addSelected">
          批量添加 {{ selectedAddablePeers.length || '' }}
        </NButton>
      </NSpace>

      <NDataTable
        v-if="syncedPeers.length"
        v-model:checked-row-keys="checkedRowKeys"
        :columns="columns"
        :data="visiblePeers"
        :bordered="false"
        :row-key="peerRowKey"
        :max-height="360"
      />
    </NSpace>
  </NCard>
</template>

<style scoped>
.account-select {
  width: 260px;
}

.peer-search {
  width: 280px;
}

.kind-select {
  width: 170px;
}

.state-select {
  width: 130px;
}
</style>
