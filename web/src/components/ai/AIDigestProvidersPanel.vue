<script setup lang="ts">
import { computed, h, reactive, shallowRef } from 'vue'
import { NButton, NTag, useDialog, useMessage, type DataTableColumns } from 'naive-ui'
import { aiApi } from '@/api/client'
import type { AIProvider, AIProviderRequest } from '@/types'
import { errText } from '@/utils/error'
import ClayIcon from '@/components/ClayIcon.vue'

const props = defineProps<{ providers: AIProvider[]; loading?: boolean }>()
const emit = defineEmits<{ changed: [] }>()

const message = useMessage()
const dialog = useDialog()
const saving = shallowRef(false)
const testingID = shallowRef<string | 'draft' | null>(null)
const editingID = shallowRef<string | null>(null)
const showForm = shallowRef(false)
const form = reactive<AIProviderRequest>(defaultForm())

const title = computed(() => (editingID.value ? '编辑 AI Provider' : '新增 AI Provider'))
const editingProvider = computed(() => props.providers.find((item) => item.id === editingID.value) ?? null)
const apiTypeOptions = [
  { label: 'Chat Completions (/v1/chat/completions)', value: 'chat_completions' },
  { label: 'Responses API (/v1/responses)', value: 'responses' },
]

const columns: DataTableColumns<AIProvider> = [
  {
    title: '名称', key: 'name',
    render: (row) => h('div', { class: 'provider-name' }, [
      h('strong', row.name || row.id),
      row.is_default ? h(NTag, { size: 'small', type: 'success', bordered: false }, { default: () => '默认' }) : null,
    ]),
  },
  { title: '接口', key: 'api_type', width: 130, render: (row) => (row.api_type === 'responses' ? 'Responses' : 'Chat') },
  { title: 'Base URL', key: 'base_url', ellipsis: { tooltip: true } },
  { title: '默认模型', key: 'model', width: 150 },
  {
    title: 'Key', key: 'has_api_key', width: 90,
    render: (row) => h(NTag, { size: 'small', type: row.has_api_key ? 'success' : 'warning', bordered: false }, { default: () => (row.has_api_key ? '已配置' : '未配置') }),
  },
  {
    title: '操作', key: 'actions', width: 210,
    render: (row) => h('div', { class: 'provider-actions' }, [
      h(NButton, { size: 'small', secondary: true, onClick: () => edit(row) }, { default: () => '编辑' }),
      h(NButton, { size: 'small', loading: testingID.value === row.id, onClick: () => test(row) }, { default: () => '测试' }),
      h(NButton, { size: 'small', type: 'error', quaternary: true, disabled: props.providers.length <= 1, onClick: () => remove(row) }, { default: () => '删除' }),
    ]),
  },
]

function defaultForm(): AIProviderRequest {
  return {
    name: '', provider_type: 'openai_compatible', api_type: 'chat_completions',
    base_url: 'https://api.openai.com/v1', model: 'gpt-4o-mini', timeout_seconds: 60,
    max_retries: 1, default_temperature: 0.2, supports_vision: false, vision_model: '',
    enabled: true, is_default: props?.providers?.length === 0, api_key: '',
  }
}

function create(): void {
  editingID.value = null
  Object.assign(form, defaultForm())
  showForm.value = true
}

function edit(provider: AIProvider): void {
  editingID.value = provider.id
  Object.assign(form, {
    name: provider.name, provider_type: provider.provider_type, api_type: provider.api_type || 'chat_completions',
    base_url: provider.base_url, model: provider.model, timeout_seconds: provider.timeout_seconds,
    max_retries: provider.max_retries, default_temperature: provider.default_temperature,
    supports_vision: provider.supports_vision, vision_model: provider.vision_model ?? '', enabled: true,
    is_default: provider.is_default, api_key: '',
  })
  showForm.value = true
}

function payload(): AIProviderRequest {
  return {
    ...form,
    vision_model: form.vision_model?.trim() || undefined,
    api_key: form.api_key?.trim() || undefined,
  }
}

async function save(): Promise<void> {
  saving.value = true
  try {
    if (editingID.value) await aiApi.providers.update(editingID.value, payload())
    else await aiApi.providers.create(payload())
    message.success('Provider 设置已保存')
    showForm.value = false
    emit('changed')
  } catch (error) {
    message.error('保存失败：' + errText(error))
  } finally {
    saving.value = false
  }
}

