<script setup lang="ts">
import { computed, onMounted, ref, shallowRef } from 'vue'
import { useMessage } from 'naive-ui'
import { settingsApi } from '@/api/client'
import type { DataCleanupTarget, DataRetentionRequest, DataRetentionSettings } from '@/types'
import { errText } from '@/utils/error'

const message = useMessage()
const form = ref<DataRetentionRequest>({
  messages_retention_days: 90,
  delivery_tasks_retention_days: 90,
  ai_runs_retention_days: 30,
})
const saved = ref<DataRetentionRequest>({ ...form.value })
const source = shallowRef<'database' | 'file'>('file')
const loading = shallowRef(false)
const loaded = shallowRef(false)
const saving = shallowRef(false)
const cleaning = shallowRef(false)
const showCleanup = shallowRef(false)
const cleanupTargets = ref<DataCleanupTarget[]>([])

const isDirty = computed(() => JSON.stringify(form.value) !== JSON.stringify(saved.value))
const canCleanup = computed(() => loaded.value && Object.values(saved.value).some((days) => days > 0))

function assignSettings(settings: DataRetentionSettings): void {
  const value: DataRetentionRequest = {
    messages_retention_days: settings.messages_retention_days,
    delivery_tasks_retention_days: settings.delivery_tasks_retention_days,
    ai_runs_retention_days: settings.ai_runs_retention_days ?? 30,
  }
  form.value = { ...value }
  saved.value = { ...value }
  source.value = settings.source
}

async function load(): Promise<void> {
  loading.value = true
  loaded.value = false
  try {
    assignSettings(await settingsApi.dataRetention.get())
    loaded.value = true
  } catch (error) {
    message.error('加载归档设置失败：' + errText(error))
  } finally {
    loading.value = false
  }
}

async function save(): Promise<void> {
  if (Object.values(form.value).some((days) => days < 0)) {
    message.warning('保留天数不能为负数')
    return
  }
  saving.value = true
  try {
    assignSettings(await settingsApi.dataRetention.update({ ...form.value }))
    message.success('归档设置已保存，将在下次定时清理时生效')
  } catch (error) {
    message.error('保存归档设置失败：' + errText(error))
  } finally {
    saving.value = false
  }
}

function openCleanup(): void {
  cleanupTargets.value = [
    ...(saved.value.messages_retention_days > 0 ? ['messages' as const] : []),
    ...(saved.value.delivery_tasks_retention_days > 0 ? ['delivery_tasks' as const] : []),
    ...(saved.value.ai_runs_retention_days > 0 ? ['ai_runs' as const] : []),
  ]
  showCleanup.value = true
}

async function cleanup(): Promise<void> {
  if (cleanupTargets.value.length === 0) {
    message.warning('至少选择一个清理目标')
    return
  }
  cleaning.value = true
  try {
    const result = await settingsApi.dataRetention.cleanup({ targets: [...cleanupTargets.value] })
    showCleanup.value = false
    const summary = `消息 ${result.deleted_messages} 条、投递记录 ${result.deleted_delivery_tasks} 条、AI 运行 ${result.deleted_ai_runs} 条`
    if (result.limit_reached.length > 0) {
      message.warning(`已清理 ${summary}；部分数据达到单次上限，后续定时任务会继续处理`)
    } else {
      message.success(`已清理 ${summary}`)
    }
  } catch (error) {
    message.error('清理失败：' + errText(error))
  } finally {
    cleaning.value = false
  }
}

onMounted(() => void load())
</script>

<template>
  <n-card title="消息与投递记录归档">
    <template #header-extra>
      <n-tag size="small" :type="source === 'database' ? 'success' : 'default'" :bordered="false">
        {{ source === 'database' ? '页面配置生效中' : '使用内置默认值' }}
      </n-tag>
    </template>
    <n-spin :show="loading">
      <n-space vertical size="large">
        <n-text depth="3">
          按保留天数自动清理历史消息、终态投递任务和 AI 运行记录。0 表示不清理，保存后热生效。
        </n-text>
        <n-form label-placement="left" label-width="160" :show-feedback="false">
          <n-form-item label="消息保留(天)">
            <n-space align="center">
              <n-input-number v-model:value="form.messages_retention_days" :min="0" class="number-input" />
              <n-text depth="3">按 received_at；关联投递任务一并级联删除</n-text>
            </n-space>
          </n-form-item>
          <n-form-item label="投递记录保留(天)">
            <n-space align="center">
              <n-input-number v-model:value="form.delivery_tasks_retention_days" :min="0" class="number-input" />
              <n-text depth="3">仅清理 success/failed/dead/cancelled；排队中任务保留</n-text>
            </n-space>
          </n-form-item>
          <n-form-item label="AI 运行保留(天)">
            <n-space align="center">
              <n-input-number v-model:value="form.ai_runs_retention_days" :min="0" class="number-input" />
              <n-text depth="3">仅清理 success/failed/cancelled Run 及其明细和输出</n-text>
            </n-space>
          </n-form-item>
        </n-form>
        <n-space>
          <n-button type="primary" :loading="saving" @click="save">保存归档设置</n-button>
          <n-button
            secondary
            type="error"
            :disabled="isDirty || !canCleanup"
            :title="isDirty ? '请先保存当前修改' : !canCleanup ? '所有保留天数均为 0' : undefined"
            @click="openCleanup"
          >
            立即清理过期数据
          </n-button>
        </n-space>
      </n-space>
    </n-spin>

    <n-modal v-model:show="showCleanup" preset="dialog" title="立即清理过期数据">
      <n-space vertical size="large">
        <n-alert type="warning" :show-icon="true">
          清理不可撤销，将严格使用当前已保存的保留天数。正在处理和排队中的记录不会删除。
        </n-alert>
        <n-checkbox-group v-model:value="cleanupTargets">
          <n-space vertical>
            <n-checkbox value="messages" :disabled="saved.messages_retention_days === 0">
              消息（{{ saved.messages_retention_days }} 天）
            </n-checkbox>
            <n-checkbox value="delivery_tasks" :disabled="saved.delivery_tasks_retention_days === 0">
              终态投递记录（{{ saved.delivery_tasks_retention_days }} 天）
            </n-checkbox>
            <n-checkbox value="ai_runs" :disabled="saved.ai_runs_retention_days === 0">
              终态 AI 运行（{{ saved.ai_runs_retention_days }} 天）
            </n-checkbox>
          </n-space>
        </n-checkbox-group>
      </n-space>
      <template #action>
        <n-space justify="end">
          <n-button :disabled="cleaning" @click="showCleanup = false">取消</n-button>
          <n-button type="error" :loading="cleaning" :disabled="cleanupTargets.length === 0" @click="cleanup">
            确认清理
          </n-button>
        </n-space>
      </template>
    </n-modal>
  </n-card>
</template>

<style scoped>
.number-input {
  width: 160px;
}
</style>
