<script setup lang="ts">
import { computed } from 'vue'
import { useMessage } from 'naive-ui'
import type { AIDigestRunDetail } from '@/types'
import ClayIcon from '@/components/ClayIcon.vue'
import AIDigestMessagesPanel from '@/components/ai/AIDigestMessagesPanel.vue'
import AIDigestRequestPanel from '@/components/ai/AIDigestRequestPanel.vue'
import AIDigestMediaAuditPanel from '@/components/ai/AIDigestMediaAuditPanel.vue'
import { formatDateTime, formatDuration } from '@/utils/datetime'

const props = defineProps<{
  detail: AIDigestRunDetail | null
  loading?: boolean
  timeZone?: string
}>()
const emit = defineEmits<{
  cloneProfile: [runId: number]
}>()

const message = useMessage()

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

async function copyOutput(): Promise<void> {
  if (!outputContent.value) return
  try {
    await navigator.clipboard.writeText(outputContent.value)
    message.success('AI 输出已复制')
  } catch {
    message.error('复制失败')
  }
}

function deliveryStatusLabel(status: string): string {
  return {
    pending: '等待中', processing: '投递中', success: '成功', failed: '失败',
    retrying: '重试中', dead: '已终止', cancelled: '已取消',
  }[status] ?? status
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
            <strong>{{ detail.run.prompt_message_count }}</strong>
            <span class="summary-sub">提交 AI / {{ detail.run.included_count }} 过滤纳入</span>
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
          {{ detail.run.error_readable || detail.run.error }}
        </NAlert>
        <NAlert v-else-if="detail.run.prompt_omitted_count" type="warning" title="部分消息未进入 Prompt">
          因 Prompt 字符上限省略 {{ detail.run.prompt_omitted_count }} 条；本次实际提交 {{ detail.run.prompt_message_count }} 条。
        </NAlert>
        <NAlert v-else-if="!outputContent" type="warning" title="本次没有 AI 输出">
          请查看“消息明细”确认窗口内是否有符合条件的消息，或检查 Provider 是否正常返回内容。
        </NAlert>

        <NTabs type="line" animated>
          <NTabPane name="output" tab="AI 输出">
            <div v-if="outputContent || detail.run.id" class="output-toolbar">
              <NButton v-if="outputContent" size="small" secondary @click="copyOutput">
                <template #icon><ClayIcon name="copy" :size="15" /></template>
                复制输出
              </NButton>
              <NButton size="small" secondary @click="emit('cloneProfile', detail.run.id)">
                从本次运行复制 Profile
              </NButton>
            </div>
            <NInput
              v-if="outputContent"
              :value="outputContent"
              type="textarea"
              readonly
              :autosize="{ minRows: 10, maxRows: 22 }"
            />
            <NEmpty v-else description="本次运行没有生成可展示的 AI 输出" />
          </NTabPane>

          <NTabPane name="request" tab="AI 请求">
            <AIDigestRequestPanel :request="detail.request" />
          </NTabPane>

          <NTabPane name="media" :tab="`图片请求（${detail.run.media_audit?.length ?? 0}）`">
            <AIDigestMediaAuditPanel :items="detail.run.media_audit" />
          </NTabPane>

          <NTabPane name="messages" :tab="`原文与过滤（${detail.items.length}）`">
            <AIDigestMessagesPanel :items="detail.items" :time-zone="timeZone" />
          </NTabPane>

          <NTabPane name="diagnostics" tab="运行诊断">
            <NDescriptions bordered :column="2" size="small">
              <NDescriptionsItem label="Run ID">{{ detail.run.id }}</NDescriptionsItem>
              <NDescriptionsItem label="触发方式">{{ detail.run.trigger_type }}</NDescriptionsItem>
              <NDescriptionsItem label="窗口">
                {{ formatDateTime(detail.run.window_start, { timeZone }) }} - {{ formatDateTime(detail.run.window_end, { timeZone }) }}
              </NDescriptionsItem>
              <NDescriptionsItem label="输入/纳入/排除">
                {{ detail.run.input_message_count }} / {{ detail.run.included_count }} / {{ detail.run.excluded_count }}
              </NDescriptionsItem>
              <NDescriptionsItem label="提交/省略">
                {{ detail.run.prompt_message_count }} / {{ detail.run.prompt_omitted_count }}
              </NDescriptionsItem>
              <NDescriptionsItem label="Prompt 字符">{{ detail.run.prompt_chars || '-' }}</NDescriptionsItem>
              <NDescriptionsItem label="Prompt Tokens">{{ detail.run.token_usage.prompt_tokens ?? '-' }}</NDescriptionsItem>
              <NDescriptionsItem label="Completion Tokens">{{ detail.run.token_usage.completion_tokens ?? '-' }}</NDescriptionsItem>
              <NDescriptionsItem label="投递任务">{{ deliveryTaskText }}</NDescriptionsItem>
              <NDescriptionsItem label="开始时间">{{ formatDateTime(detail.run.started_at, { timeZone }) }}</NDescriptionsItem>
              <NDescriptionsItem label="完成时间">{{ formatDateTime(detail.run.finished_at, { timeZone }) }}</NDescriptionsItem>
              <NDescriptionsItem label="执行耗时">{{ formatDuration(detail.run.started_at, detail.run.finished_at) }}</NDescriptionsItem>
              <NDescriptionsItem label="展示时区">{{ timeZone || '浏览器时区' }}</NDescriptionsItem>
            </NDescriptions>
            <NCard v-if="detail.run.delivery_tasks?.length" title="投递状态" size="small" class="raw-card">
              <div class="delivery-list">
                <div v-for="task in detail.run.delivery_tasks" :key="task.id" class="delivery-row">
                  <span>#{{ task.id }} · Sink {{ task.sink_id }}</span>
                  <NTag size="small" :type="task.status === 'success' ? 'success' : task.status === 'dead' || task.status === 'failed' ? 'error' : 'warning'">
                    {{ deliveryStatusLabel(task.status) }} · {{ task.attempt_count }} 次
                  </NTag>
                  <small v-if="task.last_error_readable || task.last_error" :title="task.last_error">
                    {{ task.last_error_readable || task.last_error }}
                  </small>
                </div>
              </div>
            </NCard>
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

.raw-card {
  margin-top: 14px;
}

.output-toolbar {
  display: flex;
  justify-content: flex-end;
  margin-bottom: 8px;
}

.delivery-list {
  display: grid;
  gap: 8px;
}

.delivery-row {
  display: grid;
  grid-template-columns: minmax(0, 1fr) max-content;
  gap: 4px 12px;
  align-items: center;
}

.delivery-row small {
  grid-column: 1 / -1;
  color: var(--clay-error);
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

@media (max-width: 900px) {
  .summary-strip {
    grid-template-columns: 1fr;
  }
}
</style>
