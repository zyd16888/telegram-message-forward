<script setup lang="ts">
import { h, onMounted, shallowRef } from 'vue'
import { NButton, NSpace, NText, useDialog, useMessage, type DataTableColumns } from 'naive-ui'
import FilterEditorModal from '@/components/filters/FilterEditorModal.vue'
import PageHeader from '@/components/PageHeader.vue'
import ClayIcon from '@/components/ClayIcon.vue'
import { filtersApi, rulesApi } from '@/api/client'
import type { Filter, RuleItemDescriptor } from '@/types'
import { errText } from '@/utils/error'

const message = useMessage()
const dialog = useDialog()

const filters = shallowRef<Filter[]>([])
const conditionDescriptors = shallowRef<RuleItemDescriptor[]>([])
const loading = shallowRef(false)
const showModal = shallowRef(false)
const editing = shallowRef<Filter | null>(null)

async function load(): Promise<void> {
  loading.value = true
  try {
    const [fs, meta] = await Promise.all([filtersApi.list(), rulesApi.meta()])
    filters.value = fs
    conditionDescriptors.value = meta.conditions
  } catch (e) {
    message.error('加载失败：' + errText(e))
  } finally {
    loading.value = false
  }
}

function openCreate(): void {
  editing.value = null
  showModal.value = true
}

function openEdit(row: Filter): void {
  editing.value = row
  showModal.value = true
}

function confirmDelete(row: Filter): void {
  dialog.warning({
    title: '删除过滤器',
    content: `确定删除过滤器「${row.name}」？被转发规则或 AI 整理引用时无法删除。`,
    positiveText: '删除',
    negativeText: '取消',
    onPositiveClick: async () => {
      try {
        await filtersApi.remove(row.id)
        message.success('已删除')
        await load()
      } catch (e) {
        message.error('删除失败：' + errText(e))
      }
    },
  })
}

function conditionSummary(row: Filter): string {
  if (!row.conditions.length) return '无条件'
  return row.conditions
    .map((c) => conditionDescriptors.value.find((d) => d.type === c.type)?.label ?? c.type)
    .join('、')
}

const columns: DataTableColumns<Filter> = [
  { title: 'ID', key: 'id', width: 70 },
  { title: '名称', key: 'name', ellipsis: { tooltip: true } },
  { title: '用途', key: 'description', ellipsis: { tooltip: true }, render: (row) => row.description || '—' },
  {
    title: '匹配条件',
    key: 'conditions',
    ellipsis: { tooltip: true },
    render: (row) => h(NText, { depth: row.conditions.length ? 1 : 3 }, { default: () => conditionSummary(row) }),
  },
  {
    title: '操作',
    key: 'actions',
    width: 170,
    render: (row) =>
      h(NSpace, {}, {
        default: () => [
          h(NButton, { size: 'small', secondary: true, onClick: () => openEdit(row) }, { default: () => '编辑' }),
          h(NButton, { size: 'small', type: 'error', quaternary: true, onClick: () => confirmDelete(row) }, { default: () => '删除' }),
        ],
      }),
  },
]

onMounted(load)
</script>

<template>
  <NSpace vertical size="large">
    <PageHeader
      title="过滤器"
      desc="可复用的一组匹配条件，供转发规则与 AI 整理共同引用；修改会联动所有引用它的地方"
      icon="shield"
    >
      <template #actions>
        <NButton type="primary" @click="openCreate">
          <template #icon><ClayIcon name="plus" :size="16" /></template>
          新建过滤器
        </NButton>
        <NButton secondary @click="load">
          <template #icon><ClayIcon name="refresh" :size="16" /></template>
          刷新
        </NButton>
      </template>
    </PageHeader>

    <NDataTable
      :loading="loading"
      :columns="columns"
      :data="filters"
      :bordered="false"
      :scroll-x="760"
      :row-key="(row: Filter) => row.id"
    >
      <template #empty>
        <div class="empty">
          <p>还没有共享过滤器。配好一个后，可在转发规则和 AI 整理中直接引用。</p>
          <NButton type="primary" size="small" @click="openCreate">新建过滤器</NButton>
        </div>
      </template>
    </NDataTable>

    <FilterEditorModal
      v-model:show="showModal"
      :filter="editing"
      :condition-descriptors="conditionDescriptors"
      @saved="load"
    />
  </NSpace>
</template>

<style scoped>
.empty {
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: 10px;
  padding: 28px 0;
  color: var(--clay-text-3);
}
</style>
