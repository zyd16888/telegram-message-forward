<script setup lang="ts">
import { computed } from 'vue'
import { useMessage } from 'naive-ui'
import ClayIcon from '@/components/ClayIcon.vue'
import type { AIDigestRunDetail } from '@/types'

const props = defineProps<{
  request?: AIDigestRunDetail['request']
}>()

const message = useMessage()
const config = computed(() => props.request?.request_config)

async function copyPrompt(label: string, content: string): Promise<void> {
  try {
    await navigator.clipboard.writeText(content)
    message.success(`${label} 已复制`)
  } catch {
    message.error(`${label} 复制失败`)
  }
}
</script>

<template>
  <NEmpty v-if="!request" description="该运行记录未保存 AI 请求" />
  <NSpace v-else vertical size="large">
    <div class="request-meta">
      <div class="meta-item">
        <span>Provider</span>
        <strong>{{ config?.provider_name || config?.provider_id || '-' }}</strong>
      </div>
      <div class="meta-item">
        <span>接口</span>
        <strong>{{ config?.api_type || '-' }}</strong>
      </div>
      <div class="meta-item">
        <span>模型</span>
        <strong>{{ config?.model || '-' }}</strong>
      </div>
      <div class="meta-item">
        <span>Temperature</span>
        <strong>{{ config?.temperature ?? '-' }}</strong>
      </div>
      <div class="meta-item">
        <span>Max Tokens</span>
        <strong>{{ config?.max_tokens || 'Provider 默认值' }}</strong>
      </div>
    </div>

    <section class="prompt-section">
      <div class="prompt-heading">
        <strong>System Prompt</strong>
        <NTooltip trigger="hover">
          <template #trigger>
            <NButton
              quaternary
              circle
              size="small"
              aria-label="复制 System Prompt"
              @click="copyPrompt('System Prompt', request.system_prompt)"
            >
              <template #icon><ClayIcon name="copy" :size="15" /></template>
            </NButton>
          </template>
          复制 System Prompt
        </NTooltip>
      </div>
      <NInput :value="request.system_prompt" type="textarea" readonly :autosize="{ minRows: 5, maxRows: 12 }" />
    </section>

    <section class="prompt-section">
      <div class="prompt-heading">
        <strong>User Prompt</strong>
        <NTooltip trigger="hover">
          <template #trigger>
            <NButton
              quaternary
              circle
              size="small"
              aria-label="复制 User Prompt"
              @click="copyPrompt('User Prompt', request.user_prompt)"
            >
              <template #icon><ClayIcon name="copy" :size="15" /></template>
            </NButton>
          </template>
          复制 User Prompt
        </NTooltip>
      </div>
      <NInput :value="request.user_prompt" type="textarea" readonly :autosize="{ minRows: 12, maxRows: 28 }" />
    </section>
  </NSpace>
</template>

<style scoped>
.request-meta {
  display: grid;
  grid-template-columns: repeat(5, minmax(0, 1fr));
  overflow: hidden;
  border: 1px solid var(--clay-border);
  border-radius: 8px;
}

.meta-item {
  min-width: 0;
  padding: 10px 12px;
  border-right: 1px solid var(--clay-border);
}

.meta-item:last-child {
  border-right: 0;
}

.meta-item span,
.meta-item strong {
  display: block;
}

.meta-item span {
  color: var(--clay-text-3);
  font-size: 12px;
}

.meta-item strong {
  margin-top: 4px;
  overflow: hidden;
  color: var(--clay-text);
  text-overflow: ellipsis;
  white-space: nowrap;
}

.prompt-section {
  min-width: 0;
}

.prompt-heading {
  display: flex;
  align-items: center;
  justify-content: space-between;
  min-height: 32px;
  margin-bottom: 6px;
  color: var(--clay-text);
}

@media (max-width: 760px) {
  .request-meta {
    grid-template-columns: repeat(2, minmax(0, 1fr));
  }

  .meta-item {
    border-right: 0;
    border-bottom: 1px solid var(--clay-border);
  }

  .meta-item:nth-child(odd) {
    border-right: 1px solid var(--clay-border);
  }

  .meta-item:last-child {
    grid-column: 1 / -1;
    border-right: 0;
    border-bottom: 0;
  }
}
</style>
