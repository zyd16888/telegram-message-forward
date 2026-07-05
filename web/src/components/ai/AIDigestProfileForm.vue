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
    '请整理以下窗口内的消息，严格按照输出结构模板组织内容。\n\n输出结构模板：\n{{output_template}}\n\n{{messages}}\n\n输出格式：{{output_format}}',
  output_format: 'markdown',
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
  { token: '{{output_format}}', label: 'AI 输出格式', desc: '由本区域下方的“AI 输出格式”选择项决定。' },
  { token: '{{output_template}}', label: '输出结构模板', desc: '由下方“结构模板”选择或自定义编辑。' },
]
const outputTemplatePresets = [
  {
    id: 'default',
    name: '通用简报',
    description: '适合大多数消息窗口，包含总结、摘要、分类和待关注事项。',
    template: defaultOutputTemplate(),
  },
  {
    id: 'news',
    name: '新闻简报',
    description: '适合资讯源、新闻频道和 RSS 摘要。',
    template:
      '# 今日新闻总结\n\n## 今日要点\n用 3-7 条概括最重要的新闻，每条都标注来源编号。\n\n## 分主题整理\n按主题分组，例如国际、国内、科技、财经、行业动态；没有对应内容的主题不要硬凑。\n每个主题包含：\n- 重点事实\n- 简短背景\n- 来源编号\n\n## 进展与重复报道\n标出同一事件的进展关系、重复报道和仍不确定的信息。\n\n## 值得继续关注\n列出 3-5 项值得继续关注的事件、风险或后续进展。',
  },
  {
    id: 'group',
    name: '群聊摘要',
    description: '适合群聊和频道消息，突出讨论主题、待办和风险。',
    template:
      '# {{profile_name}}\n\n## 重点摘要\n- 输出 3-7 条重点摘要。\n- 每条摘要必须标注来源编号，例如 [#1]。\n\n## 主题归类\n按讨论主题分组，每组包含：\n- 主题名称\n- 关键内容\n- 相关来源编号\n\n## 待办与风险\n- 单独列出明确的待办、风险、问题和需要跟进的人或事项。\n- 如果没有明确待办或风险，写“暂无明确待办或风险”。\n\n## 低价值内容\n用一句话说明被忽略的寒暄、表情、重复转发或无上下文短句。',
  },
  {
    id: 'tasks',
    name: '待办提取',
    description: '适合从聊天记录中提取事项、负责人和风险。',
    template:
      '# 待办提取\n\n## 结论摘要\n用 1 段话说明本窗口内最重要的事项。\n\n## 明确待办\n每项包含：\n- 事项\n- 负责人或相关人\n- 截止时间或触发条件\n- 来源编号\n\n## 待确认问题\n列出信息不足、需要追问或依赖外部确认的问题。\n\n## 风险提示\n列出可能影响执行的风险或阻塞点。',
  },
]
const baseInputVolumeOptions = [
  { label: '少量消息（最多 30 条）', value: 30 },
  { label: '常规窗口（最多 50 条）', value: 50 },
  { label: '大型群聊（最多 80 条）', value: 80 },
  { label: '资讯频道（最多 120 条）', value: 120 },
]
const baseMessageLengthOptions = [
  { label: '短消息优先（每条 600 字）', value: 'short', maxChars: 600 },
  { label: '常规消息（每条 1200 字）', value: 'normal', maxChars: 1200 },
  { label: '保留长消息（每条 2500 字）', value: 'long', maxChars: 2500 },
  { label: '尽量完整（每条 5000 字）', value: 'full', maxChars: 5000 },
]
const inputVolumeOptions = computed(() => withCustomNumberOption(baseInputVolumeOptions, form.limits.max_messages_per_run, '当前自定义消息量'))
const messageLengthOptions = computed(() => {
  const selected = selectedMessageLength.value
  if (selected !== 'custom') return baseMessageLengthOptions
  return [
    {
      label: `当前自定义（每条 ${form.limits.max_chars_per_message ?? '-'} 字）`,
      value: 'custom',
      maxChars: form.limits.max_chars_per_message ?? 0,
    },
    ...baseMessageLengthOptions,
  ]
})
const selectedMessageLength = computed({
  get: () => {
    const matched = baseMessageLengthOptions.find(
      (item) => item.maxChars === form.limits.max_chars_per_message,
    )
    return matched?.value ?? 'custom'
  },
  set: (value: string) => {
    const matched = baseMessageLengthOptions.find((item) => item.value === value)
    if (!matched) return
    form.limits.max_chars_per_message = matched.maxChars
  },
})
const promptCharCount = computed(() => [...form.prompt_template].length)
const hasMessagesVariable = computed(() => form.prompt_template.includes(requiredMessagesVariable))
const selectedOutputTemplatePreset = computed({
  get: () => {
    const matched = outputTemplatePresets.find((item) => item.template === form.output_template)
    return matched?.id ?? 'custom'
  },
  set: (id: string) => {
    const preset = outputTemplatePresets.find((item) => item.id === id)
    if (!preset) return
    form.output_template = preset.template
  },
})
const outputTemplateOptions = computed(() => {
  const options = outputTemplatePresets.map((item) => ({
    label: `${item.name} - ${item.description}`,
    value: item.id,
  }))
  if (selectedOutputTemplatePreset.value === 'custom') {
    return [{ label: '自定义结构', value: 'custom' }, ...options]
  }
  return options
})
const promptSelection = reactive({ start: -1, end: -1 })

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
      '请整理以下窗口内的消息，严格按照输出结构模板组织内容。\n\n输出结构模板：\n{{output_template}}\n\n{{messages}}\n\n输出格式：{{output_format}}'
    form.output_format = 'markdown'
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
    conditions: props.profile.conditions.map((item) => ({ type: item.type, config: { ...(item.config ?? {}) } })),
    schedule: { ...props.profile.schedule },
    window: { ...props.profile.window },
    dedupe: { ...props.profile.dedupe },
    prompt_template: props.profile.prompt_template,
    output_format: props.profile.output_format,
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

