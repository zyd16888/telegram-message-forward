<script setup lang="ts">
import { computed, reactive, watch } from 'vue'
import { useMessage } from 'naive-ui'
import ConfigFormRenderer from '@/components/ConfigFormRenderer.vue'
import type {
  AIDigestProfile,
  AIDigestProfileRequest,
  AIDigestPreset,
  AIProvider,
  ConditionConfig,
  RuleItemDescriptor,
  Sink,
  Source,
} from '@/types'

const show = defineModel<boolean>('show', { required: true })

const props = defineProps<{
  profile?: AIDigestProfile | null
  sources: Source[]
  sinks: Sink[]
  providers: AIProvider[]
  presets: AIDigestPreset[]
  conditionDescriptors: RuleItemDescriptor[]
  saving?: boolean
  previewing?: boolean
}>()

const emit = defineEmits<{
  save: [payload: AIDigestProfileRequest]
  preview: [payload: AIDigestProfileRequest]
}>()

const message = useMessage()

const form = reactive<AIDigestProfileRequest>({
  name: '',
  enabled: false,
  source_ids: [],
  conditions: [],
  schedule: { type: 'manual', interval_minutes: 60, time: '09:00', timezone: 'Asia/Shanghai', cron: '0 9 * * *' },
  window: { type: 'last_duration', duration_minutes: 60 },
  dedupe: { enabled: true },
  prompt_template:
    '请整理以下窗口内的消息，输出重点摘要、分类列表和来源编号。\n\n{{messages}}\n\n输出格式：{{output_format}}',
  output_format: 'markdown',
  target_sink_ids: [],
  model_config: { provider_id: '', model: '', temperature: 0.2, max_tokens: 1200 },
  limits: { max_messages_per_run: 50, max_chars_per_message: 1200, max_prompt_chars: 30000 },
})

const sourceOptions = computed(() => props.sources.map((item) => ({ label: `${item.name} (#${item.id})`, value: item.id })))
const sinkOptions = computed(() => props.sinks.map((item) => ({ label: `${item.name} (${item.type})`, value: item.id })))
const providerOptions = computed(() =>
  props.providers.map((item) => ({
    label: `${item.name || item.id}${item.is_default ? '（默认）' : ''} · ${item.model}`,
    value: item.id,
  })),
)
const presetOptions = computed(() => props.presets.map((item) => ({ label: item.name, value: item.id })))
const conditionOptions = computed(() => props.conditionDescriptors.map((item) => ({ label: item.label, value: item.type })))
const outputFormatOptions = [
  { label: 'Markdown', value: 'markdown' },
  { label: 'Text', value: 'text' },
  { label: 'HTML', value: 'html' },
]
const scheduleOptions = [
  { label: '仅手动', value: 'manual' },
  { label: '间隔执行', value: 'interval' },
  { label: '每日固定时间', value: 'daily' },
  { label: 'Cron 表达式', value: 'cron' },
]
const windowOptions = [
  { label: '最近一段时间', value: 'last_duration' },
  { label: '自上次成功运行', value: 'since_last_run' },
]

watch(
  () => [show.value, props.profile] as const,
  () => {
    if (!show.value) return
    reset()
  },
  { immediate: true },
)

function reset(): void {
  if (!props.profile) {
    form.name = ''
    form.enabled = false
    form.source_ids = []
    form.conditions = []
    form.schedule = { type: 'manual', interval_minutes: 60, time: '09:00', timezone: 'Asia/Shanghai', cron: '0 9 * * *' }
    form.window = { type: 'last_duration', duration_minutes: 60 }
    form.dedupe = { enabled: true }
    form.prompt_template =
      '请整理以下窗口内的消息，输出重点摘要、分类列表和来源编号。\n\n{{messages}}\n\n输出格式：{{output_format}}'
    form.output_format = 'markdown'
    form.target_sink_ids = []
    form.model_config = { provider_id: defaultProviderID(), model: '', temperature: 0.2, max_tokens: 1200 }
    form.limits = { max_messages_per_run: 50, max_chars_per_message: 1200, max_prompt_chars: 30000 }
    return
  }
  Object.assign(form, {
    name: props.profile.name,
    enabled: props.profile.enabled,
    source_ids: [...props.profile.source_ids],
    conditions: props.profile.conditions.map((item) => ({ type: item.type, config: { ...(item.config ?? {}) } })),
    schedule: { ...props.profile.schedule },
    window: { ...props.profile.window },
    dedupe: { ...props.profile.dedupe },
    prompt_template: props.profile.prompt_template,
    output_format: props.profile.output_format,
    target_sink_ids: [...props.profile.target_sink_ids],
    model_config: { ...props.profile.model_config },
    limits: { ...props.profile.limits },
  })
}

watch(
  () => props.providers,
  () => {
    if (show.value && !form.model_config.provider_id) {
      form.model_config.provider_id = defaultProviderID()
    }
  },
)

