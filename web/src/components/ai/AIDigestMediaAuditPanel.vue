<script setup lang="ts">
import { computed } from 'vue'
import type { AIDigestMediaAudit } from '@/types'

const props = defineProps<{
  items?: AIDigestMediaAudit[]
}>()

const rows = computed(() => props.items ?? [])
const included = computed(() => rows.value.filter((item) => item.status === 'included').length)
const skipped = computed(() => rows.value.filter((item) => item.status === 'skipped').length)
const failed = computed(() => rows.value.filter((item) => item.status === 'failed').length)

function statusType(status: string): 'success' | 'warning' | 'error' | 'default' {
  if (status === 'included') return 'success'
  if (status === 'skipped') return 'warning'
  if (status === 'failed') return 'error'
  return 'default'
}

function statusLabel(status: string): string {
  return { included: '已提交', skipped: '已跳过', failed: '读取失败' }[status] ?? status
}

function formatBytes(value = 0): string {
  if (value < 1024) return `${value} B`
  if (value < 1024 * 1024) return `${(value / 1024).toFixed(1)} KB`
  return `${(value / 1024 / 1024).toFixed(1)} MB`
}
</script>

<template>
  <NEmpty v-if="rows.length === 0" description="本次运行没有图片请求记录" />
  <NSpace v-else vertical size="large">
    <div class="audit-summary">
      <div><span>已提交</span><strong>{{ included }}</strong></div>
      <div><span>已跳过</span><strong>{{ skipped }}</strong></div>
      <div><span>读取失败</span><strong>{{ failed }}</strong></div>
    </div>
    <div class="audit-list">
      <div v-for="item in rows" :key="`${item.message_id}-${item.media_index}`" class="audit-row">
        <NTag :type="statusType(item.status)" :bordered="false" size="small">
          {{ statusLabel(item.status) }}
        </NTag>
        <div class="audit-main">
          <strong>{{ item.file_name || `消息 #${item.message_id} 图片 ${item.media_index + 1}` }}</strong>
          <span>
            消息 #{{ item.message_id }}
            <template v-if="item.grouped_id"> · 相册组 {{ item.grouped_id }}</template>
            · {{ item.mime_type || '未知格式' }} · {{ formatBytes(item.size) }}
          </span>
          <span v-if="item.reason" class="audit-reason">{{ item.reason }}</span>
          <code v-if="item.sha256" :title="item.sha256">SHA256 {{ item.sha256.slice(0, 16) }}...</code>
        </div>
      </div>
    </div>
  </NSpace>
</template>

<style scoped>
.audit-summary {
  display: grid;
  grid-template-columns: repeat(3, minmax(0, 1fr));
  border: 1px solid var(--clay-border);
  border-radius: 8px;
  overflow: hidden;
}

.audit-summary div {
  min-width: 0;
  padding: 10px 12px;
  border-right: 1px solid var(--clay-border);
}

.audit-summary div:last-child {
  border-right: 0;
}

.audit-summary span,
.audit-summary strong {
  display: block;
}

.audit-summary span,
.audit-main span,
.audit-main code {
  color: var(--clay-text-3);
  font-size: 12px;
}

.audit-summary strong {
  margin-top: 3px;
  font-size: 18px;
}

.audit-list {
  border: 1px solid var(--clay-border);
  border-radius: 8px;
  overflow: hidden;
}

.audit-row {
  display: grid;
  grid-template-columns: 76px minmax(0, 1fr);
  gap: 12px;
  align-items: start;
  padding: 12px;
  border-bottom: 1px solid var(--clay-border);
}

.audit-row:last-child {
  border-bottom: 0;
}

.audit-main,
.audit-main strong,
.audit-main span,
.audit-main code {
  display: block;
  min-width: 0;
  overflow-wrap: anywhere;
}

.audit-main span,
.audit-main code {
  margin-top: 3px;
}

.audit-main .audit-reason {
  color: var(--clay-warning);
}

@media (max-width: 560px) {
  .audit-row {
    grid-template-columns: 1fr;
  }
}
</style>
