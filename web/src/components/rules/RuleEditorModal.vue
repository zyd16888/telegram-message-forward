<script setup lang="ts">
import { computed, reactive, shallowRef, watch } from 'vue'
import { useMessage } from 'naive-ui'
import ConfigFormRenderer from '@/components/ConfigFormRenderer.vue'
import { rulesApi } from '@/api/client'
import type {
  ConditionConfig,
  Filter,
  ProcessorConfig,
  Rule,
  RuleInitialDraft,
  RuleItemDescriptor,
  RulePreviewResult,
  RuleTarget,
  Sink,
  Source,
  Template,
} from '@/types'
import { errText } from '@/utils/error'

type StageKey = 'route' | 'sources' | 'match' | 'process' | 'targets' | 'preview'

const show = defineModel<boolean>('show', { required: true })

const props = defineProps<{
  rule?: Rule | null
  sources: Source[]
  sinks: Sink[]
  templates: Template[]
  filters: Filter[]
  conditionDescriptors: RuleItemDescriptor[]
  processorDescriptors: RuleItemDescriptor[]
  initialDraft?: RuleInitialDraft | null
}>()

const emit = defineEmits<{
  saved: []
}>()

const message = useMessage()

const activeStage = shallowRef<StageKey>('route')

const form = reactive({
  name: '',
  enabled: true,
  priority: 0,
  stop_on_match: false,
  filter_ids: [] as number[],
  conditions: [] as ConditionConfig[],
  processors: [] as ProcessorConfig[],
  source_ids: [] as number[],
  targets: [] as RuleTarget[],
})
const preview = reactive({
  loading: false,
  source_id: null as number | null,
  message_type: 'text',
  sender_peer_type: 'channel',
  sender_id: null as number | null,
  sender_name: '',
  text: '这是一条规则预演消息',
  result: null as RulePreviewResult | null,
  error: '',
})

const editing = computed(() => Boolean(props.rule))
const usingFilter = computed(() => form.filter_ids.length > 0)
const selectedFilters = computed(() =>
  form.filter_ids
    .map((id) => props.filters.find((f) => f.id === id))
    .filter((item): item is Filter => Boolean(item)),
)
const filterOptions = computed(() =>
  props.filters.map((f) => ({ label: f.description ? `${f.name} — ${f.description}` : f.name, value: f.id })),
)
const sourceOptions = computed(() => props.sources.map((source) => ({ label: `${source.name} (#${source.id})`, value: source.id })))
const sinkOptions = computed(() => props.sinks.map((sink) => ({ label: `${sink.name} (${sink.type})`, value: sink.id })))
const conditionOptions = computed(() => props.conditionDescriptors.map((item) => ({ label: item.label, value: item.type })))
const processorOptions = computed(() => props.processorDescriptors.map((item) => ({ label: item.label, value: item.type })))
const messageTypeOptions = [
  { label: '文本', value: 'text' },
  { label: '图片', value: 'photo' },
  { label: '图片文件', value: 'image' },
  { label: '文档', value: 'document' },
  { label: '视频', value: 'video' },
]
const senderTypeOptions = [
  { label: '用户', value: 'user' },
  { label: '普通群', value: 'chat' },
  { label: '频道/超级群', value: 'channel' },
]

// --- 管道节点摘要 ---

const sourceChips = computed(() =>
  form.source_ids.map((id) => props.sources.find((s) => s.id === id)?.name ?? `#${id}`),
)
const targetChips = computed(() =>
  form.targets.map((t) => props.sinks.find((s) => s.id === t.sink_id)?.name ?? `#${t.sink_id}`),
)

