<script setup lang="ts">
import { computed, h, onMounted, shallowRef } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { NButton, NSpace, NSwitch, NTag, NText, useDialog, useMessage, type DataTableColumns } from 'naive-ui'
import PeerSyncPanel from '@/components/sources/PeerSyncPanel.vue'
import PageHeader from '@/components/PageHeader.vue'
import ClayIcon from '@/components/ClayIcon.vue'
import { accountsApi, rulesApi, sourcesApi } from '@/api/client'
import type { Account, Rule, Source } from '@/types'
import { errText } from '@/utils/error'

const message = useMessage()
const dialog = useDialog()
const route = useRoute()
const router = useRouter()

const sources = shallowRef<Source[]>([])
const accounts = shallowRef<Account[]>([])
const rules = shallowRef<Rule[]>([])
const loading = shallowRef(false)
const sourceSearch = shallowRef('')
const sourceKindFilter = shallowRef<string | null>(null)

const accountNameById = computed(() => new Map(accounts.value.map((a) => [a.id, a.name])))

const ruleCountBySourceId = computed(() => {
  const counts = new Map<number, number>()
  for (const rule of rules.value) {
    for (const id of rule.source_ids) {
      counts.set(id, (counts.get(id) ?? 0) + 1)
    }
  }
  return counts
})

const visibleSources = computed(() => {
  const keyword = sourceSearch.value.trim().toLowerCase()
  return sources.value.filter((source) => {
    const kind = sourceDisplayType(source)
    if (sourceKindFilter.value && kind !== sourceKindFilter.value) return false
    if (!keyword) return true
    return [String(source.id), source.name, source.username, String(source.peer_id), kind, source.peer_type]
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
    ;[sources.value, accounts.value, rules.value] = await Promise.all([
      sourcesApi.list(),
      accountsApi.list(),
      rulesApi.list(),
    ])
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

function accountName(source: Source): string {
  return accountNameById.value.get(source.account_id) ?? `#${source.account_id}`
}

function runnerStatusLabel(source: Source): string {
  if (!source.enabled) return '未启用'
  if (source.runner_status === 'running') return '运行中'
  return '未运行'
}

function runnerStatusType(source: Source): 'success' | 'warning' | 'default' {
  if (!source.enabled) return 'default'
  return source.runner_status === 'running' ? 'success' : 'warning'
}

function formatRuntimeTime(value?: string): string {
  if (!value) return ''
  return new Intl.DateTimeFormat('zh-CN', {
    month: '2-digit',
    day: '2-digit',
    hour: '2-digit',
    minute: '2-digit',
  }).format(new Date(value))
}

async function toggle(row: Source, value: boolean) {
  try {
    await sourcesApi.update(row.id, { enabled: value })
    // sources 是 shallowRef：直接改 row.enabled 不会触发表格重新渲染，
    // 必须替换数组里对应项的引用。
    sources.value = sources.value.map((s) => (s.id === row.id ? { ...s, enabled: value } : s))
  } catch (e) {
    message.error('更新失败：' + errText(e))
  }
}

function goToRules(sourceId: number) {
  router.push({ name: 'rules', query: { source_id: String(sourceId) } })
}

function goToFlow(sourceId: number) {
  router.push({ name: 'flow', query: { source_id: String(sourceId) } })
}

function createRuleForSource(sourceId: number) {
  router.push({ name: 'rules', query: { create: '1', source_id: String(sourceId) } })
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
  { title: '账号', key: 'account_id', width: 140, render: (row) => accountName(row) },
  {
    title: '运行',
    key: 'runner_status',
    width: 180,
    render: (row) =>
      h('div', { class: 'runtime-cell' }, [
        h(NTag, { size: 'small', type: runnerStatusType(row), bordered: false }, { default: () => runnerStatusLabel(row) }),
        h(
          NText,
          { depth: row.runner_last_error ? 1 : 3 },
          {
            default: () =>
              row.runner_last_error ||
              (row.runner_recent_message_at
                ? `最近 ${formatRuntimeTime(row.runner_recent_message_at)}`
                : row.runner_subscriptions
                  ? `${row.runner_subscriptions} 个订阅`
                  : '无运行信息'),
          },
        ),
      ]),
  },
  {
    title: '关联规则',
    key: 'rules',
    width: 110,
    render: (row) => {
      const count = ruleCountBySourceId.value.get(row.id) ?? 0
      if (!count) return h(NText, { depth: 3 }, { default: () => '无' })
      return h(
        NButton,
        { text: true, type: 'primary', onClick: () => goToRules(row.id) },
        { default: () => `${count} 条规则` },
      )
    },
  },
  {
    title: '启用',
    key: 'enabled',
    width: 90,
    render: (row) => h(NSwitch, { value: row.enabled, onUpdateValue: (value: boolean) => toggle(row, value) }),
  },
  {
    title: '操作',
    key: 'actions',
    width: 250,
    render: (row) =>
      h(NSpace, {}, {
        default: () => [
          h(NButton, { size: 'small', onClick: () => goToFlow(row.id) }, { default: () => '编排' }),
          h(NButton, { size: 'small', onClick: () => createRuleForSource(row.id) }, { default: () => '建规则' }),
          h(NButton, { size: 'small', type: 'error', onClick: () => confirmDelete(row) }, { default: () => '删除' }),
        ],
      }),
  },
]

onMounted(() => {
  const raw = route.query.source_id
  const id = Array.isArray(raw) ? raw[0] : raw
  if (id) sourceSearch.value = String(id)
  void load()
})
</script>

<template>
  <NSpace vertical size="large">
    <PageHeader title="监听源" desc="从账号同步会话，管理需要监听的来源" icon="sources" />

    <PeerSyncPanel :accounts="accounts" :sources="sources" @added="load" />

    <div class="list-toolbar">
      <NText strong class="list-title">已配置监听源</NText>
      <div class="list-filters">
        <NInput v-model:value="sourceSearch" clearable class="source-search" placeholder="搜索名称、用户名、Peer ID" />
        <NSelect
          v-model:value="sourceKindFilter"
          clearable
          class="source-kind"
          placeholder="全部类型"
          :options="sourceKindOptions"
        />
        <NButton secondary @click="load">
          <template #icon><ClayIcon name="refresh" :size="16" /></template>
          刷新
        </NButton>
      </div>
    </div>
    <NDataTable :loading="loading" :columns="columns" :data="visibleSources" :bordered="false" :scroll-x="1120" />
  </NSpace>
</template>

<style scoped>
.list-toolbar {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 14px;
  flex-wrap: wrap;
}
.list-title {
  font-size: 15px;
}
.list-filters {
  display: flex;
  align-items: center;
  gap: 10px;
  flex-wrap: wrap;
}
.source-search {
  width: 260px;
}
.source-kind {
  width: 150px;
}

.runtime-cell {
  display: grid;
  gap: 4px;
}

@media (max-width: 640px) {
  .list-filters {
    width: 100%;
  }
  .source-search {
    flex: 1 1 100%;
    width: auto;
  }
  .source-kind {
    flex: 1;
    width: auto;
  }
}
</style>
