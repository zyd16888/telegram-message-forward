<script setup lang="ts">
import { computed, reactive, ref, watch } from 'vue'
import { useMessage } from 'naive-ui'
import ConfigFormRenderer from '@/components/ConfigFormRenderer.vue'
import type {
  AIDigestProfile,
  AIDigestProfileRequest,
  AIDigestPreset,
  AIDigestOutputTemplate,
  AIProvider,
  ConditionConfig,
  Filter,
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
  templates: AIDigestOutputTemplate[]
  filters: Filter[]
  conditionDescriptors: RuleItemDescriptor[]
  saving?: boolean
  previewing?: boolean
}>()

const emit = defineEmits<{
  save: [payload: AIDigestProfileRequest]
  preview: [payload: AIDigestProfileRequest]
  manageTemplates: []
  manageFilters: []
}>()

const message = useMessage()

const activeTab = ref<'input' | 'prompt' | 'output'>('input')

const form = reactive<AIDigestProfileRequest>({
  name: '',
  enabled: false,
  source_ids: [],
  filter_id: 0,
  conditions: [],
  schedule: { type: 'manual', interval_minutes: 60, time: '09:00', timezone: 'Asia/Shanghai', cron: '0 9 * * *' },
  window: { type: 'last_duration', duration_minutes: 60 },
  dedupe: { enabled: true },
  prompt_template:
    '请整理以下窗口内的消息，严格按照输出结构模板组织内容。\n\n输出结构模板：\n{{output_template}}\n\n{{messages}}\n\n输出格式：{{output_format}}',
  output_format: 'markdown',
  output_template_id: 0,
  output_template: defaultOutputTemplate(),
  target_sink_ids: [],
  model_config: { provider_id: '', model: '', temperature: 0.2, max_tokens: 0 },
  limits: { max_messages_per_run: 50, max_chars_per_message: 1200, max_prompt_chars: 0 },
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
const requiredMessagesVariable = '{{messages}}'
const promptVariables = [
  {
    token: requiredMessagesVariable,
    label: '消息正文',
    desc: '窗口内被纳入 AI 输入的消息列表。通常必须保留，否则 AI 看不到消息内容。',
  },
  { token: '{{profile_name}}', label: 'Profile 名称', desc: '当前 AI Profile 的名称。' },
  { token: '{{window_start}}', label: '窗口开始', desc: '本次整理窗口的开始时间。' },
  { token: '{{window_end}}', label: '窗口结束', desc: '本次整理窗口的结束时间。' },
  { token: '{{message_count}}', label: '消息数量', desc: '最终纳入 Prompt 的消息条数。' },
  { token: '{{source_list}}', label: '来源列表', desc: '所选 Source 的名称列表。' },
  { token: '{{output_format}}', label: 'AI 输出格式', desc: '由下方「AI 输出格式」选择项决定。' },
  { token: '{{output_template}}', label: '输出结构模板', desc: '由「输出结构」区选择共享模板或自定义。' },
]
// 输入消息量 / 单条字数的快捷预设，点击填入，数值本身可自由编辑（不设前端上限）。
const inputVolumePresets = [30, 50, 100, 200, 500]
const messageLengthPresets = [
  { label: '短', value: 600 },
  { label: '常规', value: 1200 },
  { label: '较长', value: 2500 },
  { label: '完整', value: 5000 },
]
const hasMessagesVariable = computed(() => form.prompt_template.includes(requiredMessagesVariable))

// 输出模板：0 = 自定义内联；否则引用共享模板 id。
const templateSelectOptions = computed(() => [
  { label: '自定义（本 Profile 专用）', value: 0 },
  ...props.templates.map((t) => ({
    label: t.description ? `${t.name} — ${t.description}` : t.name,
    value: t.id,
  })),
])
const selectedTemplate = computed(() => props.templates.find((t) => t.id === form.output_template_id) ?? null)
const usingSharedTemplate = computed(() => form.output_template_id > 0)

// 过滤条件：0 = 自定义内联；否则引用共享过滤器 id。
const usingFilter = computed(() => form.filter_id > 0)
const selectedFilter = computed(() => props.filters.find((f) => f.id === form.filter_id) ?? null)
const conditionSourceOptions = computed(() => [
  { label: '自定义条件（本 Profile 专用）', value: 0 },
  ...props.filters.map((f) => ({ label: f.description ? `${f.name} — ${f.description}` : f.name, value: f.id })),
])

const promptSelection = reactive({ start: -1, end: -1 })

watch(
  () => [show.value, props.profile] as const,
  () => {
    if (!show.value) return
    activeTab.value = 'input'
    reset()
  },
  { immediate: true },
)

function reset(): void {
  if (!props.profile) {
    form.name = ''
    form.enabled = false
    form.source_ids = []
    form.filter_id = 0
    form.conditions = []
    form.schedule = { type: 'manual', interval_minutes: 60, time: '09:00', timezone: 'Asia/Shanghai', cron: '0 9 * * *' }
    form.window = { type: 'last_duration', duration_minutes: 60 }
    form.dedupe = { enabled: true }
    form.prompt_template =
      '请整理以下窗口内的消息，严格按照输出结构模板组织内容。\n\n输出结构模板：\n{{output_template}}\n\n{{messages}}\n\n输出格式：{{output_format}}'
    form.output_format = 'markdown'
    form.output_template_id = props.templates[0]?.id ?? 0
    form.output_template = defaultOutputTemplate()
    form.target_sink_ids = []
    form.model_config = { provider_id: defaultProviderID(), model: '', temperature: 0.2, max_tokens: 0 }
    form.limits = { max_messages_per_run: 50, max_chars_per_message: 1200, max_prompt_chars: 0 }
    return
  }
  Object.assign(form, {
    name: props.profile.name,
    enabled: props.profile.enabled,
    source_ids: [...props.profile.source_ids],
    filter_id: props.profile.filter_id ?? 0,
    conditions: props.profile.conditions.map((item) => ({ type: item.type, config: { ...(item.config ?? {}) } })),
    schedule: { ...props.profile.schedule },
    window: { ...props.profile.window },
    dedupe: { ...props.profile.dedupe },
    prompt_template: props.profile.prompt_template,
    output_format: props.profile.output_format,
    output_template_id: props.profile.output_template_id ?? 0,
    output_template: props.profile.output_template || defaultOutputTemplate(),
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
  // 预设内置的是内联模板文本，套用后切换为自定义模式。
  form.output_template_id = 0
  form.output_template = preset.output_template || defaultOutputTemplate()
  form.schedule = { ...preset.schedule }
  form.window = { ...preset.window }
  form.dedupe = { ...preset.dedupe }
  form.model_config = {
    ...preset.model_config,
    max_tokens: 0,
    provider_id: form.model_config.provider_id || defaultProviderID(),
  }
  form.limits = { ...preset.limits, max_prompt_chars: 0 }
}

function copySharedTemplateToInline(): void {
  if (selectedTemplate.value) {
    form.output_template = selectedTemplate.value.content
    form.output_format = selectedTemplate.value.format || form.output_format
  }
  form.output_template_id = 0
}

function insertPromptVariable(token: string): void {
  const chars = [...form.prompt_template]
  const fallback = chars.length
  const start = Math.max(0, Math.min(promptSelection.start >= 0 ? promptSelection.start : fallback, chars.length))
  const end = Math.max(start, Math.min(promptSelection.end, chars.length))
  form.prompt_template = `${chars.slice(0, start).join('')}${token}${chars.slice(end).join('')}`
  promptSelection.start = start + [...token].length
  promptSelection.end = promptSelection.start
}

function rememberPromptSelection(event: Event): void {
  const target = event.target
  if (!(target instanceof HTMLTextAreaElement) && !(target instanceof HTMLInputElement)) return
  promptSelection.start = target.selectionStart ?? form.prompt_template.length
  promptSelection.end = target.selectionEnd ?? promptSelection.start
}

function payload(): AIDigestProfileRequest {
  return {
    name: form.name.trim(),
    enabled: form.enabled,
    source_ids: [...form.source_ids],
    filter_id: form.filter_id,
    conditions: usingFilter.value ? [] : form.conditions.map((item) => ({ type: item.type, config: { ...(item.config ?? {}) } })),
    schedule: { ...form.schedule },
    window: { ...form.window },
    dedupe: { ...form.dedupe },
    prompt_template: form.prompt_template,
    output_format: form.output_format,
    output_template_id: form.output_template_id,
    output_template: usingSharedTemplate.value ? '' : form.output_template,
    target_sink_ids: [...form.target_sink_ids],
    model_config: { ...form.model_config, max_tokens: 0 },
    limits: { ...form.limits, max_prompt_chars: 0 },
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
  if (!usingSharedTemplate.value && !form.output_template.trim()) {
    message.warning('请选择共享模板或填写自定义输出结构')
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

function defaultOutputTemplate(): string {
  return '# {{profile_name}}\n\n## 一句话总结\n用 1 段话概括本窗口最重要的信息。\n\n## 重点摘要\n- 列出 3-7 条重点，每条都标注来源编号，例如 [#1]。\n- 合并重复消息，不要重复罗列同一件事。\n\n## 分类整理\n按主题分组整理，每组包含关键事实、背景线索和来源编号。\n\n## 待关注事项\n列出需要继续关注的问题、风险、待办或后续进展。\n\n## 说明\n以上内容仅基于本窗口内消息整理，未使用外部事实补全。'
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
        <NTabs v-model:value="activeTab" type="segment" size="small" class="form-tabs">
          <!-- Tab 1：输入与调度 -->
          <NTabPane name="input" tab="输入与调度">
            <div class="base-grid">
              <NFormItem label="名称" required>
                <NInput v-model:value="form.name" placeholder="例如：技术资讯小时报" />
              </NFormItem>
              <NFormItem label="启用定时">
                <NSwitch v-model:value="form.enabled" />
              </NFormItem>
            </div>

            <div class="field-head">
              <div class="field-label">输入来源与过滤</div>
              <NButton size="tiny" text type="primary" @click="emit('manageFilters'); show = false">管理过滤器</NButton>
            </div>
            <div class="field-hint">过滤只影响 AI 输入，不影响原始消息实时转发。可引用共享过滤器，或写本 Profile 专用条件。</div>
            <NFormItem label="Source">
              <NSelect v-model:value="form.source_ids" multiple filterable :options="sourceOptions" />
            </NFormItem>
            <NFormItem label="过滤条件来源">
              <NSelect v-model:value="form.filter_id" :options="conditionSourceOptions" />
            </NFormItem>

            <template v-if="usingFilter">
              <NAlert type="info" :show-icon="false" class="filter-note">
                使用共享过滤器「{{ selectedFilter?.name }}」的 {{ selectedFilter?.conditions.length ?? 0 }} 个条件。修改该过滤器会联动所有引用它的地方。
              </NAlert>
              <div v-if="selectedFilter?.conditions.length" class="filter-cond-list">
                <NTag v-for="(c, i) in selectedFilter?.conditions" :key="i" size="small" :bordered="false" type="info">
                  {{ conditionDescriptor(c.type)?.label ?? c.type }}
                </NTag>
              </div>
            </template>

            <template v-else>
              <div class="cond-head">
                <span class="cond-head-label">自定义条件</span>
                <NButton size="tiny" dashed @click="addCondition">添加过滤条件</NButton>
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
              <div v-if="!form.conditions.length" class="field-hint">未添加条件时，来源消息全部进入 AI（仅自动去空、去重）。</div>
            </template>

            <div class="field-label field-gap">窗口与调度</div>
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
              <NFormItem v-if="form.schedule.type === 'interval'" label="间隔分钟">
                <NInputNumber v-model:value="form.schedule.interval_minutes" :min="5" class="full-input" />
              </NFormItem>
              <NFormItem v-if="form.schedule.type === 'daily'" label="每日时间">
                <NInput v-model:value="form.schedule.time" placeholder="09:00" />
              </NFormItem>
              <NFormItem v-if="form.schedule.type === 'cron'" label="Cron 表达式">
                <NInput v-model:value="form.schedule.cron" placeholder="0 9 * * *" />
              </NFormItem>
              <NFormItem v-if="form.schedule.type === 'daily' || form.schedule.type === 'cron'" label="时区">
                <NInput v-model:value="form.schedule.timezone" placeholder="Asia/Shanghai" />
              </NFormItem>
            </div>
          </NTabPane>

          <!-- Tab 2：Prompt 与模型 -->
          <NTabPane name="prompt" tab="Prompt 与模型">
            <NFormItem label="常用预设">
              <NSelect :options="presetOptions" clearable placeholder="选择后会套用 Prompt、窗口、调度和限制" @update:value="applyPreset" />
            </NFormItem>
            <div class="field-head">
              <div class="field-label">Prompt</div>
              <NTag :type="hasMessagesVariable ? 'success' : 'warning'" size="small" :bordered="false">
                {{ hasMessagesVariable ? '已包含消息' : '缺少消息变量' }}
              </NTag>
            </div>
            <NInput
              v-model:value="form.prompt_template"
              type="textarea"
              :autosize="{ minRows: 8, maxRows: 16 }"
              @focus="rememberPromptSelection"
              @click="rememberPromptSelection"
              @keyup="rememberPromptSelection"
              @select="rememberPromptSelection"
            />
            <NAlert v-if="!hasMessagesVariable" type="warning" :show-icon="false" class="prompt-alert">
              当前 Prompt 没有 <code>{{ requiredMessagesVariable }}</code>，AI 可能拿不到具体消息内容。后端会兜底追加消息，但建议你显式放在希望的位置。
            </NAlert>
            <div class="variable-ref">
              <div class="variable-ref-title">可用变量（点击插入到光标处）</div>
              <div class="variable-grid">
                <button
                  v-for="item in promptVariables"
                  :key="item.token"
                  class="variable-cell"
                  type="button"
                  @click="insertPromptVariable(item.token)"
                >
                  <span class="variable-cell-head">
                    <span class="variable-token">{{ item.token }}</span>
                    <span class="variable-cell-label">{{ item.label }}</span>
                  </span>
                  <span class="variable-cell-desc" :title="item.desc">{{ item.desc }}</span>
                </button>
              </div>
            </div>

            <div class="field-label field-gap">模型</div>
            <div class="triple-grid">
              <NFormItem label="Provider">
                <NSelect v-model:value="form.model_config.provider_id" filterable clearable :options="providerOptions" />
              </NFormItem>
              <NFormItem label="模型（可空=Provider 默认）">
                <NInput v-model:value="form.model_config.model" placeholder="gpt-4o-mini" />
              </NFormItem>
              <NFormItem label="创造性">
                <NSelect
                  v-model:value="form.model_config.temperature"
                  :options="[
                    { label: '稳定严谨（0.2）', value: 0.2 },
                    { label: '稍有表达（0.5）', value: 0.5 },
                    { label: '更发散（0.8）', value: 0.8 },
                  ]"
                />
              </NFormItem>
            </div>

            <div class="field-label field-gap">输出与输入限制</div>
            <div class="triple-grid">
              <NFormItem label="AI 输出格式">
                <NSelect v-model:value="form.output_format" :options="outputFormatOptions" />
              </NFormItem>
              <NFormItem label="输入消息量（条）">
                <NInputNumber v-model:value="form.limits.max_messages_per_run" :min="1" class="full-input" placeholder="按需填写，如 500" />
              </NFormItem>
              <NFormItem label="单条消息字符上限">
                <NInputNumber v-model:value="form.limits.max_chars_per_message" :min="100" :step="100" class="full-input" />
              </NFormItem>
            </div>
            <div class="quick-presets">
              <span class="quick-label">消息量快捷：</span>
              <button
                v-for="n in inputVolumePresets"
                :key="n"
                type="button"
                class="mini-chip"
                :class="{ active: form.limits.max_messages_per_run === n }"
                @click="form.limits.max_messages_per_run = n"
              >
                {{ n }}
              </button>
              <span class="quick-label quick-gap">单条字数：</span>
              <button
                v-for="p in messageLengthPresets"
                :key="p.value"
                type="button"
                class="mini-chip"
                :class="{ active: form.limits.max_chars_per_message === p.value }"
                @click="form.limits.max_chars_per_message = p.value"
              >
                {{ p.label }}
              </button>
            </div>
            <div class="field-hint">
              输入消息量没有前端上限，可自定义（资讯频道更新快时可调大，如 300–500）。
              单条超过字数上限的消息会被自动截断并标注「已按单条消息字数上限截断」，不影响其他消息；Prompt 本身不会被截断。
            </div>
          </NTabPane>

          <!-- Tab 3：输出与投递 -->
          <NTabPane name="output" tab="输出与投递">
            <div class="field-head">
              <div class="field-label">输出结构</div>
              <NButton size="tiny" text type="primary" @click="emit('manageTemplates'); show = false">管理模板</NButton>
            </div>
            <div class="field-hint">选择共享模板（多个 Profile 可复用），或为本 Profile 单独编写结构。</div>
            <NFormItem label="结构模板">
              <NSelect v-model:value="form.output_template_id" :options="templateSelectOptions" />
            </NFormItem>

            <div v-if="usingSharedTemplate" class="shared-template">
              <div class="shared-template-head">
                <NTag type="info" size="small" :bordered="false">共享模板 · {{ selectedTemplate?.format?.toUpperCase() }}</NTag>
                <NButton size="tiny" secondary @click="copySharedTemplateToInline">复制为自定义并编辑</NButton>
              </div>
              <pre class="shared-template-preview">{{ selectedTemplate?.content }}</pre>
              <div class="field-hint">修改共享模板会影响所有引用它的 Profile；如需单独调整，请复制为自定义。</div>
            </div>

            <template v-else>
              <NFormItem label="自定义输出结构">
                <NInput
                  v-model:value="form.output_template"
                  type="textarea"
                  placeholder="定义 AI 输出的标题、章节、列表、来源标注等结构规范"
                  :autosize="{ minRows: 8, maxRows: 14 }"
                />
              </NFormItem>
              <div class="field-hint">这里定义「内容长什么样」；上面的 AI 输出格式只决定用 Markdown / Text / HTML 作为载体。</div>
            </template>

            <div class="field-label field-gap">输出渠道</div>
            <NFormItem label="Target Sinks">
              <NSelect v-model:value="form.target_sink_ids" multiple filterable :options="sinkOptions" />
            </NFormItem>
          </NTabPane>
        </NTabs>
      </NForm>
    </div>
    <template #footer>
      <div class="modal-footer">
        <div class="footer-trial">
          <NButton :loading="previewing" secondary @click="preview">试运行</NButton>
          <span class="footer-hint">用当前完整配置（来源 → 过滤 → Prompt → 模型 → 输出）试跑一次，仅查看结果，不投递、不保存。</span>
        </div>
        <NSpace>
          <NButton @click="show = false">取消</NButton>
          <NButton type="primary" :loading="saving" @click="save">保存</NButton>
        </NSpace>
      </div>
    </template>
  </NModal>
</template>

<style scoped>
.modal-body {
  max-height: min(72vh, 720px);
  overflow: auto;
  padding-right: 4px;
}

.form-tabs {
  margin-top: 2px;
}

.form-tabs :deep(.n-tab-pane) {
  padding-top: 16px;
}

/* 小节标题（Tab 内的分组） */
.field-label {
  color: var(--clay-text);
  font-size: 13px;
  font-weight: 800;
}

.field-head {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
  margin-bottom: 4px;
}

.field-hint {
  margin-bottom: 8px;
  color: var(--clay-text-3);
  font-size: 12px;
  line-height: 1.5;
}

.field-gap {
  margin-top: 18px;
  margin-bottom: 8px;
}

.base-grid,
.schedule-grid,
.triple-grid {
  display: grid;
  grid-template-columns: repeat(3, minmax(160px, 1fr));
  gap: 12px 14px;
  align-items: start;
}

.prompt-alert {
  margin-top: 12px;
}

/* Prompt 变量：文本框下方紧凑网格，带名称与说明 */
.variable-ref {
  margin-top: 12px;
  border: 1px solid var(--clay-border);
  border-radius: 8px;
  background: var(--clay-surface);
  padding: 12px;
}

.variable-ref-title {
  margin-bottom: 10px;
  color: var(--clay-text-2);
  font-size: 12px;
  font-weight: 700;
}

.variable-grid {
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  gap: 8px;
}

.variable-cell {
  display: flex;
  flex-direction: column;
  gap: 2px;
  min-width: 0;
  border: 1px solid var(--clay-border);
  border-radius: 8px;
  background: var(--clay-surface-2);
  padding: 7px 10px;
  text-align: left;
  cursor: pointer;
  transition:
    border-color 0.16s ease,
    background 0.16s ease,
    transform 0.16s ease;
}

.variable-cell:hover {
  border-color: var(--clay-primary);
  background: var(--clay-surface);
  transform: translateY(-1px);
}

.variable-cell-head {
  display: flex;
  align-items: baseline;
  gap: 6px;
  min-width: 0;
}

.variable-token {
  flex-shrink: 0;
  color: var(--clay-primary);
  font-family: ui-monospace, SFMono-Regular, Consolas, 'Liberation Mono', monospace;
  font-size: 12px;
  font-weight: 800;
}

.variable-cell-label {
  color: var(--clay-text);
  font-size: 12px;
  font-weight: 700;
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
}

.variable-cell-desc {
  color: var(--clay-text-3);
  font-size: 11px;
  line-height: 1.4;
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
}

/* 快捷预设小标签 */
.quick-presets {
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  gap: 6px;
  margin-top: 10px;
  margin-bottom: 8px;
}

.quick-label {
  color: var(--clay-text-3);
  font-size: 12px;
}

.quick-gap {
  margin-left: 8px;
}

.mini-chip {
  border: 1px solid var(--clay-border);
  border-radius: 6px;
  background: var(--clay-surface-2);
  padding: 2px 10px;
  color: var(--clay-text-2);
  font-size: 12px;
  cursor: pointer;
  transition:
    border-color 0.16s ease,
    color 0.16s ease,
    background 0.16s ease;
}

.mini-chip:hover {
  border-color: var(--clay-primary);
}

.mini-chip.active {
  border-color: var(--clay-primary);
  background: var(--clay-primary-soft);
  color: var(--clay-primary);
  font-weight: 700;
}

.shared-template {
  border: 1px solid var(--clay-border);
  border-radius: 8px;
  background: var(--clay-surface);
  padding: 12px;
}

.shared-template-head {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 10px;
  margin-bottom: 8px;
}

.shared-template-preview {
  margin: 0 0 6px;
  max-height: 240px;
  overflow: auto;
  white-space: pre-wrap;
  word-break: break-word;
  font-family: ui-monospace, SFMono-Regular, Consolas, 'Liberation Mono', monospace;
  font-size: 12px;
  line-height: 1.55;
  color: var(--clay-text-2);
}

.condition-block {
  border: 1px solid var(--clay-border);
  border-radius: 8px;
  margin: 10px 0;
  padding: 12px;
  background: var(--clay-surface);
}

.filter-note {
  margin-bottom: 10px;
}

.filter-cond-list {
  display: flex;
  flex-wrap: wrap;
  gap: 6px;
}

.cond-head {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 10px;
  margin: 8px 0 4px;
}

.cond-head-label {
  color: var(--clay-text-2);
  font-size: 12px;
  font-weight: 700;
}

.type-select,
.full-input {
  width: 100%;
}

/* 底部：试运行独立于三个标签，作用于整份配置 */
.modal-footer {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 16px;
  flex-wrap: wrap;
}

.footer-trial {
  display: flex;
  align-items: center;
  gap: 10px;
  min-width: 0;
  flex: 1;
}

.footer-hint {
  color: var(--clay-text-3);
  font-size: 12px;
  line-height: 1.4;
}

@media (max-width: 760px) {
  .base-grid,
  .schedule-grid,
  .triple-grid,
  .variable-grid {
    grid-template-columns: 1fr;
  }

  .footer-hint {
    display: none;
  }
}
</style>