const stages = computed(() => [
  {
    key: 'route' as StageKey,
    title: '路由',
    summary: form.name.trim() || '未命名规则',
    warn: !form.name.trim(),
    chips: [form.enabled ? '启用' : '停用', `优先级 ${form.priority}`, form.stop_on_match ? '命中即停' : '继续匹配'],
  },
  {
    key: 'sources' as StageKey,
    title: '来源',
    summary: form.source_ids.length ? `${form.source_ids.length} 个监听源` : '未指定来源',
    warn: form.source_ids.length === 0,
    chips: sourceChips.value,
  },
  {
    key: 'match' as StageKey,
    title: '过滤',
    summary: usingFilter.value
      ? `${form.filter_ids.length} 个共享过滤器`
      : form.conditions.length
        ? `${form.conditions.length} 个专用条件`
        : '全部消息',
    warn: false,
    chips: usingFilter.value ? selectedFilters.value.map((f) => f.name) : [],
  },
  {
    key: 'process' as StageKey,
    title: '处理',
    summary: form.processors.length ? `${form.processors.length} 个处理器` : '原样转发',
    warn: false,
    chips: form.processors.map((p) => processorDescriptor(p.type)?.label ?? p.type),
  },
  {
    key: 'targets' as StageKey,
    title: '目标/模板',
    summary: form.targets.length ? `${form.targets.length} 个渠道` : '未配置目标',
    warn: form.targets.length === 0,
    chips: targetChips.value,
  },
  {
    key: 'preview' as StageKey,
    title: '预演',
    summary: preview.result ? (preview.result.matched ? '上次：命中' : '上次：未命中') : '试跑样例消息',
    warn: false,
    chips: [],
  },
])

watch(
  () => [show.value, props.rule, props.initialDraft] as const,
  () => {
    if (!show.value) return
    resetForm()
  },
  { immediate: true },
)

function resetForm() {
  activeStage.value = 'route'
  if (props.rule) {
    form.name = props.rule.name
    form.enabled = props.rule.enabled
    form.priority = props.rule.priority
    form.stop_on_match = props.rule.stop_on_match
    form.filter_ids = [...props.rule.filter_ids]
    form.conditions = props.rule.conditions.map((item) => ({ type: item.type, config: { ...(item.config ?? {}) } }))
    form.processors = props.rule.processors.map((item) => ({ type: item.type, config: { ...(item.config ?? {}) } }))
    form.source_ids = [...props.rule.source_ids]
    form.targets = props.rule.targets.map((item) => ({ ...item }))
    resetPreview()
    return
  }
  const draft = props.initialDraft
  form.name = draft?.name ?? ''
  form.enabled = true
  form.priority = 0
  form.stop_on_match = false
  form.filter_ids = []
  form.conditions = []
  form.processors = []
  form.source_ids = [...(draft?.source_ids ?? [])]
  form.targets = (draft?.targets ?? []).map((item) => ({ ...item }))
  // 沿来源建规则时直接跳到下一步，减少一次点击。
  if (form.source_ids.length && !form.targets.length) activeStage.value = 'match'
  resetPreview()
}

function resetPreview() {
  preview.loading = false
  preview.source_id = form.source_ids[0] ?? null
  preview.message_type = 'text'
  preview.sender_peer_type = 'channel'
  preview.sender_id = null
  preview.sender_name = ''
  preview.text = '这是一条规则预演消息'
  preview.result = null
  preview.error = ''
}

function defaultsFor(desc?: RuleItemDescriptor): Record<string, unknown> {
  const out: Record<string, unknown> = {}
  for (const field of desc?.fields ?? []) {
    if (field.default !== undefined) out[field.key] = field.default
  }
  return out
}

function conditionDescriptor(type: string): RuleItemDescriptor | undefined {
  return props.conditionDescriptors.find((item) => item.type === type)
}

function processorDescriptor(type: string): RuleItemDescriptor | undefined {
  return props.processorDescriptors.find((item) => item.type === type)
}

function addCondition() {
  const desc = props.conditionDescriptors[0]
  if (!desc) return
  form.conditions.push({ type: desc.type, config: defaultsFor(desc) })
}

function addProcessor() {
  const desc = props.processorDescriptors[0]
  if (!desc) return
  form.processors.push({ type: desc.type, config: defaultsFor(desc) })
}

function removeCondition(index: number) {
  form.conditions.splice(index, 1)
}

function removeProcessor(index: number) {
  form.processors.splice(index, 1)
}

function updateConditionType(item: ConditionConfig, type: string) {
  item.type = type
  item.config = defaultsFor(conditionDescriptor(type))
}

function updateProcessorType(item: ProcessorConfig, type: string) {
  item.type = type
  item.config = defaultsFor(processorDescriptor(type))
}

function addTarget() {
  const sink = props.sinks[0]
  if (!sink) return
  form.targets.push({ sink_id: sink.id, template_id: undefined })
}

function removeTarget(index: number) {
  form.targets.splice(index, 1)
}

function templateOptionsFor(sinkId: number) {
  const sink = props.sinks.find((item) => item.id === sinkId)
  return props.templates
    .filter((template) => !sink || sinkSupportsFormat(sink, template.format))
    .map((template) => ({ label: `${template.name} (${template.format})`, value: template.id }))
}

