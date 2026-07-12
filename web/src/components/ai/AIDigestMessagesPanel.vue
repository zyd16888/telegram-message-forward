<script setup lang="ts">
import { computed } from 'vue'
import type { AIDigestRunItem } from '@/types'
import { formatDateTime } from '@/utils/datetime'

const props = defineProps<{
  items: AIDigestRunItem[]
  timeZone?: string
}>()

const included = computed(() => props.items.filter((item) => item.included))
const excluded = computed(() => props.items.filter((item) => !item.included))

function reasonLabel(reason?: string): string {
  const labels: Record<string, string> = {
    empty_text: '无文本内容',
    condition_not_match: '未命中过滤条件',
    duplicate: '重复消息',
    limit_max_messages: '超过消息数量上限',
    excluded: '已排除',
  }
  if (!reason) return labels.excluded
  if (reason.startsWith('condition_error:')) return `条件执行失败：${reason.slice('condition_error:'.length).trim()}`
  return labels[reason] ?? reason
}

function sourceMeta(item: AIDigestRunItem): string {
  const message = item.message
  const fields = [`#${item.sort_order + 1}`, `Source ${item.source_id}`]
  if (message?.external_message_id) fields.push(`消息 ${message.external_message_id}`)
  if (message?.sender_name) fields.push(message.sender_name)
  fields.push(formatDateTime(message?.sent_at || message?.received_at, { timeZone: props.timeZone }))
  return fields.join(' · ')
}
</script>

<template>
  <div class="detail-grid">
    <section class="message-group">
      <div class="group-heading">
        <strong>纳入原文</strong>
        <NTag size="small" type="success" :bordered="false">{{ included.length }}</NTag>
      </div>
      <NScrollbar class="message-scroll">
        <div v-for="item in included" :key="item.message_id" class="message-item">
          <div class="message-meta">{{ sourceMeta(item) }}</div>
          <div class="message-text">{{ item.message?.text || '[无文本]' }}</div>
          <div v-if="item.message?.media?.length" class="message-media">附件 {{ item.message.media.length }} 个</div>
        </div>
        <NEmpty v-if="!included.length" size="small" description="无纳入消息" />
      </NScrollbar>
    </section>

    <section class="message-group">
      <div class="group-heading">
        <strong>排除原文</strong>
        <NTag size="small" type="warning" :bordered="false">{{ excluded.length }}</NTag>
      </div>
      <NScrollbar class="message-scroll">
        <div v-for="item in excluded" :key="item.message_id" class="message-item excluded">
          <div class="message-meta">{{ sourceMeta(item) }}</div>
          <NTag size="tiny" type="warning" :bordered="false">{{ reasonLabel(item.reason) }}</NTag>
          <div class="message-text">{{ item.message?.text || '[无文本]' }}</div>
          <div v-if="item.message?.media?.length" class="message-media">附件 {{ item.message.media.length }} 个</div>
        </div>
        <NEmpty v-if="!excluded.length" size="small" description="无排除消息" />
      </NScrollbar>
    </section>
  </div>
</template>

<style scoped>
.detail-grid {
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  gap: 14px;
}

.message-group {
  min-width: 0;
  border: 1px solid var(--clay-border);
  border-radius: 8px;
  background: var(--clay-surface-1);
}

.group-heading {
  display: flex;
  align-items: center;
  justify-content: space-between;
  min-height: 44px;
  padding: 0 12px;
  border-bottom: 1px solid var(--clay-border);
}

.message-scroll {
  max-height: 440px;
  padding: 0 12px;
}

.message-item {
  padding: 11px 0;
  border-bottom: 1px solid var(--clay-border);
}

.message-item:last-child {
  border-bottom: 0;
}

.message-meta,
.message-media {
  color: var(--clay-text-3);
  font-size: 12px;
}

.message-text {
  margin-top: 5px;
  color: var(--clay-text);
  white-space: pre-wrap;
  word-break: break-word;
}

.message-media {
  margin-top: 6px;
}

.excluded .message-text {
  color: var(--clay-text-2);
}

.excluded .n-tag {
  margin-top: 6px;
}

@media (max-width: 900px) {
  .detail-grid {
    grid-template-columns: 1fr;
  }
}
</style>
