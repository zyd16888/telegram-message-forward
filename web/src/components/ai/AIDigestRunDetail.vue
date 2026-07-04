<script setup lang="ts">
import { computed } from 'vue'
import type { AIDigestRunDetail } from '@/types'

const props = defineProps<{
  detail: AIDigestRunDetail | null
}>()

const included = computed(() => props.detail?.items.filter((item) => item.included) ?? [])
const excluded = computed(() => props.detail?.items.filter((item) => !item.included) ?? [])
const tokenTotal = computed(() => props.detail?.run.token_usage.total_tokens ?? 0)
</script>

<template>
  <NEmpty v-if="!detail" description="暂无运行详情" />
  <NSpace v-else vertical size="large">
    <NDescriptions bordered :column="2" size="small">
      <NDescriptionsItem label="状态">{{ detail.run.status }}</NDescriptionsItem>
      <NDescriptionsItem label="触发">{{ detail.run.trigger_type }}</NDescriptionsItem>
      <NDescriptionsItem label="窗口">{{ detail.run.window_start }} - {{ detail.run.window_end }}</NDescriptionsItem>
      <NDescriptionsItem label="模型">{{ detail.run.model_name || '-' }}</NDescriptionsItem>
      <NDescriptionsItem label="输入/纳入/排除">
        {{ detail.run.input_message_count }} / {{ detail.run.included_count }} / {{ detail.run.excluded_count }}
      </NDescriptionsItem>
      <NDescriptionsItem label="Token">{{ tokenTotal || '-' }}</NDescriptionsItem>
      <NDescriptionsItem label="投递任务">
        {{ detail.run.delivery_task_ids.length ? detail.run.delivery_task_ids.join(', ') : '-' }}
      </NDescriptionsItem>
      <NDescriptionsItem label="错误">{{ detail.run.error || '-' }}</NDescriptionsItem>
    </NDescriptions>

    <NCard title="AI 输出" size="small">
      <NInput
        :value="detail.output?.content ?? ''"
        type="textarea"
        readonly
        :autosize="{ minRows: 8, maxRows: 18 }"
        placeholder="本次运行没有输出"
      />
    </NCard>

    <div class="detail-grid">
      <NCard :title="`纳入消息（${included.length}）`" size="small">
        <NScrollbar style="max-height: 360px">
          <div v-for="item in included" :key="item.message_id" class="message-item">
            <div class="message-meta">#{{ item.sort_order + 1 }} · Source {{ item.source_id }} · {{ item.message?.received_at }}</div>
            <div class="message-text">{{ item.message?.text || '[无文本]' }}</div>
          </div>
          <NEmpty v-if="!included.length" size="small" description="无纳入消息" />
        </NScrollbar>
      </NCard>

      <NCard :title="`排除消息（${excluded.length}）`" size="small">
        <NScrollbar style="max-height: 360px">
          <div v-for="item in excluded" :key="item.message_id" class="message-item excluded">
            <div class="message-meta">#{{ item.sort_order + 1 }} · {{ item.reason || 'excluded' }}</div>
            <div class="message-text">{{ item.message?.text || '[无文本]' }}</div>
          </div>
          <NEmpty v-if="!excluded.length" size="small" description="无排除消息" />
        </NScrollbar>
      </NCard>
    </div>
  </NSpace>
</template>

<style scoped>
.detail-grid {
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  gap: 14px;
}

.message-item {
  padding: 10px 0;
  border-bottom: 1px solid var(--clay-border);
}

.message-item:last-child {
  border-bottom: 0;
}

.message-meta {
  color: var(--clay-text-3);
  font-size: 12px;
}

.message-text {
  margin-top: 5px;
  color: var(--clay-text);
  white-space: pre-wrap;
  word-break: break-word;
}

.excluded .message-text {
  color: var(--clay-text-2);
}

@media (max-width: 900px) {
  .detail-grid {
    grid-template-columns: 1fr;
  }
}
</style>