function sinkSupportsFormat(sink: Sink, format: string): boolean {
  if (format === 'text') return sink.capabilities.supports_text
  if (format === 'markdown') return sink.capabilities.supports_markdown
  if (format === 'html') return sink.capabilities.supports_html
  return false
}

function validate(): boolean {
  if (!form.name.trim()) {
    message.warning('请填写规则名称')
    activeStage.value = 'route'
    return false
  }
  if (!form.targets.length) {
    message.warning('请至少添加一个目标渠道')
    activeStage.value = 'targets'
    return false
  }
  const seenSinks = new Set<number>()
  for (const target of form.targets) {
    if (!target.sink_id) {
      message.warning('规则目标中存在未选择渠道的项')
      activeStage.value = 'targets'
      return false
    }
    if (seenSinks.has(target.sink_id)) {
      message.warning('同一条规则不能重复选择同一个渠道')
      activeStage.value = 'targets'
      return false
    }
    seenSinks.add(target.sink_id)
    if (!target.template_id) continue
    const sink = props.sinks.find((item) => item.id === target.sink_id)
    const template = props.templates.find((item) => item.id === target.template_id)
    if (sink && template && !sinkSupportsFormat(sink, template.format)) {
      message.warning(`渠道「${sink.name}」不支持 ${template.format} 模板`)
      activeStage.value = 'targets'
      return false
    }
  }
  return true
}

function ruleBody() {
  return {
    name: form.name || '预演规则',
    enabled: form.enabled,
    priority: form.priority,
    stop_on_match: form.stop_on_match,
    filter_ids: form.filter_ids,
    conditions: usingFilter.value ? [] : form.conditions,
    processors: form.processors,
    source_ids: form.source_ids,
    targets: form.targets,
  }
}

async function submit() {
  if (!validate()) return
  const body = ruleBody()
  try {
    if (props.rule) {
      await rulesApi.update(props.rule.id, body)
    } else {
      await rulesApi.create(body)
    }
    message.success('已保存规则')
    show.value = false
    emit('saved')
  } catch (e) {
    message.error('保存失败：' + errText(e))
  }
}

async function runPreview() {
  if (!form.targets.length) {
    message.warning('请先添加至少一个目标渠道')
    activeStage.value = 'targets'
    return
  }
  preview.loading = true
  preview.error = ''
  preview.result = null
  try {
    preview.result = await rulesApi.preview({
      rule: ruleBody(),
      message: {
        source_id: preview.source_id ?? form.source_ids[0] ?? 0,
        message_type: preview.message_type,
        sender_peer_type: preview.sender_peer_type,
        sender_id: preview.sender_id ?? 0,
        sender_name: preview.sender_name,
        text: preview.text,
        media:
          preview.message_type === 'photo' || preview.message_type === 'image'
            ? [{ type: preview.message_type, file_name: 'preview.jpg', caption: preview.text }]
            : [],
      },
    })
  } catch (e) {
    preview.error = errText(e)
  } finally {
    preview.loading = false
  }
}

function previewTargetLabel(target: RuleTarget): string {
  const sink = props.sinks.find((item) => item.id === target.sink_id)
  const template = target.template_id ? props.templates.find((item) => item.id === target.template_id) : null
  return `${sink?.name ?? `#${target.sink_id}`}${template ? ` / ${template.name}` : ' / 纯文本'}`
}
</script>

