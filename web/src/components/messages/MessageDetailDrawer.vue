<script setup lang="ts">
import type { MessageDetail } from '@/types'
import MessageDeliverySummary from './MessageDeliverySummary.vue'

defineProps<{ show: boolean; detail: MessageDetail | null; loading: boolean }>()
const emit = defineEmits<{ 'update:show': [value: boolean]; delivery: [id: number] }>()

const statusType: Record<string, 'success' | 'warning' | 'error' | 'info' | 'default'> = {
  success: 'success', pending: 'info', processing: 'info', retrying: 'warning', failed: 'error', dead: 'error', cancelled: 'default',
}

function formatTime(value?: string): string {
  if (!value) return '-'
  return new Intl.DateTimeFormat('zh-CN', { dateStyle: 'medium', timeStyle: 'medium' }).format(new Date(value))
}

function mediaName(item: Record<string, unknown>, index: number): string {
  return String(item.file_name || item.type || `附件 ${index + 1}`)
}
</script>

<template>
  <NDrawer :show="show" :width="640" placement="right" @update:show="emit('update:show', $event)">
    <NDrawerContent title="消息详情" closable :native-scrollbar="false">
      <NSpin :show="loading">
        <NEmpty v-if="!detail && !loading" description="暂无消息详情" />
        <div v-else-if="detail" class="detail">
          <header class="detail-head">
            <div>
              <strong>{{ detail.message.source_name }}</strong>
              <p>{{ detail.message.sender_name || detail.message.source_username || '未记录发送人' }}</p>
            </div>
            <NTag size="small" :bordered="false">{{ detail.message.message_type }}</NTag>
          </header>

          <dl class="facts">
            <div><dt>接收时间</dt><dd>{{ formatTime(detail.message.received_at) }}</dd></div>
            <div><dt>消息时间</dt><dd>{{ formatTime(detail.message.sent_at) }}</dd></div>
            <div><dt>来源类型</dt><dd>{{ detail.message.source_type }}</dd></div>
            <div><dt>外部消息 ID</dt><dd>{{ detail.message.external_message_id }}</dd></div>
          </dl>

          <section>
            <h3>消息内容</h3>
            <div class="message-body">{{ detail.message.text || `【${detail.message.message_type}消息】` }}</div>
            <a v-if="detail.message.original_url" :href="detail.message.original_url" target="_blank" rel="noopener noreferrer">打开原消息</a>
          </section>

          <section v-if="detail.message.media.length">
            <h3>媒体附件</h3>
            <div class="media-list">
              <div v-for="(item, index) in detail.message.media" :key="index" class="media-item">
                <strong>{{ mediaName(item, index) }}</strong>
                <span>{{ item.mime_type || item.download_status || '已记录' }}</span>
              </div>
            </div>
          </section>

          <section>
            <div class="section-head">
              <h3>投递结果</h3>
              <MessageDeliverySummary :counts="detail.message.deliveries" />
            </div>
            <NEmpty v-if="!detail.deliveries.length" size="small" description="这条消息没有产生投递任务" />
            <button v-for="item in detail.deliveries" :key="item.id" type="button" class="delivery-row" @click="emit('delivery', item.id)">
              <div>
                <strong>{{ item.sink_name }}</strong>
                <span>{{ item.flow_name || item.origin_type }} · {{ item.attempt_count }}/{{ item.max_attempts }} 次</span>
              </div>
              <NTag size="small" :type="statusType[item.status] ?? 'default'">{{ item.status }}</NTag>
              <p v-if="item.last_error">{{ item.last_error }}</p>
            </button>
          </section>
        </div>
      </NSpin>
    </NDrawerContent>
  </NDrawer>
</template>

<style scoped>
.detail { display: grid; gap: 22px; }
.detail-head, .section-head { display: flex; align-items: flex-start; justify-content: space-between; gap: 16px; }
.detail-head strong { font-size: 18px; }
.detail-head p { margin: 4px 0 0; color: var(--clay-text-2); }
.facts { display: grid; grid-template-columns: repeat(2, minmax(0, 1fr)); gap: 10px; margin: 0; }
.facts div { padding: 10px 12px; border: 1px solid var(--clay-border); border-radius: 8px; background: var(--clay-surface-2); }
dt { color: var(--clay-text-2); font-size: 12px; }
dd { margin: 4px 0 0; font-weight: 700; }
h3 { margin: 0 0 10px; font-size: 14px; }
.message-body { max-height: 300px; overflow: auto; padding: 14px; border: 1px solid var(--clay-border); border-radius: 8px; white-space: pre-wrap; word-break: break-word; }
section > a { display: inline-block; margin-top: 10px; color: var(--clay-primary); }
.media-list { display: grid; gap: 8px; }
.media-item, .delivery-row { border: 1px solid var(--clay-border); border-radius: 8px; background: var(--clay-surface); }
.media-item { display: flex; justify-content: space-between; gap: 10px; padding: 10px 12px; }
.media-item span, .delivery-row span { color: var(--clay-text-2); font-size: 12px; }
.delivery-row { width: 100%; margin-top: 8px; padding: 11px 12px; display: grid; grid-template-columns: 1fr auto; gap: 6px 12px; color: var(--clay-text); text-align: left; cursor: pointer; }
.delivery-row:hover { border-color: var(--clay-primary); background: var(--clay-surface-2); }
.delivery-row div { display: grid; gap: 3px; }
.delivery-row p { grid-column: 1 / -1; margin: 0; overflow: hidden; color: var(--clay-error, #d14f47); font-size: 12px; text-overflow: ellipsis; white-space: nowrap; }
@media (max-width: 640px) { .facts { grid-template-columns: 1fr; } }
</style>
