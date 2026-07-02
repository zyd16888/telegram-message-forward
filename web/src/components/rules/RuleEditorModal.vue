<script setup lang="ts">
import { computed, reactive, watch } from 'vue'
import { useMessage } from 'naive-ui'
import ConfigFormRenderer from '@/components/ConfigFormRenderer.vue'
import { rulesApi } from '@/api/client'
import type { ConditionConfig, ProcessorConfig, Rule, RuleItemDescriptor, RuleTarget, Sink, Source, Template } from '@/types'
import { errText } from '@/utils/error'

const show = defineModel<boolean>('show', { required: true })

const props = defineProps<{
  rule?: Rule | null
  sources: Source[]
  sinks: Sink[]
  templates: Template[]
  conditionDescriptors: RuleItemDescriptor[]
  processorDescriptors: RuleItemDescriptor[]
}>()

const emit = defineEmits<{
  saved: []
}>()

const message = useMessage()

const form = reactive({
  name: '',
  enabled: true,
  priority: 0,
  stop_on_match: false,
  conditions: [] as ConditionConfig[],
  processors: [] as ProcessorConfig[],
  source_ids: [] as number[],
  targets: [] as RuleTarget[],
})

const editing = computed(() => Boolean(props.rule))
const sourceOptions = computed(() => props.sources.map((source) => ({ label: `${source.name} (#${source.id})`, value: source.id })))
const sinkOptions = computed(() => props.sinks.map((sink) => ({ label: `${sink.name} (${sink.type})`, value: sink.id })))
const conditionOptions = computed(() => props.conditionDescriptors.map((item) => ({ label: item.label, value: item.type })))
const processorOptions = computed(() => props.processorDescriptors.map((item) => ({ label: item.label, value: item.type })))

watch(
  () => [show.value, props.rule] as const,
  () => {
    if (!show.value) return
    resetForm()
  },
  { immediate: true },
)

function resetForm() {
  if (props.rule) {
    form.name = props.rule.name
    form.enabled = props.rule.enabled
    form.priority = props.rule.priority
    form.stop_on_match = props.rule.stop_on_match
    form.conditions = props.rule.conditions.map((item) => ({ type: item.type, config: { ...(item.config ?? {}) } }))
    form.processors = props.rule.processors.map((item) => ({ type: item.type, config: { ...(item.config ?? {}) } }))
    form.source_ids = [...props.rule.source_ids]
    form.targets = props.rule.targets.map((item) => ({ ...item }))
    return
  }
  form.name = ''
  form.enabled = true
  form.priority = 0
  form.stop_on_match = false
  form.conditions = []
  form.processors = []
  form.source_ids = []
  form.targets = []
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
    return false
  }
  if (!form.targets.length) {
    message.warning('请至少添加一个目标渠道')
    return false
  }
  const seenSinks = new Set<number>()
  for (const target of form.targets) {
    if (!target.sink_id) {
      message.warning('规则目标中存在未选择渠道的项')
      return false
    }
    if (seenSinks.has(target.sink_id)) {
      message.warning('同一条规则不能重复选择同一个渠道')
      return false
    }
    seenSinks.add(target.sink_id)
    if (!target.template_id) continue
    const sink = props.sinks.find((item) => item.id === target.sink_id)
    const template = props.templates.find((item) => item.id === target.template_id)
    if (sink && template && !sinkSupportsFormat(sink, template.format)) {
      message.warning(`渠道「${sink.name}」不支持 ${template.format} 模板`)
      return false
    }
  }
  return true
}

async function submit() {
  if (!validate()) return
  const body = {
    name: form.name,
    enabled: form.enabled,
    priority: form.priority,
    stop_on_match: form.stop_on_match,
    conditions: form.conditions,
    processors: form.processors,
    source_ids: form.source_ids,
    targets: form.targets,
  }
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
</script>

<template>
  <NModal v-model:show="show" preset="card" :title="editing ? '编辑规则' : '新建规则'" class="rule-modal">
    <NForm label-placement="left" label-width="92">
      <NFormItem label="名称" required>
        <NInput v-model:value="form.name" />
      </NFormItem>
      <NSpace>
        <NFormItem label="优先级">
          <NInputNumber v-model:value="form.priority" />
        </NFormItem>
        <NFormItem label="启用">
          <NSwitch v-model:value="form.enabled" />
        </NFormItem>
        <NFormItem label="命中即停">
          <NSwitch v-model:value="form.stop_on_match" />
        </NFormItem>
      </NSpace>
      <NFormItem label="来源">
        <NSelect v-model:value="form.source_ids" multiple :options="sourceOptions" />
      </NFormItem>

      <NDivider>条件</NDivider>
      <div v-for="(condition, index) in form.conditions" :key="index" class="rule-block">
        <NSpace align="center" justify="space-between">
          <NSelect
            :value="condition.type"
            class="type-select"
            :options="conditionOptions"
            @update:value="(value: string) => updateConditionType(condition, value)"
          />
          <NButton size="small" type="error" @click="removeCondition(index)">移除</NButton>
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
      <NButton size="small" dashed @click="addCondition">添加条件</NButton>

      <NDivider>处理器</NDivider>
      <div v-for="(processor, index) in form.processors" :key="index" class="rule-block">
        <NSpace align="center" justify="space-between">
          <NSelect
            :value="processor.type"
            class="type-select"
            :options="processorOptions"
            @update:value="(value: string) => updateProcessorType(processor, value)"
          />
          <NButton size="small" type="error" @click="removeProcessor(index)">移除</NButton>
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
      <NButton size="small" dashed @click="addProcessor">添加处理器</NButton>

      <NDivider>目标渠道</NDivider>
      <div v-for="(target, index) in form.targets" :key="index" class="target-row">
        <NSpace align="center">
          <NSelect v-model:value="target.sink_id" class="target-select" placeholder="渠道" :options="sinkOptions" />
          <NSelect
            v-model:value="target.template_id"
            class="target-select"
            clearable
            placeholder="模板（可空=纯文本）"
            :options="templateOptionsFor(target.sink_id)"
          />
          <NButton size="small" type="error" @click="removeTarget(index)">移除</NButton>
        </NSpace>
      </div>
      <NButton size="small" dashed @click="addTarget">添加目标</NButton>
    </NForm>
    <template #footer>
      <NSpace justify="end">
        <NButton @click="show = false">取消</NButton>
        <NButton type="primary" @click="submit">保存</NButton>
      </NSpace>
    </template>
  </NModal>
</template>

<style scoped>
.rule-modal {
  width: min(820px, calc(100vw - 32px));
}

.rule-block {
  border: 1px solid var(--n-border-color);
  border-radius: 6px;
  margin-bottom: 10px;
  padding: 12px;
}

.type-select {
  width: 240px;
}

.target-row {
  margin-bottom: 8px;
}

.target-select {
  width: 240px;
}
</style>