function withCustomNumberOption<T extends { label: string; value: number }>(options: T[], value: number | undefined, label: string): T[] {
  if (!value || options.some((item) => item.value === value)) return options
  return [{ label: `${label}（${value}）`, value } as T, ...options]
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
    conditions: form.conditions.map((item) => ({ type: item.type, config: { ...(item.config ?? {}) } })),
    schedule: { ...form.schedule },
    window: { ...form.window },
    dedupe: { ...form.dedupe },
    prompt_template: form.prompt_template,
    output_format: form.output_format,
    output_template: form.output_template,
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
  if (!form.output_template.trim()) {
    message.warning('请填写输出结构模板')
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
        <section class="form-section">
          <div class="section-title">基础信息</div>
          <div class="base-grid">
            <NFormItem label="名称" required>
              <NInput v-model:value="form.name" placeholder="例如：技术资讯小时报" />
            </NFormItem>
            <NFormItem label="启用定时">
              <NSwitch v-model:value="form.enabled" />
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
          <div class="section-head">
            <div>
              <div class="section-title">Prompt 与 AI 输出</div>
              <div class="section-desc">先选整理风格，再用变量把窗口消息、来源和时间放进 Prompt。</div>
            </div>
          </div>
          <NFormItem label="常用预设">
            <NSelect :options="presetOptions" clearable placeholder="选择后会套用 Prompt、窗口、调度和限制" @update:value="applyPreset" />
          </NFormItem>
          <div class="prompt-layout">
            <NFormItem label="Prompt" class="prompt-editor">
              <NInput
                v-model:value="form.prompt_template"
                type="textarea"
                :autosize="{ minRows: 10, maxRows: 18 }"
                @focus="rememberPromptSelection"
                @click="rememberPromptSelection"
                @keyup="rememberPromptSelection"
                @select="rememberPromptSelection"
              />
            </NFormItem>
            <div class="variable-panel">
              <div class="variable-head">
                <div>
                  <div class="variable-title">可用变量</div>
                  <div class="variable-desc">点击变量会插入到当前光标处。</div>
                </div>
                <NTag :type="hasMessagesVariable ? 'success' : 'warning'" size="small" :bordered="false">
                  {{ hasMessagesVariable ? '已包含消息' : '缺少消息变量' }}
                </NTag>
              </div>
              <div class="variable-list">
                <button
                  v-for="item in promptVariables"
                  :key="item.token"
                  class="variable-item"
                  type="button"
                  @click="insertPromptVariable(item.token)"
                >
                  <span class="variable-token">{{ item.token }}</span>
                  <span class="variable-label">{{ item.label }}</span>
                  <span class="variable-desc-text">{{ item.desc }}</span>
                </button>
              </div>
            </div>
          </div>
          <NAlert v-if="!hasMessagesVariable" type="warning" :show-icon="false" class="prompt-alert">
            当前 Prompt 没有 <code>{{ requiredMessagesVariable }}</code>，AI 可能拿不到具体消息内容。后端会兜底追加消息，但建议你显式放在希望的位置。
          </NAlert>
          <div class="model-grid">
            <NFormItem label="Provider">
              <NSelect v-model:value="form.model_config.provider_id" filterable clearable :options="providerOptions" />
            </NFormItem>
            <NFormItem label="模型（可空=Provider 默认）">
              <NInput v-model:value="form.model_config.model" placeholder="gpt-4o-mini" />
            </NFormItem>
            <NFormItem label="AI 输出格式">
              <NSelect v-model:value="form.output_format" :options="outputFormatOptions" />
            </NFormItem>
            <NFormItem label="结构模板">
              <NSelect v-model:value="selectedOutputTemplatePreset" :options="outputTemplateOptions" />
            </NFormItem>
            <NFormItem label="输入消息量">
              <NSelect v-model:value="form.limits.max_messages_per_run" :options="inputVolumeOptions" />
            </NFormItem>
            <NFormItem label="消息长度处理">
              <NSelect v-model:value="selectedMessageLength" :options="messageLengthOptions" />
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
          <NFormItem label="输出结构模板">
            <NInput
              v-model:value="form.output_template"
              type="textarea"
              placeholder="定义 AI 输出的标题、章节、列表、来源标注等结构规范"
              :autosize="{ minRows: 8, maxRows: 16 }"
            />
          </NFormItem>
          <NAlert type="default" :show-icon="false" class="template-note">
            这里定义“内容长什么样”，例如标题、摘要、分类、待办、风险和来源标注；上面的 AI 输出格式只决定用 Markdown、Text 还是 HTML 作为载体。
          </NAlert>
          <NCollapse class="advanced-collapse">
            <NCollapseItem title="高级限制（一般不用改）" name="limits">
              <div class="limit-grid">
                <NFormItem label="最大消息数">
                  <NInputNumber v-model:value="form.limits.max_messages_per_run" :min="1" class="full-input" />
                </NFormItem>
                <NFormItem label="单条消息字符上限">
                  <NInputNumber v-model:value="form.limits.max_chars_per_message" :min="100" class="full-input" />
                </NFormItem>
              </div>
              <div class="limit-note">
                当前 Prompt 模板约 {{ promptCharCount }} 字。系统不再硬切提示词；如需保护请求体大小，只会优先缩减消息变量内容。
              </div>
            </NCollapseItem>
          </NCollapse>
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

.model-grid,
.limit-grid {
  display: grid;
  grid-template-columns: repeat(3, minmax(170px, 1fr));
  gap: 12px;
  align-items: start;
}

.prompt-layout {
  display: grid;
  grid-template-columns: minmax(0, 1fr) minmax(280px, 34%);
  gap: 14px;
  align-items: stretch;
}

.prompt-editor {
  margin-bottom: 0;
}

.variable-panel {
  border: 1px solid var(--clay-border);
  border-radius: 8px;
  background: var(--clay-surface);
  padding: 12px;
}

.variable-head {
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  gap: 10px;
  margin-bottom: 10px;
}

.variable-title {
  color: var(--clay-text);
  font-size: 13px;
  font-weight: 800;
}

.variable-desc,
.variable-desc-text,
.limit-note {
  color: var(--clay-text-3);
  font-size: 12px;
}

.variable-list {
  display: grid;
  gap: 8px;
}

.variable-item {
  width: 100%;
  border: 1px solid var(--clay-border);
  border-radius: 8px;
  background: var(--clay-surface-2);
  padding: 9px 10px;
  text-align: left;
  cursor: pointer;
  transition:
    border-color 0.16s ease,
    background 0.16s ease,
    transform 0.16s ease;
}

.variable-item:hover {
  border-color: var(--clay-primary);
  background: var(--clay-surface);
  transform: translateY(-1px);
}

.variable-token {
  display: block;
  color: var(--clay-primary);
  font-family: ui-monospace, SFMono-Regular, Consolas, 'Liberation Mono', monospace;
  font-size: 12px;
  font-weight: 800;
}

.variable-label {
  display: block;
  margin-top: 3px;
  color: var(--clay-text);
  font-size: 12px;
  font-weight: 700;
}

.variable-desc-text {
  display: block;
  margin-top: 3px;
  line-height: 1.45;
}

.prompt-alert,
.advanced-collapse,
.template-note {
  margin-top: 12px;
}

.limit-note {
  margin-top: 2px;
  line-height: 1.5;
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
  .schedule-grid,
  .model-grid,
  .limit-grid,
  .prompt-layout {
    grid-template-columns: 1fr;
  }

  .section-head {
    flex-direction: column;
  }
}
</style>
