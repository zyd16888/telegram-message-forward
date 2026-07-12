<script setup lang="ts">
import { h } from 'vue'
import { NButton, NTag, NText, type DataTableColumns } from 'naive-ui'
import type { AIDigestOutputTemplate, AIDigestProfile } from '@/types'

const props = defineProps<{ templates: AIDigestOutputTemplate[]; profiles: AIDigestProfile[]; loading?: boolean }>()
const emit = defineEmits<{ create: []; edit: [template: AIDigestOutputTemplate]; remove: [template: AIDigestOutputTemplate] }>()

const columns: DataTableColumns<AIDigestOutputTemplate> = [
  {
    title: '名称', key: 'name',
    render: (row) => h('div', { class: 'template-name' }, [
      h('strong', row.name),
      row.built_in ? h(NTag, { size: 'small', type: 'warning', bordered: false }, { default: () => '内置' }) : null,
    ]),
  },
  { title: '用途', key: 'description', ellipsis: { tooltip: true }, render: (row) => row.description || '—' },
  { title: '格式', key: 'format', width: 110, render: (row) => row.format.toUpperCase() },
  {
    title: '引用', key: 'used', width: 90,
    render: (row) => {
      const count = props.profiles.filter((profile) => profile.output_template_id === row.id).length
      return count ? h(NText, {}, { default: () => `${count} 个` }) : h(NText, { depth: 3 }, { default: () => '未使用' })
    },
  },
  {
    title: '操作', key: 'actions', width: 170,
    render: (row) => h('div', { class: 'template-actions' }, [
      h(NButton, { size: 'small', secondary: true, onClick: () => emit('edit', row) }, { default: () => '编辑' }),
      h(NButton, { size: 'small', type: 'error', quaternary: true, disabled: row.built_in, onClick: () => emit('remove', row) }, { default: () => '删除' }),
    ]),
  },
]
</script>

<template>
  <div class="table-lead">输出模板定义 AI 结果的结构（标题、章节、来源标注等），可被多个 Profile 复用。</div>
  <NDataTable :loading="loading" :columns="columns" :data="templates" :bordered="false" :scroll-x="720" :row-key="(row: AIDigestOutputTemplate) => row.id">
    <template #empty>
      <div class="empty-state">
        <p>还没有输出模板。</p>
        <NButton type="primary" size="small" @click="emit('create')">新建模板</NButton>
      </div>
    </template>
  </NDataTable>
</template>

<style scoped>
.table-lead { margin: 2px 0 14px; color: var(--clay-text-3); font-size: 13px; }
:deep(.template-name), :deep(.template-actions) { display: flex; flex-wrap: wrap; align-items: center; gap: 8px; }
.empty-state { display: flex; flex-direction: column; align-items: center; gap: 10px; padding: 28px 0; color: var(--clay-text-3); }
</style>
