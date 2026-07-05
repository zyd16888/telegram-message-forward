<script setup lang="ts">
import { computed } from 'vue'
import type { AIDigestRunDetail } from '@/types'

const props = defineProps<{
  detail: AIDigestRunDetail | null
  loading?: boolean
}>()

const items = computed(() => props.detail?.items ?? [])
const included = computed(() => items.value.filter((item) => item.included))
const excluded = computed(() => items.value.filter((item) => !item.included))
const tokenTotal = computed(() => props.detail?.run.token_usage.total_tokens ?? 0)
const statusType = computed(() => {
  const status = props.detail?.run.status
  if (status === 'success') return 'success'
  if (status === 'failed') return 'error'
  if (status === 'running' || status === 'pending') return 'info'
  return 'default'
})
const statusText = computed(() => {
  const status = props.detail?.run.status
  if (status === 'success') return '成功'
  if (status === 'failed') return '失败'
  if (status === 'running') return '运行中'
  if (status === 'pending') return '等待中'
  if (status === 'cancelled') return '已取消'
  return status || '-'
})
const outputContent = computed(() => props.detail?.output?.content?.trim() ?? '')
const rawResponseText = computed(() => {
  const raw = props.detail?.output?.raw_response
  if (!raw) return ''
  if (typeof raw === 'string') return raw
  try {
    return JSON.stringify(raw, null, 2)
  } catch {
    return String(raw)
  }
})
const deliveryTaskText = computed(() => {
  const ids = props.detail?.run.delivery_task_ids ?? []
  return ids.length ? ids.join(', ') : '-'
})

function formatTime(value?: string): string {
  if (!value) return '-'
  return value.replace('T', ' ').replace(/\.\d+(Z)?$/, '$1')
}

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
</script>

<template>
  <NSpin :show="loading">
    <div class="detail-shell">
      <NEmpty v-if="!detail" description="暂无运行详情">
        <template #extra>
          <span class="empty-hint">如果刚点击生成预览，请等待 AI 返回；如果一直为空，通常是请求没有返回 detail 数据。</span>
        </template>
      </NEmpty>

      <NSpace v-else vertical size="large">
        <div class="summary-strip">
          <div class="summary-tile">
            <span class="summary-label">状态</span>
            <NTag :type="statusType" :bordered="false">{{ statusText }}</NTag>
          </div>
          <div class="summary-tile">
            <span class="summary-label">消息</span>
            <strong>{{ detail.run.included_count }}</strong>
            <span class="summary-sub">纳入 / {{ detail.run.input_message_count }} 输入</span>
          </div>
          <div class="summary-tile">
            <span class="summary-label">Token</span>
            <strong>{{ tokenTotal || '-' }}</strong>
            <span class="summary-sub">总消耗</span>
          </div>
          <div class="summary-tile">
            <span class="summary-label">模型</span>
            <strong>{{ detail.run.model_name || '-' }}</strong>
            <span class="summary-sub">{{ detail.run.provider_name || detail.run.provider_id || '未返回 Provider' }}</span>
          </div>
        </div>

        <NAlert v-if="detail.run.error" type="error" title="本次 AI 运行失败">
          {{ detail.run.error }}
        </NAlert>
        <NAlert v-else-if="!outputContent" type="warning" title="本次没有 AI 输出">
          请查看“消息明细”确认窗口内是否有符合条件的消息，或检查 Provider 是否正常返回内容。
        </NAlert>

        <NTabs type="line" animated>
          <NTabPane name="output" tab="AI 输出">
            <NInput
              v-if="outputContent"
              :value="outputContent"
              type="textarea"
              readonly
              :autosize="{ minRows: 10, maxRows: 22 }"
            />
            <NEmpty v-else description="本次运行没有生成可展示的 AI 输出" />
          </NTabPane>

          <NTabPane name="messages" :tab="`消息明细（${items.length}）`">
            <div class="detail-grid">
              <NCard :title="`纳入消息（${included.length}）`" size="small">
                <NScrollbar style="max-height: 360px">
                  <div v-for="item in included" :key="item.message_id" class="message-item">
                    <div class="message-meta">
                      #{{ item.sort_order + 1 }} · Source {{ item.source_id }} · {{ formatTime(item.message?.received_at) }}
                    </div>
                    <div class="message-text">{{ item.message?.text || '[无文本]' }}</div>
                  </div>
                  <NEmpty v-if="!included.length" size="small" description="无纳入消息" />
                </NScrollbar>
              </NCard>

              <NCard :title="`排除消息（${excluded.length}）`" size="small">
                <NScrollbar style="max-height: 360px">
                  <div v-for="item in excluded" :key="item.message_id" class="message-item excluded">
                    <div class="message-meta">#{{ item.sort_order + 1 }} · {{ reasonLabel(item.reason) }}</div>
                    <div class="message-text">{{ item.message?.text || '[无文本]' }}</div>
                  </div>
                  <NEmpty v-if="!excluded.length" size="small" description="无排除消息" />
                </NScrollbar>
              </NCard>
            </div>
          </NTabPane>

          <NTabPane name="diagnostics" tab="运行诊断">
            <NDescriptions bordered :column="2" size="small">
              <NDescriptionsItem label="Run ID">{{ detail.run.id }}</NDescriptionsItem>
              <NDescriptionsItem label="触发方式">{{ detail.run.trigger_type }}</NDescriptionsItem>
              <NDescriptionsItem label="窗口">
                {{ formatTime(detail.run.window_start) }} - {{ formatTime(detail.run.window_end) }}
              </NDescriptionsItem>
              <NDescriptionsItem label="输入/纳入/排除">
                {{ detail.run.input_message_count }} / {{ detail.run.included_count }} / {{ detail.run.excluded_count }}
              </NDescriptionsItem>
              <NDescriptionsItem label="Prompt Tokens">{{ detail.run.token_usage.prompt_tokens ?? '-' }}</NDescriptionsItem>
              <NDescriptionsItem label="Completion Tokens">{{ detail.run.token_usage.completion_tokens ?? '-' }}</NDescriptionsItem>
              <NDescriptionsItem label="投递任务">{{ deliveryTaskText }}</NDescriptionsItem>
              <NDescriptionsItem label="完成时间">{{ formatTime(detail.run.finished_at) }}</NDescriptionsItem>
            </NDescriptions>
            <NCard v-if="rawResponseText" title="原始响应" size="small" class="raw-card">
              <NCode :code="rawResponseText" language="json" word-wrap />
            </NCard>
          </NTabPane>
        </NTabs>
      </NSpace>
    </div>
  </NSpin>
</template>

<style scoped>
.detail-shell {
  min-height: 220px;
}

.empty-hint {
  color: var(--clay-text-3);
  font-size: 12px;
}

.summary-strip {
  display: grid;
  grid-template-columns: repeat(4, minmax(0, 1fr));
  gap: 10px;
}

.summary-tile {
  min-height: 76px;
  border: 1px solid var(--clay-border);
  border-radius: 8px;
  background: var(--clay-surface-2);
  padding: 11px 12px;
}

.summary-label,
.summary-sub {
  display: block;
  color: var(--clay-text-3);
  font-size: 12px;
}

.summary-tile strong {
  display: block;
  margin-top: 6px;
  color: var(--clay-text);
  font-size: 18px;
  line-height: 1.25;
  word-break: break-word;
}

.summary-sub {
  margin-top: 3px;
}

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

.raw-card {
  margin-top: 14px;
}

@media (max-width: 900px) {
  .summary-strip,
  .detail-grid {
    grid-template-columns: 1fr;
  }
}
</style>