function defaultProviderID(): string {
  return props.providers.find((item) => item.is_default)?.id ?? props.providers[0]?.id ?? ''
}

function conditionDescriptor(type: string): RuleItemDescriptor | undefined {
  return props.conditionDescriptors.find((item) => item.type === type)
}

function defaultsFor(desc?: RuleItemDescriptor): Record<string, unknown> {
  const out: Record<string, unknown> = {}
  for (const field of desc?.fields ?? []) {
    if (field.default !== undefined) out[field.key] = field.default
  }
  return out
}

function addCondition(): void {
  const desc = props.conditionDescriptors.find((item) => item.type !== 'source') ?? props.conditionDescriptors[0]
  if (!desc) return
  form.conditions.push({ type: desc.type, config: defaultsFor(desc) })
}

function updateConditionType(item: ConditionConfig, type: string): void {
  item.type = type
  item.config = defaultsFor(conditionDescriptor(type))
}

function removeCondition(index: number): void {
  form.conditions.splice(index, 1)
}

function applyPreset(id: string): void {
  const preset = props.presets.find((item) => item.id === id)
  if (!preset) return
  form.prompt_template = preset.prompt_template
  form.output_format = preset.output_format
  form.schedule = { ...preset.schedule }
  form.window = { ...preset.window }
  form.dedupe = { ...preset.dedupe }
  form.model_config = {
    ...preset.model_config,
    provider_id: form.model_config.provider_id || defaultProviderID(),
  }
  form.limits = { ...preset.limits }
}

function payload(): AIDigestProfileRequest {
  return {
    name: form.name.trim(),
    enabled: form.enabled,
    source_ids: [...form.source_ids],
    conditions: form.conditions.map((item) => ({ type: item.type, config: { ...(item.config ?? {}) } })),
    schedule: { ...form.schedule },
    window: { ...form.window },
    dedupe: { ...form.dedupe },
    prompt_template: form.prompt_template,
    output_format: form.output_format,
    target_sink_ids: [...form.target_sink_ids],
    model_config: { ...form.model_config },
    limits: { ...form.limits },
  }
}

function validate(forPreview = false): boolean {
  if (!form.source_ids.length) {
    message.warning('请至少选择一个输入来源')
    return false
  }
  if (!forPreview && !form.target_sink_ids.length) {
    message.warning('请至少选择一个输出渠道')
    return false
  }
  if (!form.prompt_template.trim()) {
    message.warning('请填写 Prompt')
    return false
  }
  if (form.schedule.type === 'cron' && !form.schedule.cron?.trim()) {
    message.warning('请填写 Cron 表达式')
    return false
  }
  return true
}

function save(): void {
  if (!validate(false)) return
  emit('save', payload())
}

function preview(): void {
  if (!validate(true)) return
  emit('preview', payload())
}
</script>

