<script setup lang="ts">
import type { MessageItem } from '@/types'
import MessageDeliverySummary from './MessageDeliverySummary.vue'

defineProps<{ items: MessageItem[]; loading: boolean; loadingMore: boolean; hasMore: boolean }>()
const emit = defineEmits<{ select: [id: number]; more: [] }>()

function formatTime(value: string): string {
  return new Intl.DateTimeFormat('zh-CN', { month: '2-digit', day: '2-digit', hour: '2-digit', minute: '2-digit', second: '2-digit' }).format(new Date(value))
}

function preview(item: MessageItem): string {
  return item.text?.trim() || `【${item.message_type || '未知'}消息】`
}
</script>

<template>
  <NSpin :show="loading">
    <div v-if="items.length" class="message-list">
      <button v-for="item in items" :key="item.id" class="message-row" type="button" @click="emit('select', item.id)">
        <div class="source-cell">
          <strong>{{ item.source_name }}</strong>
          <span>{{ item.sender_name || item.source_username || item.source_type }}</span>
        </div>
        <div class="content-cell">
          <div class="message-preview">{{ preview(item) }}</div>
          <div class="message-meta">
            <span>{{ item.message_type }}</span>
            <span v-if="item.media.length">{{ item.media.length }} 个附件</span>
            <span>#{{ item.external_message_id }}</span>
          </div>
        </div>
        <MessageDeliverySummary :counts="item.deliveries" />
        <time>{{ formatTime(item.received_at) }}</time>
      </button>
    </div>
    <NEmpty v-else-if="!loading" description="当前筛选条件下没有消息" />
    <div v-if="hasMore" class="load-more">
      <NButton secondary :loading="loadingMore" @click="emit('more')">加载更多</NButton>
    </div>
  </NSpin>
</template>

<style scoped>
.message-list { border: 1px solid var(--clay-border); border-radius: 8px; overflow: hidden; background: var(--clay-surface); }
.message-row {
  width: 100%; min-height: 86px; padding: 14px 16px; border: 0; border-bottom: 1px solid var(--clay-border);
  display: grid; grid-template-columns: 180px minmax(260px, 1fr) 240px 126px; gap: 18px; align-items: center;
  color: var(--clay-text); background: transparent; text-align: left; cursor: pointer;
}
.message-row:last-child { border-bottom: 0; }
.message-row:hover { background: var(--clay-surface-2); }
.message-row:focus-visible { position: relative; z-index: 1; outline: 2px solid var(--clay-primary); outline-offset: -2px; }
.source-cell, .content-cell { min-width: 0; }
.source-cell { display: grid; gap: 4px; }
.source-cell strong, .source-cell span, .message-preview { overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
.source-cell span, .message-meta, time { color: var(--clay-text-2); font-size: 12px; }
.message-preview { font-size: 14px; line-height: 1.5; }
.message-meta { display: flex; gap: 10px; margin-top: 6px; }
time { text-align: right; font-variant-numeric: tabular-nums; }
.load-more { display: flex; justify-content: center; padding: 16px; }
@media (max-width: 1000px) {
  .message-row { grid-template-columns: 150px minmax(0, 1fr) 120px; }
  .message-row :deep(.summary) { grid-column: 2 / -1; }
}
@media (max-width: 680px) {
  .message-row { grid-template-columns: 1fr auto; gap: 10px; }
  .content-cell, .message-row :deep(.summary) { grid-column: 1 / -1; }
  time { grid-column: 2; grid-row: 1; }
}
</style>