async function test(provider?: AIProvider): Promise<void> {
  testingID.value = provider?.id ?? 'draft'
  try {
    const result = provider
      ? await aiApi.providers.test(provider.id)
      : editingID.value
        ? await aiApi.providers.test(editingID.value, payload())
        : await aiApi.providers.testDraft(payload())
    if (result.success) message.success(result.text || 'Provider 测试成功')
    else message.error(result.error || 'Provider 测试失败')
  } catch (error) {
    message.error('测试失败：' + errText(error))
  } finally {
    testingID.value = null
  }
}

function remove(provider: AIProvider): void {
  dialog.warning({
    title: '删除 Provider', content: `确定删除「${provider.name || provider.id}」？`,
    positiveText: '删除', negativeText: '取消',
    onPositiveClick: async () => {
      try {
        await aiApi.providers.remove(provider.id)
        message.success('已删除 Provider')
        emit('changed')
      } catch (error) {
        message.error('删除失败：' + errText(error))
      }
    },
  })
}
</script>

<template>
  <div class="panel-toolbar">
    <span>配置 OpenAI 兼容的 AI Provider，Profile 执行时会调用选定的服务。</span>
    <NButton type="primary" size="small" @click="create">
      <template #icon><ClayIcon name="plus" :size="15" /></template>
      新增 Provider
    </NButton>
  </div>
  <NDataTable :loading="loading" :columns="columns" :data="providers" :bordered="false" :scroll-x="900" :row-key="(row: AIProvider) => row.id" />

  <NModal v-model:show="showForm" preset="card" :title="title" :style="{ width: 'min(760px, calc(100vw - 32px))' }">
    <NForm label-placement="top" :show-feedback="false">
      <div class="provider-grid">
        <NFormItem label="名称"><NInput v-model:value="form.name" placeholder="OpenAI / DeepSeek / Anthropic Gateway" /></NFormItem>
        <NFormItem label="接口类型"><NSelect v-model:value="form.api_type" :options="apiTypeOptions" /></NFormItem>
        <NFormItem label="默认 Provider"><NSwitch v-model:value="form.is_default" /></NFormItem>
        <NFormItem label="Base URL" class="span-2"><NInput v-model:value="form.base_url" placeholder="https://api.openai.com/v1" /></NFormItem>
        <NFormItem label="默认模型"><NInput v-model:value="form.model" placeholder="gpt-4o-mini" /></NFormItem>
        <NFormItem label="支持图片理解"><NSwitch v-model:value="form.supports_vision" /></NFormItem>
        <NFormItem v-if="form.supports_vision" label="视觉模型（可空=默认模型）" class="span-2"><NInput v-model:value="form.vision_model" /></NFormItem>
        <NFormItem label="API Key" class="span-2">
          <NInput v-model:value="form.api_key" type="password" show-password-on="click" :placeholder="editingProvider?.has_api_key ? '已配置，留空表示不修改' : 'sk-...'" />
        </NFormItem>
        <NFormItem label="Temperature"><NInputNumber v-model:value="form.default_temperature" :min="0" :max="2" :step="0.1" class="full-input" /></NFormItem>
        <NFormItem label="超时秒数"><NInputNumber v-model:value="form.timeout_seconds" :min="5" class="full-input" /></NFormItem>
        <NFormItem label="重试次数"><NInputNumber v-model:value="form.max_retries" :min="0" class="full-input" /></NFormItem>
      </div>
    </NForm>
    <template #footer>
      <NSpace justify="space-between">
        <NButton secondary :loading="testingID === 'draft'" @click="test()">测试当前配置</NButton>
        <NSpace>
          <NButton @click="showForm = false">取消</NButton>
          <NButton type="primary" :loading="saving" @click="save">{{ editingID ? '保存' : '创建' }}</NButton>
        </NSpace>
      </NSpace>
    </template>
  </NModal>
</template>

<style scoped>
.panel-toolbar {
  display: flex;
  justify-content: space-between;
  align-items: center;
  gap: 12px;
  margin: 2px 0 14px;
  color: var(--clay-text-3);
  font-size: 13px;
}

:deep(.provider-name),
:deep(.provider-actions) {
  display: flex;
  align-items: center;
  gap: 8px;
}

.provider-grid {
  display: grid;
  grid-template-columns: repeat(3, minmax(160px, 1fr));
  gap: 12px;
  align-items: start;
}

.span-2 { grid-column: span 2; }
.full-input { width: 100%; }

@media (max-width: 820px) {
  .panel-toolbar { align-items: flex-start; }
  .provider-grid { grid-template-columns: 1fr; }
  .span-2 { grid-column: auto; }
}
</style>
