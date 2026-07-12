<script setup lang="ts">
import { computed } from 'vue'
import type { SelectOption } from 'naive-ui'
import type { Source } from '@/types'
import type { MessageFilters } from '@/composables/useMessages'

const props = defineProps<{ sources: Source[] }>()
const emit = defineEmits<{ search: []; reset: [] }>()
const filters = defineModel<MessageFilters>('filters', { required: true })

const sourceOptions = computed<SelectOption[]>(() => props.sources.map((item) => ({ label: item.name, value: item.id })))
const sourceTypes = [
  { label: '全部来源类型', value: '' }, { label: 'Telegram', value: 'telegram' },
  { label: 'RSS', value: 'rss' }, { label: 'Webhook', value: 'webhook' },
]
const messageTypes = [
  { label: '全部消息类型', value: '' }, { label: '文本', value: 'text' }, { label: '图片', value: 'photo' },
  { label: '视频', value: 'video' }, { label: '文件', value: 'document' }, { label: '音频', value: 'audio' },
]
const deliveryStatuses = [
  { label: '全部投递状态', value: '' }, { label: '未产生投递', value: 'none' },
  { label: '成功', value: 'success' }, { label: '处理中', value: 'processing' }, { label: '待处理', value: 'pending' },
  { label: '重试中', value: 'retrying' }, { label: '失败', value: 'failed' }, { label: 'Dead', value: 'dead' },
]
const mediaOptions = [
  { label: '全部内容', value: '' }, { label: '包含媒体', value: 'true' }, { label: '纯文本', value: 'false' },
]
const timeOptions = [
  { label: '近 1 小时', value: 1 }, { label: '近 24 小时', value: 24 },
  { label: '近 7 天', value: 168 }, { label: '全部时间', value: 0 },
]
</script>

<template>
  <section class="filters" aria-label="消息筛选">
    <NInput v-model:value="filters.keyword" clearable class="search" placeholder="搜索消息正文或发送人" @keyup.enter="emit('search')" />
    <NSelect v-model:value="filters.sourceId" clearable filterable :options="sourceOptions" placeholder="全部来源" />
    <NSelect v-model:value="filters.sourceType" :options="sourceTypes" />
    <NSelect v-model:value="filters.messageType" :options="messageTypes" />
    <NSelect v-model:value="filters.deliveryStatus" :options="deliveryStatuses" />
    <NSelect v-model:value="filters.media" :options="mediaOptions" />
    <NSelect v-model:value="filters.hours" :options="timeOptions" />
    <div class="actions">
      <NButton type="primary" @click="emit('search')">查询</NButton>
      <NButton secondary @click="emit('reset')">重置</NButton>
    </div>
  </section>
</template>

<style scoped>
.filters {
  display: grid;
  grid-template-columns: minmax(240px, 1.5fr) repeat(6, minmax(128px, 0.8fr)) auto;
  gap: 10px;
  align-items: center;
  padding: 14px;
  border: 1px solid var(--clay-border);
  border-radius: 8px;
  background: var(--clay-surface);
}
.actions { display: flex; gap: 8px; }
@media (max-width: 1180px) {
  .filters { grid-template-columns: repeat(3, minmax(0, 1fr)); }
  .search { grid-column: span 2; }
}
@media (max-width: 680px) {
  .filters { grid-template-columns: 1fr; }
  .search { grid-column: auto; }
  .actions :deep(.n-button) { flex: 1; }
}
</style>