<template>
  <NModal
    v-model:show="show"
    preset="card"
    :title="editing ? '编辑规则' : '新建规则'"
    class="rule-modal"
    :style="{ width: 'min(920px, calc(100vw - 32px))' }"
  >
    <div class="modal-body">
      <!-- 管道节点条：点节点配置对应环节 -->
      <div class="pipeline">
        <template v-for="(stage, index) in stages" :key="stage.key">
          <button
            type="button"
            class="pipe-node"
            :class="{ active: activeStage === stage.key, warn: stage.warn, preview: stage.key === 'preview' }"
            @click="activeStage = stage.key"
          >
            <span class="pipe-title">
              {{ stage.title }}
              <span v-if="stage.warn" class="pipe-warn-dot" />
            </span>
            <span class="pipe-summary">{{ stage.summary }}</span>
            <span v-if="stage.chips.length" class="pipe-chips">
              <span v-for="chip in stage.chips.slice(0, 2)" :key="chip" class="pipe-chip">{{ chip }}</span>
              <span v-if="stage.chips.length > 2" class="pipe-chip more">+{{ stage.chips.length - 2 }}</span>
            </span>
          </button>
          <span v-if="index < stages.length - 1" class="pipe-arrow" :class="{ dashed: stages[index + 1].key === 'preview' }">→</span>
        </template>
      </div>

      <!-- 当前环节配置面板 -->
      <div class="stage-panel">
        <NForm label-placement="top">
          <template v-if="activeStage === 'route'">
            <div class="panel-desc">路由模块决定这条规则在整体编排中的身份、优先级和命中后的继续匹配策略。</div>
            <div class="route-grid">
              <NFormItem label="规则名称" :show-feedback="false">
                <NInput v-model:value="form.name" placeholder="规则名称（必填）" />
              </NFormItem>
              <NFormItem label="优先级" :show-feedback="false">
                <NInputNumber v-model:value="form.priority" class="full-input" />
              </NFormItem>
              <div class="route-switch">
                <span class="switch-copy">
                  <strong>启用规则</strong>
                  <small>{{ form.enabled ? '消息会进入匹配' : '暂不参与匹配' }}</small>
                </span>
                <NSwitch v-model:value="form.enabled" />
              </div>
              <div class="route-switch">
                <span class="switch-copy">
                  <strong>命中即停</strong>
                  <small>{{ form.stop_on_match ? '命中后停止低优先级规则' : '命中后继续匹配后续规则' }}</small>
                </span>
                <NSwitch v-model:value="form.stop_on_match" />
              </div>
            </div>
          </template>

          <template v-else-if="activeStage === 'sources'">
            <div class="panel-desc">选择哪些监听源的消息进入这条规则；也可以稍后在编排画布上直接连线。</div>
            <NFormItem label="监听来源" :show-feedback="false">
              <NSelect v-model:value="form.source_ids" multiple filterable :options="sourceOptions" placeholder="选择一个或多个来源" />
            </NFormItem>
          </template>

          <template v-else-if="activeStage === 'match'">
            <div class="panel-desc">过滤模块为空时表示来源消息全部进入。共享过滤器可被多条规则复用；专用条件只跟随当前规则。</div>
            <NFormItem label="共享过滤器" :show-feedback="false">
              <NSelect
                v-model:value="form.filter_ids"
                multiple
                clearable
                :options="filterOptions"
                placeholder="不选则使用本规则专用条件"
              />
            </NFormItem>

            <template v-if="usingFilter">
              <NAlert type="info" :show-icon="false" class="filter-note">
                修改共享过滤器会联动所有引用它的规则与 AI 整理；如需单独调整，请清空选择后使用本规则专用条件。
              </NAlert>
              <div class="filter-list">
                <div v-for="filter in selectedFilters" :key="filter.id" class="filter-item">
                  <div class="filter-name">{{ filter.name }}</div>
                  <div v-if="filter.conditions.length" class="filter-cond-list">
                    <NTag
                      v-for="(c, i) in filter.conditions"
                      :key="`${filter.id}-${i}`"
                      size="small"
                      :bordered="false"
                      type="info"
                    >
                      {{ conditionDescriptor(c.type)?.label ?? c.type }}
                    </NTag>
                  </div>
                  <NText v-else depth="3">未配置条件</NText>
                </div>
              </div>
            </template>

            <template v-else>
              <div v-for="(condition, index) in form.conditions" :key="index" class="rule-block">
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
                <NText v-if="conditionDescriptor(condition.type)?.description" depth="3">
                  {{ conditionDescriptor(condition.type)?.description }}
                </NText>
              </div>
              <NButton size="small" dashed block @click="addCondition">添加条件</NButton>
            </template>
          </template>

          <template v-else-if="activeStage === 'process'">
            <div class="panel-desc">处理模块在投递前按顺序改写消息快照；不添加处理器时，消息按原文进入目标模块。</div>
            <div v-for="(processor, index) in form.processors" :key="index" class="rule-block">
              <NSpace align="center" justify="space-between">
                <NSelect
                  :value="processor.type"
                  class="type-select"
                  :options="processorOptions"
                  @update:value="(value: string) => updateProcessorType(processor, value)"
                />
                <NButton size="small" type="error" secondary @click="removeProcessor(index)">移除</NButton>
              </NSpace>
              <ConfigFormRenderer
                v-if="processorDescriptor(processor.type)"
                v-model="processor.config"
                :fields="processorDescriptor(processor.type)?.fields ?? []"
              />
              <NText v-if="processorDescriptor(processor.type)?.description" depth="3">
                {{ processorDescriptor(processor.type)?.description }}
              </NText>
            </div>
            <NButton size="small" dashed block @click="addProcessor">添加处理器</NButton>
          </template>

          <template v-else-if="activeStage === 'targets'">
            <div class="panel-desc">目标模块由渠道和渲染模板组成。模板绑定在当前目标上，以匹配不同渠道的格式能力；不选模板则按纯文本投递。</div>
            <div v-for="(target, index) in form.targets" :key="index" class="target-row">
              <span class="target-index">目标 {{ index + 1 }}</span>
              <div class="target-fields">
                <NSelect v-model:value="target.sink_id" class="target-select" placeholder="渠道" :options="sinkOptions" />
                <NSelect
                  v-model:value="target.template_id"
                  class="target-select"
                  clearable
                  placeholder="模板（可空=纯文本）"
                  :options="templateOptionsFor(target.sink_id)"
                />
              </div>
              <NButton size="small" type="error" secondary @click="removeTarget(index)">移除</NButton>
            </div>
            <NButton size="small" dashed block @click="addTarget">添加目标</NButton>
          </template>

          <template v-else>
            <div class="panel-desc">用一条样例消息试跑整条管道，检查条件、处理器与目标渠道，无需先保存。</div>
            <div class="preview-grid">
              <NFormItem label="Source ID">
                <NInputNumber v-model:value="preview.source_id" class="full-input" clearable />
              </NFormItem>
              <NFormItem label="消息类型">
                <NSelect v-model:value="preview.message_type" :options="messageTypeOptions" />
              </NFormItem>
              <NFormItem label="发送者类型">
                <NSelect v-model:value="preview.sender_peer_type" :options="senderTypeOptions" />
              </NFormItem>
              <NFormItem label="发送者 ID">
                <NInputNumber v-model:value="preview.sender_id" class="full-input" clearable />
              </NFormItem>
            </div>
            <NFormItem label="发送者名称">
              <NInput v-model:value="preview.sender_name" />
            </NFormItem>
            <NFormItem label="样例文本">
              <NInput v-model:value="preview.text" type="textarea" :autosize="{ minRows: 3, maxRows: 6 }" />
            </NFormItem>
            <NButton type="primary" secondary :loading="preview.loading" @click="runPreview">运行预演</NButton>
            <NAlert v-if="preview.error" type="error" title="预演失败" class="preview-result">
              {{ preview.error }}
            </NAlert>
            <NAlert
              v-else-if="preview.result"
              :type="preview.result.matched ? 'success' : 'warning'"
              :title="preview.result.matched ? '规则命中' : '规则未命中'"
              class="preview-result"
            >
              <NSpace vertical size="small">
                <NText>{{ preview.result.processed_text || '无文本内容' }}</NText>
                <NText v-if="preview.result.targets.length" depth="3">
                  目标：{{ preview.result.targets.map(previewTargetLabel).join('，') }}
                </NText>
              </NSpace>
            </NAlert>
          </template>
        </NForm>
      </div>
    </div>
    <template #footer>
      <NSpace justify="end">
        <NButton @click="show = false">取消</NButton>
        <NButton type="primary" @click="submit">保存</NButton>
      </NSpace>
    </template>
  </NModal>
