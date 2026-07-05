<script setup lang="ts">
import { reactive, ref, watch } from 'vue'
import { useMessage } from 'naive-ui'
import ConfigFormRenderer from '@/components/ConfigFormRenderer.vue'
import { filtersApi } from '@/api/client'
import type { ConditionConfig, Filter, RuleItemDescriptor } from '@/types'
import { errText } from '@/utils/error'

const show = defineModel<boolean>('show', { required: true })

const props = defineProps<{
  filter?: Filter | null
  conditionDescriptors: RuleItemDescriptor[]
}>()

const emit = defineEmits<{ saved: [] }>()

const message = useMessage()

const form = reactive({
  name: '',
  description: '',
  conditions: [] as ConditionConfig[],
})
const saving = ref(false)

watch(
  () => [show.value, props.filter] as const,
  () => {
    if (!show.value) return
    if (props.filter) {
      form.name = props.filter.name
      form.description = props.filter.description
      form.conditions = props.filter.conditions.map((item) => ({ type: item.type, config: { ...(item.config ?? {}) } }))
    } else {
      form.name = ''
      form.description = ''
      form.conditions = []
    }
  },
  { immediate: true },
)

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
  const desc = props.conditionDescriptors[0]
  if (!desc) return
  form.conditions.push({ type: desc.type, config: defaultsFor(desc) })
}

function removeCondition(index: number): void {
  form.conditions.splice(index, 1)
}

function updateConditionType(item: ConditionConfig, type: string): void {
  item.type = type
  item.config = defaultsFor(conditionDescriptor(type))
}

const conditionOptions = () => props.conditionDescriptors.map((item) => ({ label: item.label, value: item.type }))

async function submit(): Promise<void> {
  if (!form.name.trim()) {
    message.warning('请填写过滤器名称')
    return
  }
  if (!form.conditions.length) {
    message.warning('过滤器至少需要一个匹配条件')
    return
  }
  saving.value = true
  try {
    const body = { name: form.name.trim(), description: form.description.trim(), conditions: form.conditions }
    if (props.filter?.id) {
      await filtersApi.update(props.filter.id, body)
    } else {
      await filtersApi.create(body)
    }
    message.success('过滤器已保存')
    show.value = false
    emit('saved')
  } catch (e) {
    message.error('保存失败：' + errText(e))
  } finally {
    saving.value = false
  }
}
</script>

<template>
  <NModal
    v-model:show="show"
    preset="card"
    :title="filter ? '编辑过滤器' : '新建过滤器'"
    :style="{ width: 'min(720px, calc(100vw - 32px))' }"
  >
    <NForm label-placement="top" :show-feedback="false">
      <div class="meta-grid">
        <NFormItem label="名称" required>
          <NInput v-model:value="form.name" placeholder="例如：只要含关键词且排除广告" />
        </NFormItem>
      </div>
      <NFormItem label="用途说明">
        <NInput v-model:value="form.description" placeholder="一句话说明适用场景，便于在规则/AI 整理中挑选" />
      </NFormItem>

      <div class="section-head">
        <div>
          <div class="section-title">匹配条件</div>
          <div class="section-desc">多个条件为「与」关系（全部满足才通过）。此过滤器可被转发规则与 AI 整理共同引用。</div>
        </div>
        <NButton size="small" dashed @click="addCondition">添加条件</NButton>
      </div>
      <div v-for="(condition, index) in form.conditions" :key="index" class="cond-block">
        <NSpace align="center" justify="space-between">
          <NSelect
            :value="condition.type"
            class="type-select"
            :options="conditionOptions()"
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
      <NEmpty v-if="!form.conditions.length" size="small" description="未添加条件" />
    </NForm>
    <template #footer>
      <NSpace justify="end">
        <NButton @click="show = false">取消</NButton>
        <NButton type="primary" :loading="saving" @click="submit">保存</NButton>
      </NSpace>
    </template>
  </NModal>
</template>

<style scoped>
.meta-grid {
  display: grid;
  grid-template-columns: 1fr;
  gap: 12px;
}

.section-head {
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  gap: 12px;
  margin: 14px 0 10px;
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
  line-height: 1.5;
}

.cond-block {
  border: 1px solid var(--clay-border);
  border-radius: 8px;
  margin-bottom: 10px;
  padding: 12px;
  background: var(--clay-surface);
}

.type-select {
  width: min(260px, 100%);
}
</style>