<template>
  <NModal
    v-model:show="show"
    preset="card"
    :title="profile ? '编辑 AI Profile' : '新建 AI Profile'"
    :style="{ width: 'min(940px, calc(100vw - 32px))' }"
  >
    <div class="modal-body">
      <NForm label-placement="top" :show-feedback="false">
        <section class="form-section">
          <div class="section-title">基础信息</div>
          <div class="base-grid">
            <NFormItem label="名称" required>
              <NInput v-model:value="form.name" placeholder="例如：技术资讯小时报" />
            </NFormItem>
            <NFormItem label="启用定时">
              <NSwitch v-model:value="form.enabled" />
            </NFormItem>
            <NFormItem label="输出格式">
              <NSelect v-model:value="form.output_format" :options="outputFormatOptions" />
            </NFormItem>
          </div>
        </section>

        <section class="form-section">
          <div class="section-title">输入来源</div>
          <NFormItem label="Source">
            <NSelect v-model:value="form.source_ids" multiple filterable :options="sourceOptions" />
          </NFormItem>
        </section>

        <section class="form-section">
          <div class="section-head">
            <div>
              <div class="section-title">过滤条件</div>
              <div class="section-desc">只影响 AI 输入，不影响原始消息实时转发。</div>
            </div>
            <NButton size="small" dashed @click="addCondition">添加条件</NButton>
          </div>
          <div v-for="(condition, index) in form.conditions" :key="index" class="condition-block">
            <NSpace align="center" justify="space-between">
              <NSelect
                :value="condition.type"
                class="type-select"
                :options="conditionOptions"
                @update:value="(value: string) => updateConditionType(condition, value)"
              />
              <NButton size="small" type="error" secondary @click="removeCondition(index)">移除</NButton>
            </NSpace>
            <ConfigFormRenderer
              v-if="conditionDescriptor(condition.type)"
              v-model="condition.config"
              :fields="conditionDescriptor(condition.type)?.fields ?? []"
            />
          </div>
          <NEmpty v-if="!form.conditions.length" size="small" description="未添加条件" />
        </section>

        <section class="form-section">
          <div class="section-title">窗口与调度</div>
          <div class="schedule-grid">
            <NFormItem label="窗口">
              <NSelect v-model:value="form.window.type" :options="windowOptions" />
            </NFormItem>
            <NFormItem label="窗口分钟数">
              <NInputNumber v-model:value="form.window.duration_minutes" :min="5" class="full-input" />
            </NFormItem>
            <NFormItem label="调度">
              <NSelect v-model:value="form.schedule.type" :options="scheduleOptions" />
            </NFormItem>
            <NFormItem label="间隔分钟">
              <NInputNumber v-model:value="form.schedule.interval_minutes" :min="5" class="full-input" />
            </NFormItem>
            <NFormItem label="每日时间">
              <NInput v-model:value="form.schedule.time" placeholder="09:00" />
            </NFormItem>
            <NFormItem label="时区">
              <NInput v-model:value="form.schedule.timezone" placeholder="Asia/Shanghai" />
            </NFormItem>
            <NFormItem v-if="form.schedule.type === 'cron'" label="Cron 表达式">
              <NInput v-model:value="form.schedule.cron" placeholder="0 9 * * *" />
            </NFormItem>
          </div>
        </section>

        <section class="form-section">
          <div class="section-title">Prompt 与模型限制</div>
          <NFormItem label="常用预设">
            <NSelect :options="presetOptions" clearable placeholder="选择后会套用 Prompt、窗口、调度和限制" @update:value="applyPreset" />
          </NFormItem>
          <NFormItem label="Prompt">
            <NInput v-model:value="form.prompt_template" type="textarea" :autosize="{ minRows: 8, maxRows: 16 }" />
          </NFormItem>
          <div class="schedule-grid">
            <NFormItem label="Provider">
              <NSelect v-model:value="form.model_config.provider_id" filterable clearable :options="providerOptions" />
            </NFormItem>
            <NFormItem label="模型（可空=Provider 默认）">
              <NInput v-model:value="form.model_config.model" placeholder="gpt-4o-mini" />
            </NFormItem>
            <NFormItem label="Temperature">
              <NInputNumber v-model:value="form.model_config.temperature" :min="0" :max="2" :step="0.1" class="full-input" />
            </NFormItem>
            <NFormItem label="Max Tokens">
              <NInputNumber v-model:value="form.model_config.max_tokens" :min="100" class="full-input" />
            </NFormItem>
            <NFormItem label="最大消息数">
              <NInputNumber v-model:value="form.limits.max_messages_per_run" :min="1" class="full-input" />
            </NFormItem>
            <NFormItem label="单条字符数">
              <NInputNumber v-model:value="form.limits.max_chars_per_message" :min="100" class="full-input" />
            </NFormItem>
            <NFormItem label="Prompt 字符数">
              <NInputNumber v-model:value="form.limits.max_prompt_chars" :min="1000" class="full-input" />
            </NFormItem>
          </div>
        </section>

        <section class="form-section">
          <div class="section-title">输出渠道</div>
          <NFormItem label="Target Sinks">
            <NSelect v-model:value="form.target_sink_ids" multiple filterable :options="sinkOptions" />
          </NFormItem>
        </section>
      </NForm>
    </div>
    <template #footer>
      <NSpace justify="space-between">
        <NButton :loading="previewing" @click="preview">生成预览</NButton>
        <NSpace>
          <NButton @click="show = false">取消</NButton>
          <NButton type="primary" :loading="saving" @click="save">保存</NButton>
        </NSpace>
      </NSpace>
    </template>
  </NModal>
</template>

<style scoped>
.modal-body {
  max-height: min(74vh, 760px);
  overflow: auto;
  padding-right: 4px;
}

.form-section {
  border: 1px solid var(--clay-border);
  border-radius: 8px;
  padding: 14px;
  background: var(--clay-surface-2);
}

.form-section + .form-section {
  margin-top: 12px;
}

.section-head {
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  gap: 12px;
  margin-bottom: 12px;
}

.section-title {
  color: var(--clay-text);
  font-size: 14px;
  font-weight: 800;
}

.section-desc {
  margin-top: 3px;
  color: var(--clay-text-3);
  font-size: 12px;
}

.base-grid,
.schedule-grid {
  display: grid;
  grid-template-columns: repeat(3, minmax(160px, 1fr));
  gap: 12px;
  align-items: start;
}

.condition-block {
  border: 1px solid var(--clay-border);
  border-radius: 8px;
  margin-bottom: 10px;
  padding: 12px;
  background: var(--clay-surface);
}

.type-select,
.full-input {
  width: 100%;
}

@media (max-width: 760px) {
  .base-grid,
  .schedule-grid {
    grid-template-columns: 1fr;
  }

  .section-head {
    flex-direction: column;
  }
}
</style>