</template>

<style scoped>
.modal-body {
  display: flex;
  flex-direction: column;
  gap: 12px;
}

/* 管道节点条 */

.pipeline {
  display: flex;
  align-items: stretch;
  gap: 6px;
  overflow-x: auto;
  padding: 2px;
}

.pipe-node {
  flex: 1;
  min-width: 118px;
  padding: 9px 10px;
  border: 1px solid var(--clay-border);
  border-radius: 11px;
  background: var(--clay-surface);
  box-shadow: var(--clay-out-sm);
  color: inherit;
  text-align: left;
  cursor: pointer;
  transition: border-color 0.16s ease, box-shadow 0.16s ease, transform 0.16s ease, background-color 0.16s ease;
}

.pipe-node:hover,
.pipe-node:focus-visible {
  border-color: var(--clay-border-strong);
  box-shadow: var(--clay-hover);
  transform: translateY(-1px);
  outline: none;
}

.pipe-node.active {
  border-color: var(--clay-primary);
  background: color-mix(in srgb, var(--clay-primary-soft) 55%, var(--clay-surface));
  box-shadow: 0 0 0 2px color-mix(in srgb, var(--clay-primary) 18%, transparent);
  transform: none;
}

.pipe-node.preview {
  border-style: dashed;
}

.pipe-title {
  display: flex;
  align-items: center;
  gap: 6px;
  color: var(--clay-text);
  font-size: 13px;
  font-weight: 800;
}

.pipe-warn-dot {
  width: 7px;
  height: 7px;
  border-radius: 999px;
  background: #d97706;
}

.pipe-summary {
  display: block;
  margin-top: 3px;
  color: var(--clay-text-3);
  font-size: 11px;
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
}

.pipe-node.warn .pipe-summary {
  color: #b45309;
  font-weight: 600;
}

.pipe-chips {
  display: flex;
  gap: 4px;
  margin-top: 5px;
  min-width: 0;
}

.pipe-chip {
  max-width: 90px;
  padding: 1px 7px;
  border-radius: 999px;
  border: 1px solid var(--clay-border);
  background: var(--clay-surface-2);
  color: var(--clay-text-2);
  font-size: 10px;
  font-weight: 600;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.pipe-chip.more {
  flex-shrink: 0;
}

.pipe-arrow {
  align-self: center;
  flex-shrink: 0;
  color: var(--clay-text-3);
  font-size: 14px;
  font-weight: 700;
}

.pipe-arrow.dashed {
  opacity: 0.55;
}

/* 环节面板 */

.stage-panel {
  min-height: 260px;
  max-height: min(52vh, 520px);
  overflow: auto;
  padding: 14px;
  border: 1px solid var(--clay-border);
  border-radius: 10px;
  background: var(--clay-surface-2);
}

.panel-desc {
  margin-bottom: 12px;
  color: var(--clay-text-3);
  font-size: 12px;
  line-height: 1.5;
}

.route-grid {
  display: grid;
  grid-template-columns: minmax(240px, 1.2fr) minmax(120px, 0.5fr);
  gap: 12px;
}

.route-switch {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
  min-height: 58px;
  padding: 10px 12px;
  border: 1px solid var(--clay-border);
  border-radius: 10px;
  background: var(--clay-surface);
}

.switch-copy {
  min-width: 0;
}

.switch-copy strong,
.switch-copy small {
  display: block;
}

.switch-copy strong {
  color: var(--clay-text);
  font-size: 13px;
}

.switch-copy small {
  margin-top: 3px;
  color: var(--clay-text-3);
  font-size: 11px;
  line-height: 1.35;
}

.rule-block {
  border: 1px solid var(--clay-border);
  border-radius: 8px;
  margin-bottom: 10px;
  padding: 12px;
  background: var(--clay-surface);
}

.filter-note {
  margin-bottom: 10px;
}

.filter-list {
  display: grid;
  gap: 8px;
}

.filter-item {
  border: 1px solid var(--clay-border);
  border-radius: 8px;
  padding: 10px;
  background: var(--clay-surface);
}

.filter-name {
  margin-bottom: 6px;
  color: var(--clay-text);
  font-size: 13px;
  font-weight: 700;
}

.filter-cond-list {
  display: flex;
  flex-wrap: wrap;
  gap: 6px;
}

.type-select {
  width: min(260px, 100%);
}

.target-row {
  display: grid;
  grid-template-columns: 72px minmax(0, 1fr) auto;
  gap: 10px;
  align-items: center;
  margin-bottom: 10px;
  padding: 10px;
  border: 1px solid var(--clay-border);
  border-radius: 10px;
  background: var(--clay-surface);
}

.target-index {
  color: var(--clay-text-3);
  font-size: 12px;
  font-weight: 800;
}

.target-fields {
  display: grid;
  grid-template-columns: repeat(2, minmax(160px, 1fr));
  gap: 10px;
  min-width: 0;
}

.target-select {
  width: 100%;
}

.full-input {
  width: 100%;
}

.preview-grid {
  display: grid;
  grid-template-columns: repeat(4, minmax(120px, 1fr));
  gap: 12px;
  align-items: start;
}

.preview-result {
  margin-top: 12px;
}

@media (max-width: 760px) {
  .pipeline {
    flex-wrap: wrap;
  }

  .pipe-arrow {
    display: none;
  }

  .pipe-node {
    flex: 1 1 45%;
  }

  .target-row,
  .preview-grid,
  .route-grid {
    grid-template-columns: 1fr;
  }

  .target-fields {
    grid-template-columns: 1fr;
  }
}
</style>
